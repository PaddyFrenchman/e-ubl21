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
)

// legacyDocumentsStreamName, legacyResultsStreamName, legacyUploadsBucketName,
// legacyDocumentSubjectPrefix and legacyResultsSubject are the fixed
// stream/subject/bucket names e-ubl21 used before the queue namespace
// setting existed. DefaultNamespace ("") reproduces these exactly, so
// deployments that do not set EUBL21_QUEUE_NAMESPACE keep working against
// their existing JetStream state without any migration.
const (
	legacyDocumentsStreamName   = "UBL_DOCUMENTS"
	legacyResultsStreamName     = "UBL_VALIDATION_RESULTS"
	legacyUploadsBucketName     = "UBL_UPLOADS"
	legacyDocumentSubjectPrefix = "ubl.documents."
	legacyResultsSubject        = "ubl.validation.results"
)

// DefaultNamespace is the queue namespace used when EUBL21_QUEUE_NAMESPACE
// is not set, preserving e-ubl21's original, pre-namespace stream, subject
// and KV bucket names.
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
// The zero value (DefaultNamespace) reproduces e-ubl21's original hard-coded
// names, so existing single-tenant deployments are unaffected by leaving
// EUBL21_QUEUE_NAMESPACE unset.
type Namespace string

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
// either empty (DefaultNamespace) or matching namespacePattern.
func ValidNamespace(raw string) bool {
	return raw == "" || namespacePattern.MatchString(raw)
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
	if !ValidNamespace(trimmed) {
		return "", fmt.Errorf("invalid queue namespace %q: must be empty, or one or more dot-separated lowercase alphanumeric segments (e.g. %q)", raw, "eubl21.prod")
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

// ResultsSubject is the single subject, within this namespace, validation
// results are published to, regardless of document type (the type is
// carried in the payload).
func (n Namespace) ResultsSubject() string {
	if n == DefaultNamespace {
		return legacyResultsSubject
	}
	return string(n) + ".validation.results"
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
// validation results are published to (see ResultsSubject).
func (n Namespace) ResultsStreamName() string {
	if n == DefaultNamespace {
		return legacyResultsStreamName
	}
	return n.resourcePrefix() + "_VALIDATION_RESULTS"
}

// UploadsBucketName is the JetStream KV bucket, within this namespace,
// holding the durable record (status, error, original content) for every
// upload, keyed by upload id.
func (n Namespace) UploadsBucketName() string {
	if n == DefaultNamespace {
		return legacyUploadsBucketName
	}
	return n.resourcePrefix() + "_UPLOADS"
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
