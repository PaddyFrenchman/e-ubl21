package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	fixtures "github.com/PaddyFrenchman/e-ubl21/createsamples/en16931"
	"github.com/PaddyFrenchman/e-ubl21/queue"
)

// TestEN16931PublicComplianceScenarios proves the standards outcomes through
// the public upload/status API and the complete durable worker transaction.
// It deliberately uses the real embedded NATS server, Object Store, UBL XSDs,
// xmllint, pinned Saxon runtime, EN stylesheet, KV state, and result stream.
func TestEN16931PublicComplianceScenarios(t *testing.T) {
	runtime, ok := localSaxonRuntimeConfig(t)
	if !ok {
		return
	}
	if err := ensureXMLLint(); err != nil {
		if os.Getenv("EUBL21_REQUIRE_EN16931_TEST_RUNTIME") == "1" {
			t.Fatalf("mandatory EN 16931 test runtime: %v", err)
		}
		t.Skip(err)
	}

	officialInvoice, err := os.ReadFile(filepath.Join("..", "createsamples", "en16931", "testdata", "official", "guide-example1.xml"))
	if err != nil {
		t.Fatal(err)
	}
	creditNote, err := fixtures.BuildCreditNoteXML(fixtures.NewCreditNoteVariantS())
	if err != nil {
		t.Fatal(err)
	}
	negativeCorpus, err := fixtures.NegativeInvoiceFixtures()
	if err != nil {
		t.Fatal(err)
	}
	if len(negativeCorpus) == 0 {
		t.Fatal("targeted EN 16931 negative corpus is empty")
	}
	semanticNegative := negativeCorpus[0]
	order, err := os.ReadFile(filepath.Join("..", "createsamples", "samples", "Order", "Order-sample-01.xml"))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		documentType string
		body         []byte
		wantStatus   queue.ValidationStatus
		assert       func(*testing.T, map[string]any)
	}{
		{
			name: "valid Invoice passes XSD and EN 16931", documentType: "Invoice", body: officialInvoice, wantStatus: queue.StatusValid,
			assert: func(t *testing.T, response map[string]any) {
				assertHTTPValidationPipeline(t, response, queue.StatusValid, true)
			},
		},
		{
			name: "XSD-invalid Invoice short-circuits EN 16931", documentType: "Invoice", body: []byte(invalidInvoiceXML), wantStatus: queue.StatusInvalid,
			assert: func(t *testing.T, response map[string]any) {
				assertHTTPXSDStage(t, response, false)
				stages := response["validationStages"].([]any)
				if len(stages) != 1 {
					t.Fatalf("XSD-invalid Invoice stages = %#v, want XSD only", stages)
				}
			},
		},
		{
			name: "XSD-valid semantic-invalid Invoice exposes EN diagnostics", documentType: semanticNegative.DocumentType, body: []byte(semanticNegative.XML), wantStatus: queue.StatusInvalid,
			assert: func(t *testing.T, response map[string]any) {
				assertHTTPValidationPipeline(t, response, queue.StatusInvalid, false)
				assertHTTPENIssue(t, response, semanticNegative.ExpectedRuleIDs[0])
			},
		},
		{
			name: "valid CreditNote passes XSD and EN 16931", documentType: "CreditNote", body: []byte(creditNote), wantStatus: queue.StatusValid,
			assert: func(t *testing.T, response map[string]any) {
				assertHTTPValidationPipeline(t, response, queue.StatusValid, true)
			},
		},
		{
			name: "non-Invoice UBL is explicitly XSD-only", documentType: "Order", body: order, wantStatus: queue.StatusValid,
			assert: func(t *testing.T, response map[string]any) {
				assertHTTPXSDStage(t, response, true)
				stages := response["validationStages"].([]any)
				if len(stages) != 1 {
					t.Fatalf("Order stages = %#v, want exactly the applicable XSD stage", stages)
				}
				if stages[0].(map[string]any)["layer"] == string(queue.ValidationLayerEN16931) {
					t.Fatalf("Order falsely reports EN 16931 execution: %#v", stages)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := testConfig(t, nil)
			stop := startComplianceWorker(t, cfg, func(worker *WorkerConfig) {
				configureRealSaxonWorker(worker, runtime)
			})
			defer stop()
			handler := newApplicationHandler(t, cfg)
			token := signedJWT(t, cfg.OAuthIssuer, cfg.OAuthAudience, testSecret, time.Hour)
			q := complianceQueue(t, cfg)

			id := uploadComplianceDocument(t, handler, token, test.body, test.documentType)
			response := waitForComplianceStatus(t, handler, token, q, id, test.wantStatus, true, 20*time.Second)
			test.assert(t, response)
			assertOnePermanentResult(t, q, id, test.wantStatus)
		})
	}
}

