// ha-tcp-proxy presents ordered TCP backends as one local YSQL endpoint.
// It is deliberately TLS-agnostic: YugabyteDB TLS stays end-to-end between
// the application client and each tserver.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:0", "listener address")
	backends := flag.String("backends", "", "comma-separated ordered backends")
	backendsFile := flag.String("backends-file", "", "file containing comma-separated ordered backends")
	flag.Parse()
	if *backends == "" && *backendsFile == "" {
		log.Fatal("-backends or -backends-file is required")
	}
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(ln.Addr().String())
	for {
		client, err := ln.Accept()
		if err != nil {
			log.Print(err)
			continue
		}
		current := *backends
		if *backendsFile != "" {
			contents, readErr := os.ReadFile(*backendsFile)
			if readErr != nil {
				_ = client.Close()
				log.Print(readErr)
				continue
			}
			current = strings.TrimSpace(string(contents))
		}
		go proxy(client, strings.Split(current, ","))
	}
}

func proxy(client net.Conn, backends []string) {
	defer client.Close()
	var upstream net.Conn
	var err error
	for _, backend := range backends {
		upstream, err = net.Dial("tcp", backend)
		if err == nil {
			break
		}
	}
	if err != nil {
		return
	}
	defer upstream.Close()
	done := make(chan struct{})
	go func() { _, _ = io.Copy(upstream, client); _ = upstream.Close(); close(done) }()
	_, _ = io.Copy(client, upstream)
	<-done
}
