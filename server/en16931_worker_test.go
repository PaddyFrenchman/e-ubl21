package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/orchestrator"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
	"github.com/nats-io/nats.go/jetstream"
)

func TestENWorkerPublishesPassedResultBeforeAcknowledgingRequest(t *testing.T) {
	client, ctx := newENWorkerTestClient(t)
	_, request := authorizeENWorkerRequest(t, ctx, client, []byte("<Invoice>streamed</Invoice>"))
	message := fetchENWorkerRequest(t, ctx, client)
	worker := newTestENStageWorker(client, en16931ValidatorFunc(func(_ context.Context, body io.Reader) (queue.ValidationStageResult, error) {
		content, err := io.ReadAll(body)
		if err != nil || string(content) != "<Invoice>streamed</Invoice>" {
			t.Fatalf("streamed content = %q, %v", content, err)
		}
		return queue.ValidationStageResult{
			Layer: queue.ValidationLayerEN16931, Version: validationassets.EN16931().Version, Valid: true,
			Issues: []queue.ValidationIssue{{Kind: queue.ValidationIssueSuccessfulReport, Layer: queue.ValidationLayerEN16931, Rule: "REPORT-1", Severity: "info", Message: "observed"}},
		}, nil
	}))
	if err := worker.handle(ctx, message); err != nil {
		t.Fatal(err)
	}
	result := fetchENWorkerResult(t, ctx, client)
	if result.RequestID != request.RequestID || result.Outcome != queue.StageOutcomePassed || len(result.Issues) != 1 || result.Issues[0].Kind != queue.ValidationIssueSuccessfulReport {
		t.Fatalf("EN 16931 result = %+v", result)
	}
	assertENWorkerRequestMessages(t, ctx, client, 0)
}

func TestENWorkerPreservesStandardsInvalidEvidence(t *testing.T) {
	client, ctx := newENWorkerTestClient(t)
	_, _ = authorizeENWorkerRequest(t, ctx, client, []byte("<Invoice/>"))
	message := fetchENWorkerRequest(t, ctx, client)
	issue := queue.ValidationIssue{
		Kind: queue.ValidationIssueFailedAssert, Layer: queue.ValidationLayerEN16931,
		Rule: "BR-CO-10", Severity: "fatal", Location: "/Invoice", Test: "cbc:ID", Message: "identifier required",
	}
	worker := newTestENStageWorker(client, en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
		return queue.ValidationStageResult{
			Layer: queue.ValidationLayerEN16931, Version: validationassets.EN16931().Version,
			Valid: false, Issues: []queue.ValidationIssue{issue},
		}, nil
	}))
	if err := worker.handle(ctx, message); err != nil {
		t.Fatal(err)
	}
	result := fetchENWorkerResult(t, ctx, client)
	if result.Outcome != queue.StageOutcomeStandardsInvalid || result.Reason != queue.StageReasonNone || len(result.Issues) != 1 || result.Issues[0] != issue {
		t.Fatalf("standards-invalid result = %+v", result)
	}
}

func TestENWorkerPreservesWarningFailedAssertInPassedResult(t *testing.T) {
	client, ctx := newENWorkerTestClient(t)
	_, request := authorizeENWorkerRequest(t, ctx, client, []byte("<Invoice/>"))
	message := fetchENWorkerRequest(t, ctx, client)
	issue := queue.ValidationIssue{
		Kind: queue.ValidationIssueFailedAssert, Layer: queue.ValidationLayerEN16931,
		Rule: "BR-DEC-01", Severity: "warning", Location: "/Invoice", Test: "cbc:ID", Message: "advisory",
	}
	worker := newTestENStageWorker(client, en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
		return queue.ValidationStageResult{
			Layer: queue.ValidationLayerEN16931, Version: validationassets.EN16931().Version,
			Valid: true, Issues: []queue.ValidationIssue{issue},
		}, nil
	}))
	if err := worker.handle(ctx, message); err != nil {
		t.Fatal(err)
	}
	result := fetchENWorkerResult(t, ctx, client)
	if result.RequestID != request.RequestID || result.Outcome != queue.StageOutcomePassed || result.Reason != queue.StageReasonNone || len(result.Issues) != 1 || result.Issues[0] != issue {
		t.Fatalf("passed warning result = %+v", result)
	}
	assertENWorkerRequestMessages(t, ctx, client, 0)
}

