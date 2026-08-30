package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/observability"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	defaultENWorkerValidationTimeout = 30 * time.Second
	defaultENWorkerConcurrency       = 1
	defaultENWorkerPrefetch          = 1
	defaultENWorkerProgressInterval  = 20 * time.Second
	enWorkerNAKDelay                 = time.Second
)

var errENStageRequestStale = errors.New("stale EN 16931 stage request")

// EN16931WorkerConfig configures the independently deployable EN 16931 stage
// worker. Broker topology is provisioner-owned; this runtime verifies only the
// resources it uses and never mutates execution state.
type EN16931WorkerConfig struct {
	NATSURL                string
	NATSOptions            []nats.Option
	QueueNamespace         queue.Namespace
	MetricsAddr            string
	ValidationTimeout      time.Duration
	Concurrency            int
	Prefetch               int
	ProgressInterval       time.Duration
	JavaExecutable         string
	SaxonJARPath           string
	XMLResolverJARPath     string
	SaxonConfigurationPath string
	EN16931XSLTPath        string
	SaxonStartupTimeout    time.Duration
	Logger                 *slog.Logger

	queueClient   *queue.Client
	validator     EN16931Validator
	verifyRuntime func(context.Context, saxonRuntimeConfig) error
	now           func() time.Time
}

type enWorkerSettings struct {
	validationTimeout time.Duration
	concurrency       int
	prefetch          int
	progressInterval  time.Duration
}

func resolveENWorkerSettings(cfg EN16931WorkerConfig) enWorkerSettings {
	settings := enWorkerSettings{
		validationTimeout: cfg.ValidationTimeout,
		concurrency:       cfg.Concurrency,
		prefetch:          cfg.Prefetch,
		progressInterval:  cfg.ProgressInterval,
	}
	if settings.validationTimeout <= 0 {
		settings.validationTimeout = defaultENWorkerValidationTimeout
	}
	if settings.concurrency <= 0 {
		settings.concurrency = defaultENWorkerConcurrency
	}
	if settings.prefetch <= 0 {
		settings.prefetch = defaultENWorkerPrefetch
	}
	if settings.progressInterval <= 0 {
		settings.progressInterval = defaultENWorkerProgressInterval
	}
	return settings
}

func validateENWorkerCapacity(settings enWorkerSettings, delivery queue.StageDeliveryConfig) error {
	if settings.concurrency+settings.prefetch > delivery.MaxAckPending {
		return fmt.Errorf("worker-en16931 concurrency + prefetch (%d) exceeds provisioned EN 16931 request MaxAckPending (%d)", settings.concurrency+settings.prefetch, delivery.MaxAckPending)
	}
	if settings.progressInterval >= delivery.AckWait {
		return fmt.Errorf("worker-en16931 progress interval (%s) must be shorter than provisioned EN 16931 request AckWait (%s)", settings.progressInterval, delivery.AckWait)
	}
	return nil
}

