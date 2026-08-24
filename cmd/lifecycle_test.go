package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
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

func TestNewHTTPServerAppliesResourceLimits(t *testing.T) {
	cfg := &Config{
		HTTPReadHeaderTimeout: 2 * time.Second,
		HTTPReadTimeout:       45 * time.Second,
		HTTPWriteTimeout:      30 * time.Second,
		HTTPIdleTimeout:       90 * time.Second,
		HTTPShutdownTimeout:   7 * time.Second,
		HTTPMaxHeaderBytes:    32768,
	}
	srv, shutdownTimeout := newHTTPServer(":0", http.NotFoundHandler(), cfg)
	if srv.ReadHeaderTimeout != 2*time.Second || srv.ReadTimeout != 45*time.Second || srv.WriteTimeout != 30*time.Second || srv.IdleTimeout != 90*time.Second || shutdownTimeout != 7*time.Second || srv.MaxHeaderBytes != 32768 {
		t.Fatalf("HTTP server limits = %+v, shutdown = %s", srv, shutdownTimeout)
	}
}

func TestRunHTTPServerGracefullyDrainsInFlightRequest(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	release := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusNoContent)
	})
	srv, _ := newHTTPServer(":0", handler, &Config{})
	done := make(chan error, 1)
	go func() { done <- runHTTPServer(ctx, srv, time.Second, func() error { return srv.Serve(listener) }) }()
	requestDone := make(chan error, 1)
	go func() { _, err := http.Get("http://" + listener.Addr().String()); requestDone <- err }()
	<-started
	cancel()
	select {
	case err := <-done:
		t.Fatalf("shutdown returned before in-flight request drained: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not shut down")
	}
	if err := <-requestDone; err != nil {
		t.Fatal(err)
	}
}

func TestRunHTTPServerBoundsShutdown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	srv, _ := newHTTPServer(":0", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(started)
		<-release
	}), &Config{})
	done := make(chan error, 1)
	go func() {
		done <- runHTTPServer(ctx, srv, 10*time.Millisecond, func() error { return srv.Serve(listener) })
	}()
	go func() { _, _ = http.Get("http://" + listener.Addr().String()) }()
	<-started
	cancel()
	select {
	case err := <-done:
		if err == nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("shutdown error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown was not bounded")
	}
}

func TestHTTPServerAcceptsNearLimitBodyWithinReadDeadline(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	const bodySize = (10 << 20) - 1024
	received := make(chan int64, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, err := io.Copy(io.Discard, http.MaxBytesReader(w, r.Body, 10<<20))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		received <- n
		w.WriteHeader(http.StatusNoContent)
	})
	cfg := &Config{
		HTTPReadHeaderTimeout: time.Second,
		HTTPReadTimeout:       5 * time.Second,
		HTTPWriteTimeout:      5 * time.Second,
		HTTPIdleTimeout:       time.Second,
		HTTPShutdownTimeout:   time.Second,
		HTTPMaxHeaderBytes:    64 << 10,
	}
	srv, shutdownTimeout := newHTTPServer(":0", handler, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runHTTPServer(ctx, srv, shutdownTimeout, func() error { return srv.Serve(listener) }) }()

	response, err := http.Post("http://"+listener.Addr().String(), "application/xml", strings.NewReader(strings.Repeat("x", bodySize)))
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d", response.StatusCode)
	}
	if n := <-received; n != bodySize {
		t.Fatalf("received %d bytes, want %d", n, bodySize)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