func TestENWorkerRejectsInconsistentValidatorEvidenceWithoutRedeliveryLoop(t *testing.T) {
	for _, test := range []struct {
		name   string
		valid  bool
		issues []queue.ValidationIssue
	}{
		{
			name: "passed with failed assert", valid: true,
			issues: []queue.ValidationIssue{{Kind: queue.ValidationIssueFailedAssert, Layer: queue.ValidationLayerEN16931, Severity: "error", Message: "failure"}},
		},
		{name: "invalid without failed assert", valid: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, ctx := newENWorkerTestClient(t)
			_, _ = authorizeENWorkerRequest(t, ctx, client, []byte("<Invoice/>"))
			message := fetchENWorkerRequest(t, ctx, client)
			worker := newTestENStageWorker(client, en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
				return queue.ValidationStageResult{
					Layer: queue.ValidationLayerEN16931, Version: validationassets.EN16931().Version,
					Valid: test.valid, Issues: test.issues,
				}, nil
			}))
			if err := worker.handle(ctx, message); err != nil {
				t.Fatal(err)
			}
			result := fetchENWorkerResult(t, ctx, client)
			if result.Outcome != queue.StageOutcomeOperationalFailure || result.Reason != queue.StageReasonMalformedOutput || result.Diagnostic == "" {
				t.Fatalf("inconsistent evidence result = %+v", result)
			}
			assertENWorkerRequestMessages(t, ctx, client, 0)
		})
	}
}

func TestENWorkerTerminatesMalformedNormalizedIssueEvidence(t *testing.T) {
	for _, test := range []struct {
		name  string
		issue queue.ValidationIssue
	}{
		{
			name:  "missing severity",
			issue: queue.ValidationIssue{Kind: queue.ValidationIssueSuccessfulReport, Layer: queue.ValidationLayerEN16931, Message: "advisory"},
		},
		{
			name:  "missing message",
			issue: queue.ValidationIssue{Kind: queue.ValidationIssueSuccessfulReport, Layer: queue.ValidationLayerEN16931, Severity: "info"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, ctx := newENWorkerTestClient(t)
			_, _ = authorizeENWorkerRequest(t, ctx, client, []byte("<Invoice/>"))
			message := fetchENWorkerRequest(t, ctx, client)
			worker := newTestENStageWorker(client, en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
				return queue.ValidationStageResult{
					Layer: queue.ValidationLayerEN16931, Version: validationassets.EN16931().Version,
					Valid: true, Issues: []queue.ValidationIssue{test.issue},
				}, nil
			}))
			if err := worker.handle(ctx, message); err != nil {
				t.Fatal(err)
			}
			result := fetchENWorkerResult(t, ctx, client)
			if result.Outcome != queue.StageOutcomeOperationalFailure || result.Reason != queue.StageReasonMalformedOutput || len(result.Issues) != 0 || !strings.Contains(result.Diagnostic, "malformed normalized issue evidence") {
				t.Fatalf("malformed issue result = %+v", result)
			}
			assertENWorkerRequestMessages(t, ctx, client, 0)
		})
	}
}

func TestENWorkerClassifiesOperationalFailures(t *testing.T) {
	tests := []struct {
		name       string
		content    bool
		validator  EN16931Validator
		wantReason queue.StageOperationalReason
	}{
		{
			name: "content unavailable", content: false,
			validator: en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
				t.Fatal("validator ran without content")
				return queue.ValidationStageResult{}, nil
			}),
			wantReason: queue.StageReasonContentUnavailable,
		},
		{
			name: "runtime unavailable", content: true,
			validator: en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
				return queue.ValidationStageResult{}, fmt.Errorf("%w: Saxon JAR missing", errEN16931RuntimeAsset)
			}),
			wantReason: queue.StageReasonRuntimeUnavailable,
		},
		{
			name: "malformed SVRL", content: true,
			validator: en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
				return queue.ValidationStageResult{}, fmt.Errorf("%w: bad output", errEN16931RuntimeSVRL)
			}),
			wantReason: queue.StageReasonMalformedOutput,
		},
		{
			name: "SVRL resource limit", content: true,
			validator: en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
				return queue.ValidationStageResult{}, fmt.Errorf("%w: too large", errEN16931SVRLOutputLimit)
			}),
			wantReason: queue.StageReasonResourceLimit,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, ctx := newENWorkerTestClient(t)
			_, request := authorizeENWorkerRequest(t, ctx, client, []byte("<Invoice/>"))
			if !test.content {
				if err := client.DeleteUploadContent(ctx, request.UploadID); err != nil {
					t.Fatal(err)
				}
			}
			message := fetchENWorkerRequest(t, ctx, client)
			worker := newTestENStageWorker(client, test.validator)
			if err := worker.handle(ctx, message); err != nil {
				t.Fatal(err)
			}
			result := fetchENWorkerResult(t, ctx, client)
			if result.Outcome != queue.StageOutcomeOperationalFailure || result.Reason != test.wantReason || result.Diagnostic == "" {
				t.Fatalf("operational result = %+v", result)
			}
		})
	}
}

