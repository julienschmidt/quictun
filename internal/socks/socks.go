package socks

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
)

// See https://www.ietf.org/rfc/rfc1928.txt

const socksVersion = 5

// Commands
const (
	CmdConnect   = 1
	CmdBind      = 2
	CmdAssociate = 3
)

// Address types
const (
	AtypIPv4   = 1
	AtypDomain = 3
	AtypIPv6   = 4
)

// Auth Methods
const (
	AuthNoAuthenticationRequired = 0x00
	AuthNoAcceptableMethod       = 0xFF
)

// Status
const (
	StatusSucceeded            = 0
	StatusGeneralFailure       = 1
	StatusConnectionNotAllowed = 2
	StatusNetworkUnreachable   = 3
	StatusHostUnreachable      = 4
	StatusConnectionRefused    = 5
	StatusTtlExpired           = 6
	StatusCmdNotSupported      = 7
	StatusAddrNotSupported     = 8
)

// Errors
var (
	ErrNoAuth           = errors.New("could not authenticate SOCKS connection")
	ErrAtypNotSupported = errors.New("address type is not supported")
)

func Auth(rd *bufio.Reader, w io.Writer) error {
	// 1 version
	// 1 nmethods
	// 1 method[nmethods] (we only read 1 at a time)
	var header [3]byte
	if _, err := io.ReadFull(rd, header[:]); err != nil {
		return err
	}

	// check SOCKS version
	if clVersion := header[0]; clVersion != socksVersion {
		return errors.New("incompatible SOCKS version: " +
			strconv.FormatUint(uint64(clVersion), 10))
	}

	// check auth
	// currently only NoAuthenticationRequired is supported
	acceptableAuth := false
	if nMethods := header[1]; nMethods > 0 {
		if method := header[2]; method == AuthNoAuthenticationRequired {
			acceptableAuth = true
		}
		for n := uint8(1); n < nMethods; n++ {
			// if we already have an acceptable auth method, we can skip all
			if acceptableAuth {
				if _, err := rd.Discard(int(nMethods - n)); err != nil {
					return err
				}
				break
			}

			// keep checking until we find an acceptable auth method
			method, err := rd.ReadByte()
			if err != nil {
				return err
			}
			if method == AuthNoAuthenticationRequired {
				acceptableAuth = true
			}
		}
	}

	// send auth method selection to client
	if !acceptableAuth {
		w.Write([]byte{socksVersion, AuthNoAcceptableMethod})
		return ErrNoAuth
	}
	_, err := w.Write([]byte{socksVersion, AuthNoAuthenticationRequired})
	return err
}

type Request []byte

// PeekRequest peeks
func PeekRequest(rd *bufio.Reader) (Request, error) {
	// 1 version
	// 1 command
	// 1 reserved
	// 1 atyp
	header, err := rd.Peek(4)
	if err != nil {
		return nil, err
	}

	// check SOCKS version
	if clVersion := header[0]; clVersion != socksVersion {
		return nil, errors.New("incompatible SOCKS version: " +
			strconv.FormatUint(uint64(clVersion), 10))
	}

	// read address (IPv4, IPv6 or Domain)
	const addrStart = 4
	atyp := header[3]
	switch atyp {
	case AtypIPv4:
		// read IPv4 address + port
		buf, err := rd.Peek(addrStart + net.IPv4len + 2)
		return Request(buf), err
	case AtypDomain:
		header, err = rd.Peek(addrStart + 1)
		if err != nil {
			return nil, err
		}
		domainLen := int(header[4])

		// read domain name + port
		buf, err := rd.Peek(addrStart + 1 + domainLen + 2)
		return Request(buf), err
	case AtypIPv6:
		// read IPv6 address + port
		buf, err := rd.Peek(addrStart + net.IPv6len + 2)
		return Request(buf), err
	default:
		return nil, ErrAtypNotSupported
	}
}

func (r Request) Cmd() byte {
	return r[1]
}

func (r Request) Dest() Addr {
	return Addr(r[3:])
}

// Addr is a pair of IPv4, IPv6 or Domain and a port
type Addr []byte

// Type returns the address type
func (a Addr) Type() byte {
	return a[0]
}

// Port returns the port of the address
func (a Addr) Port() int {
	var i = len(a) - 2
	return (int(a[i]) << 8) | int(a[i+1])
}

// String formats the address as a host:port string
func (a Addr) String() string {
	var host string
	switch a.Type() {
	case AtypIPv4, AtypIPv6:
		host = (net.IP(a[1 : len(a)-2])).String()
	case AtypDomain:
		host = string(a[2 : len(a)-2])
	default:
		return ""
	}
	return net.JoinHostPort(host, strconv.Itoa(a.Port()))
}

// TODO: allow to pass buffer or writer
func NewIPAddr(ip net.IP, port int) Addr {
	port1 := byte(port >> 8)
	port2 := byte(port & 0xff)
	if ip4 := ip.To4(); ip4 != nil {
		return Addr{AtypIPv4,
			ip4[0], ip4[1], ip4[2], ip4[3],
			port1, port2,
		}
	}
	if ip16 := ip.To16(); ip16 != nil {
		return Addr{AtypIPv6,
			ip16[0], ip16[1], ip16[2], ip16[3],
			ip16[4], ip16[5], ip16[6], ip16[7],
			ip16[8], ip16[9], ip16[10], ip16[11],
			ip16[12], ip16[13], ip16[14], ip16[15],
			port1, port2,
		}
	}
	return nil
}

func SendReply(wr io.Writer, status byte, addr Addr) error {
	// buffer to avoid allocations in the common cases
	var buf [64]byte
	reply := buf[:]
	if len(addr)+3 > cap(buf) {
		reply = make([]byte, len(addr)+3)
	}

	// 1 ver
	reply[0] = socksVersion

	// 1 rep
	reply[1] = status

	// 1 reserved

	if addr == nil {
		reply = reply[:4+net.IPv4len+2]

		// reply[3] = AtypDomain
		// reply[4] = 0

		// 1 address type
		reply[3] = AtypIPv4

		// 4 IPv4
		reply[4] = 0
		reply[5] = 0
		reply[6] = 0
		reply[7] = 0

		// 2 port
		reply[8] = 0
		reply[9] = 0
	} else {
		reply = reply[:3+len(addr)]
		copy(reply[3:], addr)
	}

	fmt.Println("reply", reply)

	// write reply
	_, err := wr.Write(reply)
	return err
}

func HandleRequest(req *Request) {

}
