// Package queuetest provides an embedded, JetStream-enabled NATS server for
// use in tests, so the queue package (and consumers of it, such as the
// server package's tests) can exercise real NATS/JetStream behavior without
// requiring Docker or an external NATS deployment.
package queuetest

import (
	"testing"

	natstest "github.com/nats-io/nats-server/v2/test"
)

// StartServer starts an embedded NATS server with JetStream enabled on a
// random free port, storing its JetStream state under a fresh t.TempDir().
// The server is automatically shut down via t.Cleanup. It returns the URL
// to pass to queue.Connect.
func StartServer(t *testing.T) string {
	t.Helper()

	opts := natstest.DefaultTestOptions
	opts.Port = -1 // let the OS choose a free port
	opts.JetStream = true
	opts.StoreDir = t.TempDir()

	srv := natstest.RunServer(&opts)
	t.Cleanup(func() {
		srv.Shutdown()
		srv.WaitForShutdown()
	})

	return srv.ClientURL()
}
