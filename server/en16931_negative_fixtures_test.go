package server

import (
	"context"
	"io"
	"testing"
	"time"

	fixtures "github.com/PaddyFrenchman/e-ubl21/createsamples/en16931"
	"github.com/PaddyFrenchman/e-ubl21/queue"
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
	corpus, err := fixtures.NegativeInvoiceFixtures()
	if err != nil {
		t.Fatal(err)
	}

	for _, fixture := range corpus {
		fixture := fixture
		t.Run(fixture.ID, func(t *testing.T) {
			source := &memoryValidationContentSource{content: []byte(fixture.XML)}
			pipeline := newDocumentValidationPipeline(source, newSaxonEN16931Validator(runtime, 20*time.Second), 20*time.Second)
			result, validateErr := pipeline.Validate(context.Background(), fixture.ID, fixture.DocumentType, schemaPath)
			if validateErr != nil {
				t.Fatalf("pipeline operational failure for %q: %v", fixture.Mutation, validateErr)
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
			for _, expected := range fixture.ExpectedRuleIDs {
				severity, present := actualRules[expected]
				if !present || !en16931SeverityInvalid(severity) {
					t.Fatalf("missing expected rule %s for %q; actual rules=%v", expected, fixture.Mutation, actualRules)
				}
			}
		})
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
