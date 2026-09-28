package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/documentstore"
	peppolprofile "github.com/PaddyFrenchman/e-ubl21/peppol"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	defaultPeppolWorkerConcurrency      = 1
	defaultPeppolWorkerPrefetch         = 1
	defaultPeppolWorkerProgressInterval = 20 * time.Second
	peppolWorkerNAKDelay                = time.Second
)

var errPeppolStageRequestStale = errors.New("stale Peppol stage request")

// PeppolWorkerConfig configures the independently deployable third-stage
// worker. It consumes only Peppol requests and never invokes another stage.
type PeppolWorkerConfig struct {
	// Stage defaults to payload Peppol. The envelope worker reuses the durable
	// delivery loop with its separately provisioned stage.
	Stage                  queue.ValidationLayer
	RulesetVersion         string
	NATSURL                string
	NATSOptions            []nats.Option
	QueueNamespace         queue.Namespace
	ObjectStore            documentstore.Options
	MetricsAddr            string
	ValidationTimeout      time.Duration
	Concurrency            int
	Prefetch               int
	ProgressInterval       time.Duration
	JavaExecutable         string
	SaxonJARPath           string
	XMLResolverJARPath     string
	SaxonConfigurationPath string
	PeppolXSLTPath         string
	SaxonStartupTimeout    time.Duration
	Logger                 *slog.Logger

	queueClient   *queue.Client
	validator     PeppolValidator
	verifyRuntime func(context.Context, saxonRuntimeConfig) error
	now           func() time.Time
	onStarted     func()
}

type peppolWorkerSettings struct {
	validationTimeout time.Duration
	concurrency       int
	prefetch          int
	progressInterval  time.Duration
}

func resolvePeppolWorkerSettings(cfg PeppolWorkerConfig) peppolWorkerSettings {
	settings := peppolWorkerSettings{
		validationTimeout: cfg.ValidationTimeout,
		concurrency:       cfg.Concurrency,
		prefetch:          cfg.Prefetch,
		progressInterval:  cfg.ProgressInterval,
	}
	if settings.validationTimeout <= 0 {
		settings.validationTimeout = defaultPeppolValidationTimeout
	}
	if settings.concurrency <= 0 {
		settings.concurrency = defaultPeppolWorkerConcurrency
	}
	if settings.prefetch <= 0 {
		settings.prefetch = defaultPeppolWorkerPrefetch
	}
	if settings.progressInterval <= 0 {
		settings.progressInterval = defaultPeppolWorkerProgressInterval
	}
	return settings
}

func validatePeppolWorkerCapacity(settings peppolWorkerSettings, delivery queue.StageDeliveryConfig) error {
	if settings.concurrency+settings.prefetch > delivery.MaxAckPending {
		return fmt.Errorf("worker-peppol concurrency + prefetch (%d) exceeds provisioned Peppol request MaxAckPending (%d)", settings.concurrency+settings.prefetch, delivery.MaxAckPending)
	}
	if settings.progressInterval >= delivery.AckWait {
		return fmt.Errorf("worker-peppol progress interval (%s) must be shorter than provisioned Peppol request AckWait (%s)", settings.progressInterval, delivery.AckWait)
	}
	if settings.validationTimeout >= delivery.AckWait {
		return fmt.Errorf("worker-peppol validation timeout (%s) must be shorter than provisioned Peppol request AckWait (%s)", settings.validationTimeout, delivery.AckWait)
	}
	return nil
}

