package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/documentstore"
	"unicode/utf8"

	xsdschemas "github.com/PaddyFrenchman/e-ubl21/XSD"
	"github.com/PaddyFrenchman/e-ubl21/observability"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	defaultXSDWorkerValidationTimeout = 30 * time.Second
	defaultXSDWorkerConcurrency       = 1
	defaultXSDWorkerPrefetch          = 1
	defaultXSDWorkerProgressInterval  = 20 * time.Second
	xsdWorkerNAKDelay                 = time.Second
)

// XSDWorkerConfig configures the independently deployable UBL 2.1 XSD stage
// worker. Broker topology is provisioner-owned; this runtime only verifies and
// consumes the existing XSD resources.
type XSDWorkerConfig struct {
	NATSURL           string
	NATSOptions       []nats.Option
	QueueNamespace    queue.Namespace
	ObjectStore       documentstore.Options
	MetricsAddr       string
	ValidationTimeout time.Duration
	Concurrency       int
	Prefetch          int
	ProgressInterval  time.Duration
	Logger            *slog.Logger

	// Test seams remain unexported so production always uses the embedded UBL
	// catalog and hardened xmllint adapter.
	queueClient *queue.Client
	catalog     *validationCatalog
	xsdValidate xsdStreamValidator
	now         func() time.Time
	onStarted   func()
}

type xsdWorkerSettings struct {
	validationTimeout time.Duration
	concurrency       int
	prefetch          int
	progressInterval  time.Duration
}

func resolveXSDWorkerSettings(cfg XSDWorkerConfig) (xsdWorkerSettings, error) {
	settings := xsdWorkerSettings{
		validationTimeout: cfg.ValidationTimeout,
		concurrency:       cfg.Concurrency,
		prefetch:          cfg.Prefetch,
		progressInterval:  cfg.ProgressInterval,
	}
	if settings.validationTimeout <= 0 {
		settings.validationTimeout = defaultXSDWorkerValidationTimeout
	}
	if settings.concurrency <= 0 {
		settings.concurrency = defaultXSDWorkerConcurrency
	}
	if settings.prefetch <= 0 {
		settings.prefetch = defaultXSDWorkerPrefetch
	}
	if settings.progressInterval <= 0 {
		settings.progressInterval = defaultXSDWorkerProgressInterval
	}
	return settings, nil
}

func validateXSDWorkerCapacity(settings xsdWorkerSettings, delivery queue.StageDeliveryConfig) error {
	if settings.concurrency+settings.prefetch > delivery.MaxAckPending {
		return fmt.Errorf("worker-xsd concurrency + prefetch (%d) exceeds provisioned XSD request MaxAckPending (%d)", settings.concurrency+settings.prefetch, delivery.MaxAckPending)
	}
	if settings.progressInterval >= delivery.AckWait {
		return fmt.Errorf("worker-xsd progress interval (%s) must be shorter than provisioned XSD request AckWait (%s)", settings.progressInterval, delivery.AckWait)
	}
	return nil
}

