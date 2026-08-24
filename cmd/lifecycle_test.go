package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
)

func testCommandConfig(t *testing.T) *Config {
	t.Helper()
	s := queuetest.StartServer(t)
	return &Config{
		Host:            "127.0.0.1",
		Port:            0,
		OAuthHMACSecret: "secret",
		NATSURL:         s.URL,
		NATSOptions:     s.Options,
		TLSCertFile:     "cert-only",
	}
}

func TestServeRejectsIncompleteTLSConfiguration(t *testing.T) {
	cfg := testCommandConfig(t)
	var output bytes.Buffer
	err := serve(context.Background(), &output, cfg)
	if err == nil || !strings.Contains(err.Error(), "both --tls-cert-file and --tls-key-file") {
		t.Fatalf("serve error = %v", err)
	}
}

func TestServeReportsInvalidTLSFiles(t *testing.T) {
	cfg := testCommandConfig(t)
	cfg.TLSKeyFile = "missing-key"
	var output bytes.Buffer
	err := serve(context.Background(), &output, cfg)
	if err == nil || !strings.Contains(output.String(), "(TLS)") {
		t.Fatalf("serve TLS = %v, output %q", err, output.String())
	}
}

func TestServeCommandRunsConfiguredFunction(t *testing.T) {
	cfg := testCommandConfig(t)
	cmd := newServeCmd(cfg)
	cmd.SetArgs([]string{})
	err := cmd.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "both --tls-cert-file and --tls-key-file") {
		t.Fatalf("serve command error = %v", err)
	}
}

func TestWorkerStartsAndStopsOnContextCancellation(t *testing.T) {
	s := queuetest.StartServer(t)
	cfg := &Config{
		NATSURL:           s.URL,
		NATSOptions:       s.Options,
		WorkerMetricsAddr: "",
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- worker(ctx, cfg) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not stop")
	}
}
