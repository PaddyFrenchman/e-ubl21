package regression

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// FailureDiagnostic is a redacted, per-fixture investigation bundle. It is
// deliberately separate from normalized regression equivalence artifacts.
type FailureDiagnostic struct {
	SchemaVersion        int              `json:"schemaVersion"`
	Fixture              Fixture          `json:"fixture"`
	Expected             NormalizedResult `json:"expectedVsActual"`
	UploadID             string           `json:"uploadId,omitempty"`
	TerminalStatus       string           `json:"terminalStatus"`
	ObjectStoreKey       string           `json:"objectStoreKey,omitempty"`
	ProcessingAttempts   int              `json:"processingAttempts,omitempty"`
	RuntimeError         string           `json:"runtimeError,omitempty"`
	ValidatorDiagnostics []apiStage       `json:"validatorDiagnostics,omitempty"`
	Classification       string           `json:"classification"`
	EvidenceBackedCause  string           `json:"evidenceBackedCause"`
	Reproduction         string           `json:"reproduction"`
}

func writeFailureDiagnostics(outputDir, manifestPath, baseURL string, fixtureTimeout time.Duration, fixtures []Fixture, results []NormalizedResult, runMap []RunMapEntry, failures []FixtureFailure) error {
	fixtureByID := map[string]Fixture{}
	for _, fixture := range fixtures {
		fixtureByID[fixture.ID] = fixture
	}
	mapByID := map[string]RunMapEntry{}
	for _, entry := range runMap {
		mapByID[entry.FixtureID] = entry
	}
	failureByID := map[string]string{}
	for _, failure := range failures {
		failureByID[failure.FixtureID] = redactDiagnostic(failure.Error)
	}
	var failed []NormalizedResult
	for _, result := range results {
		if result.Result == "fail" {
			failed = append(failed, result)
		}
	}
	sort.Slice(failed, func(i, j int) bool { return failed[i].FixtureID < failed[j].FixtureID })
	index := make([]string, 0, len(failed))
	for _, result := range failed {
		fixture := fixtureByID[result.FixtureID]
		entry := mapByID[result.FixtureID]
		classification, cause := classifyDiagnostic(result, entry, failureByID[result.FixtureID])
		diagnostic := FailureDiagnostic{SchemaVersion: 1, Fixture: fixture, Expected: result, UploadID: entry.UploadID, TerminalStatus: entry.TerminalStatus, ProcessingAttempts: entry.ProcessingAttempts, RuntimeError: redactDiagnostic(entry.RuntimeError), ValidatorDiagnostics: redactStageDiagnostics(entry.ValidatorDiagnostics), Classification: classification, EvidenceBackedCause: cause,
			Reproduction: fmt.Sprintf("EUBL21_REGRESSION_TOKEN=${EUBL21_REGRESSION_TOKEN:?set_token} go run . regression --base-url %s --manifest %s --profile %s --fixture-id %s --fixture-timeout %s", shellQuote(baseURL), shellQuote(manifestPath), shellQuote(fixture.Profile), shellQuote(fixture.ID), fixtureTimeout)}
		if entry.UploadID != "" {
			diagnostic.ObjectStoreKey = "uploads/" + entry.UploadID + "/document.xml"
		}
		encoded, err := json.MarshalIndent(diagnostic, "", "  ")
		if err != nil {
			return err
		}
		encoded = append(encoded, '\n')
		dir := filepath.Join(outputDir, "failures", fixture.ID)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		if err := writeFileAtomically(filepath.Join(dir, "diagnostic.json"), encoded); err != nil {
			return err
		}
		index = append(index, fmt.Sprintf("- `%s`: %s — [%s](failures/%s/diagnostic.json)\n  - Reproduce: `%s`", fixture.ID, classification, fixture.ID, fixture.ID, diagnostic.Reproduction))
	}
	if len(index) == 0 {
		return nil
	}
	handoff := "# Codex regression handoff\n\nEach failure below has a self-contained, redacted bundle with fixture metadata, expected/actual stages and rules, correlation IDs, object-store key when submitted, and an evidence-backed (not speculative) classification. Environment, logs, metrics, NATS, and Kubernetes evidence remain in their bounded sibling artifacts when collected.\n\n## Failure index\n\n" + strings.Join(index, "\n") + "\n"
	return writeFileAtomically(filepath.Join(outputDir, "codex-handoff.md"), []byte(handoff))
}

func classifyDiagnostic(result NormalizedResult, entry RunMapEntry, failure string) (string, string) {
	if failure != "" {
		if strings.Contains(strings.ToLower(failure), "deadline") || strings.Contains(strings.ToLower(failure), "timeout") {
			return "timeout/resource failure", failure
		}
		return "unknown", failure
	}
	if result.ExpectedFinal != result.ActualFinal {
		return "expectation mismatch", fmt.Sprintf("expected final %q, received %q", result.ExpectedFinal, result.ActualFinal)
	}
	return "standards-validator failure", "terminal validation stages differ from the manifest expectation"
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }

func redactDiagnostic(value string) string {
	value = credentialURLPattern.ReplaceAllString(value, "://[REDACTED]@")
	return redactBearer(value)
}

func redactStageDiagnostics(stages []apiStage) []apiStage {
	redacted := append([]apiStage(nil), stages...)
	for index := range redacted {
		redacted[index].Issues = append([]apiIssue(nil), redacted[index].Issues...)
		for issueIndex := range redacted[index].Issues {
			redacted[index].Issues[issueIndex].Message = redactDiagnostic(redacted[index].Issues[issueIndex].Message)
			redacted[index].Issues[issueIndex].Location = redactDiagnostic(redacted[index].Issues[issueIndex].Location)
		}
	}
	return redacted
}
