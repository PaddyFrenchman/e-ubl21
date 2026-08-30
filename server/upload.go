// This file implements the UBL upload and retrieval endpoints.
// POST /api/v1/ubl/upload performs only cheap, synchronous checks (that a
// syntactically valid root element is present, and that its local name and
// namespace identify a recognized UBL 2.1 document type), then stores the
// durable acceptance intent (queue.CreateUploadRecord), stores raw content
// (queue.PutUploadContent), and queues a compact work descriptor
// (queue.PublishDocument) for asynchronous XSD validation by the worker.
// GET /api/v1/ubl/{id} returns status/metadata only by default; callers can
// explicitly request the raw XML with ?includeContent=true.
package server

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/observability"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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
	queue   uploadQueue
}

type uploadQueue interface {
	CreateUploadRecord(context.Context, queue.UploadRecord) (uint64, error)
	UpdateUploadRecord(context.Context, queue.UploadRecord, uint64) (uint64, error)
	GetUploadRecord(context.Context, string) (queue.UploadRecord, error)
	GetUploadRecordVersion(context.Context, string) (queue.VersionedUploadRecord, error)
	PutUploadContent(context.Context, string, []byte) error
	GetUploadContent(context.Context, string) ([]byte, error)
	DeleteUploadContent(context.Context, string) error
	PublishDocument(context.Context, string, string, time.Time) error
}

func (h *uploadHandlers) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	ctx := observability.ExtractHTTP(r.Context(), r.Header)
	ctx, span := observability.Tracer().Start(ctx, "api.upload.accept",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(attribute.String("http.request.method", r.Method), attribute.String("http.route", "/api/v1/ubl/upload")),
	)
	r = r.WithContext(ctx)
	observability.InjectHTTP(ctx, w.Header())
	defer span.End()

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

	documentType := strings.TrimSpace(r.URL.Query().Get("documentType"))
	if documentType == "" {
		documentType = localName
	}
	expectedNamespace, ok := h.catalog.expectedNamespaceFor(documentType)
	if !ok || localName != documentType || (r.URL.Query().Get("documentType") == "" && namespace != expectedNamespace) {
		recordUploadOutcome("unsupported_document_type")
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("unsupported UBL 2.1 document type: root element %q in namespace %q is not a recognized main document matching requested document type %q", localName, namespace, documentType),
		})
		return
	}
	rootElement := documentType
	profile, err := requestedValidationProfile(r, rootElement)
	if err != nil {
		recordUploadOutcome("unsupported_profile")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	id, err := queue.NewID()
	if err != nil {
		recordUploadOutcome("internal_error")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate upload id"})
		return
	}
	span.SetAttributes(attribute.String("upload.id", id))
	receivedAt := time.Now().UTC()

	// Persist intent first so every later partial failure is observable and
	// can be reconciled. No content is written if this first boundary fails.
	record := queue.UploadRecord{
		ID: id, DocumentType: rootElement, Profile: profile, Status: queue.StatusAccepting,
		ReceivedAt: receivedAt.Format(time.RFC3339), EnqueueTrace: observability.CaptureTrace(r.Context()),
	}
	revision, err := h.queue.CreateUploadRecord(r.Context(), record)
	if err != nil {
		recordUploadOutcome("internal_error")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to persist upload intent: " + err.Error()})
		return
	}

	if err := h.queue.PutUploadContent(r.Context(), id, body); err != nil {
		deleteErr := h.queue.DeleteUploadContent(r.Context(), id)
		record.Status = queue.StatusAcceptanceFailed
		if deleteErr != nil {
			record.Status = queue.StatusCleanupPending
		}
		record.Error = "failed to persist upload content: " + err.Error()
		if deleteErr != nil {
			record.Error += "; cleanup pending: " + deleteErr.Error()
		}
		_, _ = h.queue.UpdateUploadRecord(r.Context(), record, revision)
		recordUploadOutcome("internal_error")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": record.Error})
		return
	}

	record.Status = queue.StatusEnqueuePending
	// Persisting enqueue_pending before publish makes the intent explicit.
	// If that write fails, the original accepting record plus durable content
	// is still sufficient for reconciliation.
	revision, err = h.queue.UpdateUploadRecord(r.Context(), record, revision)
	if err != nil {
		record.Status = queue.StatusAccepting
		record.Error = "awaiting acceptance reconciliation: " + err.Error()
		recordUploadOutcome("recovering")
		writeUploadAccepted(w, record)
		return
	}

	// The upload ID enables normal JetStream deduplication. A failure here
	// leaves enqueue_pending for CAS-safe retry, while the worker's terminal
	// record check suppresses duplicates even beyond the broker dedup window.
	if err := h.queue.PublishDocument(r.Context(), id, rootElement, receivedAt); err != nil {
		record.Error = "awaiting queue publication: " + err.Error()
		revision, _ = h.queue.UpdateUploadRecord(r.Context(), record, revision)
		recordUploadOutcome("recovering")
		writeUploadAccepted(w, record)
		return
	}
	record.Status = queue.StatusPending
	record.Error = ""
	if _, err := h.queue.UpdateUploadRecord(r.Context(), record, revision); err != nil {
		// The descriptor is already durable. The enqueue_pending record is
		// intentionally retained and reconciliation will deduplicate publish.
		latest, getErr := h.queue.GetUploadRecordVersion(r.Context(), id)
		if getErr == nil {
			record = latest.Record
		} else {
			record.Status = queue.StatusEnqueuePending
			record.Error = "queued; awaiting status reconciliation: " + err.Error()
		}
		recordUploadOutcome("recovering")
		writeUploadAccepted(w, record)
		return
	}

	recordUploadOutcome("queued")
	writeUploadAccepted(w, record)
}

