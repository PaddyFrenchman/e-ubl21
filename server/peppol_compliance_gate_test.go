package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/orchestrator"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/regression"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
	"github.com/nats-io/nats.go"
)

// TestPeppolPublicComplianceScenarios is the final semantic gate for the
// public Peppol contract. Every fixture crosses the authenticated HTTP API,
// Object Store, three independent stage workers, JetStream evidence stores,
// orchestrator, and permanent result stream before comparison with the
// accepted release baseline.
func TestPeppolPublicComplianceScenarios(t *testing.T) {
	topology := newSplitPeppolComplianceTopology(t)
	stopPeppol := topology.startPeppol(t, nil)
	defer stopPeppol()
	manifest := readPeppolComplianceManifest(t)

	public := httptest.NewServer(topology.handler)
	defer public.Close()
	output := t.TempDir()
	summary, err := regression.RunCorpus(context.Background(), regression.RunnerConfig{
		BaseURL: public.URL, Token: topology.token,
		ManifestPath: filepath.Join("..", "createsamples", "samples-peppol", "manifest.json"),
		OutputDir:    output, Profile: "all", PollInterval: 20 * time.Millisecond, FixtureTimeout: 60 * time.Second,
	})
	if err != nil {
		t.Fatalf("public Peppol corpus: %v", err)
	}
	if summary.Total != 30 || summary.Passed != 30 || summary.Failed != 0 {
		t.Fatalf("public Peppol corpus summary = %+v", summary)
	}
	equivalence, err := regression.CompareResultSets(
		filepath.Join("..", "regression", "baselines", "peppol", "results.jsonl"),
		filepath.Join(output, "results.jsonl"),
	)
	if err != nil {
		for _, path := range []string{
			filepath.Join("..", "regression", "baselines", "peppol", "results.jsonl"),
			filepath.Join(output, "results.jsonl"),
		} {
			content, _ := os.ReadFile(path)
			for _, line := range strings.Split(string(content), "\n") {
				for _, id := range equivalence.ChangedFixtureIDs {
					if strings.Contains(line, `"fixtureId":"`+id+`"`) {
						t.Logf("%s: %s", path, line)
					}
				}
			}
		}
		t.Fatalf("normalized Peppol release baseline: %v (%+v)", err, equivalence)
	}

	for _, id := range []string{
		"peppol.invoice.profile-01", "peppol.invoice.profile-02",
		"peppol.creditnote.profile-01", "peppol.creditnote.profile-02",
	} {
		fixture := peppolComplianceFixture(t, manifest, id)
		response, uploadID := topology.uploadFixture(t, fixture, fixture.Profile)
		assertPeppolHTTPStages(t, response, queue.StatusValid, true, true, true)
		topology.assertOneExecutionResult(t, uploadID, queue.StatusValid)
	}

	t.Run("explicit Peppol XSD failure short-circuits EN and Peppol", func(t *testing.T) {
		fixture := peppolComplianceFixture(t, manifest, "peppol.invoice.control-xsd-missing-invoice-id")
		enBefore := topology.stageRequestLastSequence(t, queue.ValidationLayerEN16931)
		peppolBefore := topology.stageRequestLastSequence(t, queue.ValidationLayerPeppolBISBilling)
		response, uploadID := topology.uploadFixture(t, fixture, string(queue.ValidationProfilePeppolBISBilling))
		assertPeppolHTTPStages(t, response, queue.StatusInvalid, false, false, false)
		if got := topology.stageRequestLastSequence(t, queue.ValidationLayerEN16931); got != enBefore {
			t.Fatalf("EN request sequence advanced from %d to %d", enBefore, got)
		}
		if got := topology.stageRequestLastSequence(t, queue.ValidationLayerPeppolBISBilling); got != peppolBefore {
			t.Fatalf("Peppol request sequence advanced from %d to %d", peppolBefore, got)
		}
		topology.assertOneExecutionResult(t, uploadID, queue.StatusInvalid)
	})

	t.Run("explicit Peppol EN failure short-circuits Peppol", func(t *testing.T) {
		fixture := peppolComplianceFixture(t, manifest, "peppol.invoice.control-en-unsupported-attachment-mime")
		before := topology.stageRequestLastSequence(t, queue.ValidationLayerPeppolBISBilling)
		response, uploadID := topology.uploadFixture(t, fixture, string(queue.ValidationProfilePeppolBISBilling))
		assertPeppolHTTPStages(t, response, queue.StatusInvalid, true, false, false)
		if got := topology.stageRequestLastSequence(t, queue.ValidationLayerPeppolBISBilling); got != before {
			t.Fatalf("Peppol request sequence advanced from %d to %d", before, got)
		}
		topology.assertOneExecutionResult(t, uploadID, queue.StatusInvalid)
	})

	for _, id := range []string{
		"peppol.invoice.invalid-endpoint-scheme",
		"peppol.invoice.missing-profile-id",
		"peppol.invoice.invalid-customization-id",
		"peppol.invoice.unsupported-profile-id",
	} {
		t.Run("explicit profile exposes "+id, func(t *testing.T) {
			fixture := peppolComplianceFixture(t, manifest, id)
			response, uploadID := topology.uploadFixture(t, fixture, string(queue.ValidationProfilePeppolBISBilling))
			assertPeppolHTTPStages(t, response, queue.StatusInvalid, true, true, false)
			assertPeppolHTTPRule(t, response, fixture.ExpectedStages[2].RuleIDs[0])
			topology.assertOneExecutionResult(t, uploadID, queue.StatusInvalid)
		})
	}

	t.Run("EN-only remains backward compatible without a Peppol claim", func(t *testing.T) {
		fixture := peppolComplianceFixture(t, manifest, "peppol.invoice.profile-01")
		response, uploadID := topology.uploadFixture(t, fixture, string(queue.ValidationProfileUBL21EN16931))
		assertPeppolHTTPStages(t, response, queue.StatusValid, true, false, false)
		if response["profile"] == string(queue.ValidationProfilePeppolBISBilling) || response["resolvedProfile"] == string(queue.ValidationProfilePeppolBISBilling) {
			t.Fatalf("EN-only response was labelled Peppol-valid: %#v", response)
		}
		topology.assertOneExecutionResult(t, uploadID, queue.StatusValid)
	})
}

