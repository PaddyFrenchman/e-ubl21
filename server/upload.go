// This file implements the UBL upload and retrieval endpoints.
// POST /api/v1/ubl/upload performs only cheap, synchronous checks (that a
// syntactically valid root element is present, and that its local name and
// namespace identify a recognized UBL 2.1 document type), then stores the
// raw document content (queue.PutUploadContent), creates a "pending"
// status/metadata record (queue.PutUploadRecord), and queues the document
// on its per-document-type NATS JetStream subject (queue.PublishDocument)
// for asynchronous XSD validation by the worker (worker.go). GET
// /api/v1/ubl/{id} reads the status/metadata record and the stored content
// back together, reflecting whatever status the worker has since recorded
// (pending/valid/invalid).
//
// Content and status/metadata are stored separately (see
// queue.UploadRecord) precisely so that this handler never needs more than
// one copy of the request body: body is read once, handed to
// PutUploadContent and PublishDocument as-is, and never converted to a
// string on this path. Converting to a string only happens once, in
// handleGetUpload, when a single upload's content is serialized into a
// JSON response.
//
// Root-element detection deliberately stops parsing after the root
// element's opening tag (see rootElementOfPayload), so it does not itself
// establish that the rest of the document is well-formed XML: that, like
// UBL 2.1 schema validity, is determined later by the worker's xmllint run.
package server

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
)

// maxUploadBytes bounds a single upload's request body. It is enforced
// exactly (see readUploadBody): a body at or under the limit is accepted in
// full, and a body over the limit is rejected outright with 413, never
// silently truncated and processed as if it were the caller's whole
// document.
const maxUploadBytes = 10 << 20

// errPayloadTooLarge is returned by readUploadBody when the request body
// exceeds maxUploadBytes.
var errPayloadTooLarge = fmt.Errorf("request body exceeds the maximum allowed size of %d bytes", maxUploadBytes)

// uploadHandlers bundles the dependencies the upload/retrieval endpoints
// need: the schema catalog used to recognize supported document types, and
// the queue client used to enqueue documents and read/write upload records.
type uploadHandlers struct {
	catalog *schemaCatalog
	queue   *queue.Client
}

func (h *uploadHandlers) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	body, err := readUploadBody(w, r)
	if err != nil {
		if errors.Is(err, errPayloadTooLarge) {
			recordUploadOutcome("payload_too_large")
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": err.Error()})
			return
		}
		recordUploadOutcome("bad_request")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	localName, namespace, err := rootElementOfPayload(body)
	if err != nil {
		recordUploadOutcome("invalid_xml")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid XML payload: " + err.Error()})
		return
	}

	// Only a cheap catalog lookup (no xmllint invocation) happens
	// synchronously here, so an unrecognized document type is rejected
	// immediately rather than being queued for a worker to discover later.
	// Both the root element's local name and its namespace must match a
	// recognized UBL 2.1 main document type - matching local name alone
	// would let an arbitrary XML document with a coincidentally-named
	// root element (e.g. a bare, unqualified "Invoice") pass this check
	// only to fail XSD validation later (see
	// SERVER-DATAFLOW-2026-08-24.md section 11.11).
	expectedNamespace, ok := h.catalog.expectedNamespaceFor(localName)
	if !ok || namespace != expectedNamespace {
		recordUploadOutcome("unsupported_document_type")
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("unsupported UBL 2.1 document type: root element %q in namespace %q is not a recognized main document", localName, namespace),
		})
		return
	}
	rootElement := localName

	id, err := queue.NewID()
	if err != nil {
		recordUploadOutcome("internal_error")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate upload id"})
		return
	}
	receivedAt := time.Now().UTC()

	// The raw content is stored separately from the mutable status/metadata
	// record (see queue.UploadRecord), so that later status updates never
	// need to read or rewrite the document body. It is written first, and
	// directly from body with no []byte<->string conversion or JSON
	// wrapping, so a GET immediately after this response never observes a
	// metadata record whose content is missing.
	if err := h.queue.PutUploadContent(r.Context(), id, body); err != nil {
		recordUploadOutcome("internal_error")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to persist upload content: " + err.Error()})
		return
	}

	record := queue.UploadRecord{
		ID:           id,
		DocumentType: rootElement,
		Status:       queue.StatusPending,
		ReceivedAt:   receivedAt.Format(time.RFC3339),
	}
	if err := h.queue.PutUploadRecord(r.Context(), record); err != nil {
		recordUploadOutcome("internal_error")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to persist upload record: " + err.Error()})
		return
	}

	// The record above is written before the document is queued, so a GET
	// immediately after this response always finds at least the pending
	// record, even if the enqueue below somehow raced with a client's very
	// first status check.
	if err := h.queue.PublishDocument(r.Context(), id, rootElement, receivedAt, body); err != nil {
		recordUploadOutcome("internal_error")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to queue upload for validation: " + err.Error()})
		return
	}

	recordUploadOutcome("queued")
	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":     string(queue.StatusPending),
		"id":         id,
		"document":   rootElement,
		"receivedAt": record.ReceivedAt,
	})
}

