package queuetest

import (
	"testing"

	"github.com/nats-io/nats.go"
)

func TestStartServerProvidesInProcessConnection(t *testing.T) {
	server := StartServer(t)
	if server.URL == "" || len(server.Options) == 0 {
		t.Fatalf("incomplete test server: %+v", server)
	}
	nc, err := nats.Connect(server.URL, server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	nc.Close()
}