// RunEN16931Worker consumes only EN 16931 StageRequests until ctx is canceled.
func RunEN16931Worker(ctx context.Context, cfg EN16931WorkerConfig) error {
	settings := resolveENWorkerSettings(cfg)
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	client := cfg.queueClient
	ownedClient := false
	var err error
	if client == nil {
		client, err = queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
		if err != nil {
			return fmt.Errorf("worker-en16931 connect to NATS: %w", err)
		}
		ownedClient = true
	}
	if ownedClient {
		defer client.Close()
	}

	requestDelivery, err := client.VerifyStageWorkerInfra(ctx, queue.ValidationLayerEN16931)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		stageStartupErrorsTotal.WithLabelValues(string(queue.ValidationLayerEN16931), "queue").Inc()
		return fmt.Errorf("worker-en16931 verify stage worker infrastructure: %w", err)
	}
	if err := validateENWorkerCapacity(settings, requestDelivery); err != nil {
		return err
	}
	if err := client.VerifyUploadContentInfra(ctx); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		stageStartupErrorsTotal.WithLabelValues(string(queue.ValidationLayerEN16931), "content_store").Inc()
		return fmt.Errorf("worker-en16931 verify upload content infrastructure: %w", err)
	}
	if err := client.VerifyValidationExecutionInfra(ctx); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		stageStartupErrorsTotal.WithLabelValues(string(queue.ValidationLayerEN16931), "execution_store").Inc()
		return fmt.Errorf("worker-en16931 verify validation execution infrastructure: %w", err)
	}

	validator := cfg.validator
	if validator == nil {
		runtime := resolveSaxonRuntimeOverrides(saxonRuntimeOverrides{
			javaExecutable: cfg.JavaExecutable, saxonJARPath: cfg.SaxonJARPath,
			xmlResolverJARPath: cfg.XMLResolverJARPath, configurationPath: cfg.SaxonConfigurationPath,
			xsltPath: cfg.EN16931XSLTPath, startupTimeout: cfg.SaxonStartupTimeout,
		})
		verifyRuntime := cfg.verifyRuntime
		if verifyRuntime == nil {
			verifyRuntime = ensureSaxonRuntime
		}
		if err := verifyRuntime(ctx, runtime); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			stageStartupErrorsTotal.WithLabelValues(string(queue.ValidationLayerEN16931), "runtime").Inc()
			return fmt.Errorf("worker-en16931 verify Saxon runtime: %w", err)
		}
		logValidationRuntimeVersions(logger)
		validator = newSaxonEN16931Validator(runtime, settings.validationTimeout)
	}
	now := cfg.now
	if now == nil {
		now = time.Now
	}

	consumer, err := client.StageRequestConsumer(ctx, queue.ValidationLayerEN16931)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("worker-en16931 open request consumer: %w", err)
	}

	var metricsServer *http.Server
	if cfg.MetricsAddr != "" {
		metricsServer = &http.Server{Addr: cfg.MetricsAddr, Handler: workerMetricsMux(client)}
		go func() {
			if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Error("worker-en16931 metrics server error", "error", err)
			}
		}()
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = metricsServer.Shutdown(shutdownCtx)
		}()
	}

	worker := &enStageWorker{
		queue: client, validator: validator, timeout: settings.validationTimeout,
		logger: logger, now: now, publishResult: client.PublishStageResult, publishDeadLetter: client.PublishStageDeadLetter, nakDelay: enWorkerNAKDelay,
	}
	localCapacity := settings.concurrency + settings.prefetch
	messages, err := consumer.Messages(jetstream.PullMaxMessages(localCapacity))
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("worker-en16931 start pulling requests: %w", err)
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

	logger.Info("worker-en16931 started",
		"queueNamespace", nonEmpty(cfg.QueueNamespace.String(), "(default)"),
		"stage", queue.ValidationLayerEN16931, "rulesetVersion", validationassets.EN16931().Version,
		"concurrency", settings.concurrency, "prefetch", settings.prefetch,
		"validationTimeout", settings.validationTimeout,
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
		message, nextErr := messages.Next()
		if nextErr != nil {
			if ctx.Err() != nil {
				break
			}
			close(pullDone)
			close(jobs)
			workers.Wait()
			return fmt.Errorf("worker-en16931 pull request: %w", nextErr)
		}
		job := newQueuedDocumentJob(message, settings.progressInterval)
		select {
		case jobs <- job:
		case <-ctx.Done():
			job.stopProgress()
			_ = message.NakWithDelay(enWorkerNAKDelay)
		}
		if ctx.Err() != nil {
			break
		}
	}
	close(pullDone)
	close(jobs)
	workers.Wait()
	logger.Info("worker-en16931 shutting down")
	return nil
}

