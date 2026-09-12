package queue

import (
	"fmt"
	"regexp"
	"strings"
)

// Headers attached to every document message published to the documents
// stream, carrying the metadata a consumer needs without having to parse
// the payload.
const (
	HeaderUploadID     = "Ubl-Upload-Id"
	HeaderDocumentType = "Ubl-Document-Type"
	HeaderReceivedAt   = "Ubl-Received-At"
	HeaderValidationID = "Ubl-Validation-Id"
	HeaderGeneration   = "Ubl-Validation-Generation"
	// HeaderReplayMessageID identifies descriptors created by operator replay.
	// Unlike Nats-Msg-Id it is retained for the worker's durable claim check.
	HeaderReplayMessageID = "Ubl-Replay-Message-Id"
)

// The legacy constants are the fixed stream/subject/bucket names e-ubl21 used
// before the queue namespace setting existed. DefaultNamespace ("") preserves
// them except for the versioned result stream: permanent per-upload result
// idempotence requires a new stream so upgrading cannot compact or rewrite the
// old single-subject UBL_VALIDATION_RESULTS stream.
const (
	// legacyPhysicalNamespace is reserved for DefaultNamespace. Allowing it
	// as an explicit Namespace would derive the same subjects and resources
	// as the default deployment and defeat namespace isolation.
	legacyPhysicalNamespace               = "ubl"
	legacyDocumentsStreamName             = "UBL_DOCUMENTS"
	defaultResultsStreamName              = "UBL_VALIDATION_RESULTS_BY_UPLOAD"
	legacyUploadsBucketName               = "UBL_UPLOADS"
	defaultValidationExecutionsBucketName = "UBL_VALIDATION_EXECUTIONS"
	defaultValidationEvidenceBucketName   = "UBL_VALIDATION_STAGE_EVIDENCE"
	defaultDeploymentGenerationBucketName = "UBL_DEPLOYMENT_GENERATION"
	legacyUploadContentBucketName         = "UBL_UPLOAD_CONTENT"
	legacyDocumentSubjectPrefix           = "ubl.documents."
	legacyResultsSubject                  = "ubl.validation.results"
	legacyDeadLettersSubject              = "ubl.validation.dead_letters"
	defaultDeadLettersStreamName          = "UBL_VALIDATION_DEAD_LETTERS"
)

// DefaultNamespace is the queue namespace used when EUBL21_QUEUE_NAMESPACE
// is not set. It preserves original resources and subjects; explicitly
// versioned resources such as the per-upload result stream use a new name so
// upgrades do not mutate legacy data.
const DefaultNamespace = Namespace("")

// Namespace scopes every JetStream stream, subject and KV bucket e-ubl21
// uses to a single logical deployment, so multiple environments (e.g. dev
// and prod) can safely share one NATS/JetStream cluster without colliding
// on stream, subject or bucket names.
//
// A Namespace is normally created from the EUBL21_QUEUE_NAMESPACE setting
// (see cmd.Config.QueueNamespace) via NewNamespace, e.g.:
//
//	ns, err := queue.NewNamespace("eubl21.prod")
//	ns.DocumentSubject("Invoice") // "eubl21.prod.documents.Invoice"
//	ns.DocumentsStreamName()      // "EUBL21_PROD_DOCUMENTS"
//
// The zero value (DefaultNamespace) retains e-ubl21's original hard-coded
// namespace. The explicit value "ubl" is reserved because it would derive
// those same physical names. Existing resources remain untouched when a
// versioned replacement is required for a new storage contract.
type Namespace string

// StageQueueNames is the complete namespace-derived physical topology for
// one logical validation stage. Request, result, and dead-letter streams are
// separate so their ownership, retention and scaling cannot overlap.
type StageQueueNames struct {
	RequestSubject          string
	ResultSubject           string
	DeadLetterSubjectPrefix string
	DeadLetterFilterSubject string
	RequestStream           string
	ResultStream            string
	DeadLetterStream        string
	RequestConsumer         string
	ResultConsumer          string
}

