package regression

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type Issue65GateConfig struct {
	BaseURL             string
	Token               string
	ManifestPath        string
	// XSDInvalidManifestPath optionally names a second manifest searched for
	// an XSD-invalid fixture when the primary manifest has none. The fixture
	// is re-profiled to ubl21-en16931 so the plan includes EN, which must
	// then never be requested.
	XSDInvalidManifestPath string
	OutputDir              string
	NATSURL                string
	QueueNamespace         string
	KubectlBin             string
	Namespace              string
	ENDeployment           string
	EquivalenceBaseline    string
	PollInterval           time.Duration
	FixtureTimeout         time.Duration
}

type issue65Env struct {
	cfg        Issue65GateConfig
	baseURL    *url.URL
	corpusRoot string
	manifest   Manifest
}

// RunIssue65Gate proves the issue #65 acceptance scenarios against a live
// split deployment reachable via cfg.BaseURL/cfg.NATSURL/cfg.Namespace:
//  1. valid EN 16931 plan: public upload -> XSD PASS -> EN PASS -> final valid
//  2. XSD-invalid: final invalid and the EN request stream receives nothing
//  3. EN-invalid: XSD PASS -> EN standards-invalid -> final invalid
//  4. EN scale-to-zero: requests accumulate durably, restoring workers drains
//     the backlog and every outage upload reaches its expected terminal state
//  5. frozen semantic-equivalence corpus matches the monolithic baseline
func RunIssue65Gate(ctx context.Context, cfg Issue65GateConfig) (*Summary, error) {
	env, err := newIssue65Env(cfg)
	if err != nil {
		return &Summary{}, err
	}

	scenarios := []struct {
		name string
		run  func(context.Context, *issue65Env) error
	}{
		{"valid plan completes through split pipeline", scenarioValidPlan},
		{"XSD-invalid finalizes without any EN request", scenarioXSDInvalidStopsBeforeEN},
		{"EN-invalid after XSD pass finalizes invalid", scenarioENInvalidAfterXSDPass},
		{"EN scale-to-zero accumulates then drains", scenarioENScaleToZero},
		{"EN worker crash mid-flight redelivers and finalizes exactly once", scenarioENWorkerCrashRedelivery},
		{"frozen semantic-equivalence corpus", scenarioSemanticEquivalence},
	}

	summary := &Summary{SchemaVersion: 1, Total: len(scenarios), ByProfile: map[string]Counts{}}
	var failures []string
	for index, scenario := range scenarios {
		fmt.Printf("\n[%d/%d] %s\n", index+1, len(scenarios), scenario.name)
		if err := scenario.run(ctx, env); err != nil {
			fmt.Printf("  FAIL: %v\n", err)
			summary.Failed++
			failures = append(failures, fmt.Sprintf("%s: %v", scenario.name, err))
			continue
		}
		fmt.Println("  PASS")
		summary.Passed++
	}

	if len(failures) > 0 {
		return summary, fmt.Errorf("issue #65 gate failed %d of %d scenarios:\n  %s",
			summary.Failed, summary.Total, strings.Join(failures, "\n  "))
	}
	return summary, nil
}

func newIssue65Env(cfg Issue65GateConfig) (*issue65Env, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.Token) == "" {
		return nil, errors.New("base URL and bearer token are required")
	}
	if cfg.ManifestPath == "" || cfg.OutputDir == "" {
		return nil, errors.New("manifest path and output directory are required")
	}
	if strings.TrimSpace(cfg.NATSURL) == "" {
		return nil, errors.New("NATS URL is required for stream assertions")
	}
	if strings.TrimSpace(cfg.KubectlBin) == "" || strings.TrimSpace(cfg.Namespace) == "" {
		return nil, errors.New("kubectl binary and Kubernetes namespace are required")
	}
	if strings.TrimSpace(cfg.EquivalenceBaseline) == "" {
		return nil, errors.New("frozen equivalence baseline path is required")
	}
	if cfg.ENDeployment == "" {
		cfg.ENDeployment = "e-ubl21-worker-en16931"
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultPollInterval
	}
	if cfg.FixtureTimeout <= 0 {
		cfg.FixtureTimeout = defaultFixtureTimeout
	}
	baseURL, err := url.Parse(strings.TrimRight(cfg.BaseURL, "/"))
	if err != nil || (baseURL.Scheme != "http" && baseURL.Scheme != "https") || baseURL.Host == "" {
		return nil, fmt.Errorf("invalid base URL %q", cfg.BaseURL)
	}
	manifestBytes, err := os.ReadFile(cfg.ManifestPath)
	if err != nil {
		return nil, fmt.Errorf("read fixture manifest: %w", err)
	}
	if err := ValidateFixtureManifest(manifestBytes); err != nil {
		return nil, err
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, fmt.Errorf("decode fixture manifest: %w", err)
	}
	return &issue65Env{
		cfg:        cfg,
		baseURL:    baseURL,
		corpusRoot: filepath.Dir(cfg.ManifestPath),
		manifest:   manifest,
	}, nil
}

