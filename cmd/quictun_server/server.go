package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/julienschmidt/quictun"
	"github.com/julienschmidt/quictun/h2quic"
	"github.com/julienschmidt/quictun/internal/testdata"
)

const listenAddr = "localhost:6121"

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13_2) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/60.0.3112.113 Safari/537.36"

const dialTimeout = 30

func main() {
	http.HandleFunc("/demo/tile", func(w http.ResponseWriter, r *http.Request) {
		// Small 40x40 png
		w.Write([]byte{
			0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
			0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x28, 0x00, 0x00, 0x00, 0x28,
			0x01, 0x03, 0x00, 0x00, 0x00, 0xb6, 0x30, 0x2a, 0x2e, 0x00, 0x00, 0x00,
			0x03, 0x50, 0x4c, 0x54, 0x45, 0x5a, 0xc3, 0x5a, 0xad, 0x38, 0xaa, 0xdb,
			0x00, 0x00, 0x00, 0x0b, 0x49, 0x44, 0x41, 0x54, 0x78, 0x01, 0x63, 0x18,
			0x61, 0x00, 0x00, 0x00, 0xf0, 0x00, 0x01, 0xe2, 0xb8, 0x75, 0x22, 0x00,
			0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
		})
	})

	http.HandleFunc("/demo/tiles", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "<html><head><style>img{width:40px;height:40px;}</style></head><body>")
		for i := 0; i < 200; i++ {
			fmt.Fprintf(w, `<img src="/demo/tile?cachebust=%d">`, i)
		}
		io.WriteString(w, "</body></html>")
	})

	http.HandleFunc("/demo/echo", func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			fmt.Printf("error reading body while handling /echo: %s\n", err.Error())
		}
		w.Write(body)
	})

	http.HandleFunc("/secret", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Connection", "Upgrade")
		w.Header().Set("Upgrade", "QTP/0.1")
		w.WriteHeader(http.StatusSwitchingProtocols)
	})

	quictunServer := quictun.Server{
		DialTimeout: dialTimeout * time.Second,
	}
	h2quic.RegisterUpgradeHandler("QTP/0.1", quictunServer.Upgrade)

	certFile, keyFile := testdata.GetCertificatePaths()

	server := h2quic.Server{
		Server: &http.Server{Addr: listenAddr},
	}
	fmt.Printf("Start listening on %s...\n", listenAddr)
	err := server.ListenAndServeTLS(certFile, keyFile)
	if err != nil {
		fmt.Println(err)
	}
}
