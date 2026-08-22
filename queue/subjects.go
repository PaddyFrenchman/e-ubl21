package queue

import "strings"

// Stream and KV bucket names used across the e-ubl21 services.
const (
	// DocumentsStreamName is the JetStream stream holding incoming UBL
	// documents, one subject per document type (see DocumentSubject). It
	// uses work-queue retention: each message is removed once the
	// validation worker acknowledges it, since it is only ever meant to be
	// processed once.
	DocumentsStreamName = "UBL_DOCUMENTS"

	// ResultsStreamName is the single output stream validation results are
	// published to (see ResultsSubject).
	ResultsStreamName = "UBL_VALIDATION_RESULTS"

	// UploadsBucketName is the JetStream KV bucket holding the durable
	// record (status, error, original content) for every upload, keyed by
	// upload id. This is the canonical store of record for uploads,
	// replacing local filesystem persistence.
	UploadsBucketName = "UBL_UPLOADS"
)

const (
	documentSubjectPrefix = "ubl.documents."

	// documentSubjectWildcard is registered as the DocumentsStreamName's
	// subject filter, matching ubl.documents.<AnyDocumentType>.
	documentSubjectWildcard = documentSubjectPrefix + "*"

	// DocumentsFilterSubject is the default consumer filter subject used to
	// receive documents of every type from a single worker. A worker can be
	// pointed at a narrower filter (e.g. "ubl.documents.Invoice") to
	// dedicate itself to one document type's queue instead.
	DocumentsFilterSubject = documentSubjectPrefix + ">"

	// ResultsSubject is the single subject validation results are published
	// to, regardless of document type (the type is carried in the payload).
	ResultsSubject = "ubl.validation.results"
)

// Headers attached to every document message published to
// DocumentsStreamName, carrying the metadata a consumer needs without
// having to parse the payload.
const (
	HeaderUploadID     = "Ubl-Upload-Id"
	HeaderDocumentType = "Ubl-Document-Type"
	HeaderReceivedAt   = "Ubl-Received-At"
)

// DocumentSubject returns the per-document-type subject a document of the
// given UBL 2.1 root element type (e.g. "Invoice", "CreditNote") is
// published to and consumed from.
func DocumentSubject(documentType string) string {
	return documentSubjectPrefix + documentType
}

// DocumentTypeFromSubject extracts the document type from a subject
// produced by DocumentSubject (the inverse operation).
func DocumentTypeFromSubject(subject string) string {
	return strings.TrimPrefix(subject, documentSubjectPrefix)
}
