package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/peppol"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

var errPeppolEnvelopeStageRequestStale = errors.New("stale Peppol envelope stage request")

type PeppolEnvelopeWorkerConfig struct {
	NATSURL               string
	NATSOptions           []nats.Option
	QueueNamespace        queue.Namespace
	MetricsAddr           string
	ValidationTimeout     time.Duration
	Concurrency, Prefetch int
	ProgressInterval      time.Duration
	Logger                *slog.Logger
	queueClient           *queue.Client
	validator             semanticStageValidator
	now                   func() time.Time
	onStarted             func()
}

// RunPeppolEnvelopeWorker consumes only the independently versioned fourth
// stage. It does not run or reinterpret payload validation.
func RunPeppolEnvelopeWorker(ctx context.Context, cfg PeppolEnvelopeWorkerConfig) error {
	timeout := cfg.ValidationTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	prefetch := cfg.Prefetch
	if prefetch <= 0 {
		prefetch = 1
	}
	progress := cfg.ProgressInterval
	if progress <= 0 {
		progress = 20 * time.Second
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	client, owned := cfg.queueClient, false
	var err error
	if client == nil {
		client, err = queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
		if err != nil {
			return fmt.Errorf("worker-peppol-envelope connect to NATS: %w", err)
		}
		owned = true
		defer client.Close()
		if err := client.RequireSplitDeployment(ctx); err != nil {
			return fmt.Errorf("worker-peppol-envelope deployment barrier: %w", err)
		}
	}
	delivery, err := client.VerifyStageWorkerInfra(ctx, queue.ValidationLayerPeppolEnvelope)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("worker-peppol-envelope verify stage infrastructure: %w", err)
	}
	if concurrency+prefetch > delivery.MaxAckPending || progress >= delivery.AckWait || timeout >= delivery.AckWait {
		return fmt.Errorf("worker-peppol-envelope capacity or timeout exceeds provisioned delivery limits")
	}
	if err := client.VerifyUploadContentInfra(ctx); err != nil {
		return fmt.Errorf("worker-peppol-envelope verify content store: %w", err)
	}
	if err := client.VerifyValidationExecutionInfra(ctx); err != nil {
		return fmt.Errorf("worker-peppol-envelope verify execution store: %w", err)
	}
	if err := validationassets.VerifyPeppolEnvelope(); err != nil {
		return fmt.Errorf("worker-peppol-envelope verify assets: %w", err)
	}
	consumer, err := client.StageRequestConsumer(ctx, queue.ValidationLayerPeppolEnvelope)
	if err != nil {
		return fmt.Errorf("worker-peppol-envelope open request consumer: %w", err)
	}
	validator := cfg.validator
	if validator == nil {
		validator = newPeppolEnvelopeValidator()
	}
	now := cfg.now
	if now == nil {
		now = time.Now
	}
	worker := &enStageWorker{
		queue: client, validator: validator, layer: queue.ValidationLayerPeppolEnvelope,
		rulesetVersion: validationassets.PeppolEnvelope().Version, workerName: "worker-peppol-envelope",
		supportsDocument: requiresEN16931, staleError: errPeppolEnvelopeStageRequestStale,
		operationalReason: peppolEnvelopeOperationalReason, timeout: timeout, logger: logger, now: now,
		publishResult: client.PublishStageResult, publishDeadLetter: client.PublishStageDeadLetter, nakDelay: time.Second,
	}
	var metricsServer *http.Server
	if cfg.MetricsAddr != "" {
		metricsServer = &http.Server{Addr: cfg.MetricsAddr, Handler: workerMetricsMux(client)}
		go func() {
			if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Error("worker-peppol-envelope metrics server error", "error", err)
			}
		}()
		defer func() {
			shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = metricsServer.Shutdown(shutdown)
		}()
	}
	messages, err := consumer.Messages(jetstream.PullMaxMessages(concurrency + prefetch))
	if err != nil {
		return fmt.Errorf("worker-peppol-envelope start pulling requests: %w", err)
	}
	defer messages.Stop()
	jobs := make(chan queuedDocumentJob, prefetch)
	var workers sync.WaitGroup
	for range concurrency {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for job := range jobs {
				worker.process(ctx, job.msg, job.stopProgress)
			}
		}()
	}
	if cfg.onStarted != nil {
		cfg.onStarted()
	}
	logger.Info("worker-peppol-envelope started", "stage", queue.ValidationLayerPeppolEnvelope, "rulesetVersion", validationassets.PeppolEnvelope().Version)
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			messages.Stop()
		case <-done:
		}
	}()
	for {
		message, nextErr := messages.Next()
		if nextErr != nil {
			if ctx.Err() != nil {
				break
			}
			close(done)
			close(jobs)
			workers.Wait()
			return fmt.Errorf("worker-peppol-envelope pull request: %w", nextErr)
		}
		job := newQueuedDocumentJob(message, progress)
		select {
		case jobs <- job:
		case <-ctx.Done():
			job.stopProgress()
			_ = message.NakWithDelay(time.Second)
		}
		if ctx.Err() != nil {
			break
		}
	}
	close(done)
	close(jobs)
	workers.Wait()
	if owned {
		logger.Info("worker-peppol-envelope shutting down")
	}
	return nil
}

func peppolEnvelopeOperationalReason(err error) queue.StageOperationalReason {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return queue.StageReasonTimeout
	case errors.Is(err, context.Canceled):
		return queue.StageReasonCancelled
	case errors.Is(err, errValidationContentStream), errors.Is(err, queue.ErrRecordNotFound):
		return queue.StageReasonContentUnavailable
	case errors.Is(err, peppol.ErrDocumentTooLarge):
		return queue.StageReasonResourceLimit
	case errors.Is(err, peppol.ErrMalformedEnvelope):
		return queue.StageReasonMalformedOutput
	default:
		return queue.StageReasonRuntimeFailure
	}
}