// namespacePattern restricts a non-default Namespace's raw value to one or
// more dot-separated segments of lowercase ASCII letters and digits (e.g.
// "eubl21", "eubl21.prod", "eubl21.prod.eu"): no leading, trailing or
// consecutive dots, and no characters outside [a-z0-9.].
//
// This restricted charset is what keeps resourcePrefix's normalization
// (dots -> underscores, upper-cased) one-to-one: every character
// resourcePrefix could otherwise "fold" into the same normalized form
// (dashes, underscores, mixed case, ...) is rejected up front, so two
// distinct valid namespaces - e.g. "eubl21.prod" and "eubl21--prod", or
// "eubl21.prod" and "Eubl21.Prod" - can never derive the same physical
// stream, subject or KV bucket name.
var namespacePattern = regexp.MustCompile(`^[a-z0-9]+(\.[a-z0-9]+)*$`)

// ValidNamespace reports whether raw is a well-formed queue namespace value:
// either empty (DefaultNamespace), or matching namespacePattern and not the
// reserved legacy physical namespace "ubl".
func ValidNamespace(raw string) bool {
	return validateNamespace(raw) == nil
}

func validateNamespace(raw string) error {
	if raw == "" {
		return nil
	}
	if raw == legacyPhysicalNamespace {
		return fmt.Errorf("queue namespace %q is reserved for the legacy/default physical namespace", raw)
	}
	if !namespacePattern.MatchString(raw) {
		return fmt.Errorf("invalid queue namespace %q: must be empty, or one or more dot-separated lowercase alphanumeric segments (e.g. %q)", raw, "eubl21.prod")
	}
	return nil
}

// NewNamespace validates and returns the Namespace for the given raw
// EUBL21_QUEUE_NAMESPACE value, trimming surrounding whitespace first. An
// empty (or all-whitespace) raw value yields DefaultNamespace. NewNamespace
// returns an error if the trimmed value is non-empty and does not match
// namespacePattern (see ValidNamespace), so a malformed or ambiguous
// namespace is rejected at configuration time rather than silently
// producing derived names that could collide with another namespace's.
func NewNamespace(raw string) (Namespace, error) {
	trimmed := strings.TrimSpace(raw)
	if err := validateNamespace(trimmed); err != nil {
		return "", err
	}
	return Namespace(trimmed), nil
}

// MustNamespace is like NewNamespace but panics if raw is not a valid
// namespace. Intended for tests and other contexts constructing a
// Namespace from a fixed, known-valid literal, where a returned error would
// only ever indicate a programming mistake.
func MustNamespace(raw string) Namespace {
	ns, err := NewNamespace(raw)
	if err != nil {
		panic(err)
	}
	return ns
}

// String returns the namespace's raw configured value ("" for
// DefaultNamespace).
func (n Namespace) String() string {
	return string(n)
}

// documentSubjectPrefix returns the prefix every per-document-type subject
// in this namespace starts with.
func (n Namespace) documentSubjectPrefix() string {
	if n == DefaultNamespace {
		return legacyDocumentSubjectPrefix
	}
	return string(n) + ".documents."
}

// documentSubjectWildcard is registered as the namespace's documents
// stream's subject filter, matching every document type subject in the
// namespace.
func (n Namespace) documentSubjectWildcard() string {
	return n.documentSubjectPrefix() + "*"
}

// DocumentsFilterSubject is the default consumer filter subject used to
// receive documents of every type, within this namespace, from a single
// worker. A worker can be pointed at a narrower filter (e.g.
// DocumentSubject("Invoice")) to dedicate itself to one document type's
// queue instead.
func (n Namespace) DocumentsFilterSubject() string {
	return n.documentSubjectPrefix() + ">"
}

// DocumentSubject returns the per-document-type subject, within this
// namespace, a document of the given UBL 2.1 root element type (e.g.
// "Invoice", "CreditNote") is published to and consumed from.
func (n Namespace) DocumentSubject(documentType string) string {
	return n.documentSubjectPrefix() + documentType
}

// DocumentTypeFromSubject extracts the document type from a subject
// produced by DocumentSubject for this namespace (the inverse operation).
func (n Namespace) DocumentTypeFromSubject(subject string) string {
	return strings.TrimPrefix(subject, n.documentSubjectPrefix())
}

