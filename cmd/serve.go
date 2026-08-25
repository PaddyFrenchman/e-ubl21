package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/server"
	"github.com/spf13/cobra"
)

func newServeCmd(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return serve(cmd.Context(), cmd.OutOrStdout(), cfg)
		},
	}
}

func serve(ctx context.Context, stdout io.Writer, cfg *Config) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))

	application, err := server.NewApplication(serverConfig(cfg))
	if err != nil {
		return fmt.Errorf("build HTTP handler: %w", err)
	}
	defer application.Close()

	httpServer, shutdownTimeout := newHTTPServer(addr, application.Handler, cfg)

	if cfg.TLSCertFile != "" || cfg.TLSKeyFile != "" {
		if cfg.TLSCertFile == "" || cfg.TLSKeyFile == "" {
			return fmt.Errorf("both --tls-cert-file and --tls-key-file must be set to enable TLS")
		}
		fmt.Fprintf(stdout, "starting server on %s (TLS)\n", addr)
		return runHTTPServer(ctx, httpServer, shutdownTimeout, func() error {
			return httpServer.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
		})
	}

	fmt.Fprintf(stdout, "starting server on %s\n", addr)
	return runHTTPServer(ctx, httpServer, shutdownTimeout, httpServer.ListenAndServe)
}

const defaultHTTPReadHeaderTimeout = 5 * time.Second
const defaultHTTPReadTimeout = 60 * time.Second
const defaultHTTPWriteTimeout = 60 * time.Second
const defaultHTTPIdleTimeout = 2 * time.Minute
const defaultHTTPShutdownTimeout = 15 * time.Second
const defaultHTTPMaxHeaderBytes = 64 << 10

func newHTTPServer(addr string, handler http.Handler, cfg *Config) (*http.Server, time.Duration) {
	readHeaderTimeout := positiveDuration(cfg.HTTPReadHeaderTimeout, defaultHTTPReadHeaderTimeout)
	readTimeout := positiveDuration(cfg.HTTPReadTimeout, defaultHTTPReadTimeout)
	writeTimeout := positiveDuration(cfg.HTTPWriteTimeout, defaultHTTPWriteTimeout)
	idleTimeout := positiveDuration(cfg.HTTPIdleTimeout, defaultHTTPIdleTimeout)
	shutdownTimeout := positiveDuration(cfg.HTTPShutdownTimeout, defaultHTTPShutdownTimeout)
	maxHeaderBytes := cfg.HTTPMaxHeaderBytes
	if maxHeaderBytes <= 0 {
		maxHeaderBytes = defaultHTTPMaxHeaderBytes
	}
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}, shutdownTimeout
}

func positiveDuration(value, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}

func runHTTPServer(ctx context.Context, httpServer *http.Server, shutdownTimeout time.Duration, listen func() error) error {
	serveErr := make(chan error, 1)
	go func() { serveErr <- listen() }()

	select {
	case err := <-serveErr:
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			_ = httpServer.Close()
			return fmt.Errorf("graceful HTTP shutdown: %w", err)
		}
		select {
		case err := <-serveErr:
			if err == nil || errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return err
		case <-shutdownCtx.Done():
			_ = httpServer.Close()
			return fmt.Errorf("wait for HTTP server shutdown: %w", shutdownCtx.Err())
		}
	}
}

func serverConfig(cfg *Config) server.Config {
	return server.Config{
		OpenAPIPath:            cfg.OpenAPIPath,
		SwaggerPath:            cfg.SwaggerPath,
		OAuthIssuer:            cfg.OAuthIssuer,
		OAuthAudience:          cfg.OAuthAudience,
		OAuthHMACSecret:        cfg.OAuthHMACSecret,
		NATSURL:                cfg.NATSURL,
		NATSOptions:            cfg.NATSOptions,
		QueueNamespace:         cfg.QueueNamespace,
		RateLimitRPS:           cfg.RateLimitRPS,
		RateLimitBurst:         cfg.RateLimitBurst,
		UploadContentRetention: cfg.UploadContentRetention,
		UploadContentMaxBytes:  cfg.UploadContentMaxBytes,
		Logger:                 slog.Default(),
	}
}
