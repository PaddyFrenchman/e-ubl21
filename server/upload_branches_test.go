package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
)

func directUploadHandler(t *testing.T) (*uploadHandlers, *queue.Client) {
	t.Helper()
	s := queuetest.StartServer(t)
	q, err := queue.ConnectWithOptions(s.URL, queue.DefaultNamespace, s.Options...)
	if err != nil {
		t.Fatal(err)
	}
	if err := q.EnsureInfra(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(q.Close)
	return &uploadHandlers{
		catalog: &documentCatalog{entryByElement: map[string]schemaEntry{
			"Invoice": {file: "unused", namespace: "urn:test"},
		}},
		queue: q,
	}, q
}

func TestUploadHandlerBadRequestBranches(t *testing.T) {
	h, _ := directUploadHandler(t)
	tests := []struct {
		body, contentType string
	}{
		{"", "application/xml"},
		{"not xml", "application/xml"},
		{"<Invoice xmlns=", "application/xml"},
		{"<Invoice xmlns=\"urn:test\"/>", "text/plain"},
	}
	for _, tc := range tests {
		req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", tc.contentType)
		res := httptest.NewRecorder()
		h.handleUpload(res, req)
		if res.Code != http.StatusBadRequest {
			t.Errorf("body %q: status = %d, body=%s", tc.body, res.Code, res.Body)
		}
	}
}

func TestUploadHandlerStorageFailuresAndContentResponse(t *testing.T) {
	h, q := directUploadHandler(t)
	q.Close()
	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(`<Invoice xmlns="urn:test"/>`))
	req.Header.Set("Content-Type", "application/xml")
	res := httptest.NewRecorder()
	h.handleUpload(res, req)
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("closed queue upload status = %d", res.Code)
	}

	h, q = directUploadHandler(t)
	id, _ := queue.NewID()
	validatedAt := time.Now().UTC().Format(time.RFC3339)
	record := queue.UploadRecord{
		ID: id, DocumentType: "Invoice", Status: queue.StatusInvalid, Error: "bad",
		ReceivedAt: validatedAt, ValidatedAt: validatedAt,
		ValidationStages: queue.ValidationStages{
			{Layer: queue.ValidationLayerUBL21XSD, Version: "2.1", Valid: true, ValidatedAt: validatedAt},
			{
				Layer: queue.ValidationLayerEN16931, Version: "1.3.16", Valid: false, ValidatedAt: validatedAt,
				Issues: []queue.ValidationIssue{{
					Kind: queue.ValidationIssueFailedAssert, Layer: queue.ValidationLayerEN16931,
					Rule: "BR-CO-13", Severity: "fatal", Location: "/ubl:Invoice/cac:LegalMonetaryTotal",
					Message: "Invoice totals do not reconcile.",
				}},
			},
		},
	}
	if err := q.PutUploadRecord(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if err := q.PutUploadContent(context.Background(), id, []byte("<Invoice/>")); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/upload?includeContent=true", nil)
	req.SetPathValue("id", id)
	res = httptest.NewRecorder()
	h.handleGetUpload(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"content":"\u003cInvoice/\u003e"`) || !strings.Contains(res.Body.String(), `"validatedAt"`) {
		t.Fatalf("content response = %d %s", res.Code, res.Body)
	}
	var response struct {
		ValidationStages queue.ValidationStages `json:"validationStages"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	semantic, ok := response.ValidationStages.Find(queue.ValidationLayerEN16931)
	if !ok || semantic.Version != "1.3.16" || semantic.Valid || len(semantic.Issues) != 1 ||
		semantic.Issues[0].Rule != "BR-CO-13" || semantic.Issues[0].Severity != "fatal" {
		t.Fatalf("structured EN 16931 response = %+v", semantic)
	}

	expiredID, _ := queue.NewID()
	expiredAt := time.Now().UTC().Format(time.RFC3339)
	if err := q.PutUploadRecord(context.Background(), queue.UploadRecord{
		ID: expiredID, DocumentType: "Invoice", Status: queue.StatusValid,
		ReceivedAt: expiredAt, ValidatedAt: expiredAt,
		ContentExpiresAt: expiredAt, ContentExpiredAt: expiredAt,
	}); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/upload", nil)
	req.SetPathValue("id", expiredID)
	res = httptest.NewRecorder()
	h.handleGetUpload(res, req)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"contentAvailable":false`) || !strings.Contains(res.Body.String(), `"contentExpiredAt"`) {
		t.Fatalf("expired metadata response = %d %s", res.Code, res.Body)
	}
	req = httptest.NewRequest(http.MethodGet, "/upload?includeContent=true", nil)
	req.SetPathValue("id", expiredID)
	res = httptest.NewRecorder()
	h.handleGetUpload(res, req)
	if res.Code != http.StatusGone || !strings.Contains(res.Body.String(), "upload content has expired") {
		t.Fatalf("expired content response = %d %s", res.Code, res.Body)
	}

	deadlinePassedID, _ := queue.NewID()
	if err := q.PutUploadRecord(context.Background(), queue.UploadRecord{
		ID: deadlinePassedID, DocumentType: "Invoice", Status: queue.StatusValid,
		ReceivedAt: expiredAt, ValidatedAt: expiredAt,
		ContentExpiresAt: time.Now().Add(-time.Minute).UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/upload?includeContent=true", nil)
	req.SetPathValue("id", deadlinePassedID)
	res = httptest.NewRecorder()
	h.handleGetUpload(res, req)
	if res.Code != http.StatusGone {
		t.Fatalf("due content missing before sweep response = %d %s", res.Code, res.Body)
	}

	missingID, _ := queue.NewID()
	if err := q.PutUploadRecord(context.Background(), queue.UploadRecord{ID: missingID}); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/upload?includeContent=true", nil)
	req.SetPathValue("id", missingID)
	res = httptest.NewRecorder()
	h.handleGetUpload(res, req)
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("missing content status = %d", res.Code)
	}
}

func TestUploadHandlerRecordPublishAndReadFailures(t *testing.T) {
	makeRequest := func(h *uploadHandlers) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(`<Invoice xmlns="urn:test"/>`))
		req.Header.Set("Content-Type", "application/xml")
		res := httptest.NewRecorder()
		h.handleUpload(res, req)
		return res
	}

	h, q := directUploadHandler(t)
	if err := q.JS.DeleteKeyValue(context.Background(), q.Namespace.UploadsBucketName()); err != nil {
		t.Fatal(err)
	}
	if res := makeRequest(h); res.Code != http.StatusInternalServerError || !strings.Contains(res.Body.String(), "persist upload intent") {
		t.Fatalf("record failure = %d %s", res.Code, res.Body)
	}

	h, q = directUploadHandler(t)
	if err := q.JS.DeleteStream(context.Background(), q.Namespace.DocumentsStreamName()); err != nil {
		t.Fatal(err)
	}
	if res := makeRequest(h); res.Code != http.StatusAccepted || !strings.Contains(res.Body.String(), string(queue.StatusEnqueuePending)) {
		t.Fatalf("publish failure = %d %s", res.Code, res.Body)
	}

	h, q = directUploadHandler(t)
	q.Close()
	req := httptest.NewRequest(http.MethodGet, "/upload", nil)
	req.SetPathValue("id", "00000000000000000000000000000000")
	res := httptest.NewRecorder()
	h.handleGetUpload(res, req)
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("read failure = %d %s", res.Code, res.Body)
	}
}
