package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegressionCommandReportsRunnerAndEquivalenceErrors(t *testing.T) {
	var output bytes.Buffer
	command := newRegressionCmd(&Config{})
	command.SetOut(&output)
	command.SetArgs([]string{"--token", "token", "--manifest", filepath.Join(t.TempDir(), "missing.json"), "--output", t.TempDir()})
	if err := command.Execute(); err == nil {
		t.Fatal("missing regression manifest succeeded")
	}
	if !strings.Contains(output.String(), "regression total=0") {
		t.Fatalf("runner summary output = %q", output.String())
	}

	missing := newRegressionEquivalenceCmd()
	missing.SetArgs(nil)
	if err := missing.Execute(); err == nil {
		t.Fatal("missing equivalence inputs succeeded")
	}

	dir := t.TempDir()
	baseline := filepath.Join(dir, "baseline.jsonl")
	candidate := filepath.Join(dir, "candidate.jsonl")
	if err := os.WriteFile(baseline, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(candidate, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	summaryPath := filepath.Join(dir, "summary.json")
	output.Reset()
	equivalence := newRegressionEquivalenceCmd()
	equivalence.SetOut(&output)
	equivalence.SetArgs([]string{"--baseline", baseline, "--candidate", candidate, "--output", summaryPath})
	if err := equivalence.Execute(); err == nil {
		t.Fatal("empty equivalence inputs succeeded")
	}
	if !strings.Contains(output.String(), "equivalence total=0") {
		t.Fatalf("equivalence summary output = %q", output.String())
	}
	if _, err := os.Stat(summaryPath); err != nil {
		t.Fatalf("equivalence summary was not written: %v", err)
	}
}
