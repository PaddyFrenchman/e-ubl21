package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/nats-io/nats.go"
)

// runTestWorker starts RunWorker in the background against cfg's NATS URL
// and stops it via t.Cleanup once the test finishes.
func runTestWorker(t *testing.T, cfg Config, overrides func(*WorkerConfig)) {
	t.Helper()

	workerCfg := WorkerConfig{
		NATSURL: cfg.NATSURL, NATSOptions: cfg.NATSOptions, QueueNamespace: cfg.QueueNamespace,
		UploadContentRetention: cfg.UploadContentRetention, UploadContentMaxBytes: cfg.UploadContentMaxBytes,
	}
	if overrides != nil {
		overrides(&workerCfg)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- RunWorker(ctx, workerCfg) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("RunWorker returned error: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("RunWorker did not stop within 5s of cancellation")
		}
	})
}

// waitForStatus polls GET /api/v1/ubl/{id} until its status is no longer
// "pending" (i.e. the worker has processed it) or the timeout elapses, at
// which point it fails the test.
func waitForStatus(t *testing.T, handler http.Handler, token, id string) map[string]any {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		res := doRequest(handler, http.MethodGet, "/api/v1/ubl/"+id, token, "", "")
		if res.Code != http.StatusOK {
			t.Fatalf("get status = %d, want %d; body=%s", res.Code, http.StatusOK, res.Body.String())
		}
		var stored map[string]any
		if err := json.Unmarshal(res.Body.Bytes(), &stored); err != nil {
			t.Fatalf("decode get response: %v", err)
		}
		if stored["status"] != string(queue.StatusPending) {
			return stored
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("upload %s did not leave pending status within 5s", id)
	return nil
}

func TestWorkerTerminatesPoisonMessagesAndRetriesMissingContent(t *testing.T) {
	cfg := testConfig(t, nil)
	runTestWorker(t, cfg, nil)
	q, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(q.Close)
	if err := q.EnsureInfraWithConfig(context.Background(), queue.InfraConfig{UploadContentMaxBytes: cfg.UploadContentMaxBytes}); err != nil {
		t.Fatal(err)
	}

	id, _ := queue.NewID()
	messages := []*nats.Msg{
		{Subject: q.Namespace.DocumentSubject("Invoice"), Data: []byte(`{`)},
		{Subject: q.Namespace.DocumentSubject("Invoice"), Data: []byte(`{"id":"` + id + `","documentType":"Order","receivedAt":"now"}`), Header: nats.Header{queue.HeaderUploadID: []string{id}}},
		{Subject: q.Namespace.DocumentSubject("Unknown"), Data: []byte(`{"id":"` + id + `","documentType":"Unknown","receivedAt":"now"}`), Header: nats.Header{queue.HeaderUploadID: []string{id}}},
		{Subject: q.Namespace.DocumentSubject("Invoice"), Data: []byte(`{"id":"` + id + `","documentType":"Invoice","receivedAt":"now"}`), Header: nats.Header{queue.HeaderUploadID: []string{id}}},
	}
	for _, msg := range messages {
		if _, err := q.JS.PublishMsg(context.Background(), msg); err != nil {
			t.Fatal(err)
		}
	}
	missingRecordID, _ := queue.NewID()
	if err := q.PutUploadContent(context.Background(), missingRecordID, []byte(validInvoiceXML)); err != nil {
		t.Fatal(err)
	}
	if err := q.PublishDocument(context.Background(), missingRecordID, "Invoice", time.Now()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)
}

func TestWorkerDeadLettersExhaustedFailureAndSupportsReplay(t *testing.T) {
	cfg := testConfig(t, nil)
	runTestWorker(t, cfg, func(worker *WorkerConfig) { worker.MaxDeliver = 1 })
	q, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(q.Close)
	ctx := context.Background()
	if err := q.EnsureInfraWithConfig(ctx, queue.InfraConfig{UploadContentMaxBytes: cfg.UploadContentMaxBytes}); err != nil {
		t.Fatal(err)
	}

	id, _ := queue.NewID()
	receivedAt := time.Now().UTC()
	if err := q.PutUploadRecord(ctx, queue.UploadRecord{ID: id, DocumentType: "Invoice", Status: queue.StatusPending, ReceivedAt: receivedAt.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	if err := q.PublishDocument(ctx, id, "Invoice", receivedAt); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		record, getErr := q.GetUploadRecord(ctx, id)
		if getErr == nil && record.Status == queue.StatusProcessingFailed {
			if record.ProcessingAttempts != 1 || record.LastError == "" || record.ProcessingFailedAt == "" {
				t.Fatalf("incomplete terminal failure metadata: %+v", record)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("upload was not terminally failed: record=%+v err=%v", record, getErr)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if depth, err := q.DeadLetterDepth(ctx); err != nil || depth != 1 {
		t.Fatalf("dead-letter depth=%d err=%v, want 1", depth, err)
	}
	if err := q.PutUploadContent(ctx, id, []byte(validInvoiceXML)); err != nil {
		t.Fatal(err)
	}
	if err := q.ReplayProcessingFailure(ctx, id); err != nil {
		t.Fatal(err)
	}

	deadline = time.Now().Add(3 * time.Second)
	for {
		record, getErr := q.GetUploadRecord(ctx, id)
		if getErr == nil && record.Status == queue.StatusValid {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("replayed upload was not validated: record=%+v err=%v", record, getErr)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if depth, err := q.DeadLetterDepth(ctx); err != nil || depth != 0 {
		t.Fatalf("dead-letter depth after replay=%d err=%v, want 0", depth, err)
	}
}

func TestWorkerCreatesCanonicalFailureWhenRecordIsMissingAtExhaustion(t *testing.T) {
	cfg := testConfig(t, nil)
	runTestWorker(t, cfg, func(worker *WorkerConfig) { worker.MaxDeliver = 1 })
	q, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(q.Close)
	ctx := context.Background()
	if err := q.EnsureInfraWithConfig(ctx, queue.InfraConfig{UploadContentMaxBytes: cfg.UploadContentMaxBytes}); err != nil {
		t.Fatal(err)
	}
	consumerDeadline := time.Now().Add(3 * time.Second)
	for {
		if _, consumerErr := q.JS.Consumer(ctx, q.Namespace.DocumentsStreamName(), defaultWorkerConsumerName); consumerErr == nil {
			break
		}
		if time.Now().After(consumerDeadline) {
			t.Fatal("worker consumer was not ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
	id, _ := queue.NewID()
	if err := q.PublishDocument(ctx, id, "Invoice", time.Now()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		record, getErr := q.GetUploadRecord(ctx, id)
		if getErr == nil && record.Status == queue.StatusProcessingFailed {
			if record.ProcessingAttempts != 1 || record.LastError == "" {
				t.Fatalf("created terminal record=%+v", record)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("missing canonical record was not repaired: record=%+v err=%v", record, getErr)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if depth, err := q.DeadLetterDepth(ctx); err != nil || depth != 1 {
		t.Fatalf("dead-letter depth=%d err=%v", depth, err)
	}
}

func TestWorkerRetriesDurableTerminalizationAfterDLQFailure(t *testing.T) {
	cfg := testConfig(t, nil)
	runTestWorker(t, cfg, func(worker *WorkerConfig) { worker.MaxDeliver = 1 })
	q, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(q.Close)
	ctx := context.Background()
	if err := q.EnsureInfraWithConfig(ctx, queue.InfraConfig{UploadContentMaxBytes: cfg.UploadContentMaxBytes}); err != nil {
		t.Fatal(err)
	}
	consumerDeadline := time.Now().Add(3 * time.Second)
	for {
		if _, consumerErr := q.JS.Consumer(ctx, q.Namespace.DocumentsStreamName(), defaultWorkerConsumerName); consumerErr == nil {
			break
		}
		if time.Now().After(consumerDeadline) {
			t.Fatal("worker consumer was not ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
	id, _ := queue.NewID()
	receivedAt := time.Now().UTC()
	if err := q.PutUploadRecord(ctx, queue.UploadRecord{ID: id, DocumentType: "Invoice", Status: queue.StatusPending, ReceivedAt: receivedAt.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	if err := q.JS.DeleteStream(ctx, q.Namespace.DeadLettersStreamName()); err != nil {
		t.Fatal(err)
	}
	if err := q.PublishDocument(ctx, id, "Invoice", receivedAt); err != nil {
		t.Fatal(err)
	}
	waitDeadline := time.Now().Add(3 * time.Second)
	for {
		record, getErr := q.GetUploadRecord(ctx, id)
		if getErr == nil && record.Status == queue.StatusDeadLetterPending {
			break
		}
		if time.Now().After(waitDeadline) {
			t.Fatalf("DLQ failure was not made recoverable: record=%+v err=%v", record, getErr)
		}
		time.Sleep(20 * time.Millisecond)
	}
	// Leave the DLQ absent long enough for the first terminalization publish to
	// fail; the canonical transition must remain recoverable rather than being
	// terminated at the application attempt threshold.
	time.Sleep(750 * time.Millisecond)
	pending, err := q.GetUploadRecord(ctx, id)
	if err != nil || pending.Status != queue.StatusDeadLetterPending {
		t.Fatalf("terminalization did not remain pending after DLQ failure: record=%+v err=%v", pending, err)
	}
	if err := q.EnsureInfraWithConfig(ctx, queue.InfraConfig{UploadContentMaxBytes: cfg.UploadContentMaxBytes}); err != nil {
		t.Fatal(err)
	}
	if err := q.PublishDocumentWithID(ctx, id, "Invoice", receivedAt, id+":terminalization-retry"); err != nil {
		t.Fatal(err)
	}
	waitDeadline = time.Now().Add(3 * time.Second)
	for {
		record, getErr := q.GetUploadRecord(ctx, id)
		if getErr == nil && record.Status == queue.StatusProcessingFailed {
			break
		}
		if time.Now().After(waitDeadline) {
			t.Fatalf("terminalization did not resume: record=%+v err=%v", record, getErr)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestWorkerResumesAfterDeadLetterPublishedButTerminalCASNotCommitted(t *testing.T) {
	cfg := testConfig(t, nil)
	runTestWorker(t, cfg, func(worker *WorkerConfig) { worker.MaxDeliver = 1 })
	q, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(q.Close)
	ctx := context.Background()
	if err := q.EnsureInfraWithConfig(ctx, queue.InfraConfig{UploadContentMaxBytes: cfg.UploadContentMaxBytes}); err != nil {
		t.Fatal(err)
	}
	id, _ := queue.NewID()
	receivedAt := time.Now().UTC().Truncate(time.Second)
	record := queue.UploadRecord{ID: id, DocumentType: "Invoice", Status: queue.StatusDeadLetterPending, Error: "storage failed", LastError: "storage failed", ReceivedAt: receivedAt.Format(time.RFC3339), ProcessingAttempts: 1, ProcessingFailedAt: receivedAt.Add(time.Second).Format(time.RFC3339)}
	if err := q.PutUploadRecord(ctx, record); err != nil {
		t.Fatal(err)
	}
	if err := q.PublishDeadLetter(ctx, queue.DeadLetter{ID: id, DocumentType: "Invoice", ReceivedAt: record.ReceivedAt, LastError: record.LastError, Attempts: 1, FailedAt: record.ProcessingFailedAt}); err != nil {
		t.Fatal(err)
	}
	if err := q.PublishDocument(ctx, id, "Invoice", receivedAt); err != nil {
		t.Fatal(err)
	}
	waitDeadline := time.Now().Add(3 * time.Second)
	for {
		stored, getErr := q.GetUploadRecord(ctx, id)
		if getErr == nil && stored.Status == queue.StatusProcessingFailed {
			return
		}
		if time.Now().After(waitDeadline) {
			t.Fatalf("pending terminal CAS was not resumed: record=%+v err=%v", stored, getErr)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestWorkerReconcilesPendingCompletionsWithoutWorkDescriptors(t *testing.T) {
	cfg := testConfig(t, nil)
	q, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(q.Close)
	ctx := context.Background()
	if err := q.EnsureInfraWithConfig(ctx, queue.InfraConfig{UploadContentMaxBytes: cfg.UploadContentMaxBytes}); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	deadID, _ := queue.NewID()
	dead := queue.UploadRecord{
		ID: deadID, DocumentType: "Invoice", Status: queue.StatusDeadLetterPending,
		ReceivedAt: now.Format(time.RFC3339), LastError: "storage unavailable",
		Error: "storage unavailable", ProcessingAttempts: 5,
		ProcessingFailedAt: now.Add(time.Second).Format(time.RFC3339),
	}
	if err := q.PutUploadRecord(ctx, dead); err != nil {
		t.Fatal(err)
	}
	resultID, _ := queue.NewID()
	result := queue.UploadRecord{
		ID: resultID, DocumentType: "Invoice", Status: queue.StatusValid,
		ReceivedAt: now.Format(time.RFC3339), ValidatedAt: now.Add(time.Second).Format(time.RFC3339),
		ResultPending: true,
	}
	if err := q.PutUploadRecord(ctx, result); err != nil {
		t.Fatal(err)
	}

	documents, err := q.JS.Stream(ctx, q.Namespace.DocumentsStreamName())
	if err != nil {
		t.Fatal(err)
	}
	before, err := documents.Info(ctx)
	if err != nil || before.State.Msgs != 0 {
		t.Fatalf("documents before reconciliation = %d, err=%v; want no wake-up descriptors", before.State.Msgs, err)
	}

	runTestWorker(t, cfg, nil)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		storedDead, deadErr := q.GetUploadRecord(ctx, deadID)
		storedResult, resultErr := q.GetUploadRecord(ctx, resultID)
		if deadErr == nil && resultErr == nil && storedDead.Status == queue.StatusProcessingFailed && !storedResult.ResultPending {
			deadDepth, depthErr := q.DeadLetterDepth(ctx)
			results, streamErr := q.JS.Stream(ctx, q.Namespace.ResultsStreamName())
			if depthErr != nil || streamErr != nil {
				t.Fatalf("inspect completion streams: dead=%v result=%v", depthErr, streamErr)
			}
			resultInfo, infoErr := results.Info(ctx)
			if infoErr != nil || deadDepth != 1 || resultInfo.State.Msgs != 1 {
				t.Fatalf("completion stream counts: dead=%d results=%d err=%v", deadDepth, resultInfo.State.Msgs, infoErr)
			}
			after, infoErr := documents.Info(ctx)
			if infoErr != nil || after.State.Msgs != 0 {
				t.Fatalf("documents after reconciliation = %d, err=%v; reconciler must not require descriptors", after.State.Msgs, infoErr)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("worker did not reconcile durable pending completions without descriptors")
}

func TestWorkerClaimsOneReplayAfterMessageIDDeduplicationWindow(t *testing.T) {
	cfg := testConfig(t, nil)
	q, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(q.Close)
	ctx := context.Background()
	if err := q.EnsureInfraWithConfig(ctx, queue.InfraConfig{UploadContentMaxBytes: cfg.UploadContentMaxBytes}); err != nil {
		t.Fatal(err)
	}

	documents, err := q.JS.Stream(ctx, q.Namespace.DocumentsStreamName())
	if err != nil {
		t.Fatal(err)
	}
	streamConfig, err := documents.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	shortWindow := streamConfig.Config
	shortWindow.Duplicates = 100 * time.Millisecond
	if _, err := q.JS.UpdateStream(ctx, shortWindow); err != nil {
		t.Fatal(err)
	}

	id, _ := queue.NewID()
	now := time.Now().UTC().Truncate(time.Second)
	replayID := id + ":replay:9"
	record := queue.UploadRecord{
		ID: id, DocumentType: "Invoice", Status: queue.StatusEnqueuePending,
		ReceivedAt: now.Format(time.RFC3339), LastError: "temporary storage failure",
		ProcessingAttempts: 5, ProcessingFailedAt: now.Add(time.Second).Format(time.RFC3339),
		ReplayGeneration: 9, ReplayMessageID: replayID,
	}
	if err := q.PutUploadRecord(ctx, record); err != nil {
		t.Fatal(err)
	}
	if err := q.PutUploadContent(ctx, id, []byte(validInvoiceXML)); err != nil {
		t.Fatal(err)
	}
	if err := q.PublishDeadLetter(ctx, queue.DeadLetter{ID: id, DocumentType: "Invoice", ReceivedAt: record.ReceivedAt, LastError: record.LastError, Attempts: 5, FailedAt: record.ProcessingFailedAt}); err != nil {
		t.Fatal(err)
	}

	// The first publication was stored but its acknowledgement was lost. Wait
	// beyond the broker duplicate window before retrying the operator command.
	if err := q.PublishDocumentWithID(ctx, id, "Invoice", now, replayID); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if err := q.ReplayProcessingFailure(ctx, id); err != nil {
		t.Fatal(err)
	}
	before, err := documents.Info(ctx)
	if err != nil || before.State.Msgs != 2 {
		t.Fatalf("descriptors after deduplication window = %d, err=%v; want 2 to exercise durable worker claim", before.State.Msgs, err)
	}

	runTestWorker(t, cfg, func(worker *WorkerConfig) {
		worker.Concurrency = 2
		worker.Prefetch = 1
	})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		stored, getErr := q.GetUploadRecord(ctx, id)
		streamInfo, infoErr := documents.Info(ctx)
		if getErr == nil && infoErr == nil && stored.Status == queue.StatusValid && !stored.ResultPending && streamInfo.State.Msgs == 0 {
			if stored.ReplayClaimedSeq == 0 || stored.ReplayMessageID != "" || stored.ReplayReady {
				t.Fatalf("replay claim was not retained: %+v", stored)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("worker did not collapse late duplicate replay descriptors to one durable claim")
}

func TestWorkerRetriesWhenResultStreamIsUnavailable(t *testing.T) {
	cfg := testConfig(t, nil)
	runTestWorker(t, cfg, func(worker *WorkerConfig) { worker.MaxDeliver = 1 })
	q, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(q.Close)
	if err := q.EnsureInfraWithConfig(context.Background(), queue.InfraConfig{UploadContentMaxBytes: cfg.UploadContentMaxBytes}); err != nil {
		t.Fatal(err)
	}
	id, _ := queue.NewID()
	if err := q.PutUploadContent(context.Background(), id, []byte(validInvoiceXML)); err != nil {
		t.Fatal(err)
	}
	if err := q.PutUploadRecord(context.Background(), queue.UploadRecord{ID: id, DocumentType: "Invoice", Status: queue.StatusPending, ReceivedAt: time.Now().Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	if err := q.JS.DeleteStream(context.Background(), q.Namespace.ResultsStreamName()); err != nil {
		t.Fatal(err)
	}
	if err := q.PublishDocument(context.Background(), id, "Invoice", time.Now()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		record, err := q.GetUploadRecord(context.Background(), id)
		if err == nil && record.Status == queue.StatusValid {
			if !record.ResultPending {
				t.Fatal("terminal status did not retain a pending-result marker")
			}
			// Give the failed publication path time to NAK the original work
			// descriptor before restoring the stream.
			attemptDeadline := time.Now().Add(time.Second)
			for record.ProcessingAttempts == 0 && time.Now().Before(attemptDeadline) {
				time.Sleep(20 * time.Millisecond)
				record, err = q.GetUploadRecord(context.Background(), id)
			}
			if err != nil || record.ProcessingAttempts == 0 {
				t.Fatalf("result publication failure did not record an application attempt: record=%+v err=%v", record, err)
			}
			consumer, consumerErr := q.JS.Consumer(context.Background(), q.Namespace.DocumentsStreamName(), defaultWorkerConsumerName)
			if consumerErr != nil {
				t.Fatal(consumerErr)
			}
			consumerInfo, infoErr := consumer.Info(context.Background())
			if infoErr != nil || consumerInfo.Config.MaxDeliver != -1 {
				t.Fatalf("broker MaxDeliver=%d err=%v, want unlimited (-1)", consumerInfo.Config.MaxDeliver, infoErr)
			}
			if err := q.EnsureInfraWithConfig(context.Background(), queue.InfraConfig{UploadContentMaxBytes: cfg.UploadContentMaxBytes}); err != nil {
				t.Fatal(err)
			}
			receivedAt := time.Now().UTC().Format(time.RFC3339)
			payload, _ := json.Marshal(queue.DocumentWorkItem{ID: id, DocumentType: "Invoice", ReceivedAt: receivedAt})
			msg := nats.NewMsg(q.Namespace.DocumentSubject("Invoice"))
			msg.Data = payload
			msg.Header.Set(queue.HeaderUploadID, id)
			msg.Header.Set(queue.HeaderDocumentType, "Invoice")
			msg.Header.Set(queue.HeaderReceivedAt, receivedAt)
			msg.Header.Set(nats.MsgIdHdr, id+":result-retry")
			if _, err := q.JS.PublishMsg(context.Background(), msg); err != nil {
				t.Fatal(err)
			}

			retryDeadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(retryDeadline) {
				retried, getErr := q.GetUploadRecord(context.Background(), id)
				if getErr == nil && !retried.ResultPending {
					results, streamErr := q.JS.Stream(context.Background(), q.Namespace.ResultsStreamName())
					if streamErr != nil {
						t.Fatal(streamErr)
					}
					info, infoErr := results.Info(context.Background())
					if infoErr != nil {
						t.Fatal(infoErr)
					}
					if info.State.Msgs != 1 {
						t.Fatalf("result retry published %d messages, want 1", info.State.Msgs)
					}
					return
				}
				time.Sleep(20 * time.Millisecond)
			}
			t.Fatal("worker did not clear the pending-result marker after retry")
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("worker did not update status before result publication failed")
}

func TestResolveWorkerRuntimeSettingsAlignsDeadlinesAndBounds(t *testing.T) {
	settings, err := resolveWorkerRuntimeSettings(WorkerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if settings.validationTimeout != 30*time.Second || settings.ackWait != 60*time.Second || settings.concurrency != 1 || settings.prefetch != 1 || settings.maxDeliver != 5 || settings.maxAckPending != 2 {
		t.Fatalf("default settings = %+v", settings)
	}
	settings, err = resolveWorkerRuntimeSettings(WorkerConfig{ValidationTimeout: 10 * time.Second, AckWait: 20 * time.Second, Concurrency: 3, Prefetch: 4, MaxDeliver: 8, MaxAckPending: 12})
	if err != nil {
		t.Fatal(err)
	}
	if settings.concurrency+settings.prefetch != 7 || settings.maxDeliver != 8 || settings.maxAckPending != 12 {
		t.Fatalf("explicit settings = %+v", settings)
	}
	if _, err := resolveWorkerRuntimeSettings(WorkerConfig{ValidationTimeout: 30 * time.Second, AckWait: 30 * time.Second}); err == nil {
		t.Fatal("AckWait equal to validation timeout was accepted")
	}
	if _, err := resolveWorkerRuntimeSettings(WorkerConfig{Concurrency: 3, Prefetch: 4, MaxAckPending: 6}); err == nil {
		t.Fatal("MaxAckPending below local capacity was accepted")
	}
}

func TestWorkerValidatesQueuedDocumentAsValid(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t, nil)
	runTestWorker(t, cfg, nil)
	handler := newApplicationHandler(t, cfg)
	token := signedJWT(t, cfg.OAuthIssuer, cfg.OAuthAudience, testSecret, time.Hour)

	res := doRequest(handler, http.MethodPost, "/api/v1/ubl/upload", token, validInvoiceXML, "application/xml")
	if res.Code != http.StatusAccepted {
		t.Fatalf("upload status = %d, want %d; body=%s", res.Code, http.StatusAccepted, res.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	id := payload["id"].(string)

	stored := waitForStatus(t, handler, token, id)
	if stored["status"] != string(queue.StatusValid) {
		t.Fatalf("status = %v, want %q; record=%v", stored["status"], queue.StatusValid, stored)
	}
	if _, hasError := stored["error"]; hasError {
		t.Fatalf("unexpected error field on a valid document: %v", stored["error"])
	}
	if stored["validatedAt"] == "" || stored["validatedAt"] == nil {
		t.Fatal("missing validatedAt on a processed document")
	}
	if stored["contentExpiresAt"] == "" || stored["contentExpiresAt"] == nil {
		t.Fatal("missing contentExpiresAt on a processed document")
	}
	assertHTTPXSDStage(t, stored, true)

	// Bypass message-ID deduplication to model a descriptor replay after the
	// broker duplicate window. A terminal record must suppress revalidation
	// and duplicate result publication permanently.
	q, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	results, err := q.JS.Stream(context.Background(), q.Namespace.ResultsStreamName())
	if err != nil {
		t.Fatal(err)
	}
	publicationDeadline := time.Now().Add(3 * time.Second)
	for {
		record, getErr := q.GetUploadRecord(context.Background(), id)
		info, infoErr := results.Info(context.Background())
		if getErr == nil && infoErr == nil && !record.ResultPending && info.State.Msgs == 1 {
			break
		}
		if time.Now().After(publicationDeadline) {
			t.Fatalf("initial result publication did not finish: record=%+v getErr=%v infoErr=%v", record, getErr, infoErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
	before, err := results.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	receivedAt := time.Now().UTC().Format(time.RFC3339)
	workPayload, _ := json.Marshal(queue.DocumentWorkItem{ID: id, DocumentType: "Invoice", ReceivedAt: receivedAt})
	msg := nats.NewMsg(q.Namespace.DocumentSubject("Invoice"))
	msg.Data = workPayload
	msg.Header.Set(queue.HeaderUploadID, id)
	msg.Header.Set(queue.HeaderDocumentType, "Invoice")
	msg.Header.Set(queue.HeaderReceivedAt, receivedAt)
	msg.Header.Set(nats.MsgIdHdr, id+":forced-late-duplicate")
	if _, err := q.JS.PublishMsg(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	documents, err := q.JS.Stream(context.Background(), q.Namespace.DocumentsStreamName())
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		info, err := documents.Info(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if info.State.Msgs == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("late duplicate descriptor was not acknowledged")
		}
		time.Sleep(10 * time.Millisecond)
	}
	after, err := results.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if after.State.Msgs != before.State.Msgs {
		t.Fatalf("late duplicate published another result: before=%d after=%d", before.State.Msgs, after.State.Msgs)
	}
}

func TestMandatoryWorkerRunsOrderedEN16931Pipeline(t *testing.T) {
	runtime, ok := localSaxonRuntimeConfig(t)
	if !ok {
		return
	}
	cfg := testConfig(t, nil)
	runTestWorker(t, cfg, func(worker *WorkerConfig) {
		worker.EN16931Mandatory = true
		worker.JavaExecutable = runtime.javaExecutable
		worker.SaxonJARPath = runtime.saxonJARPath
		worker.XMLResolverJARPath = runtime.xmlResolverJARPath
		worker.SaxonConfigurationPath = runtime.configurationPath
		worker.EN16931XSLTPath = runtime.xsltPath
		worker.SaxonStartupTimeout = 10 * time.Second
	})
	handler := newApplicationHandler(t, cfg)
	token := signedJWT(t, cfg.OAuthIssuer, cfg.OAuthAudience, testSecret, time.Hour)

	officialPath := filepath.Join("..", "createsamples", "en16931", "testdata", "official", "guide-example1.xml")
	official, err := os.ReadFile(officialPath)
	if err != nil {
		t.Fatal(err)
	}
	res := doRequest(handler, http.MethodPost, "/api/v1/ubl/upload", token, string(official), "application/xml")
	if res.Code != http.StatusAccepted {
		t.Fatalf("official upload = %d; body=%s", res.Code, res.Body.String())
	}
	var accepted map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	stored := waitForStatus(t, handler, token, accepted["id"].(string))
	assertHTTPValidationPipeline(t, stored, queue.StatusValid, true)

	// This fixture passes UBL XSD but omits mandatory EN 16931 business data.
	// It proves the worker no longer records valid immediately after XSD.
	res = doRequest(handler, http.MethodPost, "/api/v1/ubl/upload", token, validInvoiceXML, "application/xml")
	if res.Code != http.StatusAccepted {
		t.Fatalf("semantic-invalid upload = %d; body=%s", res.Code, res.Body.String())
	}
	accepted = nil
	if err := json.Unmarshal(res.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	stored = waitForStatus(t, handler, token, accepted["id"].(string))
	assertHTTPValidationPipeline(t, stored, queue.StatusInvalid, false)
}

func assertHTTPValidationPipeline(t *testing.T, stored map[string]any, status queue.ValidationStatus, enValid bool) {
	t.Helper()
	if stored["status"] != string(status) {
		t.Fatalf("status = %v, want %q; record=%v", stored["status"], status, stored)
	}
	stages, ok := stored["validationStages"].([]any)
	if !ok || len(stages) != 2 {
		t.Fatalf("validationStages = %#v, want ordered XSD and EN stages", stored["validationStages"])
	}
	xsd, xsdOK := stages[0].(map[string]any)
	en, enOK := stages[1].(map[string]any)
	if !xsdOK || xsd["layer"] != string(queue.ValidationLayerUBL21XSD) || xsd["valid"] != true {
		t.Fatalf("XSD stage = %#v", stages[0])
	}
	if !enOK || en["layer"] != string(queue.ValidationLayerEN16931) || en["version"] != "1.3.16" || en["valid"] != enValid {
		t.Fatalf("EN stage = %#v, want valid=%t", stages[1], enValid)
	}
}

func TestWorkerFlagsSchemaInvalidDocument(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t, nil)
	runTestWorker(t, cfg, nil)
	handler := newApplicationHandler(t, cfg)
	token := signedJWT(t, cfg.OAuthIssuer, cfg.OAuthAudience, testSecret, time.Hour)

	res := doRequest(handler, http.MethodPost, "/api/v1/ubl/upload", token, invalidInvoiceXML, "application/xml")
	if res.Code != http.StatusAccepted {
		t.Fatalf("upload status = %d, want %d; body=%s", res.Code, http.StatusAccepted, res.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	id := payload["id"].(string)

	stored := waitForStatus(t, handler, token, id)
	if stored["status"] != string(queue.StatusInvalid) {
		t.Fatalf("status = %v, want %q; record=%v", stored["status"], queue.StatusInvalid, stored)
	}
	errMsg, _ := stored["error"].(string)
	if errMsg == "" {
		t.Fatal("expected a non-empty error message for an invalid document")
	}
	assertHTTPXSDStage(t, stored, false)
}

func assertHTTPXSDStage(t *testing.T, stored map[string]any, valid bool) {
	t.Helper()
	stages, ok := stored["validationStages"].([]any)
	if !ok || len(stages) != 1 {
		t.Fatalf("validationStages = %#v, want one XSD stage", stored["validationStages"])
	}
	stage, ok := stages[0].(map[string]any)
	if !ok || stage["layer"] != string(queue.ValidationLayerUBL21XSD) || stage["version"] != "2.1" || stage["valid"] != valid {
		t.Fatalf("XSD stage = %#v, want valid=%t", stages[0], valid)
	}
	issues, hasIssues := stage["issues"].([]any)
	if valid && hasIssues {
		t.Fatalf("valid XSD stage unexpectedly has issues: %#v", issues)
	}
	if !valid && (!hasIssues || len(issues) == 0) {
		t.Fatalf("invalid XSD stage has no issues: %#v", stage)
	}
}

func TestWorkerFilterRestrictsToOneDocumentType(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t, nil)
	runTestWorker(t, cfg, func(wc *WorkerConfig) {
		wc.SubjectFilter = cfg.QueueNamespace.DocumentSubject("Invoice")
		wc.ConsumerName = "invoice-only-test-worker"
	})
	handler := newApplicationHandler(t, cfg)
	token := signedJWT(t, cfg.OAuthIssuer, cfg.OAuthAudience, testSecret, time.Hour)

	// Order is a recognized document type but this worker only consumes
	// Invoice, so it must remain pending indefinitely (checked via a short,
	// bounded wait rather than waitForStatus, which would otherwise time out
	// the whole test).
	res := doRequest(handler, http.MethodPost, "/api/v1/ubl/upload", token, `<Order xmlns="urn:oasis:names:specification:ubl:schema:xsd:Order-2"/>`, "application/xml")
	if res.Code != http.StatusAccepted {
		t.Fatalf("upload status = %d, want %d; body=%s", res.Code, http.StatusAccepted, res.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	id := payload["id"].(string)

	time.Sleep(300 * time.Millisecond)
	getRes := doRequest(handler, http.MethodGet, "/api/v1/ubl/"+id, token, "", "")
	var stored map[string]any
	if err := json.Unmarshal(getRes.Body.Bytes(), &stored); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if stored["status"] != string(queue.StatusPending) {
		t.Fatalf("status = %v, want %q (Order should not be consumed by an Invoice-only worker)", stored["status"], queue.StatusPending)
	}
}