// TestPeppolEnvelopeFourWorkerAcceptance crosses the authenticated API,
// immutable Object Store, all four real stage workers, JetStream evidence,
// orchestrator, and the permanent result projection with one SBDH document.
func TestPeppolEnvelopeFourWorkerAcceptance(t *testing.T) {
	topology := newSplitPeppolComplianceTopology(t)
	stopPeppol := topology.startPeppol(t, nil)
	defer stopPeppol()
	stopEnvelope := topology.startEnvelope(t)
	defer stopEnvelope()

	payload, err := os.ReadFile(filepath.Join("..", "createsamples", "samples-peppol", "Invoice", "Peppol-Invoice-Profile-01.xml"))
	if err != nil {
		t.Fatal(err)
	}
	identity, err := envelopePayloadIdentity(payload)
	if err != nil {
		t.Fatal(err)
	}
	document := []byte(testEnvelope(string(payload), identity, ""))
	uploadID := uploadComplianceDocumentWithProfile(t, topology.handler, topology.token, document, "Invoice", string(queue.ValidationProfilePeppolEnvelope))
	response := waitForSplitComplianceStatus(t, topology.handler, topology.token, uploadID, queue.StatusValid, 60*time.Second)
	stages, ok := response["validationStages"].([]any)
	if !ok || len(stages) != 4 {
		t.Fatalf("four-worker validation stages = %#v", response["validationStages"])
	}
	want := []queue.ValidationLayer{
		queue.ValidationLayerUBL21XSD,
		queue.ValidationLayerEN16931,
		queue.ValidationLayerPeppolBISBilling,
		queue.ValidationLayerPeppolEnvelope,
	}
	for index, layer := range want {
		stage, ok := stages[index].(map[string]any)
		if !ok || stage["layer"] != string(layer) || stage["valid"] != true {
			t.Fatalf("stage %d = %#v, want passing %s", index+1, stages[index], layer)
		}
	}
	stored, err := topology.queue.GetUploadContent(context.Background(), uploadID)
	if err != nil || !bytes.Equal(stored, document) {
		t.Fatalf("stored SBDH changed: err=%v", err)
	}
	topology.assertOneExecutionResult(t, uploadID, queue.StatusValid)
}