// HasDocumentSubject reports whether subject belongs to this namespace's
// documents subject space, i.e. whether it starts with this namespace's
// document subject prefix. Used to reject a worker subject filter that
// belongs to a different namespace before it is used to create a consumer.
func (n Namespace) HasDocumentSubject(subject string) bool {
	return strings.HasPrefix(subject, n.documentSubjectPrefix())
}

// ResultsSubject is the stable base of this namespace's validation-result
// subject space. Results are published to ResultSubject(resultID), not
// directly to this base subject. Legacy upload-scoped payloads use uploadID as
// their result token during migration.
func (n Namespace) ResultsSubject() string {
	if n == DefaultNamespace {
		return legacyResultsSubject
	}
	return string(n) + ".validation.results"
}

// ResultSubject returns the unique subject used to publish one permanent
// result. Legacy results use the upload ID token; execution-scoped results use
// their deterministic ResultID. The single token lets JetStream enforce
// permanent first-result insertion independently of message-ID deduplication.
func (n Namespace) ResultSubject(resultID string) string {
	return n.ResultsSubject() + "." + resultID
}

// ResultsFilterSubject matches every permanent validation-result subject in
// this namespace. Downstream consumers should use this filter rather than the
// ResultsSubject base.
func (n Namespace) ResultsFilterSubject() string {
	return n.ResultsSubject() + ".*"
}

func (n Namespace) DeadLetterSubject(uploadID string) string {
	if n == DefaultNamespace {
		return legacyDeadLettersSubject + "." + uploadID
	}
	return string(n) + ".validation.dead_letters." + uploadID
}

func (n Namespace) DeadLettersFilterSubject() string {
	if n == DefaultNamespace {
		return legacyDeadLettersSubject + ".*"
	}
	return string(n) + ".validation.dead_letters.*"
}

func (n Namespace) DeadLettersStreamName() string {
	if n == DefaultNamespace {
		return defaultDeadLettersStreamName
	}
	return n.resourcePrefix() + "_VALIDATION_DEAD_LETTERS"
}

// StageQueue returns the exact subjects, streams and durable consumers for a
// supported validation stage. Unknown stages fail closed instead of deriving
// a subject that no intended worker owns.
func (n Namespace) StageQueue(stage ValidationLayer) (StageQueueNames, error) {
	token, ok := stageQueueToken(stage)
	if !ok {
		return StageQueueNames{}, fmt.Errorf("unsupported validation stage %q", stage)
	}
	subjectPrefix := string(n)
	resourcePrefix := n.resourcePrefix()
	if n == DefaultNamespace {
		subjectPrefix = legacyPhysicalNamespace
		resourcePrefix = strings.ToUpper(legacyPhysicalNamespace)
	}
	resourceToken := strings.ToUpper(token)
	return StageQueueNames{
		RequestSubject:          subjectPrefix + ".validation.request." + token,
		ResultSubject:           subjectPrefix + ".validation.result." + token,
		DeadLetterSubjectPrefix: subjectPrefix + ".validation.dead_letter." + token + ".",
		DeadLetterFilterSubject: subjectPrefix + ".validation.dead_letter." + token + ".*",
		RequestStream:           resourcePrefix + "_VALIDATION_REQUEST_" + resourceToken,
		ResultStream:            resourcePrefix + "_VALIDATION_RESULT_" + resourceToken,
		DeadLetterStream:        resourcePrefix + "_VALIDATION_DEAD_LETTER_" + resourceToken,
		RequestConsumer:         resourcePrefix + "_VALIDATION_" + resourceToken + "_WORKER",
		ResultConsumer:          resourcePrefix + "_VALIDATION_" + resourceToken + "_ORCHESTRATOR",
	}, nil
}

func stageQueueToken(stage ValidationLayer) (string, bool) {
	switch stage {
	case ValidationLayerUBL21XSD:
		return "xsd", true
	case ValidationLayerEN16931:
		return "en16931", true
	case ValidationLayerPeppolBISBilling:
		return "peppol", true
	case ValidationLayerPeppolEnvelope:
		return "peppol_envelope", true
	default:
		return "", false
	}
}

