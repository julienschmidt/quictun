package quictun

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/julienschmidt/quictun/internal/atomic"
	"github.com/julienschmidt/quictun/internal/socks"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"

	quic "github.com/lucas-clemente/quic-go"
)

const protocolIdentifier = "QTP/0.1"

var (
	ErrInvalidResponse   = errors.New("server returned an invalid response")
	ErrInvalidSequence   = errors.New("client sequence number invalid")
	ErrNotAQuictunServer = errors.New("server does not seems to be a quictun server")
	ErrWrongCredentials  = errors.New("authentication credentials seems to be wrong")
)

// Client holds the configuration and state of a quictun client
type Client struct {
	// config
	ListenAddr  string
	TunnelAddr  string
	UserAgent   string
	TlsCfg      *tls.Config
	QuicConfig  *quic.Config
	DialTimeout time.Duration

	// state
	session   quic.Session
	connected atomic.Bool

	// replay protection
	clientID       uint64
	sequenceNumber uint32

	// header
	headerStream quic.Stream
	hDecoder     *hpack.Decoder
	h2framer     *http2.Framer
}

func (c *Client) generateClientID() {
	// generate clientID
	rand.Seed(time.Now().UnixNano())
	c.clientID = rand.Uint64()
}

func (c *Client) connect() error {
	authURL := c.TunnelAddr

	// extract hostname from auth url
	uri, err := url.ParseRequestURI(authURL)
	if err != nil {
		log.Fatal("Invalid Auth URL: ", err)
		return err
	}
	hostname := authorityAddr(uri.Hostname(), uri.Port())
	fmt.Println("Connecting to", hostname)

	c.session, err = quic.DialAddr(hostname, c.TlsCfg, c.QuicConfig)
	if err != nil {
		log.Fatal("Dial Err: ", err)
		return err
	}

	// once the version has been negotiated, open the header stream
	c.headerStream, err = c.session.OpenStream()
	if err != nil {
		log.Fatal("OpenStream Err: ", err)
		return err
	}
	//fmt.Println("Header StreamID:", c.headerStream.StreamID())

	dataStream, err := c.session.OpenStreamSync()
	if err != nil {
		log.Fatal("OpenStreamSync Err: ", err)
	}
	//fmt.Println("Data StreamID:", dataStream.StreamID())

	// build HTTP request
	// The authorization credentials are automatically encoded from the URL
	req, err := http.NewRequest("GET", authURL, nil)
	if err != nil {
		log.Fatal("NewRequest Err: ", err)
		return err
	}
	req.Header.Set("User-Agent", c.UserAgent)

	// request protocol upgrade
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", protocolIdentifier)

	// replay protection
	c.sequenceNumber++
	req.Header.Set("QTP", fmt.Sprintf("%016X%08X", c.clientID, c.sequenceNumber))

	rw := newRequestWriter(c.headerStream)
	endStream := true //endStream := !hasBody
	fmt.Println("requesting", authURL)
	err = rw.WriteRequest(req, dataStream.StreamID(), endStream)
	if err != nil {
		log.Fatal("WriteHeaders Err: ", err)
	}

	fmt.Println("Waiting...")
	// read frames from headerStream
	c.h2framer = http2.NewFramer(nil, c.headerStream)
	c.hDecoder = hpack.NewDecoder(4096, func(hf hpack.HeaderField) {})

	frame, err := c.h2framer.ReadFrame()
	if err != nil {
		// c.headerErr = qerr.Error(qerr.HeadersStreamDataDecompressFailure, "cannot read frame")
		log.Fatal("cannot read frame: ", err)
	}
	hframe, ok := frame.(*http2.HeadersFrame)
	if !ok {
		// c.headerErr = qerr.Error(qerr.InvalidHeadersStreamData, "not a headers frame")
		log.Fatal("not a headers frame: ", err)
	}
	mhframe := &http2.MetaHeadersFrame{HeadersFrame: hframe}
	mhframe.Fields, err = c.hDecoder.DecodeFull(hframe.HeaderBlockFragment())
	if err != nil {
		// c.headerErr = qerr.Error(qerr.InvalidHeadersStreamData, "cannot read header fields")
		log.Fatal("cannot read header fields: ", err)
	}

	//fmt.Println("Frame for StreamID:", hframe.StreamID)

	rsp, err := responseFromHeaders(mhframe)
	if err != nil {
		log.Fatal("responseFromHeaders: ", err)
	}
	switch rsp.StatusCode {
	case http.StatusSwitchingProtocols:
		header := rsp.Header
		if header.Get("Connection") != "Upgrade" {
			return ErrInvalidResponse
		}
		if header.Get("Upgrade") != protocolIdentifier {
			return ErrNotAQuictunServer
		}
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrWrongCredentials
	case http.StatusBadRequest:
		c.generateClientID()
		return ErrInvalidSequence
	default:
		return ErrInvalidResponse
	}
}

func (c *Client) watchCancel() {
	session := c.session
	if session == nil {
		fmt.Println("session is nil")
		return
	}

	ctx := session.Context()
	if ctx == nil {
		fmt.Println("ctx is nil")
		return
	}

	// TODO: add graceful shutdown channel
	<-ctx.Done()
	fmt.Println("session closed", ctx.Err())
	c.connected.Set(false)
}

func (c *Client) tunnelConn(local net.Conn) {
	local.(*net.TCPConn).SetKeepAlive(true)
	// TODO: SetReadTimeout(conn)

	localRd := bufio.NewReader(local)

	// initiate SOCKS connection
	if err := socks.Auth(localRd, local); err != nil {
		fmt.Println(err)
		local.Close()
		return
	}

	req, err := socks.PeekRequest(localRd)
	if err != nil {
		fmt.Println(err)
		socks.SendReply(local, socks.StatusConnectionRefused, nil)
		local.Close()
		return
	}

	fmt.Println("request", req.Dest())

	switch req.Cmd() {
	case socks.CmdConnect:
		fmt.Println("[Connect]")
		if err = socks.SendReply(local, socks.StatusSucceeded, nil); err != nil {
			fmt.Println(err)
			local.Close()
			return
		}

	default:
		socks.SendReply(local, socks.StatusCmdNotSupported, nil)
		local.Close()
		return
	}

	// TODO: check connected status again and reconnect if necessary
	stream, err := c.session.OpenStreamSync()
	if err != nil {
		fmt.Println("open stream err", err)
		local.Close()
		return
	}

	fmt.Println("Start proxying...")
	go proxy(local, stream) // recv from stream and send to local
	proxy(stream, localRd)  // recv from local and send to stream
}

// Close closes the client
func (c *Client) close(err error) error {
	if c.session == nil {
		return nil
	}
	return c.session.Close(err)
}

// Run starts the client to accept incoming SOCKS connections, which are tunneled
// to the configured quictun server.
// The tunnel connection is opened only on-demand.
func (c *Client) Run() error {
	c.generateClientID()

	listener, err := net.Listen("tcp", c.ListenAddr)
	if err != nil {
		return fmt.Errorf("Failed to listen on %s: %s", c.ListenAddr, err)
	}

	fmt.Println("Listening for incoming SOCKS connection...")
	// accept local connections and tunnel them
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Accept Err:", err)
			continue
		}

		fmt.Println("new SOCKS conn", conn.RemoteAddr().String())

		if !c.connected.IsSet() {
			err = c.connect()
			if err != nil {
				fmt.Println("Failed to connect to tunnel host:", err)
				conn.Close()
				continue
			}
			// start watcher which closes when canceled
			go c.watchCancel()

			c.connected.Set(true)
		}

		go c.tunnelConn(conn)
	}
}
