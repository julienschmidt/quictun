package quictun

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
	"golang.org/x/net/idna"
	"golang.org/x/net/lex/httplex"

	quic "github.com/lucas-clemente/quic-go"
)

// http://www.ietf.org/rfc/rfc2617.txt
func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

// rest is mostly from http2.Transport

// authorityAddr returns a given authority (a host/IP, or host:port / ip:port)
// and returns a host:port. The port 443 is added if needed.
func authorityAddr(host, port string) (addr string) {
	if port == "" {
		port = "443"
	}
	if a, err := idna.ToASCII(host); err == nil {
		host = a
	}
	// IPv6 address literal, without a port:
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		return host + ":" + port
	}
	return net.JoinHostPort(host, port)
}

// shouldSendReqContentLength reports whether the http2.Transport should send
// a "content-length" request header. This logic is basically a copy of the net/http
// transferWriter.shouldSendContentLength.
// The contentLength is the corrected contentLength (so 0 means actually 0, not unknown).
// -1 means unknown.
func shouldSendReqContentLength(method string, contentLength int64) bool {
	if contentLength > 0 {
		return true
	}
	if contentLength < 0 {
		return false
	}
	// For zero bodies, whether we send a content-length depends on the method.
	// It also kinda doesn't matter for http2 either way, with END_STREAM.
	switch method {
	case "POST", "PUT", "PATCH":
		return true
	default:
		return false
	}
}

func validPseudoPath(v string) bool {
	return (len(v) > 0 && v[0] == '/' && (len(v) == 1 || v[1] != '/')) || v == "*"
}

// actualContentLength returns a sanitized version of
// req.ContentLength, where 0 actually means zero (not unknown) and -1
// means unknown.
func actualContentLength(req *http.Request) int64 {
	if req.Body == nil {
		return 0
	}
	if req.ContentLength != 0 {
		return req.ContentLength
	}
	return -1
}

type requestWriter struct {
	headerStream quic.Stream
	henc         *hpack.Encoder
	hbuf         bytes.Buffer // HPACK encoder writes into this
}

func newRequestWriter(headerStream quic.Stream) *requestWriter {
	rw := &requestWriter{
		headerStream: headerStream,
	}
	rw.henc = hpack.NewEncoder(&rw.hbuf)
	return rw
}

func (rw *requestWriter) WriteRequest(req *http.Request, dataStreamID quic.StreamID, endStream bool) error {
	if u := req.URL.User; u != nil && req.Header.Get("Authorization") == "" {
		username := u.Username()
		password, _ := u.Password()
		req.Header.Set("Authorization", "Basic "+basicAuth(username, password))
	}

	buf, err := rw.encodeHeaders(req, actualContentLength(req))
	if err != nil {
		log.Fatal("Failed to encode request headers: ", err)
		return err
	}
	h2framer := http2.NewFramer(rw.headerStream, nil)
	return h2framer.WriteHeaders(http2.HeadersFrameParam{
		StreamID:      uint32(dataStreamID),
		EndHeaders:    true,
		EndStream:     endStream,
		BlockFragment: buf,
		Priority:      http2.PriorityParam{Weight: 0xff},
	})
}

func (w *requestWriter) encodeHeaders(req *http.Request, contentLength int64) ([]byte, error) {
	w.hbuf.Reset()

	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	host, err := httplex.PunycodeHostPort(host)
	if err != nil {
		return nil, err
	}

	path := req.URL.RequestURI()
	if !validPseudoPath(path) {
		orig := path
		path = strings.TrimPrefix(path, req.URL.Scheme+"://"+host)
		if !validPseudoPath(path) {
			if req.URL.Opaque != "" {
				return nil, fmt.Errorf("invalid request :path %q from URL.Opaque = %q", orig, req.URL.Opaque)
			} else {
				return nil, fmt.Errorf("invalid request :path %q", orig)
			}
		}
	}

	// Check for any invalid headers and return an error before we
	// potentially pollute our hpack state. (We want to be able to
	// continue to reuse the hpack encoder for future requests)
	for k, vv := range req.Header {
		if !httplex.ValidHeaderFieldName(k) {
			return nil, fmt.Errorf("invalid HTTP header name %q", k)
		}
		for _, v := range vv {
			if !httplex.ValidHeaderFieldValue(v) {
				return nil, fmt.Errorf("invalid HTTP header value %q for header %q", v, k)
			}
		}
	}

	// 8.1.2.3 Request Pseudo-Header Fields
	// The :path pseudo-header field includes the path and query parts of the
	// target URI (the path-absolute production and optionally a '?' character
	// followed by the query production (see Sections 3.3 and 3.4 of
	// [RFC3986]).
	w.writeHeader(":authority", host)
	w.writeHeader(":method", req.Method)
	w.writeHeader(":path", path)
	w.writeHeader(":scheme", req.URL.Scheme)

	var didUA bool
	for k, vv := range req.Header {
		lowKey := strings.ToLower(k)
		switch lowKey {
		case "host", "content-length":
			// Host is :authority, already sent.
			// Content-Length is automatic, set below.
			continue
		case "connection", "proxy-connection", "transfer-encoding", "upgrade", "keep-alive":
			// Per 8.1.2.2 Connection-Specific Header
			// Fields, don't send connection-specific
			// fields. We have already checked if any
			// are error-worthy so just ignore the rest.
			continue
		case "user-agent":
			// Match Go's http1 behavior: at most one
			// User-Agent. If set to nil or empty string,
			// then omit it. Otherwise if not mentioned,
			// include the default (below).
			didUA = true
			if len(vv) < 1 {
				continue
			}
			vv = vv[:1]
			if vv[0] == "" {
				continue
			}
		}
		for _, v := range vv {
			w.writeHeader(lowKey, v)
		}
	}
	if shouldSendReqContentLength(req.Method, contentLength) {
		w.writeHeader("content-length", strconv.FormatInt(contentLength, 10))
	}
	if !didUA {
		panic("user agent info is missing")
	}
	return w.hbuf.Bytes(), nil
}

func (w *requestWriter) writeHeader(name, value string) {
	//fmt.Printf("http2: Transport encoding header %q = %q\n", name, value)
	w.henc.WriteField(hpack.HeaderField{Name: name, Value: value})
}
