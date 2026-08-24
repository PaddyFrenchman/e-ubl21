// This file wires together the HTTP API: route registration, request
// observability (log/slog + Prometheus, see logging.go/metrics.go),
// OAuth2 bearer-token verification (see auth.go), and per-client rate
// limiting (see ratelimit.go). The individual endpoint handlers live in
// upload.go (UBL upload/retrieval, backed by the queue package's JetStream
// KV upload store), schema_catalog.go (XSD validation, used here only to
// recognize supported document types - actual validation happens
// asynchronously in the worker, see worker.go) and openapi.go (the served
// OpenAPI document).
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

// Config holds HTTP server configuration.
type Config struct {
	OpenAPIPath   string
	SwaggerPath   string
	OAuthIssuer   string
	OAuthAudience string
	// OAuthHMACSecret is the shared secret used to verify the HS256
	// signature of bearer tokens. Required for any protected route to work;
	// if empty, protected routes fail closed with 500 rather than skipping
	// signature verification.
	OAuthHMACSecret string
	// NATSURL is the NATS server to connect to for queuing uploaded
	// documents and reading/writing their status (see the queue package).
	// Required: NewHandler returns an error if a connection cannot be
	// established.
	NATSURL string
	// QueueNamespace scopes every JetStream stream, subject and KV bucket
	// this API instance uses (see queue.Namespace). Defaults to
	// queue.DefaultNamespace, preserving e-ubl21's original hard-coded
	// names.
	QueueNamespace queue.Namespace
	// RateLimitRPS and RateLimitBurst configure the per-client-IP token
	// bucket rate limiter applied to every route. Non-positive values fall
	// back to sensible defaults (5 req/s, burst of 10).
	RateLimitRPS   float64
	RateLimitBurst int
	// Logger receives structured request logs. Defaults to slog.Default().
	Logger *slog.Logger
}

// NewHandler builds the OAuth2-protected HTTP API for UBL validation
// uploads. It performs some upfront, potentially fallible setup (extracting
// the embedded XSD schema tree and connecting to NATS/ensuring the
// JetStream streams and KV bucket it depends on exist), so it can return an
// error.
func NewHandler(cfg Config) (http.Handler, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	openAPIPath := normalizePath(cfg.OpenAPIPath, "/openapi.json", false)
	swaggerPath := normalizePath(cfg.SwaggerPath, "/swagger/", true)

	catalog, err := loadSchemaCatalog()
	if err != nil {
		return nil, fmt.Errorf("load UBL 2.1 schema catalog: %w", err)
	}
	q, err := queue.Connect(cfg.NATSURL, cfg.QueueNamespace)
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

	// register wraps handler with request observability (always) and OAuth2
	// bearer-token verification (unless public is true), then registers it
	// under pattern on mux. route is the low-cardinality metrics label.
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

	// The OpenAPI document and the Swagger UI that renders it are left
	// unauthenticated (unlike every other route) so the API is
	// self-discoverable without first obtaining a bearer token.
	register(openAPIPath, openAPIPath, true, requireGET(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, openAPIDocument())
	}))
	register(swaggerPath, swaggerPath, true, httpSwagger.Handler(httpSwagger.URL(openAPIPath)))

	// These two are registered as bare (method-agnostic) patterns, with the
	// handlers themselves enforcing the allowed method and returning a
	// proper 405 JSON error - not "POST /api/v1/ubl/upload"/"GET
	// /api/v1/ubl/{id}" method-restricted patterns. That matters because
	// the catch-all "/" pattern above (registered with no method
	// restriction) would otherwise be the only pattern matching a
	// wrong-method request to these paths, since Go's ServeMux only
	// generates its own automatic 405 when some *other* registered pattern
	// still matches the path; a method-restricted pattern simply doesn't
	// match at all on a method mismatch, silently falling through to "/"'s
	// own 404 instead.
	register("/api/v1/ubl/upload", "/api/v1/ubl/upload", false, uploads.handleUpload)
	register("/api/v1/ubl/{id}", "/api/v1/ubl/{id}", false, uploads.handleGetUpload)

	return limiter.middleware(mux), nil
}

// normalizePath ensures path is non-empty (falling back to def), starts
// with a leading slash, and - if requireTrailingSlash is set (for subtree
// routes like the Swagger UI) - ends with a trailing slash too.
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

// requireGET wraps next so any non-GET request gets a proper 405 JSON error
// instead of silently being handled like a GET (the default for a plain,
// method-agnostic mux pattern).
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
