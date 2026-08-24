package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"

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
	addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))

	handler, err := newHandler(cfg)
	if err != nil {
		return fmt.Errorf("build HTTP handler: %w", err)
	}

	httpServer := &http.Server{
		Addr:    addr,
		Handler: handler,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}

	if cfg.TLSCertFile != "" || cfg.TLSKeyFile != "" {
		if cfg.TLSCertFile == "" || cfg.TLSKeyFile == "" {
			return fmt.Errorf("both --tls-cert-file and --tls-key-file must be set to enable TLS")
		}
		fmt.Fprintf(stdout, "starting server on %s (TLS)\n", addr)
		return httpServer.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
	}

	fmt.Fprintf(stdout, "starting server on %s\n", addr)
	return httpServer.ListenAndServe()
}

func newHandler(cfg *Config) (http.Handler, error) {
	return server.NewHandler(server.Config{
		OpenAPIPath:     cfg.OpenAPIPath,
		SwaggerPath:     cfg.SwaggerPath,
		OAuthIssuer:     cfg.OAuthIssuer,
		OAuthAudience:   cfg.OAuthAudience,
		OAuthHMACSecret: cfg.OAuthHMACSecret,
		NATSURL:         cfg.NATSURL,
		QueueNamespace:  cfg.QueueNamespace,
		RateLimitRPS:    cfg.RateLimitRPS,
		RateLimitBurst:  cfg.RateLimitBurst,
		Logger:          slog.Default(),
	})
}