func TestENWorkerOperationalResultsRetryWithoutRerunningXSD(t *testing.T) {
	for _, test := range []struct {
		name       string
		secondPass bool
		wantStatus queue.ValidationExecutionStatus
	}{
		{name: "retry passes", secondPass: true, wantStatus: queue.ExecutionStatusValid},
		{name: "retry exhausted", secondPass: false, wantStatus: queue.ExecutionStatusProcessingFailed},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, ctx := newENWorkerTestClient(t)
			machine, _ := authorizeENWorkerRequestWithRetry(t, ctx, client, []byte("<Invoice/>"), queue.FrozenValidationRetryPolicy{
				ID: queue.DefaultValidationRetryPolicyID, Version: queue.DefaultValidationRetryPolicyVersion, MaxAttempts: 2,
			})
			worker := newTestENStageWorker(client, en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
				return queue.ValidationStageResult{}, errors.New("Saxon runtime crashed")
			}))
			firstRequest := fetchENWorkerRequest(t, ctx, client)
			if err := worker.handle(ctx, firstRequest); err != nil {
				t.Fatal(err)
			}
			firstResult := fetchENWorkerResult(t, ctx, client)
			retrying, err := machine.HandleStageResult(ctx, firstResult)
			if err != nil {
				t.Fatal(err)
			}
			if retrying.Execution.Orchestration.Stages[0].AuthorizedAttempt != 1 || len(retrying.Execution.Orchestration.Stages[0].Attempts) != 1 || retrying.Execution.Orchestration.Stages[1].AuthorizedAttempt != 2 {
				t.Fatalf("independent retry state = %+v", retrying.Execution.Orchestration)
			}
			if test.secondPass {
				worker.validator = passingENValidator()
			}
			secondRequest := fetchENWorkerRequest(t, ctx, client)
			if err := worker.handle(ctx, secondRequest); err != nil {
				t.Fatal(err)
			}
			secondResult := fetchENWorkerResult(t, ctx, client)
			deadDepth, depthErr := client.StageDeadLetterDepth(ctx, queue.ValidationLayerEN16931)
			if depthErr != nil {
				t.Fatal(depthErr)
			}
			if test.secondPass && deadDepth != 0 {
				t.Fatalf("passing retry created %d EN dead letters", deadDepth)
			}
			if !test.secondPass && deadDepth != 1 {
				t.Fatalf("exhausted retry created %d EN dead letters, want 1", deadDepth)
			}
			terminal, err := machine.HandleStageResult(ctx, secondResult)
			if err != nil {
				t.Fatal(err)
			}
			if terminal.Execution.Orchestration.Status != test.wantStatus || len(terminal.Execution.Orchestration.Stages[0].Attempts) != 1 {
				t.Fatalf("terminal execution = %+v, want %q", terminal.Execution.Orchestration, test.wantStatus)
			}
		})
	}
}

func TestENWorkerLeavesRequestUnacknowledgedUntilResultPublicationSucceeds(t *testing.T) {
	client, ctx := newENWorkerTestClient(t)
	completedBefore := prometheusCounterValue(t, stageRequestsTotal.WithLabelValues(string(queue.ValidationLayerEN16931), string(queue.StageOutcomePassed)))
	_, _ = authorizeENWorkerRequest(t, ctx, client, []byte("<Invoice/>"))
	message := fetchENWorkerRequest(t, ctx, client)
	worker := newTestENStageWorker(client, passingENValidator())
	publicationFailure := errors.New("result broker unavailable")
	worker.publishResult = func(context.Context, queue.StageResult) error { return publicationFailure }
	worker.nakDelay = 10 * time.Millisecond
	if err := worker.handle(ctx, message); !errors.Is(err, publicationFailure) {
		t.Fatalf("publication error = %v, want %v", err, publicationFailure)
	}
	if got := prometheusCounterValue(t, stageRequestsTotal.WithLabelValues(string(queue.ValidationLayerEN16931), string(queue.StageOutcomePassed))) - completedBefore; got != 0 {
		t.Fatalf("completed EN requests after failed publication = %v, want 0", got)
	}
	assertENWorkerRequestMessages(t, ctx, client, 1)

	redelivered := fetchENWorkerRequest(t, ctx, client)
	metadata, err := redelivered.Metadata()
	if err != nil || metadata.NumDelivered < 2 {
		t.Fatalf("redelivery metadata = %+v, %v", metadata, err)
	}
	worker.publishResult = client.PublishStageResult
	if err := worker.handle(ctx, redelivered); err != nil {
		t.Fatal(err)
	}
	_ = fetchENWorkerResult(t, ctx, client)
	if got := prometheusCounterValue(t, stageRequestsTotal.WithLabelValues(string(queue.ValidationLayerEN16931), string(queue.StageOutcomePassed))) - completedBefore; got != 1 {
		t.Fatalf("completed EN requests after redelivery = %v, want 1", got)
	}
	assertENWorkerRequestMessages(t, ctx, client, 0)
}