func (h *uploadHandlers) handleGetUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	id := r.PathValue("id")
	record, err := h.queue.GetUploadRecord(r.Context(), id)
	if err != nil {
		if errors.Is(err, queue.ErrRecordNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "upload not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load upload: " + err.Error()})
		return
	}

	// The metadata record above always exists by the time content does
	// (handleUpload writes content first), so a missing content entry here
	// indicates a genuine data-consistency problem rather than an
	// in-progress upload; report it as an internal error rather than
	// silently returning empty content.
	content, err := h.queue.GetUploadContent(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load upload content: " + err.Error()})
		return
	}

	response := map[string]any{
		"id":         record.ID,
		"document":   record.DocumentType,
		"status":     string(record.Status),
		"receivedAt": record.ReceivedAt,
		"content":    string(content),
	}
	if record.ValidatedAt != "" {
		response["validatedAt"] = record.ValidatedAt
	}
	if record.Error != "" {
		response["error"] = record.Error
	}
	writeJSON(w, http.StatusOK, response)
}

// readUploadBody reads and returns r.Body, enforcing maxUploadBytes
// exactly: a body over the limit is rejected with errPayloadTooLarge
// rather than silently truncated to the limit and processed as if that
// were the caller's complete document (the failure mode of the previous
// io.LimitReader-based implementation). When r.ContentLength already
// reports a size over the limit, the oversized body is rejected before any
// of it is read at all.
func readUploadBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if r.ContentLength == 0 {
		return nil, errors.New("request body is required")
	}
	if r.ContentLength > maxUploadBytes {
		return nil, errPayloadTooLarge
	}
	contentType := r.Header.Get("Content-Type")
	if !strings.Contains(contentType, "xml") && !strings.Contains(contentType, "octet-stream") {
		return nil, errors.New("content-type must be application/xml, text/xml, or application/octet-stream")
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, errPayloadTooLarge
		}
		return nil, err
	}
	return body, nil
}

// rootElementOfPayload validates that body has a syntactically valid root
// element - i.e. an opening tag the encoding/xml tokenizer can parse - and
// returns that root element's local name and namespace URI without
// decoding the rest of the document: it stops as soon as the first
// xml.StartElement token is produced, so a large payload's root element is
// identified in time proportional to the position of its opening tag
// rather than the size of the whole document. Consequently, it does not
// establish that the rest of the document is well-formed XML: a payload
// with a syntactically valid root element followed by a malformed tail is
// accepted here (see TestRootElementOfPayloadStopsAtFirstStartElement)
// and is only rejected later, once the worker's xmllint run needs to parse
// the complete document to validate it against its UBL 2.1 XSD.
func rootElementOfPayload(body []byte) (localName, namespace string, err error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return "", "", errors.New("empty XML payload")
	}
	if trimmed[0] != '<' {
		return "", "", errors.New("payload is not XML")
	}

	decoder := xml.NewDecoder(bytes.NewReader(body))
	for {
		tok, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				return "", "", errors.New("XML document root element is missing")
			}
			return "", "", err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local == "" {
			return "", "", errors.New("XML document root element is missing")
		}
		return start.Name.Local, start.Name.Space, nil
	}
}