// TestEN16931OperationalFailureReplayEndToEnd proves that EN infrastructure
// failure follows retry/DLQ policy, never creates a standards result, and can
// be replayed to a successful real-runtime result after restoration.
func TestEN16931OperationalFailureReplayEndToEnd(t *testing.T) {
	runtime, ok := localSaxonRuntimeConfig(t)
	if !ok {
		return
	}
	if err := ensureXMLLint(); err != nil {
		t.Skip(err)
	}
	officialInvoice, err := os.ReadFile(filepath.Join("..", "createsamples", "en16931", "testdata", "official", "guide-example1.xml"))
	if err != nil {
		t.Fatal(err)
	}

	cfg := testConfig(t, nil)
	handler := newApplicationHandler(t, cfg)
	token := signedJWT(t, cfg.OAuthIssuer, cfg.OAuthAudience, testSecret, time.Hour)
	q := complianceQueue(t, cfg)

	stopFailedRuntime := startComplianceWorker(t, cfg, func(worker *WorkerConfig) {
		worker.MaxDeliver = 1
		worker.en16931Validator = en16931ValidatorFunc(func(_ context.Context, xml io.Reader) (queue.ValidationStageResult, error) {
			_, _ = io.Copy(io.Discard, xml)
			return queue.ValidationStageResult{}, errors.New("Saxon runtime unavailable")
		})
	})
	id := uploadComplianceDocument(t, handler, token, officialInvoice, "Invoice")
	failed := waitForComplianceStatus(t, handler, token, q, id, queue.StatusProcessingFailed, false, 10*time.Second)
	if failed["status"] != string(queue.StatusProcessingFailed) || failed["processingAttempts"] != float64(1) {
		t.Fatalf("operational failure response = %#v", failed)
	}
	if _, present := failed["validationStages"]; present {
		t.Fatalf("operational failure exposed a false standards result: %#v", failed["validationStages"])
	}
	if validationResultExists(t, q, id) {
		t.Fatal("operational failure published a false final validation result")
	}
	if depth, depthErr := q.DeadLetterDepth(context.Background()); depthErr != nil || depth != 1 {
		t.Fatalf("dead-letter depth = %d, err=%v; want 1", depth, depthErr)
	}
	stopFailedRuntime()

	if err := q.ReplayProcessingFailure(context.Background(), id); err != nil {
		t.Fatalf("replay processing failure: %v", err)
	}
	stopRestoredRuntime := startComplianceWorker(t, cfg, func(worker *WorkerConfig) {
		configureRealSaxonWorker(worker, runtime)
	})
	defer stopRestoredRuntime()
	replayed := waitForComplianceStatus(t, handler, token, q, id, queue.StatusValid, true, 20*time.Second)
	assertHTTPValidationPipeline(t, replayed, queue.StatusValid, true)
	assertOnePermanentResult(t, q, id, queue.StatusValid)
	if depth, depthErr := q.DeadLetterDepth(context.Background()); depthErr != nil || depth != 0 {
		t.Fatalf("dead-letter depth after replay = %d, err=%v; want 0", depth, depthErr)
	}
}