func TestENWorkerPermanentDeadLetterFreezesExhaustedRedeliveryOutcome(t *testing.T) {
	client, ctx := newENWorkerTestClient(t)
	machine, _ := authorizeENWorkerRequestWithRetry(t, ctx, client, []byte("<Invoice/>"), queue.FrozenValidationRetryPolicy{
		ID: queue.DefaultValidationRetryPolicyID, Version: queue.DefaultValidationRetryPolicyVersion, MaxAttempts: 1,
	})
	validatorCalls := 0
	worker := newTestENStageWorker(client, en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
		validatorCalls++
		return queue.ValidationStageResult{}, errors.New("Saxon temporarily unavailable")
	}))
	publicationFailure := errors.New("result broker unavailable")
	worker.publishResult = func(context.Context, queue.StageResult) error { return publicationFailure }
	message := fetchENWorkerRequest(t, ctx, client)
	if err := worker.handle(ctx, message); !errors.Is(err, publicationFailure) {
		t.Fatalf("publication error = %v, want %v", err, publicationFailure)
	}
	if depth, err := client.StageDeadLetterDepth(ctx, queue.ValidationLayerEN16931); err != nil || depth != 1 {
		t.Fatalf("EN dead-letter depth = %d, %v; want 1", depth, err)
	}

	worker.validator = en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
		validatorCalls++
		return queue.ValidationStageResult{Layer: queue.ValidationLayerEN16931, Version: validationassets.EN16931().Version, Valid: true}, nil
	})
	worker.publishResult = client.PublishStageResult
	redelivered := fetchENWorkerRequest(t, ctx, client)
	if err := worker.handle(ctx, redelivered); err != nil {
		t.Fatal(err)
	}
	if validatorCalls != 1 {
		t.Fatalf("validator calls = %d, want 1; permanent DLQ must prevent rerun", validatorCalls)
	}
	result := fetchENWorkerResult(t, ctx, client)
	if result.Outcome != queue.StageOutcomeOperationalFailure || result.Reason != queue.StageReasonRuntimeFailure {
		t.Fatalf("recovered EN result = %+v", result)
	}
	terminal, err := machine.HandleStageResult(ctx, result)
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Execution.Orchestration.Status != queue.ExecutionStatusProcessingFailed {
		t.Fatalf("terminal execution = %+v", terminal.Execution.Orchestration)
	}
	if depth, err := client.StageDeadLetterDepth(ctx, queue.ValidationLayerEN16931); err != nil || depth != 1 {
		t.Fatalf("final EN dead-letter depth = %d, %v; want 1", depth, err)
	}
}

func TestENWorkerShutdownCancelsValidationAndRedeliversRequest(t *testing.T) {
	client, ctx := newENWorkerTestClient(t)
	_, _ = authorizeENWorkerRequest(t, ctx, client, []byte("<Invoice/>"))
	started := make(chan struct{})
	workerCtx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunEN16931Worker(workerCtx, EN16931WorkerConfig{
			queueClient: client, MetricsAddr: "127.0.0.1:0", ValidationTimeout: time.Minute,
			Concurrency: 1, Prefetch: 1, ProgressInterval: 10 * time.Millisecond,
			validator: en16931ValidatorFunc(func(ctx context.Context, _ io.Reader) (queue.ValidationStageResult, error) {
				close(started)
				<-ctx.Done()
				return queue.ValidationStageResult{}, ctx.Err()
			}),
			Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		})
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("worker-en16931 did not start validation")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker-en16931 did not stop after cancellation")
	}
	redelivered := fetchENWorkerRequest(t, ctx, client)
	if err := redelivered.Ack(); err != nil {
		t.Fatal(err)
	}
}

