package queue_test

import (
	"strings"
	"testing"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
)

func TestDefaultNamespacePreservesLegacyNames(t *testing.T) {
	t.Parallel()
	ns := queue.DefaultNamespace

	if got, want := ns.DocumentsStreamName(), "UBL_DOCUMENTS"; got != want {
		t.Errorf("DocumentsStreamName() = %q, want %q", got, want)
	}
	if got, want := ns.ResultsStreamName(), "UBL_VALIDATION_RESULTS_BY_UPLOAD"; got != want {
		t.Errorf("ResultsStreamName() = %q, want %q", got, want)
	}
	if got, want := ns.UploadsBucketName(), "UBL_UPLOADS"; got != want {
		t.Errorf("UploadsBucketName() = %q, want %q", got, want)
	}
	if got, want := ns.ValidationExecutionsBucketName(), "UBL_VALIDATION_EXECUTIONS"; got != want {
		t.Errorf("ValidationExecutionsBucketName() = %q, want %q", got, want)
	}
	if got, want := ns.ValidationStageEvidenceBucketName(), "UBL_VALIDATION_STAGE_EVIDENCE"; got != want {
		t.Errorf("ValidationStageEvidenceBucketName() = %q, want %q", got, want)
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
	if got, want := ns.ResultSubject("00000000000000000000000000000000"), "ubl.validation.results.00000000000000000000000000000000"; got != want {
		t.Errorf("ResultSubject() = %q, want %q", got, want)
	}
	if got, want := ns.ResultsFilterSubject(), "ubl.validation.results.*"; got != want {
		t.Errorf("ResultsFilterSubject() = %q, want %q", got, want)
	}

	// NewNamespace("") must be indistinguishable from the zero value.
	if got, err := queue.NewNamespace(""); err != nil || got != queue.DefaultNamespace {
		t.Errorf("NewNamespace(\"\") = (%q, %v), want (DefaultNamespace, nil)", got, err)
	}
	if got, err := queue.NewNamespace("   "); err != nil || got != queue.DefaultNamespace {
		t.Errorf("NewNamespace(\"   \") = (%q, %v), want (DefaultNamespace, nil)", got, err)
	}
}

func TestNamespaceDerivesNamesFromExample(t *testing.T) {
	t.Parallel()
	// The exact example from TODO.md's TODO-001.
	ns := queue.MustNamespace("eubl21.prod")

	if got, want := ns.DocumentSubject("Invoice"), "eubl21.prod.documents.Invoice"; got != want {
		t.Errorf("DocumentSubject(%q) = %q, want %q", "Invoice", got, want)
	}
	if got, want := ns.DocumentSubject("CreditNote"), "eubl21.prod.documents.CreditNote"; got != want {
		t.Errorf("DocumentSubject(%q) = %q, want %q", "CreditNote", got, want)
	}
	if got, want := ns.ResultsSubject(), "eubl21.prod.validation.results"; got != want {
		t.Errorf("ResultsSubject() = %q, want %q", got, want)
	}
	if got, want := ns.ResultsFilterSubject(), "eubl21.prod.validation.results.*"; got != want {
		t.Errorf("ResultsFilterSubject() = %q, want %q", got, want)
	}
	if got, want := ns.DocumentsStreamName(), "EUBL21_PROD_DOCUMENTS"; got != want {
		t.Errorf("DocumentsStreamName() = %q, want %q", got, want)
	}
	if got, want := ns.ResultsStreamName(), "EUBL21_PROD_VALIDATION_RESULTS_BY_UPLOAD"; got != want {
		t.Errorf("ResultsStreamName() = %q, want %q", got, want)
	}
	if got, want := ns.UploadsBucketName(), "EUBL21_PROD_UPLOADS"; got != want {
		t.Errorf("UploadsBucketName() = %q, want %q", got, want)
	}
	if got, want := ns.ValidationExecutionsBucketName(), "EUBL21_PROD_VALIDATION_EXECUTIONS"; got != want {
		t.Errorf("ValidationExecutionsBucketName() = %q, want %q", got, want)
	}
	if got, want := ns.ValidationStageEvidenceBucketName(), "EUBL21_PROD_VALIDATION_STAGE_EVIDENCE"; got != want {
		t.Errorf("ValidationStageEvidenceBucketName() = %q, want %q", got, want)
	}
}

func TestNamespaceDocumentTypeRoundTrip(t *testing.T) {
	t.Parallel()
	for _, ns := range []queue.Namespace{queue.DefaultNamespace, queue.MustNamespace("eubl21.dev"), queue.MustNamespace("eubl21.prod")} {
		subject := ns.DocumentSubject("Invoice")
		if got := ns.DocumentTypeFromSubject(subject); got != "Invoice" {
			t.Errorf("namespace %q: DocumentTypeFromSubject(%q) = %q, want %q", ns, subject, got, "Invoice")
		}
		if !ns.HasDocumentSubject(subject) {
			t.Errorf("namespace %q: HasDocumentSubject(%q) = false, want true", ns, subject)
		}
	}

	dev := queue.MustNamespace("eubl21.dev")
	prod := queue.MustNamespace("eubl21.prod")
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
		{"eubl21", "EUBL21"},
		{"eubl21.dev", "EUBL21_DEV"},
		{"eubl21.prod", "EUBL21_PROD"},
		{"eubl21.prod.eu", "EUBL21_PROD_EU"},
	}
	for _, tc := range cases {
		ns, err := queue.NewNamespace(tc.namespace)
		if err != nil {
			t.Errorf("NewNamespace(%q): unexpected error: %v", tc.namespace, err)
			continue
		}
		if got := ns.DocumentsStreamName(); got != tc.want+"_DOCUMENTS" {
			t.Errorf("NewNamespace(%q).DocumentsStreamName() = %q, want %q", tc.namespace, got, tc.want+"_DOCUMENTS")
		}
	}
}

