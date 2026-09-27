package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
)

func TestSaxonPeppolEnvelopeValidatorPreservesEnvelopeLayer(t *testing.T) {
	document := `<svrl:schematron-output xmlns:svrl="http://purl.oclc.org/dsdl/svrl"><svrl:failed-assert id="BR-SBDH-1" flag="fatal"><svrl:text>Receiver does not match.</svrl:text></svrl:failed-assert></svrl:schematron-output>`
	validator := newSaxonPeppolEnvelopeValidator(fakeSaxonRuntime(t, "cat >/dev/null\nprintf '%s' '"+document+"'"), time.Second)
	result, err := validator.Validate(context.Background(), strings.NewReader(`<StandardBusinessDocument/>`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Layer != queue.ValidationLayerPeppolEnvelope || result.Version != validationassets.PeppolEnvelopeVersion || result.Valid || result.Issues[0].Layer != queue.ValidationLayerPeppolEnvelope || result.Issues[0].Rule != "BR-SBDH-1" {
		t.Fatalf("envelope result = %+v", result)
	}
}

func TestPeppolEnvelopeValidatorChecksDocumentAndProcessScopes(t *testing.T) {
	document, err := os.ReadFile(filepath.Join("testdata", "peppol-envelope", "valid-billing.xml"))
	if err != nil {
		t.Fatal(err)
	}
	validator := newSaxonPeppolEnvelopeValidator(fakeSaxonRuntime(t, "cat >/dev/null; printf '%s' '<svrl:schematron-output xmlns:svrl=\"http://purl.oclc.org/dsdl/svrl\"/>'"), time.Second)
	result, err := validator.Validate(context.Background(), strings.NewReader(string(document)))
	if err != nil || !result.Valid {
		t.Fatalf("valid envelope = %+v, %v", result, err)
	}
	for _, tc := range []struct{ name, fixture, rule string }{
		{"document", "documentid-mismatch.xml", "EUBL21-SBDH-DOCUMENTID"},
		{"process", "processid-mismatch.xml", "EUBL21-SBDH-PROCESSID"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			broken, err := os.ReadFile(filepath.Join("testdata", "peppol-envelope", tc.fixture))
			if err != nil {
				t.Fatal(err)
			}
			result, err := validator.Validate(context.Background(), strings.NewReader(string(broken)))
			if err != nil || result.Valid || len(result.Issues) != 1 || result.Issues[0].Rule != tc.rule {
				t.Fatalf("result = %+v, err=%v", result, err)
			}
		})
	}
}
