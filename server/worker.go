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
	NATSURL        string
	NATSOptions    []nats.Option
	QueueNamespace queue.Namespace
	ConsumerName   string
	SubjectFilter  string
	MetricsAddr    string
	// ValidationTimeout bounds the complete XSD + EN 16931 pipeline.
	ValidationTimeout        time.Duration
	XSDValidationTimeout     time.Duration
	EN16931ValidationTimeout time.Duration
	Concurrency              int
	Prefetch                 int
	AckWait                  time.Duration
	MaxDeliver               int
	JavaExecutable           string
	SaxonJARPath             string
	XMLResolverJARPath       string
	SaxonConfigurationPath   string
	EN16931XSLTPath          string
	SaxonStartupTimeout      time.Duration
	// MaxAckPending is the fleet-wide outstanding-message limit on the shared
	// durable consumer. Zero derives Concurrency + Prefetch for one replica.
	MaxAckPending          int
	UploadContentRetention time.Duration
	UploadContentMaxBytes  int64
	Logger                 *slog.Logger
	// en16931Validator is an internal test seam. Production callers cannot set
	// it and always pass through checksum verification below.
	en16931Validator EN16931Validator
}

const defaultWorkerConsumerName = "ubl-validator"
const defaultValidationTimeout = 30 * time.Second
const defaultWorkerConcurrency = 1
const defaultWorkerPrefetch = 1
const defaultWorkerAckWait = 60 * time.Second
const defaultWorkerMaxDeliver = 5
const workerCompletionReconcileInterval = 30 * time.Second

type workerRuntimeSettings struct {
	validationTimeout        time.Duration
	xsdValidationTimeout     time.Duration
	en16931ValidationTimeout time.Duration
	concurrency              int
	prefetch                 int
	ackWait                  time.Duration
	maxDeliver               int
	maxAckPending            int
}

