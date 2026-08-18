package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

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
	mux := http.NewServeMux()
	openAPIPath := cfg.OpenAPIPath
	if openAPIPath == "" {
		openAPIPath = "/openapi.json"
	}
	if !strings.HasPrefix(openAPIPath, "/") {
		openAPIPath = "/" + openAPIPath
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		response := map[string]any{
			"service": "e-ubl21",
			"oauth": map[string]string{
				"issuer":   cfg.OAuthIssuer,
				"audience": cfg.OAuthAudience,
			},
		}

		writeJSON(w, http.StatusOK, response)
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc(openAPIPath, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"openapi": "3.0.3",
			"info": map[string]string{
				"title":   "e-ubl21 API",
				"version": "0.1.0",
			},
			"paths": map[string]any{
				"/healthz": map[string]any{
					"get": map[string]any{
						"summary": "Health check",
					},
				},
			},
			"components": map[string]any{
				"securitySchemes": map[string]any{
					"oauth2": map[string]any{
						"type": "oauth2",
					},
				},
			},
		})
	})

	return mux
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
