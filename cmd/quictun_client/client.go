package main

import (
	"log"

	"github.com/julienschmidt/quictun"
)

const listenAddr = "localhost:1080"
const tunnelAddr = "https://very:sicher@quic.clemente.io:6121/secret"

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13_2) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/60.0.3112.113 Safari/537.36"

const dialTimeout = 30

func main() {
	client := quictun.Client{
		ListenAddr:  listenAddr,
		TunnelAddr:  tunnelAddr,
		UserAgent:   userAgent,
		DialTimeout: dialTimeout,
	}
	log.Fatal(client.Run())
}