// RunPeppolWorker consumes Peppol StageRequests until ctx is canceled.
func RunPeppolWorker(ctx context.Context, cfg PeppolWorkerConfig) error {
	stage := cfg.Stage
	if stage == "" {
		stage = queue.ValidationLayerPeppolBISBilling
	}
	if stage != queue.ValidationLayerPeppolBISBilling && stage != queue.ValidationLayerPeppolEnvelope {
		return fmt.Errorf("worker-peppol: unsupported stage %q", stage)
	}
	rulesetVersion := cfg.RulesetVersion
	if rulesetVersion == "" {
		rulesetVersion = validationassets.Peppol().ValidationArtefactsVersion
		if stage == queue.ValidationLayerPeppolEnvelope {
			rulesetVersion = validationassets.PeppolEnvelopeVersion
		}
	}
	settings := resolvePeppolWorkerSettings(cfg)
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	validator := cfg.validator
	wrapProfileResolution := validator == nil
	if validator == nil {
		runtimeOverrides := saxonRuntimeOverrides{
			javaExecutable: cfg.JavaExecutable, saxonJARPath: cfg.SaxonJARPath,
			xmlResolverJARPath: cfg.XMLResolverJARPath, configurationPath: cfg.SaxonConfigurationPath,
			xsltPath: cfg.PeppolXSLTPath, startupTimeout: cfg.SaxonStartupTimeout,
		}
		runtime := resolvePeppolSaxonRuntimeOverrides(runtimeOverrides)
		if stage == queue.ValidationLayerPeppolEnvelope {
			runtime = resolvePeppolEnvelopeSaxonRuntimeOverrides(runtimeOverrides)
		}
		verifyRuntime := cfg.verifyRuntime
		if verifyRuntime == nil {
			verifyRuntime = ensurePeppolSaxonRuntime
			if stage == queue.ValidationLayerPeppolEnvelope {
				verifyRuntime = ensurePeppolEnvelopeSaxonRuntime
			}
		}
		if err := verifyRuntime(ctx, runtime); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			stageStartupErrorsTotal.WithLabelValues(string(stage), "runtime").Inc()
			return fmt.Errorf("worker-peppol verify Saxon runtime: %w", err)
		}
		logPeppolValidationRuntimeVersions(logger)
		if stage == queue.ValidationLayerPeppolEnvelope {
			validator = newSaxonPeppolEnvelopeValidator(runtime, settings.validationTimeout)
		} else {
			validator = newSaxonPeppolValidator(runtime, settings.validationTimeout)
		}
	}
	if wrapProfileResolution && stage == queue.ValidationLayerPeppolBISBilling {
		validator = peppolProfileAwareValidator{delegate: validator}
	}
	client := cfg.queueClient
	ownedClient := false
	var err error
	if client == nil {
		client, err = queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
		if err != nil {
			return fmt.Errorf("worker-peppol connect to NATS: %w", err)
		}
		ownedClient = true
	}
	if cfg.ObjectStore.Backend != "" {
		store, storeErr := documentstore.New(cfg.ObjectStore)
		if storeErr != nil {
			if ownedClient {
				client.Close()
			}
			return fmt.Errorf("worker-peppol open document store: %w", storeErr)
		}
		client.SetDocumentStore(store)
	}
	if ownedClient {
		defer client.Close()
		if err := client.RequireSplitDeployment(ctx); err != nil {
			return fmt.Errorf("worker-peppol deployment barrier: %w", err)
		}
	}

	delivery, err := client.VerifyStageWorkerInfra(ctx, stage)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		stageStartupErrorsTotal.WithLabelValues(string(stage), "queue").Inc()
		return fmt.Errorf("worker-peppol verify stage worker infrastructure: %w", err)
	}
	if err := validatePeppolWorkerCapacity(settings, delivery); err != nil {
		return err
	}
	if err := client.VerifyUploadContentInfra(ctx); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		stageStartupErrorsTotal.WithLabelValues(string(stage), "content_store").Inc()
		return fmt.Errorf("worker-peppol verify upload content infrastructure: %w", err)
	}
	if err := client.VerifyValidationExecutionInfra(ctx); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		stageStartupErrorsTotal.WithLabelValues(string(stage), "execution_store").Inc()
		return fmt.Errorf("worker-peppol verify validation execution infrastructure: %w", err)
	}

	now := cfg.now
	if now == nil {
		now = time.Now
	}
	consumer, err := client.StageRequestConsumer(ctx, stage)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("worker-peppol open request consumer: %w", err)
	}

	var metricsServer *http.Server
	if cfg.MetricsAddr != "" {
		metricsServer = &http.Server{Addr: cfg.MetricsAddr, Handler: workerMetricsMux(client, nil)}
		go func() {
			if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Error("worker-peppol metrics server error", "error", err)
			}
		}()
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = metricsServer.Shutdown(shutdownCtx)
		}()
	}

	worker := &enStageWorker{
		queue: client, validator: validator,
		layer:          stage,
		rulesetVersion: rulesetVersion,
		workerName:     "worker-peppol", supportsDocument: requiresEN16931,
		operationalReason: peppolStageOperationalReason, staleError: errPeppolStageRequestStale,
		timeout: settings.validationTimeout, logger: logger, now: now,
		publishResult: client.PublishStageResult, publishDeadLetter: client.PublishStageDeadLetter,
		nakDelay: peppolWorkerNAKDelay,
	}
	localCapacity := settings.concurrency + settings.prefetch
	messages, err := consumer.Messages(jetstream.PullMaxMessages(localCapacity))
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("worker-peppol start pulling requests: %w", err)
	}
	defer messages.Stop()
	jobs := make(chan queuedDocumentJob, settings.prefetch)
	var workers sync.WaitGroup
	for range settings.concurrency {
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
	logger.Info("worker-peppol started",
		"queueNamespace", nonEmpty(cfg.QueueNamespace.String(), "(default)"),
		"stage", stage,
		"rulesetVersion", rulesetVersion,
		"bisVersion", validationassets.Peppol().Version,
		"concurrency", settings.concurrency, "prefetch", settings.prefetch,
		"validationTimeout", settings.validationTimeout)
	pullDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			messages.Stop()
		case <-pullDone:
		}
	}()
	for {
		message, nextErr := messages.Next()
		if nextErr != nil {
			if ctx.Err() != nil {
				break
			}
			close(pullDone)
			close(jobs)
			workers.Wait()
			return fmt.Errorf("worker-peppol pull request: %w", nextErr)
		}
		job := newQueuedDocumentJob(message, settings.progressInterval)
		select {
		case jobs <- job:
		case <-ctx.Done():
			job.stopProgress()
			_ = message.NakWithDelay(peppolWorkerNAKDelay)
		}
		if ctx.Err() != nil {
			break
		}
	}
	close(pullDone)
	close(jobs)
	workers.Wait()
	logger.Info("worker-peppol shutting down")
	return nil
}