// TestPeppolOperationalFailureReplayEndToEnd proves retry exhaustion, the
// stage-specific DLQ, absence of false standards evidence, and replay from the
// failed Peppol stage while preserving passing XSD/EN evidence.
func TestPeppolOperationalFailureReplayEndToEnd(t *testing.T) {
	topology := newSplitPeppolComplianceTopology(t)
	stopFailed := topology.startPeppol(t, peppolValidatorFunc(func(_ context.Context, input io.Reader) (queue.ValidationStageResult, error) {
		_, _ = io.Copy(io.Discard, input)
		return queue.ValidationStageResult{}, errors.New("Peppol runtime unavailable")
	}))
	manifest := readPeppolComplianceManifest(t)
	fixture := peppolComplianceFixture(t, manifest, "peppol.invoice.profile-01")
	content := readPeppolComplianceFixture(t, fixture)
	uploadID := uploadComplianceDocumentWithProfile(t, topology.handler, topology.token, content, fixture.DocumentType, string(queue.ValidationProfilePeppolBISBilling))
	failed := waitForSplitComplianceStatus(t, topology.handler, topology.token, uploadID, queue.StatusProcessingFailed, 30*time.Second)
	stages, ok := failed["validationStages"].([]any)
	if !ok || len(stages) != 3 {
		t.Fatalf("operational failure stages = %#v, want XSD/EN evidence and Peppol failure", failed["validationStages"])
	}
	peppolStage, ok := stages[2].(map[string]any)
	if !ok || peppolStage["layer"] != string(queue.ValidationLayerPeppolBISBilling) || peppolStage["outcome"] != string(queue.StageOutcomeOperationalFailure) {
		t.Fatalf("Peppol operational evidence = %#v", stages[2])
	}
	executions, err := topology.queue.ValidationExecutionsForUpload(context.Background(), uploadID)
	if err != nil || len(executions) != 1 {
		t.Fatalf("failed execution history = %+v, err=%v", executions, err)
	}
	failedExecution := executions[0]
	if failedExecution.Orchestration.Status != queue.ExecutionStatusProcessingFailed || len(failedExecution.Orchestration.Stages) != 3 {
		t.Fatalf("failed Peppol execution = %+v", failedExecution)
	}
	peppolState := failedExecution.Orchestration.Stages[2]
	if len(peppolState.Attempts) != int(failedExecution.RetryPolicy.MaxAttemptsFor(queue.ValidationLayerPeppolBISBilling)) {
		t.Fatalf("Peppol attempts = %d, want frozen retry budget", len(peppolState.Attempts))
	}
	if depth, depthErr := topology.queue.StageDeadLetterDepth(context.Background(), queue.ValidationLayerPeppolBISBilling); depthErr != nil || depth != 1 {
		t.Fatalf("Peppol DLQ depth = %d, err=%v", depth, depthErr)
	}
	dead := topology.lastPeppolDeadLetter(t)
	stopFailed()
	stopRestored := topology.startPeppol(t, nil)
	defer stopRestored()
	if _, err := orchestrator.New(topology.queue).ReplayStage(context.Background(), dead); err != nil {
		t.Fatalf("replay Peppol stage: %v", err)
	}
	replayed := waitForSplitComplianceStatus(t, topology.handler, topology.token, uploadID, queue.StatusValid, 30*time.Second)
	assertPeppolHTTPStages(t, replayed, queue.StatusValid, true, true, true)
	executions, err = topology.queue.ValidationExecutionsForUpload(context.Background(), uploadID)
	if err != nil || len(executions) != 2 || executions[1].Generation != 2 {
		t.Fatalf("replayed execution history = %+v, err=%v", executions, err)
	}
	if len(executions[1].Orchestration.Stages) != 3 || executions[1].Orchestration.Stages[0].InheritedEvidence == nil || executions[1].Orchestration.Stages[1].InheritedEvidence == nil {
		t.Fatalf("replay did not preserve XSD/EN evidence: %+v", executions[1].Orchestration.Stages)
	}
	if depth, depthErr := topology.queue.StageDeadLetterDepth(context.Background(), queue.ValidationLayerPeppolBISBilling); depthErr != nil || depth != 0 {
		t.Fatalf("Peppol DLQ depth after replay = %d, err=%v", depth, depthErr)
	}
	topology.assertExecutionResultCount(t, uploadID, 2)
}