// RunXSDWorker consumes only UBL 2.1 XSD StageRequests until ctx is canceled.
func RunXSDWorker(ctx context.Context, cfg XSDWorkerConfig) error {
	settings, err := resolveXSDWorkerSettings(cfg)
	if err != nil {
		return err
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	client := cfg.queueClient
	ownedClient := false
	if client == nil {
		client, err = queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
		if err != nil {
			return fmt.Errorf("worker-xsd connect to NATS: %w", err)
		}
		ownedClient = true
	}
	if cfg.ObjectStore.Backend != "" {
		store, storeErr := documentstore.New(cfg.ObjectStore)
		if storeErr != nil {
			if ownedClient {
				client.Close()
			}
			return fmt.Errorf("worker-xsd open document store: %w", storeErr)
		}
		client.SetDocumentStore(store)
	}
	if ownedClient {
		defer client.Close()
		if err := client.RequireSplitDeployment(ctx); err != nil {
			return fmt.Errorf("worker-xsd deployment barrier: %w", err)
		}
	}

	requestDelivery, err := client.VerifyStageWorkerInfra(ctx, queue.ValidationLayerUBL21XSD)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		stageStartupErrorsTotal.WithLabelValues(string(queue.ValidationLayerUBL21XSD), "queue").Inc()
		return fmt.Errorf("worker-xsd verify XSD worker infrastructure: %w", err)
	}
	if err := validateXSDWorkerCapacity(settings, requestDelivery); err != nil {
		return err
	}
	if err := client.VerifyUploadContentInfra(ctx); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		stageStartupErrorsTotal.WithLabelValues(string(queue.ValidationLayerUBL21XSD), "content_store").Inc()
		return fmt.Errorf("worker-xsd verify upload content infrastructure: %w", err)
	}

	catalog := cfg.catalog
	if catalog == nil {
		catalog, err = loadValidationCatalog()
		if err != nil {
			stageStartupErrorsTotal.WithLabelValues(string(queue.ValidationLayerUBL21XSD), "assets").Inc()
			return fmt.Errorf("worker-xsd load UBL 2.1 schema catalog: %w", err)
		}
	}
	validate := cfg.xsdValidate
	if validate == nil {
		if err := ensureXMLLint(); err != nil {
			stageStartupErrorsTotal.WithLabelValues(string(queue.ValidationLayerUBL21XSD), "runtime").Inc()
			return fmt.Errorf("worker-xsd verify xmllint runtime: %w", err)
		}
		validate = validateReaderAgainstSchema
	}
	now := cfg.now
	if now == nil {
		now = time.Now
	}

	consumer, err := client.StageRequestConsumer(ctx, queue.ValidationLayerUBL21XSD)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("worker-xsd open request consumer: %w", err)
	}

	var metricsServer *http.Server
	if cfg.MetricsAddr != "" {
		metricsServer = &http.Server{Addr: cfg.MetricsAddr, Handler: workerMetricsMux(client, nil)}
		go func() {
			if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Error("worker-xsd metrics server error", "error", err)
			}
		}()
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = metricsServer.Shutdown(shutdownCtx)
		}()
	}

	worker := &xsdStageWorker{
		queue: client, catalog: catalog, validate: validate, timeout: settings.validationTimeout,
		progressInterval: settings.progressInterval, logger: logger, now: now,
		publishResult: client.PublishStageResult, publishDeadLetter: client.PublishStageDeadLetter, nakDelay: xsdWorkerNAKDelay,
	}
	localCapacity := settings.concurrency + settings.prefetch
	messages, err := consumer.Messages(jetstream.PullMaxMessages(localCapacity))
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("worker-xsd start pulling requests: %w", err)
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
	logger.Info("worker-xsd started",
		"queueNamespace", nonEmpty(cfg.QueueNamespace.String(), "(default)"),
		"stage", queue.ValidationLayerUBL21XSD,
		"rulesetVersion", xsdschemas.Version,
		"concurrency", settings.concurrency,
		"prefetch", settings.prefetch,
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
			return fmt.Errorf("worker-xsd pull request: %w", nextErr)
		}
		job := newQueuedDocumentJob(message, settings.progressInterval)
		select {
		case jobs <- job:
		case <-ctx.Done():
			job.stopProgress()
			_ = message.NakWithDelay(xsdWorkerNAKDelay)
		}
		if ctx.Err() != nil {
			break
		}
	}
	close(pullDone)
	close(jobs)
	workers.Wait()
	logger.Info("worker-xsd shutting down")
	return nil
}

type xsdStageWorker struct {
	queue             *queue.Client
	catalog           *validationCatalog
	validate          xsdStreamValidator
	openContent       func(context.Context, string) (queue.UploadContentReader, error)
	authorizeRequest  func(context.Context, queue.StageRequest) (queue.ValidationExecution, error)
	timeout           time.Duration
	progressInterval  time.Duration
	logger            *slog.Logger
	now               func() time.Time
	publishResult     func(context.Context, queue.StageResult) error
	publishDeadLetter func(context.Context, queue.StageDeadLetter) error
	nakDelay          time.Duration
}

func (worker *xsdStageWorker) process(ctx context.Context, message jetstream.Msg, stopProgress func()) {
	if stopProgress != nil {
		defer stopProgress()
	}
	if err := worker.handle(ctx, message); err != nil {
		worker.logger.Error("worker-xsd request failed", "error", err)
	}
}

