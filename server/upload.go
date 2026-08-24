// This file implements the UBL upload and retrieval endpoints.
// POST /api/v1/ubl/upload performs only cheap, synchronous checks (body
// well-formedness and that the root element - local name and namespace -
// is a recognized UBL 2.1 document type), then queues the raw document on
// its per-document-type NATS JetStream subject (see queue.PublishDocument)
// for asynchronous XSD validation by the worker (worker.go), and creates a
// "pending" record in the JetStream KV upload store
// (queue.PutUploadRecord). GET /api/v1/ubl/{id} reads that same KV-backed
// record, reflecting whatever status the worker has since recorded
// (pending/valid/invalid).
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

	record := queue.UploadRecord{
		ID:           id,
		DocumentType: rootElement,
		Status:       queue.StatusPending,
		Content:      string(body),
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

	response := map[string]any{
		"id":         record.ID,
		"document":   record.DocumentType,
		"status":     string(record.Status),
		"receivedAt": record.ReceivedAt,
		"content":    record.Content,
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

// rootElementOfPayload validates that body is well-formed enough to have a
// root element, and returns that root element's local name and namespace
// URI without decoding the rest of the document: it stops as soon as the
// first xml.StartElement token is produced, so a large payload's root
// element is identified in time proportional to the position of its
// opening tag rather than the size of the whole document.
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