type enStageWorker struct {
	queue             *queue.Client
	validator         EN16931Validator
	authorizeRequest  func(context.Context, queue.StageRequest) (queue.ValidationExecution, error)
	timeout           time.Duration
	logger            *slog.Logger
	now               func() time.Time
	publishResult     func(context.Context, queue.StageResult) error
	publishDeadLetter func(context.Context, queue.StageDeadLetter) error
	nakDelay          time.Duration
}

func (worker *enStageWorker) process(ctx context.Context, message jetstream.Msg, stopProgress func()) {
	if stopProgress != nil {
		defer stopProgress()
	}
	if err := worker.handle(ctx, message); err != nil {
		worker.logger.Error("worker-en16931 request failed", "error", err)
	}
}

func (worker *enStageWorker) handle(ctx context.Context, message jetstream.Msg) (err error) {
	if ctx.Err() != nil {
		worker.nak(message)
		return nil
	}
	var request queue.StageRequest
	if err := request.UnmarshalJSON(message.Data()); err != nil {
		_ = message.Term()
		return fmt.Errorf("terminate malformed EN 16931 stage request: %w", err)
	}
	if request.Stage != queue.ValidationLayerEN16931 {
		_ = message.Term()
		return fmt.Errorf("terminate non-EN request on EN 16931 consumer: stage %q", request.Stage)
	}
	handledAt := worker.now().UTC()
	ctx, processSpan := beginStageTrace(ctx, request, message, handledAt)
	defer func() { observability.End(processSpan, err) }()
	finishObservation := beginStageObservation(request, message, handledAt)
	defer finishObservation()
	authorize := worker.authorize
	if worker.authorizeRequest != nil {
		authorize = worker.authorizeRequest
	}
	execution, err := authorize(ctx, request)
	if err != nil {
		if errors.Is(err, errENStageRequestStale) {
			_ = message.Term()
			return fmt.Errorf("terminate rejected EN 16931 stage request: %w", err)
		}
		worker.nak(message)
		return fmt.Errorf("verify EN 16931 stage authorization: %w", err)
	}
	if request.Attempt >= execution.RetryPolicy.MaxAttemptsFor(request.Stage) {
		dead, found, lookupErr := worker.queue.StageDeadLetterForRequest(ctx, request)
		if lookupErr != nil {
			worker.nak(message)
			return fmt.Errorf("recover permanent EN 16931 stage dead letter: %w", lookupErr)
		}
		if found {
			result, resultErr := dead.StageResultForRequest(request)
			if resultErr != nil {
				worker.nak(message)
				return fmt.Errorf("recover permanent EN 16931 stage result: %w", resultErr)
			}
			publishedAt := time.Now()
			if err := worker.publishResult(ctx, result); err != nil {
				observeStageResultPublication(result.Stage, result.Outcome, publishedAt)
				worker.nak(message)
				return fmt.Errorf("publish recovered EN 16931 stage result %s: %w", result.ResultID, err)
			}
			observeStageResultPublication(result.Stage, result.Outcome, publishedAt)
			recordSplitStagePublished(result, execution.RetryPolicy.MaxAttemptsFor(request.Stage))
			logSplitStageResult(ctx, worker.logger, result, true)
			if err := message.Ack(); err != nil {
				return fmt.Errorf("ack EN 16931 stage request %s: %w", request.RequestID, err)
			}
			return nil
		}
	}

	startedAt := worker.now().UTC()
	attemptCtx, cancelAttempt := context.WithTimeout(ctx, worker.timeout)
	defer cancelAttempt()
	outcome := queue.StageOutcomePassed
	reason := queue.StageReasonNone
	var issues []queue.ValidationIssue
	diagnostic := ""

	if request.RulesetVersion != validationassets.EN16931().Version {
		outcome = queue.StageOutcomeOperationalFailure
		reason = queue.StageReasonRuntimeUnavailable
		diagnostic = fmt.Sprintf("worker-en16931 supports ruleset %s, request requires %s", validationassets.EN16931().Version, request.RulesetVersion)
	} else if !requiresEN16931(request.DocumentType) {
		outcome = queue.StageOutcomeOperationalFailure
		reason = queue.StageReasonRuntimeUnavailable
		diagnostic = fmt.Sprintf("worker-en16931 does not support document type %s", request.DocumentType)
	} else {
		var validationResult queue.ValidationStageResult
		validationErr := worker.withContent(attemptCtx, request.UploadID, func(reader io.Reader) error {
			validationCtx, validationSpan := beginStageOperation(attemptCtx, "stage.validator.execute", request)
			var err error
			validationResult, err = worker.validator.Validate(validationCtx, reader)
			observability.End(validationSpan, err)
			return err
		})
		if validationErr != nil {
			if ctx.Err() != nil {
				worker.nak(message)
				return nil
			}
			outcome = queue.StageOutcomeOperationalFailure
			reason = enStageOperationalReason(validationErr)
			diagnostic = boundedStageText(validationErr.Error(), queue.MaxStageDiagnosticBytes)
		} else if validationResult.Layer != queue.ValidationLayerEN16931 || validationResult.Version != request.RulesetVersion {
			outcome = queue.StageOutcomeOperationalFailure
			reason = queue.StageReasonMalformedOutput
			diagnostic = boundedStageText(fmt.Sprintf("EN 16931 validator returned layer %q version %q", validationResult.Layer, validationResult.Version), queue.MaxStageDiagnosticBytes)
		} else if err := queue.ValidateStageIssues(queue.ValidationLayerEN16931, validationResult.Issues); err != nil {
			outcome = queue.StageOutcomeOperationalFailure
			reason = queue.StageReasonMalformedOutput
			diagnostic = boundedStageText("EN 16931 validator returned malformed normalized issue evidence: "+err.Error(), queue.MaxStageDiagnosticBytes)
		} else if validationResult.Valid == queue.StageIssuesContainStandardsFailure(validationResult.Issues) {
			outcome = queue.StageOutcomeOperationalFailure
			reason = queue.StageReasonMalformedOutput
			diagnostic = "EN 16931 validator validity does not match its failed-assert evidence"
		} else {
			issues = append([]queue.ValidationIssue(nil), validationResult.Issues...)
			if !validationResult.Valid {
				outcome = queue.StageOutcomeStandardsInvalid
			}
		}
	}

	completedAt := worker.now().UTC()
	duration := completedAt.Sub(startedAt)
	if duration < 0 {
		duration = 0
	}
	result, err := queue.NewStageResult(request, outcome, reason, issues, diagnostic, startedAt, completedAt, duration)
	if err != nil {
		worker.nak(message)
		return fmt.Errorf("build EN 16931 stage result: %w", err)
	}
	recordSplitStageExecution(result)
	if outcome == queue.StageOutcomeOperationalFailure && request.Attempt >= execution.RetryPolicy.MaxAttemptsFor(request.Stage) {
		metadata, metadataErr := message.Metadata()
		if metadataErr != nil {
			worker.nak(message)
			return fmt.Errorf("read EN 16931 stage request delivery count: %w", metadataErr)
		}
		dead, deadErr := queue.NewStageDeadLetter(result, metadata.NumDelivered, completedAt)
		if deadErr != nil {
			worker.nak(message)
			return fmt.Errorf("build EN 16931 stage dead letter: %w", deadErr)
		}
		publishDeadLetter := worker.publishDeadLetter
		if publishDeadLetter == nil && worker.queue != nil {
			publishDeadLetter = worker.queue.PublishStageDeadLetter
		}
		if publishDeadLetter == nil {
			worker.nak(message)
			return errors.New("publish EN 16931 stage dead letter: publisher is required")
		}
		if err := publishDeadLetter(ctx, dead); err != nil {
			worker.nak(message)
			return fmt.Errorf("publish EN 16931 stage dead letter %s: %w", dead.DeadLetterID, err)
		}
	}
	publishedAt := time.Now()
	if err := worker.publishResult(ctx, result); err != nil {
		observeStageResultPublication(result.Stage, result.Outcome, publishedAt)
		worker.nak(message)
		return fmt.Errorf("publish EN 16931 stage result %s: %w", result.ResultID, err)
	}
	observeStageResultPublication(result.Stage, result.Outcome, publishedAt)
	recordSplitStagePublished(result, execution.RetryPolicy.MaxAttemptsFor(request.Stage))
	logSplitStageResult(ctx, worker.logger, result, false)
	if err := message.Ack(); err != nil {
		return fmt.Errorf("ack EN 16931 stage request %s: %w", request.RequestID, err)
	}
	return nil
}