func writeUploadAccepted(w http.ResponseWriter, record queue.UploadRecord) {
	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":     string(record.Status),
		"id":         record.ID,
		"document":   record.DocumentType,
		"profile":    record.Profile,
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
		"id":               record.ID,
		"document":         record.DocumentType,
		"profile":          record.Profile,
		"status":           string(record.Status),
		"receivedAt":       record.ReceivedAt,
		"contentAvailable": uploadContentAvailable(record),
	}
	if record.ValidatedAt != "" {
		response["validatedAt"] = record.ValidatedAt
	}
	if len(record.ValidationStages) > 0 {
		response["validationStages"] = record.ValidationStages
	}
	if record.Error != "" {
		response["error"] = record.Error
	}
	if record.LastError != "" {
		response["lastError"] = record.LastError
	}
	if record.ProcessingAttempts > 0 {
		response["processingAttempts"] = record.ProcessingAttempts
	}
	if record.ProcessingFailedAt != "" {
		response["processingFailedAt"] = record.ProcessingFailedAt
	}
	if record.ContentExpiresAt != "" {
		response["contentExpiresAt"] = record.ContentExpiresAt
	}
	if record.ContentExpiredAt != "" {
		response["contentExpiredAt"] = record.ContentExpiredAt
	}

	// Polling is deliberately metadata-only unless the caller explicitly
	// opts in. This keeps repeated status checks small even for 10 MiB XML.
	if includeContent {
		if record.ContentExpiredAt != "" {
			writeJSON(w, http.StatusGone, map[string]string{"error": "upload content has expired"})
			return
		}
		content, err := h.queue.GetUploadContent(r.Context(), id)
		if err != nil {
			if errors.Is(err, queue.ErrRecordNotFound) && contentExpiryDeadlinePassed(record, time.Now()) {
				writeJSON(w, http.StatusGone, map[string]string{"error": "upload content has expired"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load upload content: " + err.Error()})
			return
		}
		response["content"] = string(content)
	}

	writeJSON(w, http.StatusOK, response)
}

func requestedValidationProfile(r *http.Request, documentType string) (queue.ValidationProfile, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("profile"))
	if raw == "" {
		if requiresEN16931(documentType) {
			return queue.ValidationProfileUBL21EN16931, nil
		}
		return queue.ValidationProfileUBL21, nil
	}
	profile := queue.ValidationProfile(raw)
	if !queue.ValidValidationProfile(profile) {
		return "", fmt.Errorf("unsupported validation profile %q", raw)
	}
	if profile == queue.ValidationProfileUBL21EN16931 && !requiresEN16931(documentType) {
		return "", fmt.Errorf("validation profile %q is only available for Invoice and CreditNote", raw)
	}
	return profile, nil
}

func uploadContentAvailable(record queue.UploadRecord) bool {
	if record.ContentExpiredAt != "" {
		return false
	}
	switch record.Status {
	case queue.StatusEnqueuePending, queue.StatusPending, queue.StatusValid, queue.StatusInvalid, queue.StatusDeadLetterPending, queue.StatusProcessingFailed:
		return true
	default:
		return false
	}
}

func contentExpiryDeadlinePassed(record queue.UploadRecord, now time.Time) bool {
	if record.ContentExpiresAt == "" {
		return false
	}
	deadline, err := time.Parse(time.RFC3339, record.ContentExpiresAt)
	return err == nil && !now.Before(deadline)
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
