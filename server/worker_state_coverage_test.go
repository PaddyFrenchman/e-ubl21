package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type workerStateMsg struct {
	data        []byte
	headers     nats.Header
	subject     string
	metadata    *jetstream.MsgMetadata
	metadataErr error
	acked       int
	nacked      int
	delayed     int
	terminated  int
}

func (m *workerStateMsg) Metadata() (*jetstream.MsgMetadata, error) { return m.metadata, m.metadataErr }
func (m *workerStateMsg) Data() []byte                              { return m.data }
func (m *workerStateMsg) Headers() nats.Header                      { return m.headers }
func (m *workerStateMsg) Subject() string                           { return m.subject }
func (*workerStateMsg) Reply() string                               { return "" }
func (m *workerStateMsg) Ack() error                                { m.acked++; return nil }
func (m *workerStateMsg) DoubleAck(context.Context) error           { m.acked++; return nil }
func (m *workerStateMsg) Nak() error                                { m.nacked++; return nil }
func (m *workerStateMsg) NakWithDelay(time.Duration) error          { m.delayed++; return nil }
func (*workerStateMsg) InProgress() error                           { return nil }
func (m *workerStateMsg) Term() error                               { m.terminated++; return nil }
func (m *workerStateMsg) TermWithReason(string) error               { m.terminated++; return nil }

