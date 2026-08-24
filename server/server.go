// This file wires together the HTTP API: route registration, request
// observability (log/slog + Prometheus, see logging.go/metrics.go),
// OAuth2 bearer-token verification (see auth.go), and per-client rate
// limiting (see ratelimit.go). The individual endpoint handlers live in
// upload.go, schema_catalog.go and openapi.go.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/nats-io/nats.go"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

type Config struct {
	OpenAPIPath     string
	SwaggerPath     string
	OAuthIssuer     string
	OAuthAudience   string
	OAuthHMACSecret string
	NATSURL         string
	NATSOptions     []nats.Option
	QueueNamespace  queue.Namespace
	RateLimitRPS    float64
	RateLimitBurst  int
	Logger          *slog.Logger
}

func NewHandler(cfg Config) (http.Handler, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	openAPIPath := normalizePath(cfg.OpenAPIPath, "/openapi.json", false)
	swaggerPath := normalizePath(cfg.SwaggerPath, "/swagger/", true)

	catalog, err := loadDocumentCatalog()
	if err != nil {
		return nil, fmt.Errorf("load UBL 2.1 schema catalog: %w", err)
	}
	q, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
	if err != nil {
		return nil, fmt.Errorf("connect to NATS: %w", err)
	}
	if err := q.EnsureInfra(context.Background()); err != nil {
		return nil, fmt.Errorf("ensure JetStream infrastructure: %w", err)
	}
	uploads := &uploadHandlers{catalog: catalog, queue: q}
	auth := newOAuthValidator(cfg.OAuthHMACSecret, cfg.OAuthIssuer, cfg.OAuthAudience)

	rps := cfg.RateLimitRPS
	if rps <= 0 {
		rps = 5
	}
	burst := cfg.RateLimitBurst
	if burst <= 0 {
		burst = 10
	}
	limiter, _ := newIPRateLimiter(rps, burst)
	mux := http.NewServeMux()

	register := func(pattern, route string, public bool, handler http.HandlerFunc) {
		if !public {
			handler = auth.requireOAuth2(handler)
		}
		mux.HandleFunc(pattern, observabilityMiddleware(logger, route, handler))
	}

	register("/", "/", false, requireGET(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"service": "e-ubl21",
			"docs":    swaggerPath,
			"oauth": map[string]string{
				"issuer":   cfg.OAuthIssuer,
				"audience": cfg.OAuthAudience,
			},
		})
	}))

	register("/healthz", "/healthz", false, requireGET(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}))
	register("/metrics", "/metrics", false, requireGET(metricsHandler()))

	register(openAPIPath, openAPIPath, true, requireGET(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, openAPIDocument())
	}))
	register(swaggerPath, swaggerPath, true, httpSwagger.Handler(httpSwagger.URL(openAPIPath)))

	register("/api/v1/ubl/upload", "/api/v1/ubl/upload", false, uploads.handleUpload)
	register("/api/v1/ubl/{id}", "/api/v1/ubl/{id}", false, uploads.handleGetUpload)

	return limiter.middleware(mux), nil
}

func normalizePath(path, def string, requireTrailingSlash bool) string {
	if path == "" {
		path = def
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if requireTrailingSlash && !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return path
}

func requireGET(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