// TestPeppolCrashRestartAndDuplicateEndToEnd interrupts an active Peppol
// worker, proves no partial public result escapes, then verifies replacement
// recovery and late duplicate request/result convergence.
func TestPeppolCrashRestartAndDuplicateEndToEnd(t *testing.T) {
	topology := newSplitPeppolComplianceTopology(t)
	entered := make(chan struct{})
	stopInterrupted := topology.startPeppol(t, peppolValidatorFunc(func(ctx context.Context, input io.Reader) (queue.ValidationStageResult, error) {
		_, _ = io.Copy(io.Discard, input)
		close(entered)
		<-ctx.Done()
		return queue.ValidationStageResult{}, ctx.Err()
	}))
	manifest := readPeppolComplianceManifest(t)
	fixture := peppolComplianceFixture(t, manifest, "peppol.creditnote.profile-02")
	uploadID := uploadComplianceDocumentWithProfile(t, topology.handler, topology.token, readPeppolComplianceFixture(t, fixture), fixture.DocumentType, string(queue.ValidationProfilePeppolBISBilling))
	select {
	case <-entered:
	case <-time.After(20 * time.Second):
		t.Fatal("interrupted worker did not enter Peppol validation")
	}
	response := doRequest(topology.handler, http.MethodGet, "/api/v1/ubl/"+uploadID, topology.token, "", "")
	if response.Code != http.StatusOK {
		t.Fatalf("pending status = %d: %s", response.Code, response.Body.String())
	}
	var pending map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &pending); err != nil {
		t.Fatal(err)
	}
	if pending["status"] != string(queue.StatusPending) {
		t.Fatalf("active Peppol validation exposed partial terminal state: %#v", pending)
	}
	stopInterrupted()
	stopReplacement := topology.startPeppol(t, nil)
	defer stopReplacement()
	completed := waitForSplitComplianceStatus(t, topology.handler, topology.token, uploadID, queue.StatusValid, 30*time.Second)
	assertPeppolHTTPStages(t, completed, queue.StatusValid, true, true, true)

	executions, err := topology.queue.ValidationExecutionsForUpload(context.Background(), uploadID)
	if err != nil || len(executions) != 1 {
		t.Fatalf("execution history = %+v, err=%v", executions, err)
	}
	execution := executions[0]
	stage := execution.Orchestration.Stages[2]
	evidence, err := topology.queue.GetValidationStageEvidence(context.Background(), uploadID, execution.ValidationID, execution.Generation, 3, stage.AuthorizedAttempt)
	if err != nil {
		t.Fatal(err)
	}
	topology.publishLateDuplicate(t, execution, evidence)
	topology.waitForStageQueuesToDrain(t, queue.ValidationLayerPeppolBISBilling)
	topology.assertExecutionResultCount(t, uploadID, 1)
	stored, err := topology.queue.GetValidationExecution(context.Background(), uploadID, execution.ValidationID, execution.Generation)
	if err != nil || stored.Orchestration.FinalResultID != execution.Orchestration.FinalResultID {
		t.Fatalf("duplicates changed terminal execution: %+v, err=%v", stored, err)
	}
}

