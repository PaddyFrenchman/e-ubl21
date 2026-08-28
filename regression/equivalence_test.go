package regression

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompareResultSetsAcceptsEquivalentReorderedResults(t *testing.T) {
	first := equivalenceResult("fixture.a", "valid", []ActualStage{{Stage: "ubl21-xsd", Outcome: "passed"}})
	second := equivalenceResult("fixture.b", "invalid", []ActualStage{{Stage: "ubl21-xsd", Outcome: "passed"}, {Stage: "en16931", Outcome: "standards-invalid", RuleIDs: []string{"BR-01"}}})
	baseline := writeEquivalenceResults(t, "baseline.jsonl", first, second)
	candidate := writeEquivalenceResults(t, "candidate.jsonl", second, first)

	summary, err := CompareResultSets(baseline, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Total != 2 || summary.Equivalent != 2 || summary.Changed+summary.Missing+summary.Unexpected != 0 {
		t.Fatalf("summary = %+v", summary)
	}
}

func TestCompareResultSetsRejectsEveryDifferenceClass(t *testing.T) {
	baselineA := equivalenceResult("fixture.a", "valid", []ActualStage{{Stage: "ubl21-xsd", Outcome: "passed"}})
	baselineB := equivalenceResult("fixture.b", "invalid", []ActualStage{{Stage: "ubl21-xsd", Outcome: "passed"}, {Stage: "en16931", Outcome: "standards-invalid", RuleIDs: []string{"BR-01"}}})
	changedB := baselineB
	changedB.ActualStages = []ActualStage{{Stage: "ubl21-xsd", Outcome: "passed"}, {Stage: "en16931", Outcome: "standards-invalid", RuleIDs: []string{"BR-02"}}}
	unexpected := equivalenceResult("fixture.c", "valid", []ActualStage{{Stage: "ubl21-xsd", Outcome: "passed"}})

	baseline := writeEquivalenceResults(t, "baseline.jsonl", baselineA, baselineB, equivalenceResult("fixture.missing", "valid", []ActualStage{{Stage: "ubl21-xsd", Outcome: "passed"}}))
	candidate := writeEquivalenceResults(t, "candidate.jsonl", baselineA, changedB, unexpected)
	summary, err := CompareResultSets(baseline, candidate)
	if err == nil || !strings.Contains(err.Error(), "changed:fixture.b") || !strings.Contains(err.Error(), "missing:fixture.missing") || !strings.Contains(err.Error(), "unexpected:fixture.c") {
		t.Fatalf("summary=%+v error=%v", summary, err)
	}
	if summary.Equivalent != 1 || summary.Changed != 1 || summary.Missing != 1 || summary.Unexpected != 1 {
		t.Fatalf("summary = %+v", summary)
	}
}

func TestCompareResultSetsRejectsInvalidAndDuplicateInput(t *testing.T) {
	valid := equivalenceResult("fixture.a", "valid", []ActualStage{{Stage: "ubl21-xsd", Outcome: "passed"}})
	baseline := writeEquivalenceResults(t, "baseline.jsonl", valid)
	duplicate := writeEquivalenceResults(t, "duplicate.jsonl", valid, valid)
	if _, err := CompareResultSets(baseline, duplicate); err == nil || !strings.Contains(err.Error(), "duplicate fixture id") {
		t.Fatalf("duplicate error = %v", err)
	}
	empty := filepath.Join(t.TempDir(), "empty.jsonl")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CompareResultSets(baseline, empty); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("empty error = %v", err)
	}
}

func TestWriteEquivalenceSummary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "equivalence.json")
	summary := EquivalenceSummary{SchemaVersion: 1, Total: 2, Equivalent: 2}
	if err := WriteEquivalenceSummary(path, summary); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil || !strings.HasSuffix(string(content), "\n") || !strings.Contains(string(content), `"equivalent": 2`) {
		t.Fatalf("content=%q error=%v", content, err)
	}
}

func equivalenceResult(fixtureID, final string, stages []ActualStage) NormalizedResult {
	expectedStages := make([]ExpectedStage, len(stages))
	for index, stage := range stages {
		expectedStages[index] = ExpectedStage{Stage: stage.Stage, Outcome: stage.Outcome, RuleIDs: append([]string(nil), stage.RuleIDs...)}
	}
	profile := "ubl21"
	if len(stages) > 1 {
		profile = "ubl21-en16931"
	}
	return NormalizedResult{
		SchemaVersion: 1, FixtureID: fixtureID, SHA256: strings.Repeat("a", 64), Profile: profile,
		ExpectedFinal: final, ActualFinal: final, ExpectedStages: expectedStages, ActualStages: stages, Result: "pass",
	}
}

func writeEquivalenceResults(t *testing.T, name string, results ...NormalizedResult) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(file)
	for _, result := range results {
		if err := encoder.Encode(result); err != nil {
			file.Close()
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