// TestEN16931CrashRestartEndToEnd models shutdown after XSD passes while EN
// validation is active. The replacement worker must rerun safely and occupy
// the permanent result slot exactly once.
func TestEN16931CrashRestartEndToEnd(t *testing.T) {
	runtime, ok := localSaxonRuntimeConfig(t)
	if !ok {
		return
	}
	if err := ensureXMLLint(); err != nil {
		t.Skip(err)
	}
	officialInvoice, err := os.ReadFile(filepath.Join("..", "createsamples", "en16931", "testdata", "official", "guide-example1.xml"))
	if err != nil {
		t.Fatal(err)
	}

	cfg := testConfig(t, nil)
	handler := newApplicationHandler(t, cfg)
	token := signedJWT(t, cfg.OAuthIssuer, cfg.OAuthAudience, testSecret, time.Hour)
	q := complianceQueue(t, cfg)
	semanticStarted := make(chan struct{})

	stopInterruptedWorker := startComplianceWorker(t, cfg, func(worker *WorkerConfig) {
		worker.ValidationTimeout = 30 * time.Second
		worker.AckWait = 60 * time.Second
		worker.en16931Validator = en16931ValidatorFunc(func(ctx context.Context, xml io.Reader) (queue.ValidationStageResult, error) {
			_, _ = io.Copy(io.Discard, xml)
			close(semanticStarted)
			<-ctx.Done()
			return queue.ValidationStageResult{}, ctx.Err()
		})
	})
	id := uploadComplianceDocument(t, handler, token, officialInvoice, "Invoice")
	select {
	case <-semanticStarted:
	case <-time.After(10 * time.Second):
		t.Fatal("interrupted worker never reached the EN 16931 stage")
	}

	pending, err := q.GetUploadRecord(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Status != queue.StatusPending || len(pending.ValidationStages) != 0 || pending.ResultPending || validationResultExists(t, q, id) {
		t.Fatalf("partial XSD success escaped before EN completion: %+v", pending)
	}
	stopInterruptedWorker()
	pending, err = q.GetUploadRecord(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Status != queue.StatusPending || pending.ProcessingAttempts != 0 || len(pending.ValidationStages) != 0 {
		t.Fatalf("shutdown consumed an attempt or persisted partial stages: %+v", pending)
	}

	stopReplacement := startComplianceWorker(t, cfg, func(worker *WorkerConfig) {
		configureRealSaxonWorker(worker, runtime)
	})
	defer stopReplacement()
	// Startup verification and validation each launch the real JVM. Keep the
	// outer full-suite budget above the worker's 30-second validation timeout so
	// host contention cannot expire the assertion before the bounded worker.
	restarted := waitForComplianceStatus(t, handler, token, q, id, queue.StatusValid, true, 60*time.Second)
	assertHTTPValidationPipeline(t, restarted, queue.StatusValid, true)
	assertOnePermanentResult(t, q, id, queue.StatusValid)
}

func configureRealSaxonWorker(worker *WorkerConfig, runtime saxonRuntimeConfig) {
	worker.en16931Validator = nil
	worker.JavaExecutable = runtime.javaExecutable
	worker.SaxonJARPath = runtime.saxonJARPath
	worker.XMLResolverJARPath = runtime.xmlResolverJARPath
	worker.SaxonConfigurationPath = runtime.configurationPath
	worker.EN16931XSLTPath = runtime.xsltPath
	worker.SaxonStartupTimeout = 10 * time.Second
	worker.ValidationTimeout = 30 * time.Second
	worker.XSDValidationTimeout = 20 * time.Second
	worker.EN16931ValidationTimeout = 20 * time.Second
	worker.AckWait = 60 * time.Second
}

func startComplianceWorker(t *testing.T, cfg Config, configure func(*WorkerConfig)) func() {
	t.Helper()
	worker := WorkerConfig{
		NATSURL: cfg.NATSURL, NATSOptions: cfg.NATSOptions, QueueNamespace: cfg.QueueNamespace,
		UploadContentRetention: cfg.UploadContentRetention, UploadContentMaxBytes: cfg.UploadContentMaxBytes,
		MetricsAddr: "", Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	if configure != nil {
		configure(&worker)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- RunWorker(ctx, worker) }()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Errorf("RunWorker returned error: %v", err)
				}
			case <-time.After(10 * time.Second):
				t.Error("RunWorker did not stop within 10 seconds")
			}
		})
	}
	t.Cleanup(stop)
	return stop
}

