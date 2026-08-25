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
	"sync"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/nats-io/nats.go"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

type Config struct {
	OpenAPIPath            string
	SwaggerPath            string
	OAuthIssuer            string
	OAuthAudience          string
	OAuthHMACSecret        string
	NATSURL                string
	NATSOptions            []nats.Option
	QueueNamespace         queue.Namespace
	RateLimitRPS           float64
	RateLimitBurst         int
	UploadContentRetention time.Duration
	UploadContentMaxBytes  int64
	Logger                 *slog.Logger
}

// Application owns the API handler and its reusable runtime dependencies.
// Close releases the NATS connection after HTTP shutdown has drained.
type Application struct {
	Handler   http.Handler
	queue     *queue.Client
	closeOnce sync.Once
	cancel    context.CancelFunc
	workers   sync.WaitGroup
}

func (a *Application) Close() {
	if a == nil {
		return
	}
	a.closeOnce.Do(func() {
		if a.cancel != nil {
			a.cancel()
		}
		a.workers.Wait()
		if a.queue != nil {
			a.queue.Close()
		}
	})
}

func NewApplication(cfg Config) (*Application, error) {
	if err := validateUploadContentLimits(cfg.UploadContentRetention, cfg.UploadContentMaxBytes); err != nil {
		return nil, err
	}
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
	if err := q.EnsureInfraWithConfig(context.Background(), queue.InfraConfig{UploadContentMaxBytes: cfg.UploadContentMaxBytes}); err != nil {
		q.Close()
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(cachedOpenAPIJSON)
	}))
	register(swaggerPath, swaggerPath, true, httpSwagger.Handler(httpSwagger.URL(openAPIPath)))

	register("/api/v1/ubl/upload", "/api/v1/ubl/upload", false, uploads.handleUpload)
	register("/api/v1/ubl/{id}", "/api/v1/ubl/{id}", false, uploads.handleGetUpload)

	reconcileCtx, cancelReconcile := context.WithCancel(context.Background())
	application := &Application{Handler: limiter.middleware(mux), queue: q, cancel: cancelReconcile}
	uploadContentStorageLimitBytes.Set(float64(cfg.UploadContentMaxBytes))
	for _, outcome := range []string{"scheduled", "unscheduled", "expired", "reclaimed", "already_missing", "error"} {
		uploadContentExpirations.WithLabelValues(outcome)
	}
	application.workers.Add(1)
	go func() {
		defer application.workers.Done()
		runUploadMaintenance(reconcileCtx, q, cfg.UploadContentRetention, logger)
	}()
	return application, nil
}

func validateUploadContentLimits(retention time.Duration, maxBytes int64) error {
	if retention < 0 {
		return fmt.Errorf("upload content retention must be non-negative")
	}
	if maxBytes < 0 {
		return fmt.Errorf("upload content max bytes must be non-negative")
	}
	return nil
}

const uploadReconcileInterval = 30 * time.Second
const uploadReconcileMinAge = time.Minute

func runUploadMaintenance(ctx context.Context, q *queue.Client, contentRetention time.Duration, logger *slog.Logger) {
	reconcile := func() {
		report, err := q.ReconcileUploads(ctx, uploadReconcileMinAge)
		if err != nil && ctx.Err() == nil {
			logger.Error("upload acceptance reconciliation failed", "error", err)
			return
		}
		if report.Requeued > 0 || report.Failed > 0 || report.Errors > 0 {
			uploadReconciliations.WithLabelValues("requeued").Add(float64(report.Requeued))
			uploadReconciliations.WithLabelValues("acceptance_failed").Add(float64(report.Failed))
			uploadReconciliations.WithLabelValues("error").Add(float64(report.Errors))
			logger.Info("upload acceptance reconciliation completed",
				"scanned", report.Scanned, "requeued", report.Requeued,
				"failed", report.Failed, "errors", report.Errors)
		}
		expiry, expiryErr := q.ExpireUploadContent(ctx, contentRetention, time.Now())
		if expiryErr != nil && ctx.Err() == nil {
			logger.Error("upload content expiry scan failed", "error", expiryErr)
			uploadContentExpirations.WithLabelValues("error").Inc()
		} else if expiryErr == nil {
			uploadContentExpirations.WithLabelValues("scheduled").Add(float64(expiry.Scheduled))
			uploadContentExpirations.WithLabelValues("unscheduled").Add(float64(expiry.Unscheduled))
			uploadContentExpirations.WithLabelValues("expired").Add(float64(expiry.Expired))
			uploadContentExpirations.WithLabelValues("reclaimed").Add(float64(expiry.Reclaimed))
			uploadContentExpirations.WithLabelValues("already_missing").Add(float64(expiry.AlreadyMissing))
			uploadContentExpirations.WithLabelValues("error").Add(float64(expiry.Errors))
			uploadContentExpiredBytes.Add(float64(expiry.ExpiredBytes))
			if expiry.Scheduled > 0 || expiry.Unscheduled > 0 || expiry.Expired > 0 || expiry.Reclaimed > 0 || expiry.Errors > 0 {
				logger.Info("upload content expiry scan completed",
					"scanned", expiry.Scanned, "scheduled", expiry.Scheduled,
					"unscheduled", expiry.Unscheduled, "expired", expiry.Expired,
					"reclaimed", expiry.Reclaimed,
					"alreadyMissing", expiry.AlreadyMissing,
					"expiredBytes", expiry.ExpiredBytes, "errors", expiry.Errors)
			}
		}
		if size, sizeErr := q.UploadContentStorageSize(ctx); sizeErr != nil {
			if ctx.Err() == nil {
				logger.Error("read upload content storage usage", "error", sizeErr)
			}
		} else {
			uploadContentStorageBytes.Set(float64(size))
		}
	}
	reconcile()
	ticker := time.NewTicker(uploadReconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			reconcile()
		case <-ctx.Done():
			return
		}
	}
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
