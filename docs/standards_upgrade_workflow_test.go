package docs_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStandardsUpgradeWorkflowRetainsReviewGuards(t *testing.T) {
	content, err := os.ReadFile("STANDARDS-UPGRADE-WORKFLOW.md")
	if err != nil {
		t.Fatal(err)
	}
	workflow := strings.Join(strings.Fields(string(content)), " ")
	required := []string{
		"## Non-negotiable invariants",
		"accepted commit",
		"candidate commit",
		"Do not update canonical manifests or baselines",
		"accepted-runtime/accepted-corpus",
		"candidate-runtime/accepted-corpus",
		"candidate-runtime/candidate-corpus",
		"correctness/ubl21/results.jsonl",
		"correctness/en16931/results.jsonl",
		"accepted-runtime/accepted-corpus/peppol",
		"staged-corpus/en16931",
		"runtime-on-accepted-corpus",
		"corpus-under-candidate-runtime",
		"complete-release-change",
		"immutable external artifact URI",
		"does not satisfy retention",
		"evidence.sha256",
		"fixture-change-ledger.md",
		"upstream-required semantic change",
		"implementation regression",
		"unexplained",
		"./run-regression-tests.sh",
		"make regression-equivalence",
		"diff -u",
		"## Blocking conditions",
		"UBL schemas",
		"EN rules",
		"Peppol rules/code lists",
	}
	for _, phrase := range required {
		if !strings.Contains(workflow, phrase) {
			t.Errorf("standards upgrade workflow is missing %q", phrase)
		}
	}
}

func TestStandardsUpgradeWorkflowIsLinkedFromRelatedPolicies(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "standard", "README.md"),
		filepath.Join("..", "regression", "README.md"),
		"EN16931-ASSET-UPGRADE.md",
		"PEPPOL-ASSET-UPGRADE.md",
	} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), "STANDARDS-UPGRADE-WORKFLOW.md") {
			t.Errorf("%s does not link to the canonical standards upgrade workflow", path)
		}
	}
}