// Fixture selection is deterministic: it keys off the manifest's declared
// expected stages, never off file-name conventions.

func (e *issue65Env) findFixture(match func(Fixture) bool, description string) (Fixture, error) {
	for _, fixture := range e.manifest.Fixtures {
		if match(fixture) {
			return fixture, nil
		}
	}
	return Fixture{}, fmt.Errorf("manifest %s has no fixture that is %s", e.cfg.ManifestPath, description)
}

func fixtureIsValidTwoStage(f Fixture) bool {
	return f.ExpectedFinal == "valid" &&
		len(f.ExpectedStages) == 2 &&
		f.ExpectedStages[0].Stage == "ubl21-xsd" && f.ExpectedStages[0].Outcome == "passed" &&
		f.ExpectedStages[1].Stage == "en16931" && f.ExpectedStages[1].Outcome == "passed"
}

func fixtureIsXSDInvalid(f Fixture) bool {
	return f.ExpectedFinal == "invalid" &&
		len(f.ExpectedStages) == 1 &&
		f.ExpectedStages[0].Stage == "ubl21-xsd" && f.ExpectedStages[0].Outcome == "standards-invalid"
}

func fixtureIsENInvalid(f Fixture) bool {
	return f.ExpectedFinal == "invalid" &&
		len(f.ExpectedStages) == 2 &&
		f.ExpectedStages[0].Stage == "ubl21-xsd" && f.ExpectedStages[0].Outcome == "passed" &&
		f.ExpectedStages[1].Stage == "en16931" && f.ExpectedStages[1].Outcome == "standards-invalid"
}

func (e *issue65Env) readFixture(fixture Fixture) ([]byte, error) {
	return readFixtureFrom(e.corpusRoot, fixture)
}

func readFixtureFrom(corpusRoot string, fixture Fixture) ([]byte, error) {
	content, err := os.ReadFile(filepath.Join(corpusRoot, filepath.FromSlash(fixture.File)))
	if err != nil {
		return nil, fmt.Errorf("read fixture %s: %w", fixture.ID, err)
	}
	if actual := fmt.Sprintf("%x", sha256.Sum256(content)); actual != fixture.SHA256 {
		return nil, fmt.Errorf("fixture %s SHA-256 is %s, manifest has %s", fixture.ID, actual, fixture.SHA256)
	}
	return content, nil
}

// xsdInvalidFixture returns an XSD-invalid fixture and the corpus root its
// file lives under. If the primary manifest has none, the secondary manifest
// is searched and the fixture is re-profiled to ubl21-en16931 so that the
// validation plan contains the EN stage that must then never be requested.
func (e *issue65Env) xsdInvalidFixture() (Fixture, string, error) {
	if fixture, err := e.findFixture(fixtureIsXSDInvalid, ""); err == nil {
		return fixture, e.corpusRoot, nil
	}
	if e.cfg.XSDInvalidManifestPath == "" {
		return Fixture{}, "", fmt.Errorf("manifest %s has no XSD-invalid fixture and no --xsd-invalid-manifest was provided", e.cfg.ManifestPath)
	}
	manifestBytes, err := os.ReadFile(e.cfg.XSDInvalidManifestPath)
	if err != nil {
		return Fixture{}, "", fmt.Errorf("read XSD-invalid manifest: %w", err)
	}
	if err := ValidateFixtureManifest(manifestBytes); err != nil {
		return Fixture{}, "", err
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return Fixture{}, "", fmt.Errorf("decode XSD-invalid manifest: %w", err)
	}
	for _, fixture := range manifest.Fixtures {
		if fixtureIsXSDInvalid(fixture) {
			fixture.Profile = "ubl21-en16931"
			return fixture, filepath.Dir(e.cfg.XSDInvalidManifestPath), nil
		}
	}
	return Fixture{}, "", fmt.Errorf("neither %s nor %s contains an XSD-invalid fixture", e.cfg.ManifestPath, e.cfg.XSDInvalidManifestPath)
}

