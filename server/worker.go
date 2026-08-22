// This file implements the background validation worker: it consumes
// queued UBL documents from the per-document-type NATS JetStream subjects
// (see queue.DocumentsConsumer), validates each against its UBL 2.1 XSD
// using the same xmllint-based logic the HTTP API previously ran
// synchronously (schema_catalog.go), records the outcome in the JetStream
// KV upload store so GET /api/v1/ubl/{id} reflects it, and publishes a
// ValidationResult to the single output results stream
// (queue.ResultsSubject). It is exposed to the CLI as `e-ubl21 worker`
// (see cmd/worker.go).
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/nats-io/nats.go/jetstream"
)

// WorkerConfig configures the validation worker.
type WorkerConfig struct {
	// NATSURL is the NATS server to connect to.
	NATSURL string
	// ConsumerName is the durable JetStream consumer name. Defaults to
	// "ubl-validator". Running multiple worker replicas with the same
	// ConsumerName and SubjectFilter load-balances documents across them.
	ConsumerName string
	// SubjectFilter selects which document type(s) this worker processes.
	// Defaults to queue.DocumentsFilterSubject (every document type). Set
	// it to queue.DocumentSubject("Invoice") (for example) to dedicate a
	// worker to a single document type's queue.
	SubjectFilter string
	// MetricsAddr, if non-empty, serves GET /metrics (Prometheus) and
	// GET /healthz on this address for the worker process. Unlike the API's
	// routes, these are unauthenticated: they are meant to be reachable
	// only from an internal operator/monitoring network, not the public
	// internet.
	MetricsAddr string
	// Logger receives structured logs. Defaults to slog.Default().
	Logger *slog.Logger
}

const defaultWorkerConsumerName = "ubl-validator"

// RunWorker connects to NATS, ensures the required JetStream
// streams/KV bucket exist, and processes documents until ctx is canceled.
// It returns once the consumer and any metrics server have shut down.
func RunWorker(ctx context.Context, cfg WorkerConfig) error {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	q, err := queue.Connect(cfg.NATSURL)
	if err != nil {
		return fmt.Errorf("connect to NATS: %w", err)
	}
	defer q.Close()

	if err := q.EnsureInfra(ctx); err != nil {
		return fmt.Errorf("ensure JetStream infrastructure: %w", err)
	}

	catalog, err := loadSchemaCatalog()
	if err != nil {
		return fmt.Errorf("load UBL 2.1 schema catalog: %w", err)
	}

	consumerName := cfg.ConsumerName
	if consumerName == "" {
		consumerName = defaultWorkerConsumerName
	}
	consumer, err := q.DocumentsConsumer(ctx, consumerName, cfg.SubjectFilter)
	if err != nil {
		return fmt.Errorf("create documents consumer: %w", err)
	}

	var metricsServer *http.Server
	if cfg.MetricsAddr != "" {
		metricsServer = &http.Server{Addr: cfg.MetricsAddr, Handler: workerMetricsMux(q)}
		go func() {
			if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Error("worker metrics server error", "error", err)
			}
		}()
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = metricsServer.Shutdown(shutdownCtx)
		}()
	}

	worker := &documentWorker{queue: q, catalog: catalog, logger: logger}
	consumeCtx, err := consumer.Consume(worker.handle)
	if err != nil {
		return fmt.Errorf("start consuming: %w", err)
	}
	defer consumeCtx.Stop()

	logger.Info("worker started",
		"consumer", consumerName,
		"subjectFilter", nonEmpty(cfg.SubjectFilter, queue.DocumentsFilterSubject),
	)
	<-ctx.Done()
	logger.Info("worker shutting down")
	return nil
}

func nonEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// documentWorker processes a single queued document per handle call.
type documentWorker struct {
	queue   *queue.Client
	catalog *schemaCatalog
	logger  *slog.Logger
}

// nakRetryDelay is used when a message could not be fully processed for a
// transient reason (the validator is unavailable, or the KV/results-stream
// writes failed), so JetStream retries delivery shortly instead of
// immediately hot-looping.
const nakRetryDelay = 10 * time.Second

func (w *documentWorker) handle(msg jetstream.Msg) {
	start := time.Now()
	ctx := context.Background()

	id := msg.Headers().Get(queue.HeaderUploadID)
	documentType := queue.DocumentTypeFromSubject(msg.Subject())
	logger := w.logger.With("id", id, "documentType", documentType)

	schemaPath, ok := w.catalog.schemaPathFor(documentType)
	if !ok {
		// The API rejects unrecognized document types before ever queuing
		// them, so this should not happen in practice; treat it as a
		// permanent failure (retrying will never succeed) rather than
		// redelivering forever.
		logger.Error("no schema found for queued document type; terminating message")
		recordValidationOutcome("unsupported_document_type")
		_ = msg.Term()
		return
	}

	validationErr := validateAgainstSchema(schemaPath, msg.Data())
	validatedAt := time.Now()

	if validationErr != nil && errors.Is(validationErr, errXMLLintUnavailable) {
		logger.Error("xmllint unavailable; will retry", "error", validationErr)
		recordValidationOutcome("validator_unavailable")
		_ = msg.NakWithDelay(nakRetryDelay)
		return
	}

	status := queue.StatusValid
	errMsg := ""
	if validationErr != nil {
		status = queue.StatusInvalid
		errMsg = validationErr.Error()
	}

	if err := w.queue.UpdateUploadStatus(ctx, id, status, errMsg, validatedAt); err != nil {
		logger.Error("failed to update upload status; will retry", "error", err)
		recordValidationOutcome("internal_error")
		_ = msg.NakWithDelay(nakRetryDelay)
		return
	}

	if err := w.queue.PublishResult(ctx, queue.ValidationResult{
		ID:           id,
		DocumentType: documentType,
		Status:       status,
		Error:        errMsg,
		ValidatedAt:  validatedAt.UTC().Format(time.RFC3339),
	}); err != nil {
		logger.Error("failed to publish validation result; will retry", "error", err)
		recordValidationOutcome("internal_error")
		_ = msg.NakWithDelay(nakRetryDelay)
		return
	}

	recordValidationOutcome(string(status))
	recordValidationDuration(documentType, time.Since(start).Seconds())
	if err := msg.Ack(); err != nil {
		logger.Error("failed to ack message", "error", err)
		return
	}
	logger.Info("processed document", "status", status, "duration_ms", time.Since(start).Milliseconds())
}
