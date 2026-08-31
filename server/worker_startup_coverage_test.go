package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
)

func TestSplitWorkersReportConnectionAndMissingInfrastructure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := RunXSDWorker(context.Background(), XSDWorkerConfig{NATSURL: "nats://127.0.0.1:1", Logger: logger}); err == nil {
		t.Fatal("XSD worker accepted unavailable NATS")
	}
	if err := RunEN16931Worker(context.Background(), EN16931WorkerConfig{NATSURL: "nats://127.0.0.1:1", Logger: logger}); err == nil {
		t.Fatal("EN worker accepted unavailable NATS")
	}

	server := queuetest.StartServer(t)
	client, err := queue.ConnectWithOptions(server.URL, queue.MustNamespace("worker.startup"), server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := RunXSDWorker(ctx, XSDWorkerConfig{queueClient: client, catalog: testXSDCatalog(), xsdValidate: func(context.Context, string, io.Reader, time.Duration) error { return nil }, Logger: logger}); err == nil {
		t.Fatal("XSD worker accepted missing stage infrastructure")
	}
	if err := RunEN16931Worker(ctx, EN16931WorkerConfig{queueClient: client, validator: en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
		return queue.ValidationStageResult{}, nil
	}), Logger: logger}); err == nil {
		t.Fatal("EN worker accepted missing stage infrastructure")
	}

	if err := client.ProvisionStageQueueInfra(ctx, queue.DefaultStageQueueInfraConfig()); err != nil {
		t.Fatal(err)
	}
	if err := RunXSDWorker(ctx, XSDWorkerConfig{queueClient: client, catalog: testXSDCatalog(), xsdValidate: func(context.Context, string, io.Reader, time.Duration) error { return nil }, Logger: logger}); err == nil {
		t.Fatal("XSD worker accepted missing content infrastructure")
	}
	if err := RunEN16931Worker(ctx, EN16931WorkerConfig{queueClient: client, validator: en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
		return queue.ValidationStageResult{}, nil
	}), Logger: logger}); err == nil {
		t.Fatal("EN worker accepted missing content infrastructure")
	}

	if err := client.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	if err := RunEN16931Worker(ctx, EN16931WorkerConfig{queueClient: client, validator: en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
		return queue.ValidationStageResult{}, nil
	}), Logger: logger}); err == nil {
		t.Fatal("EN worker accepted missing execution infrastructure")
	}
}

func TestSplitWorkersTreatCanceledInfrastructureVerificationAsShutdown(t *testing.T) {
	server := queuetest.StartServer(t)
	client, err := queue.ConnectWithOptions(server.URL, queue.MustNamespace("worker.canceled"), server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := RunXSDWorker(ctx, XSDWorkerConfig{queueClient: client, Logger: logger}); err != nil {
		t.Fatalf("XSD canceled startup = %v", err)
	}
	if err := RunEN16931Worker(ctx, EN16931WorkerConfig{queueClient: client, Logger: logger}); err != nil {
		t.Fatalf("EN canceled startup = %v", err)
	}
}

func TestSplitWorkersOwnSuccessfulBrokerConnections(t *testing.T) {
	server := queuetest.StartServer(t)
	namespace := queue.MustNamespace("worker.owned")
	client, err := queue.ConnectWithOptions(server.URL, namespace, server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := client.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	if err := client.ProvisionStageQueueInfra(ctx, queue.DefaultStageQueueInfraConfig()); err != nil {
		t.Fatal(err)
	}
	if err := client.ProvisionValidationExecutionInfra(ctx); err != nil {
		t.Fatal(err)
	}
	client.Close()

	for name, test := range map[string]struct {
		startedMessage string
		run            func(context.Context, *slog.Logger) error
	}{
		"xsd": {
			startedMessage: "worker-xsd started",
			run: func(runCtx context.Context, logger *slog.Logger) error {
				return RunXSDWorker(runCtx, XSDWorkerConfig{
					NATSURL: server.URL, NATSOptions: server.Options, QueueNamespace: namespace,
					catalog: testXSDCatalog(), xsdValidate: func(context.Context, string, io.Reader, time.Duration) error { return nil },
					Logger: logger,
				})
			},
		},
		"en": {
			startedMessage: "worker-en16931 started",
			run: func(runCtx context.Context, logger *slog.Logger) error {
				return RunEN16931Worker(runCtx, EN16931WorkerConfig{
					NATSURL: server.URL, NATSOptions: server.Options, QueueNamespace: namespace,
					validator: en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
						return queue.ValidationStageResult{}, nil
					}),
					Logger: logger,
				})
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			runCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			started := make(chan struct{})
			startupLogger := slog.New(&workerStartupLogHandler{message: test.startedMessage, started: started})
			go func() { done <- test.run(runCtx, startupLogger) }()
			select {
			case <-started:
			case err := <-done:
				t.Fatalf("owned worker connection exited before startup: %v", err)
			case <-time.After(3 * time.Second):
				t.Fatal("owned worker connection did not start")
			}
			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("owned worker connection did not stop")
			}
		})
	}
}