// uploadAndAwait pushes one fixture through the public API and returns the
// terminal API response together with the durable upload ID.
func (e *issue65Env) uploadAndAwait(ctx context.Context, fixture Fixture) (string, apiResponse, error) {
	return e.uploadAndAwaitFrom(ctx, e.corpusRoot, fixture)
}

func (e *issue65Env) uploadAndAwaitFrom(ctx context.Context, corpusRoot string, fixture Fixture) (string, apiResponse, error) {
	content, err := readFixtureFrom(corpusRoot, fixture)
	if err != nil {
		return "", apiResponse{}, err
	}
	fixtureCtx, cancel := context.WithTimeout(ctx, e.cfg.FixtureTimeout)
	defer cancel()
	id, err := uploadFixture(fixtureCtx, http.DefaultClient, e.baseURL, e.cfg.Token, fixture, content)
	if err != nil {
		return "", apiResponse{}, err
	}
	response, err := pollFixture(fixtureCtx, http.DefaultClient, e.baseURL, e.cfg.Token, id, e.cfg.PollInterval)
	if err != nil {
		return id, apiResponse{}, err
	}
	return id, response, nil
}

func (e *issue65Env) awaitTerminal(ctx context.Context, uploadID string) (apiResponse, error) {
	fixtureCtx, cancel := context.WithTimeout(ctx, e.cfg.FixtureTimeout)
	defer cancel()
	return pollFixture(fixtureCtx, http.DefaultClient, e.baseURL, e.cfg.Token, uploadID, e.cfg.PollInterval)
}

// Scenario 1: valid document travels XSD -> EN through the real workers and
// finalizes valid with both stage results present.
func scenarioValidPlan(ctx context.Context, e *issue65Env) error {
	fixture, err := e.findFixture(fixtureIsValidTwoStage, "expected-valid with XSD and EN stages")
	if err != nil {
		return err
	}
	_, response, err := e.uploadAndAwait(ctx, fixture)
	if err != nil {
		return err
	}
	if response.Status != "valid" {
		return fmt.Errorf("fixture %s: expected terminal valid, got %q", fixture.ID, response.Status)
	}
	if err := requireStageOutcomes(response, []expectedAPIStage{
		{"ubl21-xsd", true}, {"en16931", true},
	}); err != nil {
		return fmt.Errorf("fixture %s: %w", fixture.ID, err)
	}
	fmt.Printf("  fixture %s finalized valid with XSD and EN evidence\n", fixture.ID)
	return nil
}

// Scenario 2: XSD-invalid document finalizes invalid and the EN request
// stream's last sequence never moves, proving EN was not asked to work.
func scenarioXSDInvalidStopsBeforeEN(ctx context.Context, e *issue65Env) error {
	fixture, corpusRoot, err := e.xsdInvalidFixture()
	if err != nil {
		return err
	}
	before, err := e.enRequestStreamState(ctx)
	if err != nil {
		return err
	}
	_, response, err := e.uploadAndAwaitFrom(ctx, corpusRoot, fixture)
	if err != nil {
		return err
	}
	if response.Status != "invalid" {
		return fmt.Errorf("fixture %s: expected terminal invalid, got %q", fixture.ID, response.Status)
	}
	if err := requireStageOutcomes(response, []expectedAPIStage{{"ubl21-xsd", false}}); err != nil {
		return fmt.Errorf("fixture %s: %w", fixture.ID, err)
	}
	after, err := e.enRequestStreamState(ctx)
	if err != nil {
		return err
	}
	if after.LastSeq != before.LastSeq {
		return fmt.Errorf("fixture %s: EN request stream advanced from sequence %d to %d; EN must never be queued for an XSD-invalid document",
			fixture.ID, before.LastSeq, after.LastSeq)
	}
	fmt.Printf("  fixture %s finalized invalid; EN request stream stayed at sequence %d\n", fixture.ID, after.LastSeq)
	return nil
}