// DocumentsStreamName is the JetStream stream, within this namespace,
// holding incoming UBL documents, one subject per document type (see
// DocumentSubject). It uses work-queue retention: each message is removed
// once the validation worker acknowledges it, since it is only ever meant
// to be processed once.
func (n Namespace) DocumentsStreamName() string {
	if n == DefaultNamespace {
		return legacyDocumentsStreamName
	}
	return n.resourcePrefix() + "_DOCUMENTS"
}

// ResultsStreamName is the single output stream, within this namespace,
// validation results are published to (see ResultSubject).
func (n Namespace) ResultsStreamName() string {
	if n == DefaultNamespace {
		return defaultResultsStreamName
	}
	return n.resourcePrefix() + "_VALIDATION_RESULTS_BY_UPLOAD"
}

// UploadsBucketName is the JetStream KV bucket, within this namespace,
// holding the durable metadata record (status, error, timestamps) for
// every upload, keyed by upload id. The raw XML content itself is stored
// separately - see UploadContentBucketName - so that updating an upload's
// status never has to read or rewrite its (potentially large) content.
func (n Namespace) UploadsBucketName() string {
	if n == DefaultNamespace {
		return legacyUploadsBucketName
	}
	return n.resourcePrefix() + "_UPLOADS"
}

// ValidationExecutionsBucketName is the namespace-scoped KV bucket containing
// durable execution generations and their frozen plans. It is separate from
// upload acceptance metadata because one immutable upload may have multiple
// validation lineages and generations.
func (n Namespace) ValidationExecutionsBucketName() string {
	if n == DefaultNamespace {
		return defaultValidationExecutionsBucketName
	}
	return n.resourcePrefix() + "_VALIDATION_EXECUTIONS"
}

// ValidationStageEvidenceBucketName is the namespace-scoped KV bucket
// containing immutable complete StageResult evidence, one entry per logical
// stage attempt.
func (n Namespace) ValidationStageEvidenceBucketName() string {
	if n == DefaultNamespace {
		return defaultValidationEvidenceBucketName
	}
	return n.resourcePrefix() + "_VALIDATION_STAGE_EVIDENCE"
}

// DeploymentGenerationBucketName is the namespace-scoped KV bucket holding
// the migration barrier between the legacy combined worker and split stages.
func (n Namespace) DeploymentGenerationBucketName() string {
	if n == DefaultNamespace {
		return defaultDeploymentGenerationBucketName
	}
	return n.resourcePrefix() + "_DEPLOYMENT_GENERATION"
}

// UploadContentBucketName is the JetStream KV bucket, within this
// namespace, holding the immutable raw XML content of every upload, keyed
// by the same upload id as UploadsBucketName. Kept separate from the
// mutable status/metadata record so that repeated status updates/polling
// never need to read or rewrite the (potentially large) document content.
func (n Namespace) UploadContentBucketName() string {
	if n == DefaultNamespace {
		return legacyUploadContentBucketName
	}
	return n.resourcePrefix() + "_UPLOAD_CONTENT"
}

// resourcePrefix normalizes the namespace's raw value into the prefix used
// to derive JetStream stream/KV bucket names, which - unlike subjects -
// only allow a restricted character set (alphanumerics, dashes,
// underscores): every "." separator becomes "_" and the result is
// upper-cased, so "eubl21.prod" derives "EUBL21_PROD".
//
// This normalization is only guaranteed to be one-to-one (i.e. two
// distinct namespaces can never derive the same resource prefix) for a
// namespace value that has passed NewNamespace's validation - see
// namespacePattern. The fallback handling of characters outside
// [a-z0-9.] below is defense in depth for a Namespace constructed by
// direct string conversion rather than NewNamespace; it deliberately does
// not attempt to be collision-free for such values.
func (n Namespace) resourcePrefix() string {
	var b strings.Builder
	lastWasSeparator := true // trims any leading separator
	for _, r := range string(n) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r - ('a' - 'A'))
			lastWasSeparator = false
		case (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastWasSeparator = false
		default:
			if !lastWasSeparator {
				b.WriteByte('_')
				lastWasSeparator = true
			}
		}
	}
	return strings.TrimSuffix(b.String(), "_")
}