func (worker *enStageWorker) authorize(ctx context.Context, request queue.StageRequest) (queue.ValidationExecution, error) {
	execution, err := worker.queue.GetValidationExecution(ctx, request.UploadID, request.ValidationID, request.Generation)
	if err != nil {
		if errors.Is(err, queue.ErrValidationExecutionNotFound) {
			return queue.ValidationExecution{}, fmt.Errorf("%w: execution generation does not exist", errENStageRequestStale)
		}
		return queue.ValidationExecution{}, err
	}
	if request.DocumentType != execution.DocumentType || request.Plan != execution.Plan.ID || request.PlanVersion != execution.Plan.Version ||
		request.StageOrdinal == 0 || int(request.StageOrdinal) > len(execution.Plan.Stages) {
		return queue.ValidationExecution{}, fmt.Errorf("%w: request identity does not match durable execution", errENStageRequestStale)
	}
	planStage := execution.Plan.Stages[request.StageOrdinal-1]
	if planStage.ID != request.Stage || planStage.RulesetVersion != request.RulesetVersion {
		return queue.ValidationExecution{}, fmt.Errorf("%w: request does not match frozen plan stage", errENStageRequestStale)
	}
	state := execution.Orchestration
	if state.Status != queue.ExecutionStatusStageQueued || state.CurrentStageOrdinal != request.StageOrdinal || len(state.Stages) != int(request.StageOrdinal) {
		return queue.ValidationExecution{}, fmt.Errorf("%w: request is not the current queued stage", errENStageRequestStale)
	}
	current := state.Stages[request.StageOrdinal-1]
	if current.Stage != request.Stage || current.AuthorizedAttempt != request.Attempt || current.RequestID != request.RequestID {
		return queue.ValidationExecution{}, fmt.Errorf("%w: request is not the currently authorized attempt", errENStageRequestStale)
	}

	xsdPassed := false
	for index := 0; index < int(request.StageOrdinal)-1; index++ {
		prerequisite := state.Stages[index]
		if prerequisite.TerminalOutcome != queue.StageOutcomePassed || (len(prerequisite.Attempts) == 0 && prerequisite.InheritedEvidence == nil) {
			return queue.ValidationExecution{}, fmt.Errorf("%w: prerequisite stage %d has no passing terminal outcome", errENStageRequestStale, index+1)
		}
		if prerequisite.Stage == queue.ValidationLayerUBL21XSD {
			xsdPassed = true
		}
		var evidence queue.StageResult
		if prerequisite.InheritedEvidence != nil {
			reference := prerequisite.InheritedEvidence
			evidence, err = worker.queue.GetValidationStageEvidence(ctx, request.UploadID, reference.ValidationID, reference.Generation, reference.StageOrdinal, reference.Attempt)
		} else {
			accepted := prerequisite.Attempts[len(prerequisite.Attempts)-1]
			evidence, err = worker.queue.GetValidationStageEvidence(ctx, request.UploadID, request.ValidationID, request.Generation, prerequisite.Ordinal, accepted.Attempt)
		}
		if err != nil {
			return queue.ValidationExecution{}, fmt.Errorf("load prerequisite stage %d evidence: %w", prerequisite.Ordinal, err)
		}
		if err := evidence.Validate(); err != nil {
			return queue.ValidationExecution{}, fmt.Errorf("validate prerequisite stage %d evidence: %w", prerequisite.Ordinal, err)
		}
		if evidence.Outcome != queue.StageOutcomePassed || evidence.Stage != prerequisite.Stage || evidence.StageOrdinal != prerequisite.Ordinal || evidence.RulesetVersion != execution.Plan.Stages[index].RulesetVersion {
			return queue.ValidationExecution{}, fmt.Errorf("%w: prerequisite stage %d evidence does not match execution state", errENStageRequestStale, prerequisite.Ordinal)
		}
		if prerequisite.InheritedEvidence != nil {
			reference := prerequisite.InheritedEvidence
			if evidence.ValidationID != reference.ValidationID || evidence.Generation != reference.Generation || evidence.Attempt != reference.Attempt || evidence.RequestID != reference.RequestID || evidence.ResultID != reference.ResultID {
				return queue.ValidationExecution{}, fmt.Errorf("%w: inherited prerequisite stage %d evidence does not match reference", errENStageRequestStale, prerequisite.Ordinal)
			}
		} else {
			accepted := prerequisite.Attempts[len(prerequisite.Attempts)-1]
			if evidence.ResultID != accepted.ResultID || evidence.RequestID != accepted.RequestID || evidence.Attempt != accepted.Attempt {
				return queue.ValidationExecution{}, fmt.Errorf("%w: prerequisite stage %d evidence does not match accepted attempt", errENStageRequestStale, prerequisite.Ordinal)
			}
		}
	}
	if !xsdPassed {
		return queue.ValidationExecution{}, fmt.Errorf("%w: no passing UBL 2.1 XSD prerequisite evidence", errENStageRequestStale)
	}
	return execution, nil
}