func complianceQueue(t *testing.T, cfg Config) *queue.Client {
	t.Helper()
	q, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(q.Close)
	if err := q.EnsureInfraWithConfig(context.Background(), queue.InfraConfig{UploadContentMaxBytes: cfg.UploadContentMaxBytes}); err != nil {
		t.Fatal(err)
	}
	return q
}

func uploadComplianceDocument(t *testing.T, handler http.Handler, token string, body []byte, wantDocumentType string) string {
	t.Helper()
	response := doRequest(handler, http.MethodPost, "/api/v1/ubl/upload", token, string(body), "application/xml")
	if response.Code != http.StatusAccepted {
		t.Fatalf("%s upload = %d, want %d; body=%s", wantDocumentType, response.Code, http.StatusAccepted, response.Body.String())
	}
	var accepted map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	if accepted["document"] != wantDocumentType || accepted["status"] != string(queue.StatusPending) {
		t.Fatalf("accepted upload = %#v", accepted)
	}
	id, ok := accepted["id"].(string)
	if !ok || !queue.ValidID(id) {
		t.Fatalf("accepted upload ID = %#v", accepted["id"])
	}
	return id
}

func waitForComplianceStatus(t *testing.T, handler http.Handler, token string, q *queue.Client, id string, want queue.ValidationStatus, requireResult bool, timeout time.Duration) map[string]any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		record, err := q.GetUploadRecord(context.Background(), id)
		if err == nil && record.Status == want && !record.ResultPending {
			hasResult, resultErr := q.HasValidationResult(context.Background(), id)
			if resultErr == nil && hasResult == requireResult {
				response := doRequest(handler, http.MethodGet, "/api/v1/ubl/"+id, token, "", "")
				if response.Code != http.StatusOK {
					t.Fatalf("get %s = %d; body=%s", id, response.Code, response.Body.String())
				}
				var body map[string]any
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				return body
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	record, err := q.GetUploadRecord(context.Background(), id)
	t.Fatalf("upload %s did not reach durable %q completion: record=%+v err=%v", id, want, record, err)
	return nil
}

func assertHTTPENIssue(t *testing.T, response map[string]any, wantRule string) {
	t.Helper()
	stages := response["validationStages"].([]any)
	en := stages[1].(map[string]any)
	issues, ok := en["issues"].([]any)
	if !ok || len(issues) == 0 {
		t.Fatalf("EN stage has no API diagnostics: %#v", en)
	}
	for _, raw := range issues {
		issue, ok := raw.(map[string]any)
		if ok && issue["rule"] == wantRule {
			severity, severityOK := issue["severity"].(string)
			message, messageOK := issue["message"].(string)
			if !severityOK || severity == "" || !messageOK || message == "" || issue["kind"] != string(queue.ValidationIssueFailedAssert) || issue["layer"] != string(queue.ValidationLayerEN16931) {
				t.Fatalf("EN issue lost structured fields: %#v", issue)
			}
			return
		}
	}
	t.Fatalf("API diagnostics do not contain expected rule %q: %#v", wantRule, issues)
}

func assertOnePermanentResult(t *testing.T, q *queue.Client, id string, wantStatus queue.ValidationStatus) {
	t.Helper()
	stream, err := q.JS.Stream(context.Background(), q.Namespace.ResultsStreamName())
	if err != nil {
		t.Fatal(err)
	}
	info, err := stream.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.State.Msgs != 1 {
		t.Fatalf("permanent result stream contains %d messages, want exactly 1", info.State.Msgs)
	}
	message, err := stream.GetLastMsgForSubject(context.Background(), q.Namespace.ResultSubject(id))
	if err != nil {
		t.Fatal(err)
	}
	var result queue.ValidationResult
	if err := json.Unmarshal(message.Data, &result); err != nil {
		t.Fatal(err)
	}
	if result.ID != id || result.Status != wantStatus || !terminalValidationStagesComplete(queue.UploadRecord{DocumentType: result.DocumentType, Status: result.Status, ValidationStages: result.ValidationStages}) {
		t.Fatalf("permanent validation result = %+v", result)
	}
}
