package quictun

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/julienschmidt/quictun/internal/socks"
	quic "github.com/lucas-clemente/quic-go"
)

// SequenceCache is a cache for client sequence numbers.
// Implementations should limit the number of cached key-value pairs using a
// strategy like least recently used (LRU).
type SequenceCache interface {
	Set(key uint64, value uint32) (old uint32)
	Get(key uint64) (value uint32)
}

// Server is a quictun server which handles QUIC sessions upgraded to the
// quictun protocol.
type Server struct {
	DialTimeout   time.Duration
	SequenceCache SequenceCache
}

// CheckSequenceNumber checks and caches the sequence number sent by a client
func (s *Server) CheckSequenceNumber(header string) bool {
	// parse clientID and sequenceNumber from header value
	if len(header) != 24 {
		return false
	}
	clientID, err := strconv.ParseUint(header[:16], 16, 64)
	if err != nil {
		return false
	}
	sequenceNumber, err := strconv.ParseUint(header[16:], 16, 32)
	if err != nil {
		return false
	}

	// the new sequence number must be larger than any previously seen number
	return s.SequenceCache.Set(clientID, uint32(sequenceNumber)) < uint32(sequenceNumber)
}

// Upgrade starts using a given QUIC session with the quictun protocol.
// The quictun server immediately starts accepting new QUIC streams and assumes
// them to speak the quictun protocol (QTP).
// The actual protocol upgrade (via a HTTP/2 request-response) is handled
// entirely by the web server.
func (s *Server) Upgrade(session quic.Session) {
	for {
		fmt.Println("Waiting for stream...")
		stream, err := session.AcceptStream()
		if err != nil {
			fmt.Println("accept stream:", err)
			session.Close(err)
			return
		}

		go s.handleQuictunStream(stream)
	}
}

func (s *Server) handleQuictunStream(stream quic.Stream) {
	streamID := stream.StreamID()
	fmt.Println("got stream", streamID)

	streamRd := bufio.NewReader(stream)
	req, err := socks.PeekRequest(streamRd)
	if err != nil {
		stream.Reset(err)
		stream.Close()
		fmt.Println("stream", streamID, ":", err)
		return
	}

	switch req.Cmd() {
	case socks.CmdConnect:
		remote, err := net.DialTimeout("tcp", req.Dest().String(), s.DialTimeout)
		if err != nil {
			fmt.Printf("stream %d: %#v\n", streamID, err)
			stream.Reset(nil)
			stream.Close()
			return
		}
		// remove request header from buffer
		if _, err = streamRd.Discard(len(req)); err != nil {
			stream.Reset(nil)
			stream.Close()
			remote.Close()
			fmt.Println("stream", streamID, ":", err)
			return
		}

		fmt.Println("Start proxying...")
		go proxy(stream, remote) // recv from remote and send to stream
		proxy(remote, streamRd)  // recv from stream and send to remote
	default:
		socks.SendReply(stream, socks.StatusCmdNotSupported, nil)
		stream.Reset(nil)
		stream.Close()
		return
	}
}