// Scenario 3: XSD-valid but EN-invalid document reaches EN and finalizes invalid.
func scenarioENInvalidAfterXSDPass(ctx context.Context, e *issue65Env) error {
	fixture, err := e.findFixture(fixtureIsENInvalid, "expected XSD-pass and EN standards-invalid")
	if err != nil {
		return err
	}
	_, response, err := e.uploadAndAwait(ctx, fixture)
	if err != nil {
		return err
	}
	if response.Status != "invalid" {
		return fmt.Errorf("fixture %s: expected terminal invalid, got %q", fixture.ID, response.Status)
	}
	if err := requireStageOutcomes(response, []expectedAPIStage{
		{"ubl21-xsd", true}, {"en16931", false},
	}); err != nil {
		return fmt.Errorf("fixture %s: %w", fixture.ID, err)
	}
	fmt.Printf("  fixture %s: XSD passed, EN reported standards-invalid, final invalid\n", fixture.ID)
	return nil
}

// Scenario 4: with the EN deployment scaled to zero, XSD keeps draining and
// EN requests accumulate durably; restoring the deployment drains the backlog
// and every outage upload reaches its expected terminal state.
func scenarioENScaleToZero(ctx context.Context, e *issue65Env) error {
	const outageUploads = 3
	fixture, err := e.findFixture(fixtureIsValidTwoStage, "expected-valid with XSD and EN stages")
	if err != nil {
		return err
	}
	content, err := e.readFixture(fixture)
	if err != nil {
		return err
	}

	if err := e.kubectl(ctx, "scale", "deployment/"+e.cfg.ENDeployment, "--replicas=0"); err != nil {
		return fmt.Errorf("scale EN deployment to zero: %w", err)
	}
	// Always restore the deployment, even when the scenario fails midway.
	defer func() {
		restoreCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		_ = e.kubectl(restoreCtx, "scale", "deployment/"+e.cfg.ENDeployment, "--replicas=1")
	}()
	if err := e.awaitENPodsGone(ctx); err != nil {
		return err
	}

	uploadIDs := make([]string, 0, outageUploads)
	for i := 0; i < outageUploads; i++ {
		uploadCtx, cancel := context.WithTimeout(ctx, e.cfg.FixtureTimeout)
		id, err := uploadFixture(uploadCtx, http.DefaultClient, e.baseURL, e.cfg.Token, fixture, content)
		cancel()
		if err != nil {
			return fmt.Errorf("upload %d during EN outage: %w", i+1, err)
		}
		uploadIDs = append(uploadIDs, id)
	}

	// XSD must keep draining: the EN request stream has to accumulate one
	// durable request per outage upload while no EN worker is running.
	if err := e.awaitENBacklog(ctx, outageUploads); err != nil {
		return err
	}

	if err := e.kubectl(ctx, "scale", "deployment/"+e.cfg.ENDeployment, "--replicas=1"); err != nil {
		return fmt.Errorf("restore EN deployment: %w", err)
	}
	if err := e.kubectl(ctx, "rollout", "status", "deployment/"+e.cfg.ENDeployment, "--timeout=5m"); err != nil {
		return fmt.Errorf("wait for restored EN deployment: %w", err)
	}

	// Recovery is proven per upload: every document submitted during the
	// outage must reach its expected terminal state with both stage results
	// present — none of them may skip EN after recovery.
	for index, uploadID := range uploadIDs {
		response, err := e.awaitTerminal(ctx, uploadID)
		if err != nil {
			return fmt.Errorf("outage upload %d (%s) did not reach a terminal state after EN restore: %w", index+1, uploadID, err)
		}
		if response.Status != fixture.ExpectedFinal {
			return fmt.Errorf("outage upload %d (%s): expected terminal %q after EN restore, got %q",
				index+1, uploadID, fixture.ExpectedFinal, response.Status)
		}
		if err := requireStageOutcomes(response, expectedStagesFromFixture(fixture)); err != nil {
			return fmt.Errorf("outage upload %d (%s): %w", index+1, uploadID, err)
		}
	}

	// The durable backlog itself must also drain to zero.
	if err := e.awaitENDrain(ctx); err != nil {
		return err
	}
	fmt.Printf("  %d uploads accumulated during EN outage, drained and finalized %q after restore\n", outageUploads, fixture.ExpectedFinal)
	return nil
}

