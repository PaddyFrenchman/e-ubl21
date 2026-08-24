// This file implements the UBL upload and retrieval endpoints.
// POST /api/v1/ubl/upload performs only cheap, synchronous checks (that a
// syntactically valid root element is present, and that its local name and
// namespace identify a recognized UBL 2.1 document type), then stores the
// raw document content (queue.PutUploadContent), creates a "pending"
// status/metadata record (queue.PutUploadRecord), and queues a compact work
// descriptor on its per-document-type NATS JetStream subject
// (queue.PublishDocument) for asynchronous XSD validation by the worker.
// GET /api/v1/ubl/{id} returns status/metadata only by default; callers can
// explicitly request the raw XML with ?includeContent=true.
package server

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
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

var errPayloadTooLarge = fmt.Errorf("request body exceeds the maximum allowed size of %d bytes", maxUploadBytes)

type uploadHandlers struct {
	catalog *documentCatalog
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

	// Raw XML is durably stored before metadata and queue publication. The
	// work stream therefore needs only the upload id/type/timestamp and the
	// worker can safely dereference the immutable object after delivery.
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

	if err := h.queue.PublishDocument(r.Context(), id, rootElement, receivedAt); err != nil {
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

	includeContent := false
	if raw := r.URL.Query().Get("includeContent"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "includeContent must be true or false"})
			return
		}
		includeContent = parsed
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
	}
	if record.ValidatedAt != "" {
		response["validatedAt"] = record.ValidatedAt
	}
	if record.Error != "" {
		response["error"] = record.Error
	}

	// Polling is deliberately metadata-only unless the caller explicitly
	// opts in. This keeps repeated status checks small even for 10 MiB XML.
	if includeContent {
		content, err := h.queue.GetUploadContent(r.Context(), id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load upload content: " + err.Error()})
			return
		}
		response["content"] = string(content)
	}

	writeJSON(w, http.StatusOK, response)
}

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
// element and returns its local name and namespace URI without decoding the
// rest of the document. Full well-formedness is established later by the
// worker's xmllint run.
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
