package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/documentstore"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
)

type testMetadataStore struct {
	records     map[string]queue.VersionedUploadRecord
	failID      string
	afterCreate func()
	getCalls    int
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
	if s.afterCreate != nil {
		s.afterCreate()
	}
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
	s.getCalls++
	record, ok := s.records[id]
	if !ok {
		return queue.VersionedUploadRecord{}, queue.ErrRecordNotFound
	}
	return record, nil
}
func (s *testMetadataStore) Keys(context.Context) ([]string, error) {
	keys := make([]string, 0, len(s.records))
	for id := range s.records {
		keys = append(keys, id)
	}
	return keys, nil
}

func TestRollbackUploadMetadataRestoresVerifiedLegacyKV(t *testing.T) {
	ctx := context.Background()
	nats := queuetest.StartServer(t)
	client, err := queue.ConnectWithOptions(nats.URL, queue.MustNamespace("metadata.rollback"), nats.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	if err := client.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	id := "0123456789abcdef0123456789abcdef"
	objects, err := documentstore.New(documentstore.Options{Backend: documentstore.BackendFilesystem, Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := objects.Put(ctx, documentstore.ObjectKey(id), strings.NewReader("rollback-content")); err != nil {
		t.Fatal(err)
	}
	legacy, err := client.JS.KeyValue(ctx, client.Namespace.UploadsBucketName())
	if err != nil {
		t.Fatal(err)
	}
	source := &testMetadataStore{records: map[string]queue.VersionedUploadRecord{}}
	if _, err := migrateLegacyUploadMetadata(ctx, legacy, []string{}, objects, source); err != nil {
		t.Fatal(err)
	}
	// Populate the authoritative source directly with the checksum created by
	// the importer path, then restore it into the legacy bucket.
	if err := client.PutUploadRecord(ctx, queue.UploadRecord{ID: id, DocumentType: "Invoice", Status: queue.StatusValid, ReceivedAt: time.Now().UTC().Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	if _, err := migrateLegacyUploadMetadata(ctx, legacy, []string{id}, objects, source); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Purge(ctx, id); err != nil {
		t.Fatal(err)
	}
	report, err := rollbackUploadMetadata(ctx, source, legacy, objects)
	if err != nil || report.Imported != 1 || report.Verified != 1 {
		t.Fatalf("rollback report=%+v err=%v", report, err)
	}
	entry, err := legacy.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(entry.Value()), "contentSha256") {
		t.Fatalf("restored record lacks checksum: %s", entry.Value())
	}
}

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
	if report.LegacyRevisions[ids[0]] == 0 || target.getCalls == 0 {
		t.Fatalf("migration did not retain legacy revision or reload YSQL row: %+v", report)
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

func TestMigrateLegacyUploadMetadataRejectsSourceRevisionChange(t *testing.T) {
	ctx := context.Background()
	nats := queuetest.StartServer(t)
	client, err := queue.ConnectWithOptions(nats.URL, queue.MustNamespace("metadata.revision"), nats.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	if err := client.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	id := "0123456789abcdef0123456789abcdef"
	record := queue.UploadRecord{ID: id, DocumentType: "Invoice", Status: queue.StatusValid, ReceivedAt: time.Now().UTC().Format(time.RFC3339)}
	if err := client.PutUploadRecord(ctx, record); err != nil {
		t.Fatal(err)
	}
	legacy, err := client.JS.KeyValue(ctx, client.Namespace.UploadsBucketName())
	if err != nil {
		t.Fatal(err)
	}
	objects, err := documentstore.New(documentstore.Options{Backend: documentstore.BackendFilesystem, Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := objects.Put(ctx, documentstore.ObjectKey(id), strings.NewReader("xml")); err != nil {
		t.Fatal(err)
	}
	target := &testMetadataStore{records: map[string]queue.VersionedUploadRecord{}}
	target.afterCreate = func() {
		encoded, marshalErr := json.Marshal(record)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if _, putErr := legacy.Put(ctx, id, encoded); putErr != nil {
			t.Fatal(putErr)
		}
	}
	_, err = migrateLegacyUploadMetadata(ctx, legacy, []string{id}, objects, target)
	if err == nil || !strings.Contains(err.Error(), "changed during migration") {
		t.Fatalf("migration error = %v, want revision-change failure", err)
	}
}