func (worker *enStageWorker) withContent(ctx context.Context, uploadID string, validate func(io.Reader) error) error {
	fetchCtx, fetchSpan := observability.Tracer().Start(ctx, "stage.xml.fetch", trace.WithAttributes(attribute.String("upload.id", uploadID)))
	content, err := worker.queue.OpenUploadContent(fetchCtx, uploadID)
	observability.End(fetchSpan, err)
	if err != nil {
		return fmt.Errorf("%w: open upload content: %w", errValidationContentStream, err)
	}
	validationErr := validate(content)
	streamErr := content.Error()
	closeErr := content.Close()
	if streamErr == nil {
		streamErr = closeErr
	}
	if streamErr != nil {
		contentErr := fmt.Errorf("%w: stream upload content: %w", errValidationContentStream, streamErr)
		if validationErr != nil {
			return errors.Join(validationErr, contentErr)
		}
		return contentErr
	}
	return validationErr
}

func (worker *enStageWorker) nak(message jetstream.Msg) {
	delay := worker.nakDelay
	if delay <= 0 {
		delay = enWorkerNAKDelay
	}
	_ = message.NakWithDelay(delay)
}

func enStageOperationalReason(err error) queue.StageOperationalReason {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return queue.StageReasonTimeout
	case errors.Is(err, context.Canceled):
		return queue.StageReasonCancelled
	case errors.Is(err, errValidationContentStream), errors.Is(err, errEN16931RuntimeInput), errors.Is(err, queue.ErrRecordNotFound):
		return queue.StageReasonContentUnavailable
	case errors.Is(err, errEN16931RuntimeAsset):
		return queue.StageReasonRuntimeUnavailable
	case errors.Is(err, errEN16931SVRLOutputLimit):
		return queue.StageReasonResourceLimit
	case errors.Is(err, errEN16931RuntimeSVRL):
		return queue.StageReasonMalformedOutput
	default:
		return queue.StageReasonRuntimeFailure
	}
}