// peppolProfileAwareValidator adds the product-level explicit-profile failures
// frozen by the public contract without manufacturing official Schematron rule
// IDs. The delegate still streams the input to Saxon; the bounded tee is used
// only to resolve the same top-level identifiers after that run.
type peppolProfileAwareValidator struct {
	delegate PeppolValidator
}

func (validator peppolProfileAwareValidator) Validate(ctx context.Context, input io.Reader) (queue.ValidationStageResult, error) {
	var document bytes.Buffer
	result, err := validator.delegate.Validate(ctx, io.TeeReader(io.LimitReader(input, peppolprofile.MaxResolutionDocumentBytes+1), &document))
	if document.Len() > peppolprofile.MaxResolutionDocumentBytes {
		return queue.ValidationStageResult{}, peppolprofile.ErrDocumentTooLarge
	}
	if err != nil {
		return queue.ValidationStageResult{}, err
	}
	documentType, _, err := rootElementOfPayload(document.Bytes())
	if err != nil {
		return queue.ValidationStageResult{}, fmt.Errorf("resolve Peppol validation profile: %w", err)
	}
	resolution, err := peppolprofile.Resolve(documentType, queue.ValidationProfilePeppolBISBilling, document.Bytes())
	if err != nil {
		return queue.ValidationStageResult{}, fmt.Errorf("resolve Peppol validation profile: %w", err)
	}
	for _, failure := range resolution.Failures {
		result.Issues = append(result.Issues, queue.ValidationIssue{
			Kind: queue.ValidationIssueFailedAssert, Layer: queue.ValidationLayerPeppolBISBilling,
			Rule: string(failure.Code), Severity: "error", Message: failure.Message,
		})
	}
	if err := queue.ValidateStageIssues(queue.ValidationLayerPeppolBISBilling, result.Issues); err != nil {
		if errors.Is(err, queue.ErrStageIssueLimitExceeded) {
			return queue.ValidationStageResult{}, fmt.Errorf("%w: normalized Peppol evidence: %w", errPeppolSVRLOutputLimit, err)
		}
		return queue.ValidationStageResult{}, fmt.Errorf("%w: %w", errPeppolMalformedEvidence, err)
	}
	result.Valid = !queue.StageIssuesContainStandardsFailure(result.Issues)
	return result, nil
}

func peppolStageOperationalReason(err error) queue.StageOperationalReason {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return queue.StageReasonTimeout
	case errors.Is(err, context.Canceled):
		return queue.StageReasonCancelled
	case errors.Is(err, errValidationContentStream), errors.Is(err, errPeppolRuntimeInput), errors.Is(err, queue.ErrRecordNotFound):
		return queue.StageReasonContentUnavailable
	case errors.Is(err, errPeppolRuntimeAsset):
		return queue.StageReasonRuntimeUnavailable
	case errors.Is(err, errPeppolSVRLOutputLimit), errors.Is(err, peppolprofile.ErrDocumentTooLarge):
		return queue.StageReasonResourceLimit
	case errors.Is(err, errPeppolMalformedEvidence):
		return queue.StageReasonMalformedOutput
	case errors.Is(err, errPeppolRuntimeSVRL):
		return queue.StageReasonMalformedOutput
	default:
		return queue.StageReasonRuntimeFailure
	}
}