// TestNewNamespaceRejectsAmbiguousOrMalformedValues locks in the hardening
// fix for the normalization-collision issue: without character-set
// validation, distinct raw namespace values such as "eubl21.prod" and
// "eubl21--prod" (or "Eubl21.Prod") would derive the exact same physical
// stream/subject/bucket names (both "EUBL21_PROD..."), silently colliding.
// NewNamespace now rejects every value outside the safe, unambiguous
// dot-separated-lowercase-alphanumeric format up front.
func TestNewNamespaceRejectsAmbiguousOrMalformedValues(t *testing.T) {
	t.Parallel()
	invalid := []string{
		"ubl",          // reserved: collides with DefaultNamespace's physical names
		"eubl21--prod", // dashes are not part of the allowed charset
		"Eubl21.Prod",  // mixed case would collide with "eubl21.prod"
		"eubl21.prod.", // trailing separator
		".eubl21.prod", // leading separator
		"eubl21..prod", // consecutive separators
		"eubl21_prod",  // underscores collide with the derived resource-name separator
		"eubl21 prod",  // whitespace inside the value (not merely surrounding it)
		"eubl21.pröd",  // non-ASCII
	}
	for _, raw := range invalid {
		if _, err := queue.NewNamespace(raw); err == nil {
			t.Errorf("NewNamespace(%q): expected an error, got none", raw)
		}
	}

	// Confirm the specific collision example from the hardening review no
	// longer exists: "eubl21.prod" and "eubl21--prod" cannot both become
	// valid namespaces deriving "EUBL21_PROD_DOCUMENTS".
	if _, err := queue.NewNamespace("eubl21--prod"); err == nil {
		t.Fatal("NewNamespace(\"eubl21--prod\"): expected an error")
	}
	prod, err := queue.NewNamespace("eubl21.prod")
	if err != nil {
		t.Fatalf("NewNamespace(\"eubl21.prod\"): unexpected error: %v", err)
	}
	if got, want := prod.DocumentsStreamName(), "EUBL21_PROD_DOCUMENTS"; got != want {
		t.Errorf("DocumentsStreamName() = %q, want %q", got, want)
	}
}

