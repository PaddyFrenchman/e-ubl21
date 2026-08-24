// This file implements the background validation worker. The documents
// stream carries compact work descriptors; the worker dereferences each
// upload's immutable raw XML from JetStream Object Store, validates it
// against the UBL 2.1 XSD, records the outcome in metadata KV, and publishes
// a ValidationResult.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type WorkerConfig struct {
	NATSURL        string
	NATSOptions    []nats.Option
	QueueNamespace queue.Namespace
	ConsumerName   string
	SubjectFilter  string
	MetricsAddr    string
	Logger         *slog.Logger
}

const defaultWorkerConsumerName = "ubl-validator"

func RunWorker(ctx context.Context, cfg WorkerConfig) error {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	q, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
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
		"queueNamespace", nonEmpty(cfg.QueueNamespace.String(), "(default)"),
		"subjectFilter", nonEmpty(cfg.SubjectFilter, q.Namespace.DocumentsFilterSubject()),
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

type documentWorker struct {
	queue   *queue.Client
	catalog *schemaCatalog
	logger  *slog.Logger
}

const nakRetryDelay = 10 * time.Second

func (w *documentWorker) handle(msg jetstream.Msg) {
	start := time.Now()
	ctx := context.Background()

	item, err := queue.DecodeDocumentWorkItem(msg.Data())
	if err != nil {
		w.logger.Error("invalid document work descriptor; terminating message", "error", err, "subject", msg.Subject())
		recordValidationOutcome("invalid_work_item")
		_ = msg.Term()
		return
	}

	id := msg.Headers().Get(queue.HeaderUploadID)
	documentType := w.queue.Namespace.DocumentTypeFromSubject(msg.Subject())
	headerDocumentType := msg.Headers().Get(queue.HeaderDocumentType)
	logger := w.logger.With("id", item.ID, "documentType", item.DocumentType)

	// The descriptor, subject and headers deliberately duplicate identity
	// fields. Refuse inconsistent messages before dereferencing content so a
	// corrupt/misrouted work item cannot validate another upload's object.
	if id != item.ID || documentType != item.DocumentType || (headerDocumentType != "" && headerDocumentType != item.DocumentType) {
		logger.Error("inconsistent document work descriptor; terminating message",
			"headerID", id,
			"subjectDocumentType", documentType,
			"headerDocumentType", headerDocumentType,
		)
		recordValidationOutcome("invalid_work_item")
		_ = msg.Term()
		return
	}

	schemaPath, ok := w.catalog.schemaPathFor(documentType)
	if !ok {
		logger.Error("no schema found for queued document type; terminating message")
		recordValidationOutcome("unsupported_document_type")
		_ = msg.Term()
		return
	}

	content, err := w.queue.GetUploadContent(ctx, id)
	if err != nil {
		// The API stores content before publishing the descriptor. Failure to
		// retrieve it is therefore an operational/storage failure worth retrying.
		logger.Error("failed to load upload content; will retry", "error", err)
		recordValidationOutcome("content_unavailable")
		_ = msg.NakWithDelay(nakRetryDelay)
		return
	}

	validationErr := validateAgainstSchema(schemaPath, content)
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