type splitPeppolComplianceTopology struct {
	cfg     Config
	handler http.Handler
	token   string
	queue   *queue.Client
	runtime saxonRuntimeConfig
}

func newSplitPeppolComplianceTopology(t *testing.T) *splitPeppolComplianceTopology {
	t.Helper()
	runtime, ok := localSaxonRuntimeConfig(t)
	if !ok {
		t.Skip("pinned Saxon runtime is unavailable")
	}
	if err := ensureXMLLint(); err != nil {
		if os.Getenv("EUBL21_REQUIRE_EN16931_TEST_RUNTIME") == "1" {
			t.Fatalf("mandatory compliance runtime: %v", err)
		}
		t.Skip(err)
	}
	cfg := testConfig(t, nil)
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	client, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	ctx := context.Background()
	if err := client.EnsureInfraWithConfig(ctx, queue.InfraConfig{UploadContentMaxBytes: cfg.UploadContentMaxBytes}); err != nil {
		t.Fatal(err)
	}
	if err := client.ProvisionValidationExecutionInfra(ctx); err != nil {
		t.Fatal(err)
	}
	if err := client.ProvisionStageQueueInfra(ctx, queue.DefaultStageQueueInfraConfig()); err != nil {
		t.Fatal(err)
	}
	legacy, err := client.ProvisionDeploymentGenerationInfra(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	draining, err := client.TransitionDeploymentMode(ctx, legacy, queue.DeploymentModeDraining, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	activating, err := client.TransitionDeploymentMode(ctx, draining, queue.DeploymentModeActivating, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.TransitionDeploymentMode(ctx, activating, queue.DeploymentModeSplit, time.Now()); err != nil {
		t.Fatal(err)
	}
	topology := &splitPeppolComplianceTopology{
		cfg: cfg, handler: newApplicationHandler(t, cfg),
		token: signedJWT(t, cfg.OAuthIssuer, cfg.OAuthAudience, testSecret, time.Hour), queue: client, runtime: runtime,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	topology.startComponent(t, "orchestrator", func(runCtx context.Context) error {
		return orchestrator.RunService(runCtx, orchestrator.ServiceConfig{
			NATSURL: cfg.NATSURL, NATSOptions: cfg.NATSOptions, QueueNamespace: cfg.QueueNamespace,
			MetricsAddr: "", ReconciliationInterval: 10 * time.Millisecond, Logger: logger,
		})
	})
	topology.startComponent(t, "worker-xsd", func(runCtx context.Context) error {
		return RunXSDWorker(runCtx, XSDWorkerConfig{
			NATSURL: cfg.NATSURL, NATSOptions: cfg.NATSOptions, QueueNamespace: cfg.QueueNamespace,
			MetricsAddr: "", ValidationTimeout: 20 * time.Second, Concurrency: 1, Prefetch: 1,
			ProgressInterval: 10 * time.Second, Logger: logger,
		})
	})
	topology.startComponent(t, "worker-en16931", func(runCtx context.Context) error {
		return RunEN16931Worker(runCtx, EN16931WorkerConfig{
			NATSURL: cfg.NATSURL, NATSOptions: cfg.NATSOptions, QueueNamespace: cfg.QueueNamespace,
			MetricsAddr: "", ValidationTimeout: 20 * time.Second, Concurrency: 1, Prefetch: 1,
			ProgressInterval: 10 * time.Second, JavaExecutable: runtime.javaExecutable,
			SaxonJARPath: runtime.saxonJARPath, XMLResolverJARPath: runtime.xmlResolverJARPath,
			SaxonConfigurationPath: runtime.configurationPath, EN16931XSLTPath: runtime.xsltPath,
			SaxonStartupTimeout: 10 * time.Second, Logger: logger,
		})
	})
	return topology
}

func (topology *splitPeppolComplianceTopology) startPeppol(t *testing.T, validator PeppolValidator) func() {
	t.Helper()
	profile := validationassets.Peppol()
	return topology.startComponent(t, "worker-peppol", func(runCtx context.Context) error {
		return RunPeppolWorker(runCtx, PeppolWorkerConfig{
			NATSURL: topology.cfg.NATSURL, NATSOptions: topology.cfg.NATSOptions, QueueNamespace: topology.cfg.QueueNamespace,
			MetricsAddr: "", ValidationTimeout: 20 * time.Second, Concurrency: 1, Prefetch: 1,
			ProgressInterval: 10 * time.Second, JavaExecutable: topology.runtime.javaExecutable,
			SaxonJARPath: topology.runtime.saxonJARPath, XMLResolverJARPath: topology.runtime.xmlResolverJARPath,
			SaxonConfigurationPath: topology.runtime.configurationPath,
			PeppolXSLTPath:         filepath.Join("..", "validation-assets", filepath.FromSlash(profile.EmbeddedXSLTPath)),
			SaxonStartupTimeout:    10 * time.Second, validator: validator,
			Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		})
	})
}

func (topology *splitPeppolComplianceTopology) startEnvelope(t *testing.T) func() {
	t.Helper()
	return topology.startComponent(t, "worker-peppol-envelope", func(runCtx context.Context) error {
		return RunPeppolEnvelopeWorker(runCtx, PeppolEnvelopeWorkerConfig{
			NATSURL: topology.cfg.NATSURL, NATSOptions: topology.cfg.NATSOptions, QueueNamespace: topology.cfg.QueueNamespace,
			MetricsAddr: "", ValidationTimeout: 20 * time.Second, Concurrency: 1, Prefetch: 1,
			ProgressInterval: 10 * time.Second, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		})
	})
}

func (topology *splitPeppolComplianceTopology) startComponent(t *testing.T, name string, run func(context.Context) error) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx) }()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Errorf("%s stopped with error: %v", name, err)
				}
			case <-time.After(10 * time.Second):
				t.Errorf("%s did not stop", name)
			}
		})
	}
	t.Cleanup(stop)
	select {
	case err := <-done:
		cancel()
		t.Fatalf("%s failed during startup: %v", name, err)
	case <-time.After(50 * time.Millisecond):
	}
	return stop
}