func resolveWorkerRuntimeSettings(cfg WorkerConfig) (workerRuntimeSettings, error) {
	s := workerRuntimeSettings{
		validationTimeout:        cfg.ValidationTimeout,
		xsdValidationTimeout:     cfg.XSDValidationTimeout,
		en16931ValidationTimeout: cfg.EN16931ValidationTimeout,
		concurrency:              cfg.Concurrency,
		prefetch:                 cfg.Prefetch,
		ackWait:                  cfg.AckWait,
		maxDeliver:               cfg.MaxDeliver,
		maxAckPending:            cfg.MaxAckPending,
	}
	if s.validationTimeout <= 0 {
		s.validationTimeout = defaultValidationTimeout
	}
	if s.xsdValidationTimeout <= 0 {
		s.xsdValidationTimeout = s.validationTimeout
	}
	if s.en16931ValidationTimeout <= 0 {
		s.en16931ValidationTimeout = s.validationTimeout
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
	if s.validationTimeout < s.xsdValidationTimeout {
		return workerRuntimeSettings{}, fmt.Errorf("worker validation timeout (%s) must be at least XSD validation timeout (%s)", s.validationTimeout, s.xsdValidationTimeout)
	}
	if s.validationTimeout < s.en16931ValidationTimeout {
		return workerRuntimeSettings{}, fmt.Errorf("worker validation timeout (%s) must be at least EN 16931 validation timeout (%s)", s.validationTimeout, s.en16931ValidationTimeout)
	}
	return s, nil
}

func RunWorker(ctx context.Context, cfg WorkerConfig) error {
	if err := ValidateUploadContentLimits(cfg.UploadContentRetention, cfg.UploadContentMaxBytes); err != nil {
		return err
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	settings, err := resolveWorkerRuntimeSettings(cfg)
	if err != nil {
		return err
	}
	en16931Validator := cfg.en16931Validator
	if en16931Validator == nil {
		saxonRuntime := resolveSaxonRuntimeConfig(cfg)
		if err := ensureSaxonRuntime(ctx, saxonRuntime); err != nil {
			return err
		}
		en16931Validator = newSaxonEN16931Validator(saxonRuntime, settings.en16931ValidationTimeout)
	}

	q, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
	if err != nil {
		return fmt.Errorf("connect to NATS: %w", err)
	}
	defer q.Close()

	if err := q.EnsureInfraWithConfig(ctx, queue.InfraConfig{UploadContentMaxBytes: cfg.UploadContentMaxBytes}); err != nil {
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
	// Broker delivery is deliberately unlimited. maxDeliver is an application
	// attempt threshold; once reached, redeliveries retry only the durable
	// dead-letter transition until both KV and DLQ state are confirmed.
	consumer, err := q.DocumentsConsumerWithConfig(ctx, consumerName, cfg.SubjectFilter, settings.ackWait, -1, settings.maxAckPending)
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

	validationPipeline := newDocumentValidationPipelineWithTimeouts(q, en16931Validator, settings.validationTimeout, settings.xsdValidationTimeout)
	validationPipeline.logger = logger
	worker := &documentWorker{queue: q, catalog: catalog, validator: validationPipeline, logger: logger, lifecycleCtx: ctx, progressInterval: settings.ackWait / 3, contentRetention: cfg.UploadContentRetention, maxDeliver: uint64(settings.maxDeliver)}
	messages, err := consumer.Messages(jetstream.PullMaxMessages(localCapacity))
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("start pulling documents: %w", err)
	}
	defer messages.Stop()
	reconcileCtx, cancelReconcile := context.WithCancel(ctx)
	var reconciler sync.WaitGroup
	reconciler.Add(1)
	go func() {
		defer reconciler.Done()
		worker.runCompletionReconciler(reconcileCtx, workerCompletionReconcileInterval)
	}()
	defer func() {
		cancelReconcile()
		reconciler.Wait()
	}()
	jobs := make(chan queuedDocumentJob, settings.prefetch)
	workerJobsChannelCapacity.Set(float64(settings.prefetch))
	workerNATSIteratorCapacity.Set(float64(localCapacity))
	defer workerJobsChannelDepth.Set(0)

	var workers sync.WaitGroup
	for i := 0; i < settings.concurrency; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for job := range jobs {
				workerJobsChannelDepth.Set(float64(len(jobs)))
				worker.processWithProgress(job.msg, job.stopProgress)
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
		"validationTimeout", settings.validationTimeout,
		"xsdValidationTimeout", settings.xsdValidationTimeout,
		"en16931ValidationTimeout", settings.en16931ValidationTimeout,
		"applicationMaxAttempts", settings.maxDeliver,
		"brokerMaxDeliver", "unlimited",
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
		job := newQueuedDocumentJob(msg, worker.progressInterval)
		select {
		case jobs <- job:
			workerJobsChannelDepth.Set(float64(len(jobs)))
		case <-ctx.Done():
			job.stopProgress()
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
	queue            *queue.Client
	catalog          *validationCatalog
	validator        *documentValidationPipeline
	logger           *slog.Logger
	lifecycleCtx     context.Context
	progressInterval time.Duration
	contentRetention time.Duration
	maxDeliver       uint64
}

// queuedDocumentJob starts acknowledgement heartbeats at pull time, not when
// a worker eventually begins validation. Prefetched documents can therefore
// wait behind a full two-stage validation without consuming their AckWait.
type queuedDocumentJob struct {
	msg          jetstream.Msg
	stopProgress func()
}

func newQueuedDocumentJob(msg jetstream.Msg, progressInterval time.Duration) queuedDocumentJob {
	return queuedDocumentJob{msg: msg, stopProgress: startProgressHeartbeats(msg, progressInterval)}
}

const nakRetryDelay = 10 * time.Second
const replayHandoffRetryDelay = 100 * time.Millisecond

func (w *documentWorker) process(msg jetstream.Msg) {
	w.processWithProgress(msg, startProgressHeartbeats(msg, w.progressInterval))
}

func (w *documentWorker) processWithProgress(msg jetstream.Msg, stopProgress func()) {
	workerValidationsInFlight.Inc()
	defer workerValidationsInFlight.Dec()
	if metadata, err := msg.Metadata(); err == nil && metadata.NumDelivered > 1 {
		workerRedeliveries.Inc()
	}
	if stopProgress != nil {
		defer stopProgress()
	}
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
	ctx := w.lifecycleCtx
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		w.deferForShutdown(msg, queue.DocumentWorkItem{})
		return
	}

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

	versioned, err := w.queue.GetUploadRecordVersion(ctx, id)
	if err != nil {
		recordValidationOutcome("internal_error")
		w.retryOperationalFailure(ctx, msg, item, nil, fmt.Errorf("load upload status: %w", err))
		return
	}
	if versioned.Record.Status == queue.StatusDeadLetterPending {
		w.finalizeDeadLetter(ctx, msg, versioned)
		return
	}
	if versioned.Record.Status == queue.StatusValid || versioned.Record.Status == queue.StatusInvalid {
		needsUpgrade := versioned.Record.ResultPending && !terminalValidationStagesComplete(versioned.Record)
		if needsUpgrade {
			published, inspectErr := w.queue.HasValidationResult(ctx, id)
			if inspectErr != nil {
				w.retryOperationalFailure(ctx, msg, item, &versioned, fmt.Errorf("inspect legacy validation result: %w", inspectErr))
				return
			}
			if !published {
				claimed := versioned.Record
				claimed.Status = queue.StatusPending
				claimed.Error = ""
				claimed.ValidatedAt = ""
				claimed.ValidationStages = nil
				claimed.ContentExpiresAt = ""
				claimed.ResultPending = false
				claimed.LastError = ""
				claimed.ProcessingAttempts = 0
				claimed.ProcessingFailedAt = ""
				revision, claimErr := w.queue.UpdateUploadRecord(ctx, claimed, versioned.Revision)
				if claimErr != nil {
					logger.Info("legacy validation upgrade claim raced another worker; will retry", "error", claimErr)
					_ = msg.NakWithDelay(replayHandoffRetryDelay)
					return
				}
				versioned = queue.VersionedUploadRecord{Record: claimed, Revision: revision}
				logger.Info("claimed incomplete legacy terminal result for mandatory EN 16931 revalidation")
			} else {
				// The permanent first-result slot is already occupied. Preserve that
				// historical pre-EN result and repair only the ambiguous pending marker.
				if err := w.ensureValidationResultPublished(ctx, versioned); err != nil {
					w.retryOperationalFailure(ctx, msg, item, &versioned, fmt.Errorf("finish prior legacy validation result: %w", err))
					return
				}
				recordValidationOutcome("duplicate")
				_ = msg.Ack()
				return
			}
		} else {
			if err := w.ensureValidationResultPublished(ctx, versioned); err != nil {
				w.retryOperationalFailure(ctx, msg, item, &versioned, fmt.Errorf("finish prior validation result: %w", err))
				return
			}
			recordValidationOutcome("duplicate")
			_ = msg.Ack()
			return
		}
	}
	if versioned.Record.Status == queue.StatusAcceptanceFailed || versioned.Record.Status == queue.StatusCleanupPending || versioned.Record.Status == queue.StatusProcessingFailed {
		logger.Error("work descriptor references terminally unaccepted upload; terminating message", "status", versioned.Record.Status)
		_ = msg.Term()
		return
	}
	if versioned.Record.Status != queue.StatusPending && versioned.Record.Status != queue.StatusEnqueuePending {
		w.retryOperationalFailure(ctx, msg, item, &versioned, fmt.Errorf("upload is not queueable: status %s", versioned.Record.Status))
		return
	}

	replayID := msg.Headers().Get(queue.HeaderReplayMessageID)
	if replayID == "" && versioned.Record.ReplayMessageID != "" {
		logger.Info("replay handoff is not complete; deferring descriptor")
		_ = msg.NakWithDelay(replayHandoffRetryDelay)
		return
	}
	if replayID != "" {
		metadata, metadataErr := msg.Metadata()
		if metadataErr != nil {
			logger.Error("cannot identify replay descriptor sequence; will retry", "error", metadataErr)
			_ = msg.NakWithDelay(replayHandoffRetryDelay)
			return
		}
		sequence := metadata.Sequence.Stream
		if versioned.Record.ReplayClaimedSeq != 0 {
			if versioned.Record.ReplayClaimedSeq != sequence {
				logger.Info("discarding duplicate replay descriptor",
					"claimedSequence", versioned.Record.ReplayClaimedSeq,
					"duplicateSequence", sequence,
				)
				recordValidationOutcome("duplicate")
				_ = msg.Ack()
				return
			}
		} else {
			if versioned.Record.ReplayMessageID != replayID {
				logger.Error("replay descriptor does not match canonical replay generation; terminating message", "replayMessageID", replayID)
				_ = msg.Term()
				return
			}
			if !versioned.Record.ReplayReady {
				logger.Info("replay handoff is not complete; deferring descriptor")
				_ = msg.NakWithDelay(replayHandoffRetryDelay)
				return
			}
			claimed := versioned.Record
			claimed.ReplayMessageID = ""
			claimed.ReplayReady = false
			claimed.ReplayClaimedSeq = sequence
			revision, claimErr := w.queue.UpdateUploadRecord(ctx, claimed, versioned.Revision)
			if claimErr != nil {
				logger.Info("replay descriptor claim raced another worker; will retry", "error", claimErr)
				_ = msg.NakWithDelay(replayHandoffRetryDelay)
				return
			}
			versioned = queue.VersionedUploadRecord{Record: claimed, Revision: revision}
		}
	}

	validationResult, validationErr := w.validator.Validate(ctx, id, documentType, schemaPath)
	if validationErr != nil {
		if errors.Is(validationErr, errValidationContentStream) {
			recordValidationOutcome("content_unavailable")
		} else if errors.Is(validationErr, errXMLLintUnavailable) {
			recordValidationOutcome("validator_unavailable")
		} else if errors.Is(validationErr, errValidationTimeout) || errors.Is(validationErr, context.DeadlineExceeded) {
			recordValidationOutcome("validation_timeout")
		} else {
			recordValidationOutcome("internal_error")
		}
		w.retryOperationalFailure(ctx, msg, item, &versioned, validationErr)
		return
	}

	status := validationResult.Status
	errMsg := validationResult.Error
	validatedAt := validationResult.ValidatedAt
	validatedAtText := validatedAt.UTC().Format(time.RFC3339)
	if status != queue.StatusValid && status != queue.StatusInvalid {
		recordValidationOutcome("internal_error")
		w.retryOperationalFailure(ctx, msg, item, &versioned, fmt.Errorf("validation pipeline returned non-terminal status %q", status))
		return
	}

	record := versioned.Record
	record.Status = status
	record.Error = errMsg
	record.ValidatedAt = validatedAtText
	record.ValidationStages = validationResult.Stages
	if w.contentRetention > 0 {
		record.ContentExpiresAt = validatedAt.Add(w.contentRetention).UTC().Format(time.RFC3339)
	}
	record.ResultPending = true
	revision, err := w.queue.UpdateUploadRecord(ctx, record, versioned.Revision)
	if err != nil {
		latest, getErr := w.queue.GetUploadRecordVersion(ctx, id)
		if getErr == nil && (latest.Record.Status == queue.StatusValid || latest.Record.Status == queue.StatusInvalid) {
			if publishErr := w.ensureValidationResultPublished(ctx, latest); publishErr != nil {
				w.retryOperationalFailure(ctx, msg, item, &latest, fmt.Errorf("finish concurrent validation result: %w", publishErr))
				return
			}
			recordValidationOutcome("duplicate")
			_ = msg.Ack()
			return
		}
		if getErr == nil && (latest.Record.Status == queue.StatusPending || latest.Record.Status == queue.StatusEnqueuePending) {
			record = latest.Record
			record.Status = status
			record.Error = errMsg
			record.ValidatedAt = validatedAtText
			record.ValidationStages = validationResult.Stages
			if w.contentRetention > 0 {
				record.ContentExpiresAt = validatedAt.Add(w.contentRetention).UTC().Format(time.RFC3339)
			}
			record.ResultPending = true
			revision, err = w.queue.UpdateUploadRecord(ctx, record, latest.Revision)
			if err == nil {
				goto publishResult
			}
		}
		recordValidationOutcome("internal_error")
		if getErr == nil {
			w.retryOperationalFailure(ctx, msg, item, &latest, fmt.Errorf("update upload validation status: %w", err))
		} else {
			w.retryOperationalFailure(ctx, msg, item, nil, fmt.Errorf("update upload validation status: %w (reload: %v)", err, getErr))
		}
		return
	}

publishResult:
	if err := w.ensureValidationResultPublished(ctx, queue.VersionedUploadRecord{Record: record, Revision: revision}); err != nil {
		recordValidationOutcome("internal_error")
		latest, getErr := w.queue.GetUploadRecordVersion(ctx, id)
		if getErr == nil {
			w.retryOperationalFailure(ctx, msg, item, &latest, fmt.Errorf("publish validation result: %w", err))
		} else {
			w.retryOperationalFailure(ctx, msg, item, nil, fmt.Errorf("publish validation result: %w (reload: %v)", err, getErr))
		}
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

func (w *documentWorker) retryOperationalFailure(ctx context.Context, msg jetstream.Msg, item queue.DocumentWorkItem, versioned *queue.VersionedUploadRecord, processingErr error) {
	if ctx.Err() != nil {
		w.deferForShutdown(msg, item)
		return
	}
	logger := w.logger.With("id", item.ID, "documentType", item.DocumentType)
	if versioned == nil {
		metadata, metadataErr := msg.Metadata()
		if metadataErr != nil || metadata.NumDelivered < w.maxDeliver || !errors.Is(processingErr, queue.ErrRecordNotFound) {
			logger.Error("document processing failed before canonical metadata was available; will retry", "error", processingErr)
			_ = msg.NakWithDelay(nakRetryDelay)
			return
		}
		failedAt := time.Now().UTC().Format(time.RFC3339)
		record := queue.UploadRecord{ID: item.ID, DocumentType: item.DocumentType, Status: queue.StatusDeadLetterPending, Error: processingErr.Error(), LastError: processingErr.Error(), ReceivedAt: item.ReceivedAt, ProcessingAttempts: metadata.NumDelivered, ProcessingFailedAt: failedAt}
		revision, err := w.queue.CreateUploadRecord(ctx, record)
		if err != nil {
			logger.Error("failed to create canonical dead-letter transition; will retry", "error", err)
			_ = msg.NakWithDelay(nakRetryDelay)
			return
		}
		w.finalizeDeadLetter(ctx, msg, queue.VersionedUploadRecord{Record: record, Revision: revision})
		return
	}

	if versioned.Record.Status == queue.StatusDeadLetterPending {
		w.finalizeDeadLetter(ctx, msg, *versioned)
		return
	}
	attempts := versioned.Record.ProcessingAttempts + 1
	logger = logger.With("attempt", attempts)
	if (versioned.Record.Status == queue.StatusValid || versioned.Record.Status == queue.StatusInvalid) && versioned.Record.ResultPending {
		record := versioned.Record
		record.LastError = processingErr.Error()
		record.ProcessingAttempts = attempts
		if _, err := w.queue.UpdateUploadRecord(ctx, record, versioned.Revision); err != nil {
			logger.Error("failed to record result-publication retry; will retry", "error", err)
		} else {
			logger.Error("validation result is pending publication; will retry", "error", processingErr)
		}
		_ = msg.NakWithDelay(nakRetryDelay)
		return
	}

	record := versioned.Record
	record.LastError = processingErr.Error()
	record.ProcessingAttempts = attempts
	if attempts < w.maxDeliver {
		if _, err := w.queue.UpdateUploadRecord(ctx, record, versioned.Revision); err != nil {
			logger.Error("failed to record processing attempt; will retry", "error", err)
		} else {
			logger.Error("document processing failed; will retry", "error", processingErr)
		}
		_ = msg.NakWithDelay(nakRetryDelay)
		return
	}
	record.Status = queue.StatusDeadLetterPending
	record.Error = processingErr.Error()
	record.ProcessingFailedAt = time.Now().UTC().Format(time.RFC3339)
	revision, err := w.queue.UpdateUploadRecord(ctx, record, versioned.Revision)
	if err != nil {
		logger.Error("failed to persist dead-letter transition; will retry", "error", err)
		_ = msg.NakWithDelay(nakRetryDelay)
		return
	}
	w.finalizeDeadLetter(ctx, msg, queue.VersionedUploadRecord{Record: record, Revision: revision})
}

// deferForShutdown deliberately bypasses processing-attempt accounting. A
// worker lifecycle cancellation is not a document or validator failure; NAK
// leaves the descriptor available for another worker while active subprocesses
// are terminated through the same context.
func (w *documentWorker) deferForShutdown(msg jetstream.Msg, item queue.DocumentWorkItem) {
	w.logger.Info("worker shutdown interrupted document processing; leaving message for redelivery",
		"id", item.ID,
		"documentType", item.DocumentType,
	)
	_ = msg.Nak()
}

func (w *documentWorker) finalizeDeadLetter(ctx context.Context, msg jetstream.Msg, versioned queue.VersionedUploadRecord) {
	record := versioned.Record
	logger := w.logger.With("id", record.ID, "documentType", record.DocumentType, "attempt", record.ProcessingAttempts)
	transitioned, err := w.finalizeDeadLetterRecord(ctx, versioned)
	if err != nil {
		logger.Error("failed to finish pending dead letter; will retry", "error", err)
		_ = msg.NakWithDelay(nakRetryDelay)
		return
	}
	if transitioned {
		workerTerminalFailures.Inc()
		recordValidationOutcome(string(queue.StatusProcessingFailed))
		logger.Error("document exhausted processing attempts; moved to dead-letter stream", "error", record.LastError)
	}
	_ = msg.Term()
}

// finalizeDeadLetterRecord completes the durable dead-letter handoff without
// relying on a documents-stream message. The boolean reports whether this
// caller won the terminal CAS transition.
func (w *documentWorker) finalizeDeadLetterRecord(ctx context.Context, versioned queue.VersionedUploadRecord) (bool, error) {
	record := versioned.Record
	dead := queue.DeadLetter{ID: record.ID, DocumentType: record.DocumentType, ReceivedAt: record.ReceivedAt, LastError: record.LastError, Attempts: record.ProcessingAttempts, FailedAt: record.ProcessingFailedAt}
	if err := w.queue.PublishDeadLetter(ctx, dead); err != nil {
		return false, err
	}
	record.Status = queue.StatusProcessingFailed
	if _, err := w.queue.UpdateUploadRecord(ctx, record, versioned.Revision); err != nil {
		latest, getErr := w.queue.GetUploadRecordVersion(ctx, record.ID)
		if getErr == nil && latest.Record.Status == queue.StatusProcessingFailed {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (w *documentWorker) ensureValidationResultPublished(ctx context.Context, versioned queue.VersionedUploadRecord) error {
	record := versioned.Record
	if !record.ResultPending {
		return nil
	}
	if !terminalValidationStagesComplete(record) {
		published, err := w.queue.HasValidationResult(ctx, record.ID)
		if err != nil {
			return err
		}
		if !published {
			return fmt.Errorf("refuse to publish incomplete validation stages for terminal %s record", record.Status)
		}
	} else {
		if err := w.queue.PublishResult(ctx, queue.ValidationResult{
			ID: record.ID, DocumentType: record.DocumentType, Status: record.Status,
			Error: record.Error, ValidatedAt: record.ValidatedAt, ValidationStages: record.ValidationStages,
		}); err != nil {
			return err
		}
	}
	record.ResultPending = false
	if _, err := w.queue.UpdateUploadRecord(ctx, record, versioned.Revision); err != nil {
		latest, getErr := w.queue.GetUploadRecordVersion(ctx, record.ID)
		if getErr == nil && !latest.Record.ResultPending && latest.Record.Status == record.Status {
			return nil
		}
		return err
	}
	return nil
}

type workerCompletionReconcileReport struct {
	Scanned, DeadLettersFinalized, ResultsPublished, ValidationUpgradesQueued, Errors int
}

// reconcilePendingCompletions repairs durable worker states whose original
// work descriptor may have expired. Records are handled serially, and every
// mutation remains guarded by the revision loaded during this scan.
func (w *documentWorker) reconcilePendingCompletions(ctx context.Context) workerCompletionReconcileReport {
	var report workerCompletionReconcileReport
	keys, err := w.queue.UploadRecordKeys(ctx)
	if err != nil {
		report.Errors++
		return report
	}
	for _, id := range keys {
		if ctx.Err() != nil {
			break
		}
		versioned, err := w.queue.GetUploadRecordVersion(ctx, id)
		if err != nil {
			report.Errors++
			continue
		}
		record := versioned.Record
		if record.Status != queue.StatusDeadLetterPending && !((record.Status == queue.StatusValid || record.Status == queue.StatusInvalid) && record.ResultPending) {
			continue
		}
		report.Scanned++
		if record.Status == queue.StatusDeadLetterPending {
			transitioned, finalizeErr := w.finalizeDeadLetterRecord(ctx, versioned)
			if finalizeErr != nil {
				report.Errors++
				continue
			}
			if transitioned {
				report.DeadLettersFinalized++
				workerTerminalFailures.Inc()
				recordValidationOutcome(string(queue.StatusProcessingFailed))
			}
			continue
		}
		if (record.Status == queue.StatusValid || record.Status == queue.StatusInvalid) && record.ResultPending && !terminalValidationStagesComplete(record) {
			published, inspectErr := w.queue.HasValidationResult(ctx, record.ID)
			if inspectErr != nil {
				report.Errors++
				continue
			}
			if !published {
				receivedAt, parseErr := time.Parse(time.RFC3339, record.ReceivedAt)
				if parseErr != nil {
					report.Errors++
					continue
				}
				if publishErr := w.queue.PublishDocumentForValidationUpgrade(ctx, record.ID, record.DocumentType, receivedAt); publishErr != nil {
					report.Errors++
					continue
				}
				report.ValidationUpgradesQueued++
				continue
			}
			// An earlier publication succeeded but its KV acknowledgement was
			// ambiguous. Permanent result uniqueness means it is historical and
			// cannot be replaced after the required worker-generation barrier.
		}
		if err := w.ensureValidationResultPublished(ctx, versioned); err != nil {
			report.Errors++
			continue
		}
		report.ResultsPublished++
	}
	return report
}

func (w *documentWorker) runCompletionReconciler(ctx context.Context, interval time.Duration) {
	reconcile := func() {
		report := w.reconcilePendingCompletions(ctx)
		workerCompletionReconciliations.WithLabelValues("dead_letter_finalized").Add(float64(report.DeadLettersFinalized))
		workerCompletionReconciliations.WithLabelValues("result_published").Add(float64(report.ResultsPublished))
		workerCompletionReconciliations.WithLabelValues("validation_upgrade_queued").Add(float64(report.ValidationUpgradesQueued))
		workerCompletionReconciliations.WithLabelValues("error").Add(float64(report.Errors))
		if ctx.Err() == nil && (report.DeadLettersFinalized > 0 || report.ResultsPublished > 0 || report.ValidationUpgradesQueued > 0 || report.Errors > 0) {
			w.logger.Info("worker completion reconciliation finished",
				"scanned", report.Scanned,
				"deadLettersFinalized", report.DeadLettersFinalized,
				"resultsPublished", report.ResultsPublished,
				"validationUpgradesQueued", report.ValidationUpgradesQueued,
				"errors", report.Errors,
			)
		}
	}
	reconcile()
	ticker := time.NewTicker(interval)
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
