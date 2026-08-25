package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
)

type failingUploadQueue struct {
	record             queue.UploadRecord
	putRecordCalls     int
	failPutRecordCalls map[int]bool
	contentErr         error
	publishErr         error
	deleteContentErr   error
	putContentCalls    int
	deleteContentCalls int
	publishCalls       int
	revision           uint64
}

func (f *failingUploadQueue) CreateUploadRecord(_ context.Context, record queue.UploadRecord) (uint64, error) {
	f.putRecordCalls++
	if f.failPutRecordCalls[f.putRecordCalls] {
		return 0, errors.New("injected record failure")
	}
	f.record = record
	f.revision++
	return f.revision, nil
}
func (f *failingUploadQueue) UpdateUploadRecord(_ context.Context, record queue.UploadRecord, expected uint64) (uint64, error) {
	f.putRecordCalls++
	if f.failPutRecordCalls[f.putRecordCalls] {
		return 0, errors.New("injected record failure")
	}
	if expected != f.revision {
		return 0, errors.New("injected revision mismatch")
	}
	f.record = record
	f.revision++
	return f.revision, nil
}
func (f *failingUploadQueue) GetUploadRecord(context.Context, string) (queue.UploadRecord, error) {
	return f.record, nil
}
func (f *failingUploadQueue) GetUploadRecordVersion(context.Context, string) (queue.VersionedUploadRecord, error) {
	return queue.VersionedUploadRecord{Record: f.record, Revision: f.revision}, nil
}
func (f *failingUploadQueue) PutUploadContent(context.Context, string, []byte) error {
	f.putContentCalls++
	return f.contentErr
}
func (*failingUploadQueue) GetUploadContent(context.Context, string) ([]byte, error) {
	return nil, nil
}
func (f *failingUploadQueue) DeleteUploadContent(context.Context, string) error {
	f.deleteContentCalls++
	return f.deleteContentErr
}
func (f *failingUploadQueue) PublishDocument(context.Context, string, string, time.Time) error {
	f.publishCalls++
	return f.publishErr
}

func runInjectedUpload(t *testing.T, q *failingUploadQueue) *httptest.ResponseRecorder {
	t.Helper()
	h := &uploadHandlers{
		catalog: &documentCatalog{entryByElement: map[string]schemaEntry{"Invoice": {namespace: "urn:test"}}},
		queue:   q,
	}
	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(`<Invoice xmlns="urn:test"/>`))
	req.Header.Set("Content-Type", "application/xml")
	res := httptest.NewRecorder()
	h.handleUpload(res, req)
	return res
}

func TestUploadAcceptanceFailureBoundaries(t *testing.T) {
	t.Run("intent", func(t *testing.T) {
		q := &failingUploadQueue{failPutRecordCalls: map[int]bool{1: true}}
		res := runInjectedUpload(t, q)
		if res.Code != http.StatusInternalServerError || q.putContentCalls != 0 {
			t.Fatalf("response=%d content calls=%d", res.Code, q.putContentCalls)
		}
	})
	t.Run("content cleanup", func(t *testing.T) {
		q := &failingUploadQueue{contentErr: errors.New("injected content failure"), deleteContentErr: errors.New("injected cleanup failure")}
		res := runInjectedUpload(t, q)
		if res.Code != http.StatusInternalServerError || q.record.Status != queue.StatusCleanupPending {
			t.Fatalf("response=%d record=%+v", res.Code, q.record)
		}
	})
	t.Run("content", func(t *testing.T) {
		q := &failingUploadQueue{contentErr: errors.New("injected content failure")}
		res := runInjectedUpload(t, q)
		if res.Code != http.StatusInternalServerError || q.deleteContentCalls != 1 || q.record.Status != queue.StatusAcceptanceFailed {
			t.Fatalf("response=%d deletes=%d record=%+v", res.Code, q.deleteContentCalls, q.record)
		}
	})
	t.Run("enqueue marker", func(t *testing.T) {
		q := &failingUploadQueue{failPutRecordCalls: map[int]bool{2: true}}
		res := runInjectedUpload(t, q)
		if res.Code != http.StatusAccepted || q.record.Status != queue.StatusAccepting || q.publishCalls != 0 {
			t.Fatalf("response=%d publishes=%d record=%+v", res.Code, q.publishCalls, q.record)
		}
	})
	t.Run("publish", func(t *testing.T) {
		q := &failingUploadQueue{publishErr: errors.New("injected publish failure")}
		res := runInjectedUpload(t, q)
		if res.Code != http.StatusAccepted || q.record.Status != queue.StatusEnqueuePending || q.publishCalls != 1 {
			t.Fatalf("response=%d publishes=%d record=%+v", res.Code, q.publishCalls, q.record)
		}
	})
	t.Run("final status", func(t *testing.T) {
		q := &failingUploadQueue{failPutRecordCalls: map[int]bool{3: true}}
		res := runInjectedUpload(t, q)
		if res.Code != http.StatusAccepted || q.record.Status != queue.StatusEnqueuePending || q.publishCalls != 1 {
			t.Fatalf("response=%d publishes=%d record=%+v", res.Code, q.publishCalls, q.record)
		}
	})
}
