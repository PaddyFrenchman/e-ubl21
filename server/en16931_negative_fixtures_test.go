package server

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/regression"
)

func TestTargetedEN16931NegativeCorpus(t *testing.T) {
	runtime, ok := localSaxonRuntimeConfig(t)
	if !ok {
		return
	}
	if err := ensureXMLLint(); err != nil {
		t.Skip(err)
	}
	catalog, err := loadValidationCatalog()
	if err != nil {
		t.Fatal(err)
	}
	schemaPath, found := catalog.schemaPathFor("Invoice")
	if !found {
		t.Fatal("missing Invoice schema")
	}
	corpusRoot := filepath.Join("..", "createsamples", "samples-en16931")
	manifestBytes, err := os.ReadFile(filepath.Join(corpusRoot, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := regression.ValidateFixtureManifest(manifestBytes); err != nil {
		t.Fatalf("tracked EN 16931 manifest rejected: %v", err)
	}
	var manifest struct {
		Fixtures []struct {
			ID            string `json:"id"`
			DocumentType  string `json:"documentType"`
			Profile       string `json:"profile"`
			File          string `json:"file"`
			ExpectedFinal string `json:"expectedFinal"`
			Mutation      *struct {
				ID          string `json:"id"`
				Description string `json:"description"`
			} `json:"mutation"`
			ExpectedStages []struct {
				Stage   string   `json:"stage"`
				Outcome string   `json:"outcome"`
				RuleIDs []string `json:"ruleIds"`
			} `json:"expectedStages"`
		} `json:"fixtures"`
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}

	negativeCount := 0
	for _, fixture := range manifest.Fixtures {
		if fixture.Profile != "ubl21-en16931" || fixture.ExpectedFinal != "invalid" {
			continue
		}
		negativeCount++
		fixture := fixture
		t.Run(fixture.ID, func(t *testing.T) {
			if fixture.Mutation == nil || fixture.Mutation.ID == "" || fixture.Mutation.Description == "" {
				t.Fatalf("fixture lacks mutation provenance: %+v", fixture.Mutation)
			}
			var expectedRuleIDs []string
			for _, stage := range fixture.ExpectedStages {
				if stage.Stage == "en16931" && stage.Outcome == "standards-invalid" {
					expectedRuleIDs = stage.RuleIDs
				}
			}
			if len(expectedRuleIDs) == 0 {
				t.Fatal("fixture lacks expected EN 16931 rule IDs")
			}
			content, err := os.ReadFile(filepath.Join(corpusRoot, filepath.FromSlash(fixture.File)))
			if err != nil {
				t.Fatal(err)
			}
			source := &memoryValidationContentSource{content: content}
			pipeline := newDocumentValidationPipeline(source, newSaxonEN16931Validator(runtime, 20*time.Second), 20*time.Second)
			result, validateErr := pipeline.Validate(context.Background(), fixture.ID, fixture.DocumentType, schemaPath)
			if validateErr != nil {
				t.Fatalf("pipeline operational failure for %q: %v", fixture.Mutation.Description, validateErr)
			}
			xsd, hasXSD := result.Stages.Find(queue.ValidationLayerUBL21XSD)
			if !hasXSD || !xsd.Valid {
				t.Fatalf("fixture is not UBL 2.1 XSD-valid: %+v", xsd)
			}
			semantic, hasEN := result.Stages.Find(queue.ValidationLayerEN16931)
			if result.Status != queue.StatusInvalid || !hasEN || semantic.Valid {
				t.Fatalf("semantic outcome = status %q stage %+v", result.Status, semantic)
			}
			actualRules := map[string]string{}
			for _, issue := range semantic.Issues {
				if issue.Kind == queue.ValidationIssueFailedAssert {
					actualRules[issue.Rule] = issue.Severity
				}
			}
			for _, expected := range expectedRuleIDs {
				severity, present := actualRules[expected]
				if !present || !en16931SeverityInvalid(severity) {
					t.Fatalf("missing expected rule %s for %q; actual rules=%v", expected, fixture.Mutation.Description, actualRules)
				}
			}
		})
	}
	if negativeCount != 22 {
		t.Fatalf("tracked EN 16931 negative fixture count = %d, want 22", negativeCount)
	}
}

func TestXSDInvalidControlSkipsEN16931(t *testing.T) {
	catalog, err := loadValidationCatalog()
	if err != nil {
		t.Fatal(err)
	}
	schemaPath, found := catalog.schemaPathFor("Invoice")
	if !found {
		t.Fatal("missing Invoice schema")
	}
	enCalls := 0
	semantic := en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
		enCalls++
		return queue.ValidationStageResult{Layer: queue.ValidationLayerEN16931, Valid: true}, nil
	})
	source := &memoryValidationContentSource{content: []byte(`<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"/>`)}
	result, err := newDocumentValidationPipeline(source, semantic, 5*time.Second).Validate(context.Background(), "xsd-invalid-control", "Invoice", schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != queue.StatusInvalid || enCalls != 0 || len(result.Stages) != 1 || result.Stages[0].Layer != queue.ValidationLayerUBL21XSD || result.Stages[0].Valid {
		t.Fatalf("control result=%+v EN calls=%d", result, enCalls)
	}
}
