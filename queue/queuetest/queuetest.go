// Package queuetest provides an embedded, JetStream-enabled NATS server for
// use in tests, so the queue package (and consumers of it, such as the
// server package's tests) can exercise real NATS/JetStream behavior without
// requiring Docker or an external NATS deployment.
package queuetest

import (
	"testing"

	natstest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
)

// Server describes an embedded NATS server and the option clients need to
// connect to it without opening a TCP socket.
type Server struct {
	URL     string
	Options []nats.Option
}

// StartServer starts an embedded, JetStream-enabled NATS server without a
// TCP listener. Clients communicate over net.Pipe via nats.InProcessServer,
// allowing the tests to run in network-restricted sandboxes.
func StartServer(t testing.TB) Server {
	t.Helper()

	opts := natstest.DefaultTestOptions
	opts.DontListen = true
	opts.JetStream = true
	opts.StoreDir = t.TempDir()
	// The V1 StageResult queue permits a 2 MiB payload after accounting for
	// worst-case JSON escaping. Keep the embedded broker representative of the
	// required deployment-level max_payload setting.
	opts.MaxPayload = 2 << 20

	srv := natstest.RunServer(&opts)
	t.Cleanup(func() {
		srv.Shutdown()
		srv.WaitForShutdown()
	})

	return Server{
		URL:     "nats://in-process:4222",
		Options: []nats.Option{nats.InProcessServer(srv)},
	}
}
