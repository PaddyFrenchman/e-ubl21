package queue_test

import (
	"testing"

	"github.com/PaddyFrenchman/e-ubl21/queue"
)

func TestDefaultNamespacePreservesLegacyNames(t *testing.T) {
	t.Parallel()
	ns := queue.DefaultNamespace

	if got, want := ns.DocumentsStreamName(), "UBL_DOCUMENTS"; got != want {
		t.Errorf("DocumentsStreamName() = %q, want %q", got, want)
	}
	if got, want := ns.ResultsStreamName(), "UBL_VALIDATION_RESULTS"; got != want {
		t.Errorf("ResultsStreamName() = %q, want %q", got, want)
	}
	if got, want := ns.UploadsBucketName(), "UBL_UPLOADS"; got != want {
		t.Errorf("UploadsBucketName() = %q, want %q", got, want)
	}
	if got, want := ns.DocumentSubject("Invoice"), "ubl.documents.Invoice"; got != want {
		t.Errorf("DocumentSubject(%q) = %q, want %q", "Invoice", got, want)
	}
	if got, want := ns.DocumentsFilterSubject(), "ubl.documents.>"; got != want {
		t.Errorf("DocumentsFilterSubject() = %q, want %q", got, want)
	}
	if got, want := ns.ResultsSubject(), "ubl.validation.results"; got != want {
		t.Errorf("ResultsSubject() = %q, want %q", got, want)
	}

	// NewNamespace("") must be indistinguishable from the zero value.
	if got := queue.NewNamespace(""); got != queue.DefaultNamespace {
		t.Errorf("NewNamespace(\"\") = %q, want DefaultNamespace", got)
	}
	if got := queue.NewNamespace("   "); got != queue.DefaultNamespace {
		t.Errorf("NewNamespace(\"   \") = %q, want DefaultNamespace", got)
	}
}

func TestNamespaceDerivesNamesFromExample(t *testing.T) {
	t.Parallel()
	// The exact example from TODO.md's TODO-001.
	ns := queue.NewNamespace("eubl21.prod")

	if got, want := ns.DocumentSubject("Invoice"), "eubl21.prod.documents.Invoice"; got != want {
		t.Errorf("DocumentSubject(%q) = %q, want %q", "Invoice", got, want)
	}
	if got, want := ns.DocumentSubject("CreditNote"), "eubl21.prod.documents.CreditNote"; got != want {
		t.Errorf("DocumentSubject(%q) = %q, want %q", "CreditNote", got, want)
	}
	if got, want := ns.ResultsSubject(), "eubl21.prod.validation.results"; got != want {
		t.Errorf("ResultsSubject() = %q, want %q", got, want)
	}
	if got, want := ns.DocumentsStreamName(), "EUBL21_PROD_DOCUMENTS"; got != want {
		t.Errorf("DocumentsStreamName() = %q, want %q", got, want)
	}
	if got, want := ns.ResultsStreamName(), "EUBL21_PROD_VALIDATION_RESULTS"; got != want {
		t.Errorf("ResultsStreamName() = %q, want %q", got, want)
	}
	if got, want := ns.UploadsBucketName(), "EUBL21_PROD_UPLOADS"; got != want {
		t.Errorf("UploadsBucketName() = %q, want %q", got, want)
	}
}

func TestNamespaceDocumentTypeRoundTrip(t *testing.T) {
	t.Parallel()
	for _, ns := range []queue.Namespace{queue.DefaultNamespace, queue.NewNamespace("eubl21.dev"), queue.NewNamespace("eubl21.prod")} {
		subject := ns.DocumentSubject("Invoice")
		if got := ns.DocumentTypeFromSubject(subject); got != "Invoice" {
			t.Errorf("namespace %q: DocumentTypeFromSubject(%q) = %q, want %q", ns, subject, got, "Invoice")
		}
		if !ns.HasDocumentSubject(subject) {
			t.Errorf("namespace %q: HasDocumentSubject(%q) = false, want true", ns, subject)
		}
	}

	dev := queue.NewNamespace("eubl21.dev")
	prod := queue.NewNamespace("eubl21.prod")
	if dev.HasDocumentSubject(prod.DocumentSubject("Invoice")) {
		t.Error("dev namespace must not recognize a prod namespace subject as its own")
	}
}

func TestNamespaceResourceNameNormalization(t *testing.T) {
	t.Parallel()
	cases := []struct {
		namespace string
		want      string
	}{
		{"eubl21.dev", "EUBL21_DEV"},
		{"eubl21.prod", "EUBL21_PROD"},
		{"Eubl21.Prod", "EUBL21_PROD"},
		{"eubl21--prod", "EUBL21_PROD"},
		{"eubl21.prod.", "EUBL21_PROD"},
		{".eubl21.prod", "EUBL21_PROD"},
	}
	for _, tc := range cases {
		ns := queue.NewNamespace(tc.namespace)
		if got := ns.DocumentsStreamName(); got != tc.want+"_DOCUMENTS" {
			t.Errorf("NewNamespace(%q).DocumentsStreamName() = %q, want %q", tc.namespace, got, tc.want+"_DOCUMENTS")
		}
	}
}
