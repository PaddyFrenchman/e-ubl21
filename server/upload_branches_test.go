package server

import (
	"context"
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
		catalog: &schemaCatalog{entryByElement: map[string]schemaEntry{
			"Invoice": {path: "unused", namespace: "urn:test"},
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
	record := queue.UploadRecord{ID: id, DocumentType: "Invoice", Status: queue.StatusInvalid, Error: "bad", ReceivedAt: time.Now().Format(time.RFC3339), ValidatedAt: time.Now().Format(time.RFC3339)}
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
	if res := makeRequest(h); res.Code != http.StatusInternalServerError || !strings.Contains(res.Body.String(), "persist upload record") {
		t.Fatalf("record failure = %d %s", res.Code, res.Body)
	}

	h, q = directUploadHandler(t)
	if err := q.JS.DeleteStream(context.Background(), q.Namespace.DocumentsStreamName()); err != nil {
		t.Fatal(err)
	}
	if res := makeRequest(h); res.Code != http.StatusInternalServerError || !strings.Contains(res.Body.String(), "queue upload") {
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