// Scenario: an EN worker pod is force-deleted while a validation is in
// flight. JetStream must redeliver the unacknowledged stage request to the
// replacement pod, the validation must finalize with both stage results, and
// the durable results stream must hold exactly one permanent final result
// for the upload.
func scenarioENWorkerCrashRedelivery(ctx context.Context, e *issue65Env) error {
	fixture, err := e.findFixture(fixtureIsValidTwoStage, "expected-valid with XSD and EN stages")
	if err != nil {
		return err
	}
	content, err := e.readFixture(fixture)
	if err != nil {
		return err
	}
	before, err := e.enRequestStreamState(ctx)
	if err != nil {
		return err
	}

	uploadCtx, cancel := context.WithTimeout(ctx, e.cfg.FixtureTimeout)
	uploadID, err := uploadFixture(uploadCtx, http.DefaultClient, e.baseURL, e.cfg.Token, fixture, content)
	cancel()
	if err != nil {
		return err
	}

	// Wait for the EN stage request to be published (XSD passed, EN work is
	// now in flight), then hard-kill the EN worker pod. EN validation takes
	// on the order of seconds, so the kill lands while the request is
	// unacknowledged in the common case; when the worker manages to finish
	// first, the scenario still proves restart safety and result uniqueness.
	if err := e.awaitENSequenceAdvance(ctx, before.LastSeq); err != nil {
		return fmt.Errorf("upload %s: %w", uploadID, err)
	}
	if err := e.kubectl(ctx, "delete", "pods",
		"--selector", "app.kubernetes.io/component=worker-en16931",
		"--force", "--grace-period=0", "--wait=false"); err != nil {
		return fmt.Errorf("force-delete EN worker pod: %w", err)
	}
	if err := e.kubectl(ctx, "rollout", "status", "deployment/"+e.cfg.ENDeployment, "--timeout=5m"); err != nil {
		return fmt.Errorf("wait for replacement EN worker: %w", err)
	}

	// Redelivery of an unacknowledged request waits for the stage ack
	// deadline (120s for EN 16931), so allow for it on top of normal
	// validation time.
	terminalCtx, cancelTerminal := context.WithTimeout(ctx, e.cfg.FixtureTimeout+150*time.Second)
	response, err := pollFixture(terminalCtx, http.DefaultClient, e.baseURL, e.cfg.Token, uploadID, e.cfg.PollInterval)
	cancelTerminal()
	if err != nil {
		return fmt.Errorf("upload %s did not reach a terminal state after the EN worker crash: %w", uploadID, err)
	}
	if response.Status != fixture.ExpectedFinal {
		return fmt.Errorf("upload %s: expected terminal %q after EN worker crash, got %q", uploadID, fixture.ExpectedFinal, response.Status)
	}
	if err := requireStageOutcomes(response, expectedStagesFromFixture(fixture)); err != nil {
		return fmt.Errorf("upload %s: %w", uploadID, err)
	}

	resultCount, err := e.countFinalResults(ctx, uploadID)
	if err != nil {
		return err
	}
	if resultCount != 1 {
		return fmt.Errorf("upload %s: expected exactly one permanent final result, found %d", uploadID, resultCount)
	}
	fmt.Printf("  upload %s survived the EN worker crash, finalized %q with exactly one permanent result\n", uploadID, response.Status)
	return nil
}