func TestLegacyPhysicalNamespaceIsReserved(t *testing.T) {
	t.Parallel()

	if queue.ValidNamespace("ubl") {
		t.Fatal("ValidNamespace(\"ubl\") = true, want false")
	}
	if _, err := queue.NewNamespace("ubl"); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("NewNamespace(\"ubl\") error = %v, want reserved-namespace error", err)
	}
	if _, err := queue.NewNamespace(" ubl "); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("NewNamespace(\" ubl \") error = %v, want reserved-namespace error", err)
	}
}

func TestConnectRevalidatesDirectlyConvertedNamespace(t *testing.T) {
	server := queuetest.StartServer(t)
	for _, raw := range []string{"ubl", "bad_namespace", "EUBL21", " eubl21 "} {
		client, err := queue.ConnectWithOptions(server.URL, queue.Namespace(raw), server.Options...)
		if client != nil {
			client.Close()
			t.Errorf("ConnectWithOptions with namespace %q returned a client", raw)
		}
		if err == nil {
			t.Errorf("ConnectWithOptions with namespace %q returned no error", raw)
		}
	}
}

func TestExplicitNamespacesCannotDeriveDefaultPhysicalNames(t *testing.T) {
	t.Parallel()

	defaultStage, err := queue.DefaultNamespace.StageQueue(queue.ValidationLayerUBL21XSD)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"a", "ubl2", "customer1", "eubl21", "eubl21.prod", "z9.eu1"} {
		ns := queue.MustNamespace(raw)
		stage, err := ns.StageQueue(queue.ValidationLayerUBL21XSD)
		if err != nil {
			t.Fatalf("namespace %q: StageQueue: %v", raw, err)
		}
		if stage == defaultStage {
			t.Errorf("namespace %q derived the default stage topology: %+v", raw, stage)
		}
		if got, want := ns.DocumentSubject("Invoice"), queue.DefaultNamespace.DocumentSubject("Invoice"); got == want {
			t.Errorf("namespace %q derived default document subject %q", raw, got)
		}
		if got, want := ns.ResultsStreamName(), queue.DefaultNamespace.ResultsStreamName(); got == want {
			t.Errorf("namespace %q derived default results stream %q", raw, got)
		}
		if got, want := ns.UploadsBucketName(), queue.DefaultNamespace.UploadsBucketName(); got == want {
			t.Errorf("namespace %q derived default uploads bucket %q", raw, got)
		}
		if got, want := ns.ValidationExecutionsBucketName(), queue.DefaultNamespace.ValidationExecutionsBucketName(); got == want {
			t.Errorf("namespace %q derived default executions bucket %q", raw, got)
		}
		if got, want := ns.ValidationStageEvidenceBucketName(), queue.DefaultNamespace.ValidationStageEvidenceBucketName(); got == want {
			t.Errorf("namespace %q derived default stage-evidence bucket %q", raw, got)
		}
		if got, want := ns.UploadContentBucketName(), queue.DefaultNamespace.UploadContentBucketName(); got == want {
			t.Errorf("namespace %q derived default content bucket %q", raw, got)
		}
	}
}

// TestNamespaceMatchesTODO001CheckExamples verifies NewNamespace's accepted
// and rejected forms match TODO-001-CHECK.MD's rule
// (^[a-z0-9]+(?:\.[a-z0-9]+)*$) and its worked examples verbatim.
func TestNamespaceMatchesTODO001CheckExamples(t *testing.T) {
	t.Parallel()

	valid := []string{
		"eubl21",
		"eubl21.dev",
		"eubl21.test",
		"eubl21.prod",
		"customer1.prod",
	}
	for _, raw := range valid {
		if _, err := queue.NewNamespace(raw); err != nil {
			t.Errorf("NewNamespace(%q): unexpected error: %v", raw, err)
		}
	}

	invalid := []string{
		"eubl21--prod",
		"eubl21_prod",
		".eubl21.prod",
		"eubl21.prod.",
		"eubl21..prod",
		"EUBL21.Prod",
	}
	for _, raw := range invalid {
		if _, err := queue.NewNamespace(raw); err == nil {
			t.Errorf("NewNamespace(%q): expected an error, got none", raw)
		}
	}

	// The check doc's worked example: eubl21.prod derives the subjects
	// eubl21.prod.documents.Invoice / eubl21.prod.validation.results.<upload-id> and
	// the NATS resource prefix EUBL21_PROD.
	ns := queue.MustNamespace("eubl21.prod")
	if got, want := ns.DocumentSubject("Invoice"), "eubl21.prod.documents.Invoice"; got != want {
		t.Errorf("DocumentSubject(%q) = %q, want %q", "Invoice", got, want)
	}
	if got, want := ns.ResultsSubject(), "eubl21.prod.validation.results"; got != want {
		t.Errorf("ResultsSubject() = %q, want %q", got, want)
	}
	if got, want := ns.DocumentsStreamName(), "EUBL21_PROD_DOCUMENTS"; got != want {
		t.Errorf("DocumentsStreamName() = %q, want %q", got, want)
	}
}