func TestWorkerHandleHonorsDurableStateAndReplayGuards(t *testing.T) {
	cfg := testConfig(t, nil)
	q, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(q.Close)
	ctx := context.Background()
	if err := q.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pipeline := newDocumentValidationPipeline(q, en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
		return queue.ValidationStageResult{Layer: queue.ValidationLayerEN16931, Version: "test", Valid: true, ValidatedAt: time.Now().UTC().Format(time.RFC3339)}, nil
	}), time.Second)
	pipeline.xsdValidate = func(context.Context, string, io.Reader, time.Duration) error { return nil }
	worker := &documentWorker{
		queue: q,
		catalog: &validationCatalog{documentCatalog: &documentCatalog{entryByElement: map[string]schemaEntry{
			"Invoice": {file: "Invoice.xsd", namespace: "urn:test"},
		}}, dir: t.TempDir()},
		validator: pipeline, logger: logger, lifecycleCtx: ctx, maxDeliver: 5,
	}

	newMessage := func(id string) *workerStateMsg {
		receivedAt := time.Now().UTC().Format(time.RFC3339)
		data, marshalErr := json.Marshal(queue.DocumentWorkItem{ID: id, DocumentType: "Invoice", ReceivedAt: receivedAt})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		return &workerStateMsg{
			data: data, subject: q.Namespace.DocumentSubject("Invoice"),
			headers:  nats.Header{queue.HeaderUploadID: []string{id}, queue.HeaderDocumentType: []string{"Invoice"}},
			metadata: &jetstream.MsgMetadata{NumDelivered: 1, Sequence: jetstream.SequencePair{Stream: 10}},
		}
	}
	put := func(status queue.ValidationStatus, mutate func(*queue.UploadRecord)) (string, *workerStateMsg) {
		id, _ := queue.NewID()
		record := queue.UploadRecord{ID: id, DocumentType: "Invoice", Status: status, ReceivedAt: time.Now().UTC().Format(time.RFC3339)}
		if mutate != nil {
			mutate(&record)
		}
		if err := q.PutUploadRecord(ctx, record); err != nil {
			t.Fatal(err)
		}
		return id, newMessage(id)
	}

	t.Run("canceled lifecycle", func(t *testing.T) {
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		copy := *worker
		copy.lifecycleCtx = canceled
		msg := newMessage("00000000000000000000000000000000")
		copy.process(msg)
		if msg.nacked != 1 {
			t.Fatalf("NAKs = %d", msg.nacked)
		}
	})

	t.Run("missing canonical record", func(t *testing.T) {
		msg := newMessage("11111111111111111111111111111111")
		msg.metadataErr = errors.New("metadata unavailable")
		worker.handle(msg)
		if msg.delayed != 1 {
			t.Fatalf("delayed NAKs = %d", msg.delayed)
		}
	})

	for _, status := range []queue.ValidationStatus{queue.StatusAcceptanceFailed, queue.StatusCleanupPending, queue.StatusProcessingFailed} {
		t.Run("terminal "+string(status), func(t *testing.T) {
			_, msg := put(status, nil)
			worker.handle(msg)
			if msg.terminated != 1 {
				t.Fatalf("Term calls = %d", msg.terminated)
			}
		})
	}

	t.Run("nonqueueable accepting", func(t *testing.T) {
		id, msg := put(queue.StatusAccepting, nil)
		worker.handle(msg)
		stored, err := q.GetUploadRecord(ctx, id)
		if err != nil || stored.ProcessingAttempts != 1 || msg.delayed != 1 {
			t.Fatalf("record=%+v delayed=%d err=%v", stored, msg.delayed, err)
		}
	})

	t.Run("completed duplicate", func(t *testing.T) {
		_, msg := put(queue.StatusValid, nil)
		worker.handle(msg)
		if msg.acked != 1 {
			t.Fatalf("Ack calls = %d", msg.acked)
		}
	})

	t.Run("valid document completes and publishes result", func(t *testing.T) {
		id, msg := put(queue.StatusPending, nil)
		if err := q.PutUploadContent(ctx, id, []byte(`<Invoice/>`)); err != nil {
			t.Fatal(err)
		}
		copy := *worker
		copy.contentRetention = time.Hour
		copy.handle(msg)
		stored, err := q.GetUploadRecord(ctx, id)
		if err != nil || stored.Status != queue.StatusValid || stored.ResultPending || stored.ContentExpiresAt == "" || msg.acked != 1 {
			t.Fatalf("record=%+v Ack=%d err=%v", stored, msg.acked, err)
		}
		if published, err := q.HasValidationResult(ctx, id); err != nil || !published {
			t.Fatalf("published=%t err=%v", published, err)
		}
	})

	t.Run("standards-invalid document completes", func(t *testing.T) {
		id, msg := put(queue.StatusPending, nil)
		if err := q.PutUploadContent(ctx, id, []byte(`<Invoice/>`)); err != nil {
			t.Fatal(err)
		}
		copyPipeline := *pipeline
		copyPipeline.xsdValidate = func(context.Context, string, io.Reader, time.Duration) error {
			return fmt.Errorf("%w: invalid invoice", errSchemaStandardsInvalid)
		}
		copy := *worker
		copy.validator = &copyPipeline
		copy.handle(msg)
		stored, err := q.GetUploadRecord(ctx, id)
		if err != nil || stored.Status != queue.StatusInvalid || stored.ResultPending || msg.acked != 1 {
			t.Fatalf("record=%+v Ack=%d err=%v", stored, msg.acked, err)
		}
	})

	for name, validationErr := range map[string]error{
		"validator unavailable":      errXMLLintUnavailable,
		"validation timeout":         context.DeadlineExceeded,
		"internal validator failure": errors.New("validator crashed"),
	} {
		t.Run(name, func(t *testing.T) {
			id, msg := put(queue.StatusPending, nil)
			if err := q.PutUploadContent(ctx, id, []byte(`<Invoice/>`)); err != nil {
				t.Fatal(err)
			}
			copyPipeline := *pipeline
			copyPipeline.xsdValidate = func(context.Context, string, io.Reader, time.Duration) error { return validationErr }
			copy := *worker
			copy.validator = &copyPipeline
			copy.handle(msg)
			stored, err := q.GetUploadRecord(ctx, id)
			if err != nil || stored.ProcessingAttempts != 1 || msg.delayed != 1 {
				t.Fatalf("record=%+v delayed=%d err=%v", stored, msg.delayed, err)
			}
		})
	}

	t.Run("legacy result inspection failure", func(t *testing.T) {
		id, msg := put(queue.StatusValid, func(record *queue.UploadRecord) { record.ResultPending = true })
		if err := q.JS.DeleteStream(ctx, q.Namespace.ResultsStreamName()); err != nil {
			t.Fatal(err)
		}
		worker.handle(msg)
		if msg.delayed != 1 {
			t.Fatalf("delayed NAKs = %d for %s", msg.delayed, id)
		}
		if err := q.EnsureInfra(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("legacy result upgrade claim", func(t *testing.T) {
		id, msg := put(queue.StatusValid, func(record *queue.UploadRecord) { record.ResultPending = true })
		worker.handle(msg)
		stored, err := q.GetUploadRecord(ctx, id)
		if err != nil || stored.Status != queue.StatusPending || stored.ResultPending || msg.delayed != 1 {
			t.Fatalf("record=%+v delayed=%d err=%v", stored, msg.delayed, err)
		}
	})

	t.Run("replay handoff missing header", func(t *testing.T) {
		_, msg := put(queue.StatusPending, func(record *queue.UploadRecord) { record.ReplayMessageID = "generation" })
		worker.handle(msg)
		if msg.delayed != 1 {
			t.Fatalf("delayed NAKs = %d", msg.delayed)
		}
	})

	t.Run("replay metadata unavailable", func(t *testing.T) {
		_, msg := put(queue.StatusPending, nil)
		msg.headers.Set(queue.HeaderReplayMessageID, "generation")
		msg.metadataErr = errors.New("metadata unavailable")
		worker.handle(msg)
		if msg.delayed != 1 {
			t.Fatalf("delayed NAKs = %d", msg.delayed)
		}
	})

	t.Run("replay generation mismatch", func(t *testing.T) {
		_, msg := put(queue.StatusPending, func(record *queue.UploadRecord) {
			record.ReplayMessageID = "canonical"
			record.ReplayReady = true
		})
		msg.headers.Set(queue.HeaderReplayMessageID, "other")
		worker.handle(msg)
		if msg.terminated != 1 {
			t.Fatalf("Term calls = %d", msg.terminated)
		}
	})

	t.Run("replay not ready", func(t *testing.T) {
		_, msg := put(queue.StatusPending, func(record *queue.UploadRecord) { record.ReplayMessageID = "canonical" })
		msg.headers.Set(queue.HeaderReplayMessageID, "canonical")
		worker.handle(msg)
		if msg.delayed != 1 {
			t.Fatalf("delayed NAKs = %d", msg.delayed)
		}
	})

	t.Run("ready replay is claimed", func(t *testing.T) {
		id, msg := put(queue.StatusPending, func(record *queue.UploadRecord) {
			record.ReplayMessageID = "canonical"
			record.ReplayReady = true
		})
		msg.headers.Set(queue.HeaderReplayMessageID, "canonical")
		worker.handle(msg)
		stored, err := q.GetUploadRecord(ctx, id)
		if err != nil || stored.ReplayMessageID != "" || stored.ReplayReady || stored.ReplayClaimedSeq != 10 || msg.delayed != 1 {
			t.Fatalf("record=%+v delayed=%d err=%v", stored, msg.delayed, err)
		}
	})

	t.Run("duplicate replay sequence", func(t *testing.T) {
		_, msg := put(queue.StatusPending, func(record *queue.UploadRecord) { record.ReplayClaimedSeq = 9 })
		msg.headers.Set(queue.HeaderReplayMessageID, "canonical")
		worker.handle(msg)
		if msg.acked != 1 {
			t.Fatalf("Ack calls = %d", msg.acked)
		}
	})

	t.Run("missing content exhausts attempts", func(t *testing.T) {
		id, msg := put(queue.StatusPending, nil)
		copy := *worker
		copy.maxDeliver = 1
		copy.handle(msg)
		stored, err := q.GetUploadRecord(ctx, id)
		if err != nil || stored.Status != queue.StatusProcessingFailed || msg.terminated != 1 {
			t.Fatalf("record=%+v Term=%d err=%v", stored, msg.terminated, err)
		}
	})

	t.Run("nil lifecycle defaults to background", func(t *testing.T) {
		_, msg := put(queue.StatusAcceptanceFailed, nil)
		copy := *worker
		copy.lifecycleCtx = nil
		copy.handle(msg)
		if msg.terminated != 1 {
			t.Fatalf("Term calls = %d", msg.terminated)
		}
	})

	t.Run("pending result records publication retry", func(t *testing.T) {
		id, msg := put(queue.StatusValid, func(record *queue.UploadRecord) {
			record.ResultPending = true
			record.ValidationStages = queue.ValidationStages{{Layer: queue.ValidationLayerUBL21XSD, Valid: true}}
		})
		versioned, err := q.GetUploadRecordVersion(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		worker.retryOperationalFailure(ctx, msg, queue.DocumentWorkItem{ID: id, DocumentType: "Invoice"}, &versioned, errors.New("publish failed"))
		stored, err := q.GetUploadRecord(ctx, id)
		if err != nil || stored.ProcessingAttempts != 1 || stored.LastError != "publish failed" || msg.delayed != 1 {
			t.Fatalf("record=%+v delayed=%d err=%v", stored, msg.delayed, err)
		}
	})

	t.Run("existing dead-letter transition finalizes", func(t *testing.T) {
		id, msg := put(queue.StatusDeadLetterPending, func(record *queue.UploadRecord) {
			record.LastError = "exhausted"
			record.Error = "exhausted"
			record.ProcessingAttempts = 5
			record.ProcessingFailedAt = time.Now().UTC().Format(time.RFC3339)
		})
		worker.handle(msg)
		stored, err := q.GetUploadRecord(ctx, id)
		if err != nil || stored.Status != queue.StatusProcessingFailed || msg.terminated != 1 {
			t.Fatalf("record=%+v Term=%d err=%v", stored, msg.terminated, err)
		}
	})
}

func TestRunWorkerRejectsConnectionAndRuntimeConfiguration(t *testing.T) {
	validator := en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
		return queue.ValidationStageResult{}, nil
	})
	if err := RunWorker(context.Background(), WorkerConfig{NATSURL: "://bad", en16931Validator: validator}); err == nil {
		t.Fatal("malformed NATS URL accepted")
	}
	if err := RunWorker(context.Background(), WorkerConfig{NATSURL: nats.DefaultURL, UploadContentRetention: -time.Second, en16931Validator: validator}); err == nil {
		t.Fatal("negative content retention accepted")
	}
	cfg := testConfig(t, nil)
	if err := RunWorker(context.Background(), WorkerConfig{
		NATSURL: cfg.NATSURL, NATSOptions: cfg.NATSOptions, QueueNamespace: cfg.QueueNamespace,
		SubjectFilter: "outside.documents.>", en16931Validator: validator,
	}); err == nil || !strings.Contains(err.Error(), "subject filter") {
		t.Fatalf("foreign subject filter error = %v", err)
	}
}

func TestWorkerCompletionReconcilerErrorPaths(t *testing.T) {
	cfg := testConfig(t, nil)
	q, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := q.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	worker := &documentWorker{queue: q, logger: logger}
	now := time.Now().UTC().Format(time.RFC3339)

	legacyID, _ := queue.NewID()
	legacy := queue.UploadRecord{ID: legacyID, DocumentType: "Invoice", Status: queue.StatusValid, ReceivedAt: "bad-time", ResultPending: true}
	if err := q.PutUploadRecord(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	deadID, _ := queue.NewID()
	dead := queue.UploadRecord{ID: deadID, DocumentType: "Invoice", Status: queue.StatusDeadLetterPending, ReceivedAt: now, LastError: "failed", ProcessingAttempts: 1, ProcessingFailedAt: now}
	if err := q.PutUploadRecord(ctx, dead); err != nil {
		t.Fatal(err)
	}
	if err := q.JS.DeleteStream(ctx, q.Namespace.DeadLettersStreamName()); err != nil {
		t.Fatal(err)
	}
	report := worker.reconcilePendingCompletions(ctx)
	if report.Scanned != 2 || report.Errors != 2 {
		t.Fatalf("error reconciliation report = %+v", report)
	}

	q.Close()
	report = worker.reconcilePendingCompletions(ctx)
	if report.Errors != 1 {
		t.Fatalf("closed-client reconciliation report = %+v", report)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	worker.runCompletionReconciler(canceled, time.Millisecond)
}

func TestWorkerCompletionReconcilerRepairsDurableStates(t *testing.T) {
	cfg := testConfig(t, nil)
	q, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(q.Close)
	ctx := context.Background()
	if err := q.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	worker := &documentWorker{queue: q, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	now := time.Now().UTC().Format(time.RFC3339)
	put := func(record queue.UploadRecord) string {
		t.Helper()
		record.ID, _ = queue.NewID()
		record.DocumentType = "Invoice"
		record.ReceivedAt = now
		if err := q.PutUploadRecord(ctx, record); err != nil {
			t.Fatal(err)
		}
		return record.ID
	}
	deadID := put(queue.UploadRecord{Status: queue.StatusDeadLetterPending, Error: "failed", LastError: "failed", ProcessingAttempts: 3, ProcessingFailedAt: now})
	resultID := put(queue.UploadRecord{
		Status: queue.StatusValid, ValidatedAt: now, ResultPending: true,
		ValidationStages: queue.ValidationStages{
			{Layer: queue.ValidationLayerUBL21XSD, Version: "2.1", Valid: true, ValidatedAt: now},
			{Layer: queue.ValidationLayerEN16931, Version: "test", Valid: true, ValidatedAt: now},
		},
	})
	upgradeID := put(queue.UploadRecord{Status: queue.StatusValid, ValidatedAt: now, ResultPending: true})

	report := worker.reconcilePendingCompletions(ctx)
	if report.Scanned != 3 || report.DeadLettersFinalized != 1 || report.ResultsPublished != 1 || report.ValidationUpgradesQueued != 1 || report.Errors != 0 {
		t.Fatalf("reconciliation report = %+v", report)
	}
	dead, err := q.GetUploadRecord(ctx, deadID)
	if err != nil || dead.Status != queue.StatusProcessingFailed {
		t.Fatalf("dead-letter record=%+v err=%v", dead, err)
	}
	result, err := q.GetUploadRecord(ctx, resultID)
	if err != nil || result.ResultPending {
		t.Fatalf("result record=%+v err=%v", result, err)
	}
	consumer, err := q.DocumentsConsumer(ctx, "coverage-upgrade", "")
	if err != nil {
		t.Fatal(err)
	}
	messages, err := consumer.Fetch(1, jetstream.FetchMaxWait(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	for msg := range messages.Messages() {
		item, decodeErr := queue.DecodeDocumentWorkItem(msg.Data())
		if decodeErr != nil || item.ID != upgradeID {
			t.Fatalf("upgrade work item=%+v err=%v", item, decodeErr)
		}
		_ = msg.Ack()
	}
}

func TestWorkerCASRaceRecoveryPaths(t *testing.T) {
	cfg := testConfig(t, nil)
	q, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(q.Close)
	ctx := context.Background()
	if err := q.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	now := time.Now().UTC().Format(time.RFC3339)
	worker := &documentWorker{queue: q, logger: logger, lifecycleCtx: ctx, maxDeliver: 5}
	put := func(status queue.ValidationStatus, pending bool) (string, queue.VersionedUploadRecord) {
		t.Helper()
		id, _ := queue.NewID()
		record := queue.UploadRecord{ID: id, DocumentType: "Invoice", Status: status, ReceivedAt: now, ResultPending: pending}
		if err := q.PutUploadRecord(ctx, record); err != nil {
			t.Fatal(err)
		}
		versioned, err := q.GetUploadRecordVersion(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return id, versioned
	}
	bump := func(versioned queue.VersionedUploadRecord, mutate func(*queue.UploadRecord)) {
		t.Helper()
		record := versioned.Record
		mutate(&record)
		if _, err := q.UpdateUploadRecord(ctx, record, versioned.Revision); err != nil {
			t.Fatal(err)
		}
	}
	newMsg := func(id string) *workerStateMsg {
		item := queue.DocumentWorkItem{ID: id, DocumentType: "Invoice", ReceivedAt: now}
		data, _ := json.Marshal(item)
		return &workerStateMsg{data: data, subject: q.Namespace.DocumentSubject("Invoice"), headers: nats.Header{
			queue.HeaderUploadID: []string{id}, queue.HeaderDocumentType: []string{"Invoice"},
		}, metadata: &jetstream.MsgMetadata{NumDelivered: 5}}
	}

	for _, test := range []struct {
		name       string
		status     queue.ValidationStatus
		resultWait bool
		maxDeliver uint64
	}{
		{name: "pending result retry CAS loss", status: queue.StatusValid, resultWait: true, maxDeliver: 5},
		{name: "processing attempt CAS loss", status: queue.StatusPending, maxDeliver: 5},
		{name: "dead-letter transition CAS loss", status: queue.StatusPending, maxDeliver: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			id, stale := put(test.status, test.resultWait)
			bump(stale, func(record *queue.UploadRecord) { record.LastError = "concurrent" })
			copy := *worker
			copy.maxDeliver = test.maxDeliver
			msg := newMsg(id)
			copy.retryOperationalFailure(ctx, msg, queue.DocumentWorkItem{ID: id, DocumentType: "Invoice"}, &stale, errors.New("failed"))
			if msg.delayed != 1 {
				t.Fatalf("delayed NAKs = %d", msg.delayed)
			}
		})
	}

	t.Run("stale dead-letter finalization observes winner", func(t *testing.T) {
		id, stale := put(queue.StatusDeadLetterPending, false)
		stale.Record.LastError = "failed"
		stale.Record.ProcessingFailedAt = now
		bump(stale, func(record *queue.UploadRecord) { record.Status = queue.StatusProcessingFailed })
		won, err := worker.finalizeDeadLetterRecord(ctx, stale)
		if err != nil || won {
			t.Fatalf("won=%t err=%v for %s", won, err, id)
		}
	})

	t.Run("stale result marker observes winner", func(t *testing.T) {
		_, stale := put(queue.StatusValid, true)
		stale.Record.ValidatedAt = now
		stale.Record.ValidationStages = queue.ValidationStages{
			{Layer: queue.ValidationLayerUBL21XSD, Version: "2.1", Valid: true, ValidatedAt: now},
			{Layer: queue.ValidationLayerEN16931, Version: "test", Valid: true, ValidatedAt: now},
		}
		bump(stale, func(record *queue.UploadRecord) {
			record.ResultPending = false
			record.ValidatedAt = now
			record.ValidationStages = stale.Record.ValidationStages
		})
		if err := worker.ensureValidationResultPublished(ctx, stale); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("incomplete terminal stages are refused", func(t *testing.T) {
		_, versioned := put(queue.StatusValid, true)
		if err := worker.ensureValidationResultPublished(ctx, versioned); err == nil || !strings.Contains(err.Error(), "incomplete validation stages") {
			t.Fatalf("error = %v", err)
		}
	})
}
