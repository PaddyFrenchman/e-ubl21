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
	"sync"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/documentstore"
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

	handlesMu                    sync.RWMutex
	cachedUploadsKV              jetstream.KeyValue
	cachedValidationExecutionsKV jetstream.KeyValue
	cachedValidationEvidenceKV   jetstream.KeyValue
	cachedDeploymentGenerationKV jetstream.KeyValue
	cachedUploadContentStore     jetstream.ObjectStore
	documentStore                documentstore.Store
	claimStore                   *documentstore.ClaimStore
	uploadMetadataStore          UploadMetadataStore
}

func (c *Client) SetUploadMetadataStore(store UploadMetadataStore) {
	c.handlesMu.Lock()
	c.uploadMetadataStore = store
	c.handlesMu.Unlock()
}

func (c *Client) UploadMetadataStore() UploadMetadataStore {
	c.handlesMu.RLock()
	defer c.handlesMu.RUnlock()
	return c.uploadMetadataStore
}

func (c *Client) SetClaimStore(store *documentstore.ClaimStore) {
	c.handlesMu.Lock()
	c.claimStore = store
	c.handlesMu.Unlock()
}

func (c *Client) ClaimStore() *documentstore.ClaimStore {
	c.handlesMu.RLock()
	defer c.handlesMu.RUnlock()
	return c.claimStore
}

// SetDocumentStore selects the backend-neutral raw-document store for this
// client. It is set by the runtime configuration layer before upload or
// worker work begins; JetStream Object Store remains only for migration.
func (c *Client) SetDocumentStore(store documentstore.Store) {
	c.handlesMu.Lock()
	c.documentStore = store
	c.handlesMu.Unlock()
}

func (c *Client) DocumentStore() documentstore.Store {
	c.handlesMu.RLock()
	defer c.handlesMu.RUnlock()
	return c.documentStore
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
	// Namespace is an exported string type, so callers can bypass
	// NewNamespace with a direct conversion. Revalidate here, before dialing,
	// so malformed or legacy-colliding values can never become live clients.
	if err := validateNamespace(namespace.String()); err != nil {
		return nil, err
	}
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
