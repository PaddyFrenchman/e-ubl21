package queue

import "strings"

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
//	ns := queue.NewNamespace("eubl21.prod")
//	ns.DocumentSubject("Invoice") // "eubl21.prod.documents.Invoice"
//	ns.DocumentsStreamName()      // "EUBL21_PROD_DOCUMENTS"
//
// The zero value (DefaultNamespace) reproduces e-ubl21's original hard-coded
// names, so existing single-tenant deployments are unaffected by leaving
// EUBL21_QUEUE_NAMESPACE unset.
type Namespace string

// NewNamespace returns the Namespace for the given raw
// EUBL21_QUEUE_NAMESPACE value, trimming surrounding whitespace. An empty
// (or all-whitespace) raw value yields DefaultNamespace.
func NewNamespace(raw string) Namespace {
	return Namespace(strings.TrimSpace(raw))
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
// underscores). Every character outside that set (e.g. "." between
// namespace segments) becomes "_", runs of such separators collapse to one,
// and the result is upper-cased, so "eubl21.prod" derives "EUBL21_PROD".
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
