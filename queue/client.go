// Package queue wires the e-ubl21 services to NATS JetStream: it defines
// the per-document-type input streams UBL documents are queued on, the
// single output stream validation results are published to, and a
// JetStream-backed KV bucket used as the durable record of every upload's
// status - replacing local filesystem persistence entirely, so both the
// HTTP API (producer) and the validation worker (consumer) can run as
// independent processes/replicas with no shared disk.
package queue

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Client bundles a NATS connection with its JetStream context, and is the
// single dependency both the HTTP API and the validation worker need to
// interact with the queue.
type Client struct {
	NC *nats.Conn
	JS jetstream.JetStream
}

// Connect dials the NATS server at url and opens a JetStream context on
// top of it. The connection is configured to reconnect indefinitely (with
// backoff) rather than giving up, since e-ubl21's services are expected to
// tolerate NATS restarts.
func Connect(url string) (*Client, error) {
	nc, err := nats.Connect(url,
		nats.Name("e-ubl21"),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.RetryOnFailedConnect(true),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to NATS at %s: %w", url, err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("create JetStream context: %w", err)
	}
	return &Client{NC: nc, JS: js}, nil
}

// Close closes the underlying NATS connection.
func (c *Client) Close() {
	if c.NC != nil {
		c.NC.Close()
	}
}