// Scenario: the full frozen corpus runs through the split deployment and
// the normalized results must be semantically equivalent to the monolithic
// baseline. A missing baseline fails the gate.
func scenarioSemanticEquivalence(ctx context.Context, e *issue65Env) error {
	if _, err := os.Stat(e.cfg.EquivalenceBaseline); err != nil {
		return fmt.Errorf("frozen equivalence baseline %s is not readable: %w", e.cfg.EquivalenceBaseline, err)
	}
	corpusOutput := filepath.Join(e.cfg.OutputDir, "equivalence-corpus")
	summary, err := RunCorpus(ctx, RunnerConfig{
		BaseURL:        e.cfg.BaseURL,
		Token:          e.cfg.Token,
		ManifestPath:   e.cfg.ManifestPath,
		OutputDir:      corpusOutput,
		Profile:        "all",
		PollInterval:   e.cfg.PollInterval,
		FixtureTimeout: e.cfg.FixtureTimeout,
	})
	if err != nil {
		return fmt.Errorf("corpus run against split deployment: %w", err)
	}
	equivalence, err := CompareResultSets(e.cfg.EquivalenceBaseline, filepath.Join(corpusOutput, "results.jsonl"))
	if summaryPath := filepath.Join(e.cfg.OutputDir, "equivalence.json"); true {
		if writeErr := WriteEquivalenceSummary(summaryPath, equivalence); writeErr != nil {
			fmt.Printf("  warning: could not write equivalence summary: %v\n", writeErr)
		}
	}
	if err != nil {
		return fmt.Errorf("semantic equivalence against frozen baseline: %w", err)
	}
	fmt.Printf("  corpus %d/%d fixtures passed; %d results equivalent to frozen baseline\n",
		summary.Passed, summary.Total, equivalence.Equivalent)
	return nil
}

type expectedAPIStage struct {
	layer string
	valid bool
}

// expectedStagesFromFixture converts the manifest's declared stage outcomes
// into the layer/valid pairs the public API reports.
func expectedStagesFromFixture(fixture Fixture) []expectedAPIStage {
	expected := make([]expectedAPIStage, 0, len(fixture.ExpectedStages))
	for _, stage := range fixture.ExpectedStages {
		expected = append(expected, expectedAPIStage{layer: stage.Stage, valid: stage.Outcome == "passed"})
	}
	return expected
}

func requireStageOutcomes(response apiResponse, expected []expectedAPIStage) error {
	if len(response.Stages) != len(expected) {
		layers := make([]string, 0, len(response.Stages))
		for _, stage := range response.Stages {
			layers = append(layers, stage.Layer)
		}
		return fmt.Errorf("expected %d stage results, got %d (%s)", len(expected), len(response.Stages), strings.Join(layers, ", "))
	}
	for index, want := range expected {
		got := response.Stages[index]
		if got.Layer != want.layer || got.Valid != want.valid {
			return fmt.Errorf("stage %d: expected layer %q valid=%t, got layer %q valid=%t",
				index, want.layer, want.valid, got.Layer, got.Valid)
		}
	}
	return nil
}

type streamState struct {
	Msgs    uint64
	LastSeq uint64
}

