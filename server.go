package quictun

import (
	"bufio"
	"fmt"
	"net"
	"time"

	"github.com/julienschmidt/quictun/internal/socks"
	quic "github.com/lucas-clemente/quic-go"
)

type Server struct {
	DialTimeout time.Duration
}

func (s *Server) Upgrade(session quic.Session) {
	fmt.Println("Upgrade session to quictun...")
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
