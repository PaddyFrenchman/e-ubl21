package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/documentstore"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
)

type testMetadataStore struct {
	records map[string]queue.VersionedUploadRecord
	failID  string
}

func (s *testMetadataStore) Put(_ context.Context, record queue.UploadRecord) error {
	s.records[record.ID] = queue.VersionedUploadRecord{Record: record, Revision: 1}
	return nil
}
func (s *testMetadataStore) Create(_ context.Context, record queue.UploadRecord) (uint64, error) {
	if record.ID == s.failID {
		return 0, errors.New("simulated interruption")
	}
	if _, ok := s.records[record.ID]; ok {
		return 0, errors.New("already exists")
	}
	s.records[record.ID] = queue.VersionedUploadRecord{Record: record, Revision: 1}
	return 1, nil
}
func (s *testMetadataStore) Update(_ context.Context, record queue.UploadRecord, revision uint64) (uint64, error) {
	current, ok := s.records[record.ID]
	if !ok || current.Revision != revision {
		return 0, errors.New("conflict")
	}
	current.Record, current.Revision = record, revision+1
	s.records[record.ID] = current
	return current.Revision, nil
}
func (s *testMetadataStore) Get(_ context.Context, id string) (queue.VersionedUploadRecord, error) {
	record, ok := s.records[id]
	if !ok {
		return queue.VersionedUploadRecord{}, queue.ErrRecordNotFound
	}
	return record, nil
}
func (s *testMetadataStore) Keys(context.Context) ([]string, error) { return nil, nil }

func TestMigrateLegacyUploadMetadataRecoversInterruptedRerun(t *testing.T) {
	ctx := context.Background()
	nats := queuetest.StartServer(t)
	client, err := queue.ConnectWithOptions(nats.URL, queue.MustNamespace("metadata.migration"), nats.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	if err := client.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	ids := []string{"0123456789abcdef0123456789abcdef", "fedcba9876543210fedcba9876543210"}
	for _, id := range ids {
		if err := client.PutUploadRecord(ctx, queue.UploadRecord{ID: id, DocumentType: "Invoice", Status: queue.StatusValid, ReceivedAt: time.Now().UTC().Format(time.RFC3339), ValidatedAt: time.Now().UTC().Format(time.RFC3339)}); err != nil {
			t.Fatal(err)
		}
	}
	legacy, err := client.JS.KeyValue(ctx, client.Namespace.UploadsBucketName())
	if err != nil {
		t.Fatal(err)
	}
	store, err := documentstore.New(documentstore.Options{Backend: documentstore.BackendFilesystem, Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if err := store.Put(ctx, documentstore.ObjectKey(id), strings.NewReader("xml-"+id)); err != nil {
			t.Fatal(err)
		}
	}
	target := &testMetadataStore{records: map[string]queue.VersionedUploadRecord{}, failID: ids[1]}
	report, err := migrateLegacyUploadMetadata(ctx, legacy, ids, store, target)
	if err == nil || report.Imported != 1 || report.Verified != 1 {
		t.Fatalf("interrupted report=%+v err=%v", report, err)
	}
	target.failID = ""
	report, err = migrateLegacyUploadMetadata(ctx, legacy, ids, store, target)
	if err != nil || report.Skipped != 1 || report.Imported != 1 || report.Verified != 2 {
		t.Fatalf("rerun report=%+v err=%v", report, err)
	}
	for _, id := range ids {
		if got := target.records[id].Record.ContentSHA256; len(got) != 64 {
			t.Fatalf("checksum %s=%q", id, got)
		}
	}
	conflicting := target.records[ids[0]]
	conflicting.Record.Status = queue.StatusInvalid
	target.records[ids[0]] = conflicting
	if _, err := migrateLegacyUploadMetadata(ctx, legacy, ids, store, target); err == nil {
		t.Fatal("divergent existing YSQL record was accepted")
	}
}
