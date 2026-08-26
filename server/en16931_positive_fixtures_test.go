package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	fixtures "github.com/PaddyFrenchman/e-ubl21/createsamples/en16931"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
)

func TestPositiveEN16931FixturesPassCompleteValidationPipeline(t *testing.T) {
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
	catalog, err := loadValidationCatalog()
	if err != nil {
		t.Fatal(err)
	}

	type positiveFixture struct {
		documentType string
		body         []byte
	}
	positive := map[string]positiveFixture{}
	invoiceVariants := map[string]func() fixtures.InvoiceDocument{
		"S": fixtures.NewInvoiceVariantS, "Z": fixtures.NewInvoiceVariantZ,
		"E": fixtures.NewInvoiceVariantE, "AE": fixtures.NewInvoiceVariantAE,
		"G": fixtures.NewInvoiceVariantG, "K": fixtures.NewInvoiceVariantK,
		"O": fixtures.NewInvoiceVariantO,
	}
	for category, build := range invoiceVariants {
		xml, buildErr := fixtures.BuildInvoiceXML(build())
		if buildErr != nil {
			t.Fatalf("build Invoice VAT %s: %v", category, buildErr)
		}
		positive["generated Invoice VAT "+category] = positiveFixture{documentType: "Invoice", body: []byte(xml)}
	}
	creditNote, err := fixtures.BuildCreditNoteXML(fixtures.NewCreditNoteVariantS())
	if err != nil {
		t.Fatal(err)
	}
	positive["generated CreditNote VAT S"] = positiveFixture{documentType: "CreditNote", body: []byte(creditNote)}
	for _, name := range []string{"guide-example1.xml", "guide-example2.xml"} {
		body, readErr := os.ReadFile(filepath.Join("..", "createsamples", "en16931", "testdata", "official", name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		positive["official "+name] = positiveFixture{documentType: "Invoice", body: body}
	}
	if len(positive) != 10 {
		t.Fatalf("positive EN 16931 fixture count = %d, want 10", len(positive))
	}

	for name, fixture := range positive {
		fixture := fixture
		t.Run(name, func(t *testing.T) {
			schemaPath, found := catalog.schemaPathFor(fixture.documentType)
			if !found {
				t.Fatalf("missing %s schema", fixture.documentType)
			}
			source := &memoryValidationContentSource{content: fixture.body}
			pipeline := newDocumentValidationPipeline(source, newSaxonEN16931Validator(runtime, 20*time.Second), 20*time.Second)
			result, validateErr := pipeline.Validate(context.Background(), "positive-fixture", fixture.documentType, schemaPath)
			if validateErr != nil {
				t.Fatalf("validate pipeline: %v", validateErr)
			}
			if result.Status != queue.StatusValid || len(result.Stages) != 2 {
				t.Fatalf("overall status = %q, stages=%+v error=%q", result.Status, result.Stages, result.Error)
			}
			xsd, hasXSD := result.Stages.Find(queue.ValidationLayerUBL21XSD)
			semantic, hasEN := result.Stages.Find(queue.ValidationLayerEN16931)
			if !hasXSD || !xsd.Valid {
				t.Fatalf("UBL 2.1 XSD stage = %+v", xsd)
			}
			if !hasEN || !semantic.Valid || semantic.Version != validationassets.EN16931().Version {
				t.Fatalf("EN 16931 stage = %+v, want valid version %q", semantic, validationassets.EN16931().Version)
			}
			for _, issue := range semantic.Issues {
				if strings.EqualFold(issue.Severity, "fatal") {
					t.Fatalf("fatal EN 16931 issue: %+v", issue)
				}
			}
		})
	}
}