func TestENWorkerFailsStartupBeforePullingWhenPinnedRuntimeVerificationFails(t *testing.T) {
	client, ctx := newENWorkerTestClient(t)
	_, _ = authorizeENWorkerRequest(t, ctx, client, []byte("<Invoice/>"))
	verificationFailure := errors.New("Saxon smoke transform failed")
	var inspected saxonRuntimeConfig
	err := RunEN16931Worker(ctx, EN16931WorkerConfig{
		queueClient: client, MetricsAddr: "", JavaExecutable: "/test/java",
		SaxonJARPath: "/test/saxon.jar", XMLResolverJARPath: "/test/resolver.jar",
		SaxonConfigurationPath: "/test/saxon.xml", EN16931XSLTPath: "/test/en.xslt",
		SaxonStartupTimeout: 7 * time.Second,
		verifyRuntime: func(_ context.Context, runtime saxonRuntimeConfig) error {
			inspected = runtime
			return verificationFailure
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if !errors.Is(err, verificationFailure) {
		t.Fatalf("startup verification error = %v", err)
	}
	if inspected.javaExecutable != "/test/java" || inspected.saxonJARPath != "/test/saxon.jar" || inspected.xmlResolverJARPath != "/test/resolver.jar" || inspected.configurationPath != "/test/saxon.xml" || inspected.xsltPath != "/test/en.xslt" || inspected.startupTimeout != 7*time.Second {
		t.Fatalf("inspected Saxon runtime = %+v", inspected)
	}
	assertENWorkerRequestMessages(t, ctx, client, 1)
}

func TestENWorkerRequiresCurrentRequestAndPassingXSDEvidence(t *testing.T) {
	t.Run("EN is not current before XSD passes", func(t *testing.T) {
		client, ctx := newENWorkerTestClient(t)
		execution := startENWorkerExecution(t, ctx, client, []byte("<Invoice/>"))
		request, err := queue.NewStageRequest(execution, 2, 1, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		if err := client.PublishStageRequest(ctx, request); err != nil {
			t.Fatal(err)
		}
		message := fetchENWorkerRequest(t, ctx, client)
		called := false
		worker := newTestENStageWorker(client, en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
			called = true
			return queue.ValidationStageResult{}, nil
		}))
		if err := worker.handle(ctx, message); !errors.Is(err, errENStageRequestStale) {
			t.Fatalf("prerequisite rejection = %v", err)
		}
		if called {
			t.Fatal("EN validator ran before XSD passing evidence")
		}
		assertENWorkerRequestMessages(t, ctx, client, 0)
	})

	t.Run("frozen plan mismatch", func(t *testing.T) {
		client, ctx := newENWorkerTestClient(t)
		execution := startENWorkerExecution(t, ctx, client, []byte("<Invoice/>"))
		_ = fetchAndAckStageRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
		alternatePlan := enWorkerTestPlan()
		alternatePlan.ID = "alternate-plan"
		alternate, err := queue.NewValidationExecution(queue.ValidationExecutionIntent{
			IntentID: execution.CreationIntentID, UploadID: execution.UploadID, DocumentType: execution.DocumentType,
			Trigger: queue.ValidationTriggerInitial, RequestedProfile: execution.RequestedProfile,
			Plan: alternatePlan, CreatedAt: time.Now().UTC(),
		})
		if err != nil {
			t.Fatal(err)
		}
		request, err := queue.NewStageRequest(alternate, 2, 1, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		if err := client.PublishStageRequest(ctx, request); err != nil {
			t.Fatal(err)
		}
		message := fetchENWorkerRequest(t, ctx, client)
		worker := newTestENStageWorker(client, passingENValidator())
		if err := worker.handle(ctx, message); !errors.Is(err, errENStageRequestStale) {
			t.Fatalf("plan mismatch rejection = %v", err)
		}
		assertENWorkerRequestMessages(t, ctx, client, 0)
	})

	t.Run("superseded attempt", func(t *testing.T) {
		client, ctx := newENWorkerTestClient(t)
		machine, firstRequest := authorizeENWorkerRequest(t, ctx, client, []byte("<Invoice/>"))
		firstDelivery := fetchENWorkerRequest(t, ctx, client)
		if err := firstDelivery.Ack(); err != nil {
			t.Fatal(err)
		}
		started := time.Now().UTC()
		failure, err := queue.NewStageResult(firstRequest, queue.StageOutcomeOperationalFailure, queue.StageReasonRuntimeFailure, nil, "Saxon crashed", started, started.Add(time.Millisecond), time.Millisecond)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := machine.HandleStageResult(ctx, failure); err != nil {
			t.Fatal(err)
		}
		currentDelivery := fetchENWorkerRequest(t, ctx, client)
		if err := currentDelivery.Ack(); err != nil {
			t.Fatal(err)
		}
		payload, err := json.Marshal(firstRequest)
		if err != nil {
			t.Fatal(err)
		}
		names, _ := client.Namespace.StageQueue(queue.ValidationLayerEN16931)
		if _, err := client.JS.Publish(ctx, names.RequestSubject, payload); err != nil {
			t.Fatal(err)
		}
		staleDelivery := fetchENWorkerRequest(t, ctx, client)
		called := false
		worker := newTestENStageWorker(client, en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
			called = true
			return queue.ValidationStageResult{}, nil
		}))
		if err := worker.handle(ctx, staleDelivery); !errors.Is(err, errENStageRequestStale) {
			t.Fatalf("superseded attempt rejection = %v", err)
		}
		if called {
			t.Fatal("EN validator ran for a superseded attempt")
		}
		assertENWorkerRequestMessages(t, ctx, client, 0)
	})
}

func TestENWorkerScalesFromZeroAndFinalizesWithoutXSDWorker(t *testing.T) {
	client, ctx := newENWorkerTestClient(t)
	machine, _ := authorizeENWorkerRequest(t, ctx, client, []byte("<Invoice/>"))
	assertENWorkerRequestMessages(t, ctx, client, 1)

	workerCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() {
		done <- RunEN16931Worker(workerCtx, EN16931WorkerConfig{
			queueClient: client, MetricsAddr: "", ValidationTimeout: 3 * time.Second,
			Concurrency: 1, Prefetch: 1, ProgressInterval: 10 * time.Millisecond,
			validator: passingENValidator(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		})
	}()
	result := fetchENWorkerResult(t, ctx, client)
	final, err := machine.HandleStageResult(ctx, result)
	if err != nil {
		t.Fatal(err)
	}
	if final.Execution.Orchestration.Status != queue.ExecutionStatusValid {
		t.Fatalf("terminal execution = %+v", final.Execution.Orchestration)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker-en16931 did not stop")
	}
}

func TestENWorkerStartsWithActualNonDefaultRequestConsumerSettings(t *testing.T) {
	config := queue.DefaultStageQueueInfraConfig()
	for index := range config.Stages {
		if config.Stages[index].Stage == queue.ValidationLayerEN16931 {
			config.Stages[index].RequestDelivery = queue.StageDeliveryConfig{AckWait: 90 * time.Second, MaxAckPending: 20}
			config.Stages[index].ResultDelivery = queue.StageDeliveryConfig{AckWait: 120 * time.Second, MaxAckPending: 40}
		}
	}
	client, ctx := newENWorkerTestClientWithConfig(t, config)
	_, _ = authorizeENWorkerRequest(t, ctx, client, []byte("<Invoice/>"))
	workerCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() {
		done <- RunEN16931Worker(workerCtx, EN16931WorkerConfig{
			queueClient: client, MetricsAddr: "", ValidationTimeout: 3 * time.Second,
			Concurrency: 10, Prefetch: 10, ProgressInterval: 80 * time.Second,
			validator: passingENValidator(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		})
	}()
	if result := fetchENWorkerResult(t, ctx, client); result.Outcome != queue.StageOutcomePassed {
		t.Fatalf("EN result = %+v", result)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker-en16931 did not stop")
	}
}

func TestENWorkerUsesPinnedSaxonAdapter(t *testing.T) {
	runtime, ok := localSaxonRuntimeConfig(t)
	if !ok {
		return
	}
	content, err := os.ReadFile(filepath.Join("..", "createsamples", "en16931", "testdata", "official", "guide-example1.xml"))
	if err != nil {
		t.Fatal(err)
	}
	client, _ := newENWorkerTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	_, _ = authorizeENWorkerRequest(t, ctx, client, content)
	message := fetchENWorkerRequest(t, ctx, client)
	worker := newTestENStageWorker(client, newSaxonEN16931Validator(runtime, 45*time.Second))
	// The shared unit-test worker uses a three-second attempt timeout. This test
	// launches the real JVM, so its worker and validator need the same budget
	// when other packages are also exercising Saxon under `go test ./...`.
	worker.timeout = 45 * time.Second
	if err := worker.handle(ctx, message); err != nil {
		t.Fatal(err)
	}
	if result := fetchENWorkerResult(t, ctx, client); result.Outcome != queue.StageOutcomePassed {
		t.Fatalf("pinned Saxon result = %+v", result)
	}
}

func TestENWorkerSettingsRespectProvisionedConsumerBounds(t *testing.T) {
	settings := resolveENWorkerSettings(EN16931WorkerConfig{})
	if settings.validationTimeout != defaultENWorkerValidationTimeout || settings.concurrency != 1 || settings.prefetch != 1 || settings.progressInterval != 20*time.Second {
		t.Fatalf("default EN worker settings = %+v", settings)
	}
	delivery := queue.StageDeliveryConfig{AckWait: 90 * time.Second, MaxAckPending: 20}
	settings = resolveENWorkerSettings(EN16931WorkerConfig{Concurrency: 10, Prefetch: 10, ProgressInterval: 80 * time.Second})
	if err := validateENWorkerCapacity(settings, delivery); err != nil {
		t.Fatalf("actual provisioned settings rejected: %v", err)
	}
	settings.concurrency, settings.prefetch = 20, 1
	if err := validateENWorkerCapacity(settings, delivery); err == nil || !strings.Contains(err.Error(), "MaxAckPending") {
		t.Fatalf("capacity validation error = %v", err)
	}
	settings.concurrency, settings.prefetch, settings.progressInterval = 1, 1, 90*time.Second
	if err := validateENWorkerCapacity(settings, delivery); err == nil || !strings.Contains(err.Error(), "AckWait") {
		t.Fatalf("progress validation error = %v", err)
	}
}

func TestENWorkerTerminatesMalformedRequest(t *testing.T) {
	client, ctx := newENWorkerTestClient(t)
	names, err := client.Namespace.StageQueue(queue.ValidationLayerEN16931)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.JS.Publish(ctx, names.RequestSubject, []byte(`{"contractVersion":99}`)); err != nil {
		t.Fatal(err)
	}
	message := fetchENWorkerRequest(t, ctx, client)
	worker := newTestENStageWorker(client, passingENValidator())
	if err := worker.handle(ctx, message); err == nil || !strings.Contains(err.Error(), "terminate malformed") {
		t.Fatalf("malformed request error = %v", err)
	}
	assertENWorkerRequestMessages(t, ctx, client, 0)
}

func newENWorkerTestClient(t *testing.T) (*queue.Client, context.Context) {
	return newENWorkerTestClientWithConfig(t, queue.DefaultStageQueueInfraConfig())
}

func newENWorkerTestClientWithConfig(t *testing.T, config queue.StageQueueInfraConfig) (*queue.Client, context.Context) {
	t.Helper()
	server := queuetest.StartServer(t)
	client, err := queue.ConnectWithOptions(server.URL, queue.MustNamespace("workeren"), server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	if err := client.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	if err := client.ProvisionStageQueueInfra(ctx, config); err != nil {
		t.Fatal(err)
	}
	if err := client.ProvisionValidationExecutionInfra(ctx); err != nil {
		t.Fatal(err)
	}
	return client, ctx
}

func startENWorkerExecution(t *testing.T, ctx context.Context, client *queue.Client, content []byte) queue.ValidationExecution {
	return startENWorkerExecutionWithRetry(t, ctx, client, content, queue.FrozenValidationRetryPolicy{})
}

func startENWorkerExecutionWithRetry(t *testing.T, ctx context.Context, client *queue.Client, content []byte, retryPolicy queue.FrozenValidationRetryPolicy) queue.ValidationExecution {
	t.Helper()
	uploadID := enWorkerTestID(t)
	if err := client.PutUploadContent(ctx, uploadID, content); err != nil {
		t.Fatal(err)
	}
	machine := orchestrator.New(client)
	state, err := machine.Start(ctx, queue.ValidationExecutionIntent{
		IntentID: enWorkerTestID(t), UploadID: uploadID, DocumentType: "Invoice",
		Trigger: queue.ValidationTriggerInitial, RequestedProfile: queue.ValidationProfileUBL21EN16931,
		Plan: enWorkerTestPlan(), RetryPolicy: retryPolicy, CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return state.Execution
}

func authorizeENWorkerRequest(t *testing.T, ctx context.Context, client *queue.Client, content []byte) (*orchestrator.Machine, queue.StageRequest) {
	return authorizeENWorkerRequestWithRetry(t, ctx, client, content, queue.FrozenValidationRetryPolicy{})
}

func authorizeENWorkerRequestWithRetry(t *testing.T, ctx context.Context, client *queue.Client, content []byte, retryPolicy queue.FrozenValidationRetryPolicy) (*orchestrator.Machine, queue.StageRequest) {
	t.Helper()
	execution := startENWorkerExecutionWithRetry(t, ctx, client, content, retryPolicy)
	machine := orchestrator.New(client)
	xsdRequest := fetchAndAckStageRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	started := time.Now().UTC()
	xsdResult, err := queue.NewStageResult(xsdRequest, queue.StageOutcomePassed, queue.StageReasonNone, nil, "", started, started.Add(time.Millisecond), time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	advanced, err := machine.HandleStageResult(ctx, xsdResult)
	if err != nil {
		t.Fatal(err)
	}
	if advanced.Execution.ValidationID != execution.ValidationID || advanced.Execution.Orchestration.CurrentStageOrdinal != 2 {
		t.Fatalf("execution did not advance to EN: %+v", advanced.Execution.Orchestration)
	}
	stageState := advanced.Execution.Orchestration.Stages[1]
	createdAt, err := time.Parse(time.RFC3339Nano, stageState.RequestCreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	request, err := queue.NewStageRequest(advanced.Execution, 2, stageState.AuthorizedAttempt, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	return machine, request
}

func passingENValidator() EN16931Validator {
	return en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
		return queue.ValidationStageResult{Layer: queue.ValidationLayerEN16931, Version: validationassets.EN16931().Version, Valid: true}, nil
	})
}

func newTestENStageWorker(client *queue.Client, validator EN16931Validator) *enStageWorker {
	clock := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	return &enStageWorker{
		queue: client, validator: validator, timeout: 3 * time.Second,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)), publishResult: client.PublishStageResult,
		nakDelay: 10 * time.Millisecond,
		now: func() time.Time {
			clock = clock.Add(time.Millisecond)
			return clock
		},
	}
}

func enWorkerTestPlan() queue.FrozenValidationPlan {
	return queue.FrozenValidationPlan{ID: "worker-en-test", Version: "1", Stages: []queue.FrozenValidationStage{
		{ID: queue.ValidationLayerUBL21XSD, Ordinal: 1, Requirement: queue.ValidationStageRequired, RulesetVersion: "2.1"},
		{ID: queue.ValidationLayerEN16931, Ordinal: 2, Requirement: queue.ValidationStageRequired, RulesetVersion: validationassets.EN16931().Version},
	}}
}

func enWorkerTestID(t *testing.T) string {
	t.Helper()
	id, err := queue.NewID()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func fetchENWorkerRequest(t *testing.T, ctx context.Context, client *queue.Client) jetstream.Msg {
	t.Helper()
	return fetchStageRequest(t, ctx, client, queue.ValidationLayerEN16931)
}

func fetchAndAckStageRequest(t *testing.T, ctx context.Context, client *queue.Client, stage queue.ValidationLayer) queue.StageRequest {
	t.Helper()
	message := fetchStageRequest(t, ctx, client, stage)
	request := decodeENWorkerRequest(t, message.Data())
	if err := message.Ack(); err != nil {
		t.Fatal(err)
	}
	return request
}

func fetchStageRequest(t *testing.T, ctx context.Context, client *queue.Client, stage queue.ValidationLayer) jetstream.Msg {
	t.Helper()
	consumer, err := client.StageRequestConsumer(ctx, stage)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := consumer.Fetch(1, jetstream.FetchMaxWait(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	for message := range batch.Messages() {
		return message
	}
	if err := batch.Error(); err != nil {
		t.Fatal(err)
	}
	t.Fatalf("no %s request received", stage)
	return nil
}

func decodeENWorkerRequest(t *testing.T, data []byte) queue.StageRequest {
	t.Helper()
	var request queue.StageRequest
	if err := request.UnmarshalJSON(data); err != nil {
		t.Fatal(err)
	}
	return request
}

func fetchENWorkerResult(t *testing.T, ctx context.Context, client *queue.Client) queue.StageResult {
	t.Helper()
	consumer, err := client.StageResultConsumer(ctx, queue.ValidationLayerEN16931)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := consumer.Fetch(1, jetstream.FetchMaxWait(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	for message := range batch.Messages() {
		var result queue.StageResult
		if err := result.UnmarshalJSON(message.Data()); err != nil {
			t.Fatal(err)
		}
		if err := message.Ack(); err != nil {
			t.Fatal(err)
		}
		return result
	}
	if err := batch.Error(); err != nil {
		t.Fatal(err)
	}
	t.Fatal("no EN 16931 result received")
	return queue.StageResult{}
}

func assertENWorkerRequestMessages(t *testing.T, ctx context.Context, client *queue.Client, want uint64) {
	t.Helper()
	names, err := client.Namespace.StageQueue(queue.ValidationLayerEN16931)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := client.JS.Stream(ctx, names.RequestStream)
	if err != nil {
		t.Fatal(err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.State.Msgs != want {
		t.Fatalf("EN request stream messages = %d, want %d", info.State.Msgs, want)
	}
}