func readPeppolComplianceManifest(t *testing.T) regression.Manifest {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "createsamples", "samples-peppol", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest regression.Manifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func peppolComplianceFixture(t *testing.T, manifest regression.Manifest, id string) regression.Fixture {
	t.Helper()
	for _, fixture := range manifest.Fixtures {
		if fixture.ID == id {
			return fixture
		}
	}
	t.Fatalf("Peppol fixture %s is missing", id)
	return regression.Fixture{}
}

func readPeppolComplianceFixture(t *testing.T, fixture regression.Fixture) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "createsamples", "samples-peppol", filepath.FromSlash(fixture.File)))
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func (topology *splitPeppolComplianceTopology) uploadFixture(t *testing.T, fixture regression.Fixture, profile string) (map[string]any, string) {
	t.Helper()
	id := uploadComplianceDocumentWithProfile(t, topology.handler, topology.token, readPeppolComplianceFixture(t, fixture), fixture.DocumentType, profile)
	response := waitForSplitComplianceStatus(t, topology.handler, topology.token, id, queue.ValidationStatus(fixture.ExpectedFinal), 30*time.Second)
	return response, id
}

func waitForSplitComplianceStatus(t *testing.T, handler http.Handler, token, id string, want queue.ValidationStatus, timeout time.Duration) map[string]any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last map[string]any
	for time.Now().Before(deadline) {
		response := doRequest(handler, http.MethodGet, "/api/v1/ubl/"+id, token, "", "")
		if response.Code == http.StatusOK {
			last = nil
			if err := json.Unmarshal(response.Body.Bytes(), &last); err != nil {
				t.Fatal(err)
			}
			if last["status"] == string(want) {
				return last
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("upload %s did not reach public durable %q completion: last=%#v", id, want, last)
	return nil
}

func uploadComplianceDocumentWithProfile(t *testing.T, handler http.Handler, token string, body []byte, documentType, profile string) string {
	t.Helper()
	path := fmt.Sprintf("/api/v1/ubl/upload?documentType=%s&profile=%s", documentType, profile)
	response := doRequest(handler, http.MethodPost, path, token, string(body), "application/xml")
	if response.Code != http.StatusAccepted {
		t.Fatalf("%s/%s upload = %d, want %d; body=%s", documentType, profile, response.Code, http.StatusAccepted, response.Body.String())
	}
	var accepted map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	id, ok := accepted["id"].(string)
	if !ok || !queue.ValidID(id) || accepted["requestedProfile"] != profile {
		t.Fatalf("accepted upload = %#v", accepted)
	}
	return id
}

func assertPeppolHTTPStages(t *testing.T, response map[string]any, status queue.ValidationStatus, enPresent, peppolPresent, peppolValid bool) {
	t.Helper()
	if response["status"] != string(status) {
		t.Fatalf("status response = %#v", response)
	}
	stages, ok := response["validationStages"].([]any)
	if !ok || len(stages) == 0 {
		t.Fatalf("validationStages = %#v", response["validationStages"])
	}
	want := 1
	if enPresent {
		want++
	}
	if peppolPresent {
		want++
	}
	if len(stages) != want {
		t.Fatalf("validationStages = %#v, want %d stages", stages, want)
	}
	if stages[0].(map[string]any)["layer"] != string(queue.ValidationLayerUBL21XSD) {
		t.Fatalf("first stage = %#v", stages[0])
	}
	if peppolPresent {
		stage := stages[len(stages)-1].(map[string]any)
		if stage["layer"] != string(queue.ValidationLayerPeppolBISBilling) || stage["valid"] != peppolValid {
			t.Fatalf("Peppol stage = %#v", stage)
		}
	}
}

func assertPeppolHTTPRule(t *testing.T, response map[string]any, want string) {
	t.Helper()
	stages := response["validationStages"].([]any)
	stage := stages[len(stages)-1].(map[string]any)
	for _, raw := range stage["issues"].([]any) {
		issue := raw.(map[string]any)
		if issue["rule"] == want {
			if issue["layer"] != string(queue.ValidationLayerPeppolBISBilling) || issue["kind"] != string(queue.ValidationIssueFailedAssert) || issue["severity"] == "" || issue["message"] == "" {
				t.Fatalf("Peppol issue lost structured fields: %#v", issue)
			}
			return
		}
	}
	t.Fatalf("Peppol response does not contain rule %s: %#v", want, stage)
}

func (topology *splitPeppolComplianceTopology) stageRequestLastSequence(t *testing.T, stage queue.ValidationLayer) uint64 {
	t.Helper()
	names, err := topology.queue.Namespace.StageQueue(stage)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := topology.queue.JS.Stream(context.Background(), names.RequestStream)
	if err != nil {
		t.Fatal(err)
	}
	info, err := stream.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return info.State.LastSeq
}

func (topology *splitPeppolComplianceTopology) assertOneExecutionResult(t *testing.T, uploadID string, status queue.ValidationStatus) {
	t.Helper()
	executions, err := topology.queue.ValidationExecutionsForUpload(context.Background(), uploadID)
	if err != nil || len(executions) != 1 {
		t.Fatalf("execution history = %+v, err=%v", executions, err)
	}
	result, err := topology.queue.GetValidationExecutionResult(context.Background(), executions[0].ValidationID, executions[0].Generation)
	if err != nil || result.Status != status || result.ID != uploadID {
		t.Fatalf("permanent execution result = %+v, err=%v", result, err)
	}
	topology.assertExecutionResultCount(t, uploadID, 1)
}

func (topology *splitPeppolComplianceTopology) assertExecutionResultCount(t *testing.T, uploadID string, want int) {
	t.Helper()
	executions, err := topology.queue.ValidationExecutionsForUpload(context.Background(), uploadID)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, execution := range executions {
		if _, err := topology.queue.GetValidationExecutionResult(context.Background(), execution.ValidationID, execution.Generation); err == nil {
			count++
		}
	}
	if count != want {
		t.Fatalf("upload %s has %d permanent generation results, want %d", uploadID, count, want)
	}
}

func (topology *splitPeppolComplianceTopology) lastPeppolDeadLetter(t *testing.T) queue.StageDeadLetter {
	t.Helper()
	names, _ := topology.queue.Namespace.StageQueue(queue.ValidationLayerPeppolBISBilling)
	stream, err := topology.queue.JS.Stream(context.Background(), names.DeadLetterStream)
	if err != nil {
		t.Fatal(err)
	}
	info, err := stream.Info(context.Background())
	if err != nil || info.State.LastSeq == 0 {
		t.Fatalf("Peppol DLQ state = %+v, err=%v", info, err)
	}
	message, err := stream.GetMsg(context.Background(), info.State.LastSeq)
	if err != nil {
		t.Fatal(err)
	}
	var dead queue.StageDeadLetter
	if err := json.Unmarshal(message.Data, &dead); err != nil {
		t.Fatal(err)
	}
	return dead
}

func (topology *splitPeppolComplianceTopology) publishLateDuplicate(t *testing.T, execution queue.ValidationExecution, evidence queue.StageResult) {
	t.Helper()
	names, _ := topology.queue.Namespace.StageQueue(queue.ValidationLayerPeppolBISBilling)
	payload, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	resultMessage := nats.NewMsg(names.ResultSubject)
	resultMessage.Data = payload
	resultMessage.Header.Set(nats.MsgIdHdr, evidence.ResultID+"-late-duplicate")
	if _, err := topology.queue.JS.PublishMsg(context.Background(), resultMessage); err != nil {
		t.Fatal(err)
	}
	stage := execution.Orchestration.Stages[2]
	createdAt, err := time.Parse(time.RFC3339Nano, stage.RequestCreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	request, err := queue.NewStageRequest(execution, 3, stage.AuthorizedAttempt, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	payload, err = json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	requestMessage := nats.NewMsg(names.RequestSubject)
	requestMessage.Data = payload
	requestMessage.Header.Set(nats.MsgIdHdr, request.RequestID+"-late-duplicate")
	if _, err := topology.queue.JS.PublishMsg(context.Background(), requestMessage); err != nil {
		t.Fatal(err)
	}
}

func (topology *splitPeppolComplianceTopology) waitForStageQueuesToDrain(t *testing.T, stage queue.ValidationLayer) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		names, _ := topology.queue.Namespace.StageQueue(stage)
		requestStream, requestErr := topology.queue.JS.Stream(context.Background(), names.RequestStream)
		resultStream, resultErr := topology.queue.JS.Stream(context.Background(), names.ResultStream)
		if requestErr == nil && resultErr == nil {
			requestInfo, _ := requestStream.Info(context.Background())
			resultInfo, _ := resultStream.Info(context.Background())
			if requestInfo != nil && resultInfo != nil && requestInfo.State.Msgs == 0 && resultInfo.State.Msgs == 0 {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("duplicate Peppol stage queues did not drain")
}