// enRequestStreamState reads the live JetStream state of the EN 16931 stage
// request stream. LastSeq counts every request ever published (it does not
// decrease when work-queue messages are consumed), so it detects EN requests
// even if a worker would consume them quickly. Msgs counts the durable backlog.
func (e *issue65Env) enRequestStreamState(ctx context.Context) (streamState, error) {
	names, err := queue.Namespace(e.cfg.QueueNamespace).StageQueue(queue.ValidationLayerEN16931)
	if err != nil {
		return streamState{}, err
	}
	connection, err := nats.Connect(e.cfg.NATSURL)
	if err != nil {
		return streamState{}, fmt.Errorf("connect to NATS %s: %w", e.cfg.NATSURL, err)
	}
	defer connection.Close()
	js, err := jetstream.New(connection)
	if err != nil {
		return streamState{}, err
	}
	stream, err := js.Stream(ctx, names.RequestStream)
	if err != nil {
		return streamState{}, fmt.Errorf("open stream %s: %w", names.RequestStream, err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		return streamState{}, fmt.Errorf("read stream %s info: %w", names.RequestStream, err)
	}
	return streamState{Msgs: info.State.Msgs, LastSeq: info.State.LastSeq}, nil
}

func (e *issue65Env) awaitENBacklog(ctx context.Context, minimum int) error {
	deadline := time.Now().Add(e.cfg.FixtureTimeout)
	var last streamState
	for time.Now().Before(deadline) {
		state, err := e.enRequestStreamState(ctx)
		if err != nil {
			return err
		}
		last = state
		if state.Msgs >= uint64(minimum) {
			fmt.Printf("  EN request stream accumulated %d durable requests during outage\n", state.Msgs)
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("EN request stream never accumulated %d requests during the outage (last saw %d)", minimum, last.Msgs)
}

func (e *issue65Env) awaitENDrain(ctx context.Context) error {
	deadline := time.Now().Add(e.cfg.FixtureTimeout)
	var last streamState
	for time.Now().Before(deadline) {
		state, err := e.enRequestStreamState(ctx)
		if err != nil {
			return err
		}
		last = state
		if state.Msgs == 0 {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("EN request stream did not drain after restore; %d requests still pending", last.Msgs)
}

// awaitENSequenceAdvance waits until the EN request stream publishes past the
// given sequence, meaning an EN stage request for new work is in flight.
func (e *issue65Env) awaitENSequenceAdvance(ctx context.Context, baseline uint64) error {
	deadline := time.Now().Add(e.cfg.FixtureTimeout)
	for time.Now().Before(deadline) {
		state, err := e.enRequestStreamState(ctx)
		if err != nil {
			return err
		}
		if state.LastSeq > baseline {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("EN request stream never advanced past sequence %d; XSD may not have passed", baseline)
}

// countFinalResults counts permanent final results recorded for one upload in
// the durable results stream. The stream keeps at most one message per result
// subject (MaxMsgsPerSubject=1), and every result message carries the upload
// ID header, so counting messages with that header proves result uniqueness
// across restarts and redeliveries.
func (e *issue65Env) countFinalResults(ctx context.Context, uploadID string) (int, error) {
	namespace := queue.Namespace(e.cfg.QueueNamespace)
	connection, err := nats.Connect(e.cfg.NATSURL)
	if err != nil {
		return 0, fmt.Errorf("connect to NATS %s: %w", e.cfg.NATSURL, err)
	}
	defer connection.Close()
	js, err := jetstream.New(connection)
	if err != nil {
		return 0, err
	}
	stream, err := js.Stream(ctx, namespace.ResultsStreamName())
	if err != nil {
		return 0, fmt.Errorf("open results stream %s: %w", namespace.ResultsStreamName(), err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		return 0, fmt.Errorf("read results stream info: %w", err)
	}
	if info.State.Msgs == 0 {
		return 0, nil
	}
	consumer, err := js.OrderedConsumer(ctx, namespace.ResultsStreamName(), jetstream.OrderedConsumerConfig{
		FilterSubjects: []string{namespace.ResultsFilterSubject()},
	})
	if err != nil {
		return 0, fmt.Errorf("create results inspection consumer: %w", err)
	}
	batch, err := consumer.FetchNoWait(int(info.State.Msgs))
	if err != nil {
		return 0, fmt.Errorf("fetch results for inspection: %w", err)
	}
	count := 0
	for message := range batch.Messages() {
		if message.Headers().Get(queue.HeaderUploadID) == uploadID {
			count++
		}
	}
	if err := batch.Error(); err != nil {
		return 0, fmt.Errorf("read results for inspection: %w", err)
	}
	return count, nil
}

// awaitENPodsGone waits until no EN worker pod remains, so the outage is real
// before any document is submitted.
func (e *issue65Env) awaitENPodsGone(ctx context.Context) error {
	deadline := time.Now().Add(2 * time.Minute)
	selector := "app.kubernetes.io/component=worker-en16931"
	for time.Now().Before(deadline) {
		output, err := e.kubectlOutput(ctx, "get", "pods", "--selector", selector, "--no-headers", "--ignore-not-found")
		if err != nil {
			return fmt.Errorf("list EN worker pods: %w", err)
		}
		if strings.TrimSpace(output) == "" {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return errors.New("EN worker pods were still running two minutes after scaling to zero")
}

func (e *issue65Env) kubectl(ctx context.Context, args ...string) error {
	_, err := e.kubectlOutput(ctx, args...)
	return err
}

func (e *issue65Env) kubectlOutput(ctx context.Context, args ...string) (string, error) {
	full := append([]string{"--namespace", e.cfg.Namespace}, args...)
	output, err := exec.CommandContext(ctx, e.cfg.KubectlBin, full...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("kubectl %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}