type workerStartupLogHandler struct {
	message string
	started chan struct{}
	once    sync.Once
}

func (handler *workerStartupLogHandler) Enabled(context.Context, slog.Level) bool { return true }

func (handler *workerStartupLogHandler) Handle(_ context.Context, record slog.Record) error {
	if record.Message == handler.message {
		handler.once.Do(func() { close(handler.started) })
	}
	return nil
}

func (handler *workerStartupLogHandler) WithAttrs([]slog.Attr) slog.Handler { return handler }

func (handler *workerStartupLogHandler) WithGroup(string) slog.Handler { return handler }

func TestStageOperationalReasonClassificationAndTextBounds(t *testing.T) {
	for err, want := range map[error]queue.StageOperationalReason{
		errValidationTimeout:       queue.StageReasonTimeout,
		context.DeadlineExceeded:   queue.StageReasonTimeout,
		context.Canceled:           queue.StageReasonCancelled,
		errValidationContentStream: queue.StageReasonContentUnavailable,
		queue.ErrRecordNotFound:    queue.StageReasonContentUnavailable,
		errXMLLintUnavailable:      queue.StageReasonRuntimeUnavailable,
		errors.New("validator"):    queue.StageReasonRuntimeFailure,
	} {
		if got := xsdStageOperationalReason(err); got != want {
			t.Errorf("XSD reason for %v = %q, want %q", err, got, want)
		}
	}
	for err, want := range map[error]queue.StageOperationalReason{
		context.DeadlineExceeded:   queue.StageReasonTimeout,
		context.Canceled:           queue.StageReasonCancelled,
		errValidationContentStream: queue.StageReasonContentUnavailable,
		errEN16931RuntimeInput:     queue.StageReasonContentUnavailable,
		errEN16931RuntimeAsset:     queue.StageReasonRuntimeUnavailable,
		errEN16931SVRLOutputLimit:  queue.StageReasonResourceLimit,
		errEN16931RuntimeSVRL:      queue.StageReasonMalformedOutput,
		errors.New("validator"):    queue.StageReasonRuntimeFailure,
	} {
		if got := enStageOperationalReason(err); got != want {
			t.Errorf("EN reason for %v = %q, want %q", err, got, want)
		}
	}
	if got := boundedStageText(" value ", 20); got != "value" {
		t.Fatalf("bounded text = %q", got)
	}
	if got := boundedStageText("value", 0); got != "" {
		t.Fatalf("zero bound = %q", got)
	}
	if got := boundedStageText("long value", 3); got != "..." {
		t.Fatalf("short truncation = %q", got)
	}
	if got := boundedStageText("1234567890", 8); len(got) != 8 {
		t.Fatalf("bounded truncation = %q (%d bytes)", got, len(got))
	}
}
