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
	"sync"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type WorkerConfig struct {
	NATSURL           string
	NATSOptions       []nats.Option
	QueueNamespace    queue.Namespace
	ConsumerName      string
	SubjectFilter     string
	MetricsAddr       string
	ValidationTimeout time.Duration
	Concurrency       int
	Prefetch          int
	AckWait           time.Duration
	MaxDeliver        int
	// MaxAckPending is the fleet-wide outstanding-message limit on the shared
	// durable consumer. Zero derives Concurrency + Prefetch for one replica.
	MaxAckPending int
	Logger        *slog.Logger
}

const defaultWorkerConsumerName = "ubl-validator"
const defaultValidationTimeout = 30 * time.Second
const defaultWorkerConcurrency = 1
const defaultWorkerPrefetch = 1
const defaultWorkerAckWait = 60 * time.Second
const defaultWorkerMaxDeliver = 5

type workerRuntimeSettings struct {
	validationTimeout time.Duration
	concurrency       int
	prefetch          int
	ackWait           time.Duration
	maxDeliver        int
	maxAckPending     int
}

func resolveWorkerRuntimeSettings(cfg WorkerConfig) (workerRuntimeSettings, error) {
	s := workerRuntimeSettings{
		validationTimeout: cfg.ValidationTimeout,
		concurrency:       cfg.Concurrency,
		prefetch:          cfg.Prefetch,
		ackWait:           cfg.AckWait,
		maxDeliver:        cfg.MaxDeliver,
		maxAckPending:     cfg.MaxAckPending,
	}
	if s.validationTimeout <= 0 {
		s.validationTimeout = defaultValidationTimeout
	}
	if s.concurrency <= 0 {
		s.concurrency = defaultWorkerConcurrency
	}
	if s.prefetch <= 0 {
		s.prefetch = defaultWorkerPrefetch
	}
	if s.ackWait <= 0 {
		s.ackWait = defaultWorkerAckWait
	}
	if s.maxDeliver <= 0 {
		s.maxDeliver = defaultWorkerMaxDeliver
	}
	localCapacity := s.concurrency + s.prefetch
	if s.maxAckPending <= 0 {
		s.maxAckPending = localCapacity
	}
	if s.maxAckPending < localCapacity {
		return workerRuntimeSettings{}, fmt.Errorf("worker max ack pending (%d) must be at least concurrency + prefetch (%d)", s.maxAckPending, localCapacity)
	}
	if s.ackWait <= s.validationTimeout {
		return workerRuntimeSettings{}, fmt.Errorf("worker ack wait (%s) must exceed validation timeout (%s)", s.ackWait, s.validationTimeout)
	}
	return s, nil
}

func RunWorker(ctx context.Context, cfg WorkerConfig) error {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	settings, err := resolveWorkerRuntimeSettings(cfg)
	if err != nil {
		return err
	}

	q, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
	if err != nil {
		return fmt.Errorf("connect to NATS: %w", err)
	}
	defer q.Close()

	if err := q.EnsureInfra(ctx); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("ensure JetStream infrastructure: %w", err)
	}

	catalog, err := loadValidationCatalog()
	if err != nil {
		return fmt.Errorf("load UBL 2.1 schema catalog: %w", err)
	}

	consumerName := cfg.ConsumerName
	if consumerName == "" {
		consumerName = defaultWorkerConsumerName
	}
	localCapacity := settings.concurrency + settings.prefetch
	consumer, err := q.DocumentsConsumerWithConfig(ctx, consumerName, cfg.SubjectFilter, settings.ackWait, settings.maxDeliver, settings.maxAckPending)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
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

	worker := &documentWorker{queue: q, catalog: catalog, logger: logger, validationTimeout: settings.validationTimeout, progressInterval: settings.ackWait / 2}
	messages, err := consumer.Messages(jetstream.PullMaxMessages(localCapacity))
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("start pulling documents: %w", err)
	}
	defer messages.Stop()
	jobs := make(chan jetstream.Msg, settings.prefetch)
	workerJobsChannelCapacity.Set(float64(settings.prefetch))
	workerNATSIteratorCapacity.Set(float64(localCapacity))
	defer workerJobsChannelDepth.Set(0)

	var workers sync.WaitGroup
	for i := 0; i < settings.concurrency; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for msg := range jobs {
				workerJobsChannelDepth.Set(float64(len(jobs)))
				worker.process(msg)
			}
		}()
	}

	logger.Info("worker started",
		"consumer", consumerName,
		"queueNamespace", nonEmpty(cfg.QueueNamespace.String(), "(default)"),
		"subjectFilter", nonEmpty(cfg.SubjectFilter, q.Namespace.DocumentsFilterSubject()),
		"concurrency", settings.concurrency,
		"prefetch", settings.prefetch,
		"ackWait", settings.ackWait,
		"maxAckPending", settings.maxAckPending,
	)

	pullDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			messages.Stop()
		case <-pullDone:
		}
	}()
	for {
		msg, nextErr := messages.Next()
		if nextErr != nil {
			if ctx.Err() != nil {
				break
			}
			close(pullDone)
			close(jobs)
			workers.Wait()
			return fmt.Errorf("pull document: %w", nextErr)
		}
		select {
		case jobs <- msg:
			workerJobsChannelDepth.Set(float64(len(jobs)))
		case <-ctx.Done():
			break
		}
		if ctx.Err() != nil {
			break
		}
	}
	close(pullDone)
	close(jobs)
	workers.Wait()
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
	queue             *queue.Client
	catalog           *validationCatalog
	logger            *slog.Logger
	validationTimeout time.Duration
	progressInterval  time.Duration
}

const nakRetryDelay = 10 * time.Second

func (w *documentWorker) process(msg jetstream.Msg) {
	workerValidationsInFlight.Inc()
	defer workerValidationsInFlight.Dec()
	if metadata, err := msg.Metadata(); err == nil && metadata.NumDelivered > 1 {
		workerRedeliveries.Inc()
	}
	stopProgress := startProgressHeartbeats(msg, w.progressInterval)
	defer stopProgress()
	w.handle(msg)
}

func startProgressHeartbeats(msg jetstream.Msg, interval time.Duration) func() {
	if interval <= 0 {
		return func() {}
	}
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = msg.InProgress()
			case <-stop:
				return
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { close(stop) }) }
}

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

	validationErr := validateAgainstSchema(ctx, schemaPath, content, w.validationTimeout)
	validatedAt := time.Now()

	if validationErr != nil && errors.Is(validationErr, errXMLLintUnavailable) {
		logger.Error("xmllint unavailable; will retry", "error", validationErr)
		recordValidationOutcome("validator_unavailable")
		_ = msg.NakWithDelay(nakRetryDelay)
		return
	}
	if validationErr != nil && errors.Is(validationErr, errValidationTimeout) {
		logger.Error("schema validation timed out; will retry", "error", validationErr)
		recordValidationOutcome("validation_timeout")
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
