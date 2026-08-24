// Package queue wires the e-ubl21 services to NATS JetStream: it defines
// the per-document-type input streams UBL documents are queued on, the
// single output stream validation results are published to, and a
// JetStream-backed KV bucket used as the durable record of every upload's
// status - replacing local filesystem persistence entirely, so both the
// HTTP API (producer) and the validation worker (consumer) can run as
// independent processes/replicas with no shared disk.
//
// Every physical stream/subject/bucket name is derived from a Namespace
// (see Namespace and Client.Namespace), so multiple e-ubl21 deployments
// (e.g. dev and prod) can share one NATS/JetStream cluster without
// colliding on names.
package queue

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Client bundles a NATS connection with its JetStream context and the
// queue Namespace it operates in, and is the single dependency both the
// HTTP API and the validation worker need to interact with the queue.
type Client struct {
	NC *nats.Conn
	JS jetstream.JetStream
	// Namespace scopes every stream, subject and KV bucket this client's
	// methods (EnsureInfra, PublishDocument, DocumentsConsumer, ...)
	// derive names from. See Namespace for details.
	Namespace Namespace
}

// Connect dials the NATS server at url and opens a JetStream context on
// top of it, scoping the returned Client to namespace (use DefaultNamespace
// to preserve e-ubl21's original, pre-namespace stream/subject/bucket
// names). The connection is configured to reconnect indefinitely (with
// backoff) rather than giving up, since e-ubl21's services are expected to
// tolerate NATS restarts.
func Connect(url string, namespace Namespace) (*Client, error) {
	return ConnectWithOptions(url, namespace)
}

// ConnectWithOptions is Connect with additional low-level NATS client
// options. It is primarily useful for dependency injection in tests, such as
// connecting to an embedded server without opening a TCP listener.
func ConnectWithOptions(url string, namespace Namespace, options ...nats.Option) (*Client, error) {
	options = append([]nats.Option{
		nats.Name("e-ubl21"),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2 * time.Second),
		nats.RetryOnFailedConnect(true),
	}, options...)
	nc, err := nats.Connect(url,
		options...,
	)
	if err != nil {
		return nil, fmt.Errorf("connect to NATS at %s: %w", url, err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("create JetStream context: %w", err)
	}
	return &Client{NC: nc, JS: js, Namespace: namespace}, nil
}

// Close closes the underlying NATS connection.
func (c *Client) Close() {
	if c.NC != nil {
		c.NC.Close()
	}
}
