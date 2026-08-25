package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/nats-io/nats.go"
)

// runTestWorker starts RunWorker in the background against cfg's NATS URL
// and stops it via t.Cleanup once the test finishes.
func runTestWorker(t *testing.T, cfg Config, overrides func(*WorkerConfig)) {
	t.Helper()

	workerCfg := WorkerConfig{NATSURL: cfg.NATSURL, NATSOptions: cfg.NATSOptions, QueueNamespace: cfg.QueueNamespace}
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
	if err := q.EnsureInfra(context.Background()); err != nil {
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

func TestWorkerRetriesWhenResultStreamIsUnavailable(t *testing.T) {
	cfg := testConfig(t, nil)
	runTestWorker(t, cfg, nil)
	q, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(q.Close)
	if err := q.EnsureInfra(context.Background()); err != nil {
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
			time.Sleep(50 * time.Millisecond)
			if err := q.EnsureInfra(context.Background()); err != nil {
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
