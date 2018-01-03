package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/julienschmidt/quictun"
)

const (
	// the User-Agent string is not observable, but should have the same length as a regular browser UA, e.g. that of Chrome
	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13_3) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/63.0.3239.108 X-quictun/0.1"

	// timeout for establishing connections to quictun server (in seconds)
	dialTimeout = 30
)

func main() {
	// command-line flags and args
	listenFlag := flag.String("l", "localhost:1080", "local SOCKS listen address")
	flag.Usage = func() {
		fmt.Printf("Usage: %s [OPTIONS] QUICTUN_URL\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		flag.Usage()
		return
	}
	tunnelAddr := args[0]

	// configure and run quictun client
	client := quictun.Client{
		ListenAddr:  *listenFlag,
		TunnelAddr:  tunnelAddr,
		UserAgent:   userAgent,
		DialTimeout: dialTimeout * time.Second,
	}
	log.Fatal(client.Run())
}
