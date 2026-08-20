package cmd

import (
	"context"
	"fmt"
	"io"
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

	server := &http.Server{
		Addr:    addr,
		Handler: newHandler(cfg),
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}

	fmt.Fprintf(stdout, "starting server on %s\n", addr)
	return server.ListenAndServe()
}

func newHandler(cfg *Config) http.Handler {
	return server.NewHandler(server.Config{
		OpenAPIPath:   cfg.OpenAPIPath,
		OAuthIssuer:   cfg.OAuthIssuer,
		OAuthAudience: cfg.OAuthAudience,
	})
}
