// This file implements the UBL upload and retrieval endpoints.
// POST /api/v1/ubl/upload performs only cheap, synchronous checks (body
// well-formedness and that the root element is a recognized UBL 2.1
// document type), then queues the raw document on its per-document-type
// NATS JetStream subject (see queue.PublishDocument) for asynchronous XSD
// validation by the worker (worker.go), and creates a "pending" record in
// the JetStream KV upload store (queue.PutUploadRecord). GET
// /api/v1/ubl/{id} reads that same KV-backed record, reflecting whatever
// status the worker has since recorded (pending/valid/invalid).
package server

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
)

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

	body, err := readUploadBody(r)
	if err != nil {
		recordUploadOutcome("bad_request")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		recordUploadOutcome("bad_request")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "request body is empty"})
		return
	}

	rootElement, err := rootElementOfPayload(body)
	if err != nil {
		recordUploadOutcome("invalid_xml")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid XML payload: " + err.Error()})
		return
	}

	// Only a cheap catalog lookup (no xmllint invocation) happens
	// synchronously here, so an unrecognized document type is rejected
	// immediately rather than being queued for a worker to discover later.
	if _, ok := h.catalog.schemaPathFor(rootElement); !ok {
		recordUploadOutcome("unsupported_document_type")
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("unsupported UBL 2.1 document type: root element %q is not a recognized main document", rootElement),
		})
		return
	}

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

func readUploadBody(r *http.Request) ([]byte, error) {
	if r.ContentLength == 0 {
		return nil, errors.New("request body is required")
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	if err != nil {
		return nil, err
	}
	contentType := r.Header.Get("Content-Type")
	if !strings.Contains(contentType, "xml") && !strings.Contains(contentType, "octet-stream") {
		return nil, errors.New("content-type must be application/xml, text/xml, or application/octet-stream")
	}
	return body, nil
}

// rootElementOfPayload validates that body is well-formed XML with a root
// element and returns that root element's local name.
func rootElementOfPayload(body []byte) (string, error) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "", errors.New("empty XML payload")
	}
	if !strings.HasPrefix(trimmed, "<") {
		return "", errors.New("payload is not XML")
	}

	var doc struct {
		XMLName xml.Name
	}
	if err := xml.Unmarshal(body, &doc); err != nil {
		return "", err
	}
	if doc.XMLName.Local == "" {
		return "", errors.New("XML document root element is missing")
	}
	return doc.XMLName.Local, nil
}