func TestNamespaceDerivesIndependentStageQueueNames(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		namespace queue.Namespace
		stage     queue.ValidationLayer
		want      queue.StageQueueNames
	}{
		{
			name: "default XSD", namespace: queue.DefaultNamespace, stage: queue.ValidationLayerUBL21XSD,
			want: queue.StageQueueNames{
				RequestSubject: "ubl.validation.request.xsd", ResultSubject: "ubl.validation.result.xsd",
				DeadLetterSubjectPrefix: "ubl.validation.dead_letter.xsd.", DeadLetterFilterSubject: "ubl.validation.dead_letter.xsd.*",
				RequestStream: "UBL_VALIDATION_REQUEST_XSD", ResultStream: "UBL_VALIDATION_RESULT_XSD",
				DeadLetterStream: "UBL_VALIDATION_DEAD_LETTER_XSD",
				RequestConsumer:  "UBL_VALIDATION_XSD_WORKER", ResultConsumer: "UBL_VALIDATION_XSD_ORCHESTRATOR",
			},
		},
		{
			name: "namespaced EN", namespace: queue.MustNamespace("eubl21.prod"), stage: queue.ValidationLayerEN16931,
			want: queue.StageQueueNames{
				RequestSubject: "eubl21.prod.validation.request.en16931", ResultSubject: "eubl21.prod.validation.result.en16931",
				DeadLetterSubjectPrefix: "eubl21.prod.validation.dead_letter.en16931.", DeadLetterFilterSubject: "eubl21.prod.validation.dead_letter.en16931.*",
				RequestStream: "EUBL21_PROD_VALIDATION_REQUEST_EN16931", ResultStream: "EUBL21_PROD_VALIDATION_RESULT_EN16931",
				DeadLetterStream: "EUBL21_PROD_VALIDATION_DEAD_LETTER_EN16931",
				RequestConsumer:  "EUBL21_PROD_VALIDATION_EN16931_WORKER", ResultConsumer: "EUBL21_PROD_VALIDATION_EN16931_ORCHESTRATOR",
			},
		},
		{
			name: "reserved Peppol", namespace: queue.MustNamespace("eubl21.dev"), stage: queue.ValidationLayerPeppolBISBilling,
			want: queue.StageQueueNames{
				RequestSubject: "eubl21.dev.validation.request.peppol", ResultSubject: "eubl21.dev.validation.result.peppol",
				DeadLetterSubjectPrefix: "eubl21.dev.validation.dead_letter.peppol.", DeadLetterFilterSubject: "eubl21.dev.validation.dead_letter.peppol.*",
				RequestStream: "EUBL21_DEV_VALIDATION_REQUEST_PEPPOL", ResultStream: "EUBL21_DEV_VALIDATION_RESULT_PEPPOL",
				DeadLetterStream: "EUBL21_DEV_VALIDATION_DEAD_LETTER_PEPPOL",
				RequestConsumer:  "EUBL21_DEV_VALIDATION_PEPPOL_WORKER", ResultConsumer: "EUBL21_DEV_VALIDATION_PEPPOL_ORCHESTRATOR",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.namespace.StageQueue(test.stage)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("StageQueue(%q) = %+v, want %+v", test.stage, got, test.want)
			}
		})
	}
	if _, err := queue.DefaultNamespace.StageQueue("worker-internal-stage"); err == nil {
		t.Fatal("unknown validation stage acquired queue names")
	}
}
