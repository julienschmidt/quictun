package main

import (
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/julienschmidt/quictun"
	"github.com/julienschmidt/quictun/h2quic"
	"github.com/julienschmidt/quictun/internal/lru"
	"github.com/julienschmidt/quictun/internal/testdata"
)

const (
	dialTimeout = 30
)

func main() {
	// command-line args
	listenFlag := flag.String("l", "localhost:6121", "QUIC listen address")
	flag.Parse()
	args := flag.Args()
	if len(args) > 0 {
		flag.Usage()
		return
	}
	listenAddr := *listenFlag

	quictunServer := quictun.Server{
		DialTimeout:   dialTimeout * time.Second,
		SequenceCache: lru.New(10),
	}

	// Register the upgrade handler for the quictun protocol
	h2quic.RegisterUpgradeHandler("QTP/0.1", quictunServer.Upgrade)

	http.HandleFunc("/secret", func(w http.ResponseWriter, r *http.Request) {
		// replay protection
		if !quictunServer.CheckSequenceNumber(r.Header.Get("QTP")) {
			w.Header().Set("Connection", "close")
			w.WriteHeader(http.StatusBadRequest)
			r.Close = true
			return
		}

		// switch to quictun protocol (version 0.1)
		w.Header().Set("Connection", "Upgrade")
		w.Header().Set("Upgrade", "QTP/0.1")
		w.WriteHeader(http.StatusSwitchingProtocols)
	})

	// HTTP server
	// Implementations for production usage should be embedded in an existing web server instead.
	server := h2quic.Server{
		Server: &http.Server{Addr: listenAddr},
	}
	certFile, keyFile := testdata.GetCertificatePaths()
	fmt.Printf("Start listening on %s...\n", listenAddr)
	err := server.ListenAndServeTLS(certFile, keyFile)
	if err != nil {
		fmt.Println(err)
	}
}