func (worker *xsdStageWorker) handle(ctx context.Context, message jetstream.Msg) (err error) {
	if ctx.Err() != nil {
		worker.nak(message)
		return nil
	}
	var request queue.StageRequest
	if err := request.UnmarshalJSON(message.Data()); err != nil {
		_ = message.Term()
		return fmt.Errorf("terminate malformed XSD stage request: %w", err)
	}
	if request.Stage != queue.ValidationLayerUBL21XSD {
		_ = message.Term()
		return fmt.Errorf("terminate non-XSD request on XSD consumer: stage %q", request.Stage)
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
		if errors.Is(err, errXSDStageRequestStale) {
			_ = message.Term()
			return fmt.Errorf("terminate rejected XSD stage request: %w", err)
		}
		worker.nak(message)
		return fmt.Errorf("verify XSD stage authorization: %w", err)
	}
	if request.Attempt >= execution.RetryPolicy.MaxAttemptsFor(request.Stage) {
		dead, found, lookupErr := worker.queue.StageDeadLetterForRequest(ctx, request)
		if lookupErr != nil {
			worker.nak(message)
			return fmt.Errorf("recover permanent XSD stage dead letter: %w", lookupErr)
		}
		if found {
			result, resultErr := dead.StageResultForRequest(request)
			if resultErr != nil {
				worker.nak(message)
				return fmt.Errorf("recover permanent XSD stage result: %w", resultErr)
			}
			publishedAt := time.Now()
			if err := worker.publishResult(ctx, result); err != nil {
				observeStageResultPublication(result.Stage, result.Outcome, publishedAt)
				worker.nak(message)
				return fmt.Errorf("publish recovered XSD stage result %s: %w", result.ResultID, err)
			}
			observeStageResultPublication(result.Stage, result.Outcome, publishedAt)
			recordSplitStagePublished(result, execution.RetryPolicy.MaxAttemptsFor(request.Stage))
			logSplitStageResult(ctx, worker.logger, result, true)
			if err := message.Ack(); err != nil {
				return fmt.Errorf("ack XSD stage request %s: %w", request.RequestID, err)
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

	if request.RulesetVersion != xsdschemas.Version {
		outcome = queue.StageOutcomeOperationalFailure
		reason = queue.StageReasonRuntimeUnavailable
		diagnostic = fmt.Sprintf("worker-xsd supports UBL schema version %s, request requires %s", xsdschemas.Version, request.RulesetVersion)
	} else if schemaPath, ok := worker.catalog.schemaPathFor(request.DocumentType); !ok {
		outcome = queue.StageOutcomeOperationalFailure
		reason = queue.StageReasonRuntimeUnavailable
		diagnostic = fmt.Sprintf("worker-xsd has no UBL 2.1 schema for document type %s", request.DocumentType)
	} else {
		validationErr := worker.withContent(attemptCtx, request.UploadID, request.Plan, func(reader io.Reader) error {
			validationCtx, validationSpan := beginStageOperation(attemptCtx, "stage.validator.execute", request)
			err := worker.validate(validationCtx, schemaPath, reader, worker.timeout)
			observability.End(validationSpan, err)
			return err
		})
		if validationErr != nil {
			if ctx.Err() != nil {
				worker.nak(message)
				return nil
			}
			if !xsdOperationalFailure(attemptCtx, validationErr) {
				outcome = queue.StageOutcomeStandardsInvalid
				issues = []queue.ValidationIssue{{
					Kind: queue.ValidationIssueXSDReport, Layer: queue.ValidationLayerUBL21XSD,
					Severity: "error", Message: boundedStageText(validationErr.Error(), queue.MaxStageIssueMessageBytes),
				}}
			} else {
				outcome = queue.StageOutcomeOperationalFailure
				reason = xsdStageOperationalReason(validationErr)
				diagnostic = boundedStageText(validationErr.Error(), queue.MaxStageDiagnosticBytes)
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
		return fmt.Errorf("build XSD stage result: %w", err)
	}
	recordSplitStageExecution(result)
	if outcome == queue.StageOutcomeOperationalFailure && request.Attempt >= execution.RetryPolicy.MaxAttemptsFor(request.Stage) {
		metadata, metadataErr := message.Metadata()
		if metadataErr != nil {
			worker.nak(message)
			return fmt.Errorf("read XSD stage request delivery count: %w", metadataErr)
		}
		dead, deadErr := queue.NewStageDeadLetter(result, metadata.NumDelivered, completedAt)
		if deadErr != nil {
			worker.nak(message)
			return fmt.Errorf("build XSD stage dead letter: %w", deadErr)
		}
		publishDeadLetter := worker.publishDeadLetter
		if publishDeadLetter == nil && worker.queue != nil {
			publishDeadLetter = worker.queue.PublishStageDeadLetter
		}
		if publishDeadLetter == nil {
			worker.nak(message)
			return errors.New("publish XSD stage dead letter: publisher is required")
		}
		if err := publishDeadLetter(ctx, dead); err != nil {
			worker.nak(message)
			return fmt.Errorf("publish XSD stage dead letter %s: %w", dead.DeadLetterID, err)
		}
	}
	publishedAt := time.Now()
	if err := worker.publishResult(ctx, result); err != nil {
		observeStageResultPublication(result.Stage, result.Outcome, publishedAt)
		worker.nak(message)
		return fmt.Errorf("publish XSD stage result %s: %w", result.ResultID, err)
	}
	observeStageResultPublication(result.Stage, result.Outcome, publishedAt)
	recordSplitStagePublished(result, execution.RetryPolicy.MaxAttemptsFor(request.Stage))
	logSplitStageResult(ctx, worker.logger, result, false)
	if err := message.Ack(); err != nil {
		return fmt.Errorf("ack XSD stage request %s: %w", request.RequestID, err)
	}
	return nil
}

var errXSDStageRequestStale = errors.New("stale XSD stage request")

func (worker *xsdStageWorker) authorize(ctx context.Context, request queue.StageRequest) (queue.ValidationExecution, error) {
	execution, err := worker.queue.GetValidationExecution(ctx, request.UploadID, request.ValidationID, request.Generation)
	if err != nil {
		if errors.Is(err, queue.ErrValidationExecutionNotFound) {
			return queue.ValidationExecution{}, fmt.Errorf("%w: execution generation does not exist", errXSDStageRequestStale)
		}
		return queue.ValidationExecution{}, err
	}
	if request.DocumentType != execution.DocumentType || request.Plan != execution.Plan.ID || request.PlanVersion != execution.Plan.Version ||
		request.StageOrdinal == 0 || int(request.StageOrdinal) > len(execution.Plan.Stages) {
		return queue.ValidationExecution{}, fmt.Errorf("%w: request identity does not match durable execution", errXSDStageRequestStale)
	}
	planStage := execution.Plan.Stages[request.StageOrdinal-1]
	if planStage.ID != request.Stage || planStage.RulesetVersion != request.RulesetVersion {
		return queue.ValidationExecution{}, fmt.Errorf("%w: request does not match frozen plan stage", errXSDStageRequestStale)
	}
	state := execution.Orchestration
	if state.Status != queue.ExecutionStatusStageQueued || int(request.StageOrdinal) > len(state.Stages) ||
		(execution.RequestedProfile != queue.ValidationProfilePeppolEnvelope && (state.CurrentStageOrdinal != request.StageOrdinal || len(state.Stages) != int(request.StageOrdinal))) {
		return queue.ValidationExecution{}, fmt.Errorf("%w: request is not the current queued stage", errXSDStageRequestStale)
	}
	current := state.Stages[request.StageOrdinal-1]
	if current.Stage != request.Stage || current.AuthorizedAttempt != request.Attempt || current.RequestID != request.RequestID {
		return queue.ValidationExecution{}, fmt.Errorf("%w: request is not the currently authorized attempt", errXSDStageRequestStale)
	}
	return execution, nil
}

func (worker *xsdStageWorker) nak(message jetstream.Msg) {
	delay := worker.nakDelay
	if delay <= 0 {
		delay = xsdWorkerNAKDelay
	}
	_ = message.NakWithDelay(delay)
}

func (worker *xsdStageWorker) withContent(ctx context.Context, uploadID, plan string, validate func(io.Reader) error) error {
	fetchCtx, fetchSpan := observability.Tracer().Start(ctx, "stage.xml.fetch", trace.WithAttributes(attribute.String("upload.id", uploadID)))
	openContent := worker.openContent
	if openContent == nil {
		openContent = worker.queue.OpenUploadContent
	}
	content, err := openContent(fetchCtx, uploadID)
	if err != nil {
		observability.End(fetchSpan, err)
		return fmt.Errorf("%w: open upload content: %w", errValidationContentStream, err)
	}
	content = traceUploadContent(content, fetchSpan)
	var validationErr error
	if plan == string(queue.ValidationProfilePeppolEnvelope) {
		document, readErr := io.ReadAll(content)
		if readErr != nil {
			validationErr = readErr
		} else if input, inputErr := stageInput(plan, queue.ValidationLayerUBL21XSD, document); inputErr != nil {
			validationErr = inputErr
		} else {
			validationErr = validate(bytes.NewReader(input))
		}
	} else {
		validationErr = validate(content)
	}
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

func xsdStageOperationalReason(err error) queue.StageOperationalReason {
	switch {
	case errors.Is(err, errValidationTimeout), errors.Is(err, context.DeadlineExceeded):
		return queue.StageReasonTimeout
	case errors.Is(err, context.Canceled):
		return queue.StageReasonCancelled
	case errors.Is(err, errValidationContentStream), errors.Is(err, queue.ErrRecordNotFound):
		return queue.StageReasonContentUnavailable
	case errors.Is(err, errXMLLintUnavailable):
		return queue.StageReasonRuntimeUnavailable
	default:
		return queue.StageReasonRuntimeFailure
	}
}

func boundedStageText(value string, maximum int) string {
	value = strings.ToValidUTF8(value, "\uFFFD")
	value = strings.TrimSpace(value)
	if maximum <= 0 {
		return ""
	}
	if len(value) <= maximum {
		return value
	}
	const suffix = "... [truncated]"
	if maximum <= len(suffix) {
		return suffix[:maximum]
	}
	limit := maximum - len(suffix)
	for limit > 0 && !utf8.ValidString(value[:limit]) {
		limit--
	}
	return value[:limit] + suffix
}
