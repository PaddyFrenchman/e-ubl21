package queue

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/database"
	"github.com/PaddyFrenchman/e-ubl21/documentstore"
)

const recoveryQualificationUploadID = "0123456789abcdef0123456789abcdef"

func TestYSQLUploadMetadataStore(t *testing.T) {
	rawURL := os.Getenv("EUBL21_TEST_DATABASE_URL")
	if rawURL == "" {
		t.Skip("set EUBL21_TEST_DATABASE_URL to run YSQL upload metadata integration")
	}
	cfg, err := database.ParseConfig(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	client, err := database.Connect(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	namespace, err := NewNamespace("integration.uploadmetadata")
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewYSQLUploadMetadataStore(client, namespace)
	if err != nil {
		t.Fatal(err)
	}
	id := "89" + time.Now().UTC().Format("20060102150405") + "0000000000000000"
	id = id[:32]
	record := UploadRecord{ID: id, DocumentType: "Invoice", Status: StatusPending, ReceivedAt: time.Now().UTC().Format(time.RFC3339), ContentSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	revision, err := store.Create(context.Background(), record)
	if err != nil || revision != 1 {
		t.Fatalf("create revision=%d err=%v", revision, err)
	}
	if _, err := store.Create(context.Background(), record); err == nil {
		t.Fatal("duplicate create succeeded")
	}
	loaded, err := store.Get(context.Background(), id)
	if err != nil || loaded.Record.ContentSHA256 != record.ContentSHA256 {
		t.Fatalf("get=%+v err=%v", loaded, err)
	}
	var wg sync.WaitGroup
	successes := make(chan bool, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			candidate := loaded.Record
			candidate.Status = StatusValid
			candidate.ValidatedAt = time.Now().UTC().Format(time.RFC3339)
			_, updateErr := store.Update(context.Background(), candidate, loaded.Revision)
			successes <- updateErr == nil
		}()
	}
	wg.Wait()
	close(successes)
	count := 0
	for ok := range successes {
		if ok {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("CAS successes=%d, want 1", count)
	}
	latest, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	latest.Record.ContentExpiresAt = time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	latest.Record.ContentExpiredAt = time.Now().UTC().Format(time.RFC3339)
	if _, err := store.Update(context.Background(), latest.Record, latest.Revision); err != nil {
		t.Fatal(err)
	}
	keys, err := store.Keys(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, key := range keys {
		found = found || key == id
	}
	if !found {
		t.Fatalf("keys missing %s", id)
	}
	if _, err := store.Get(context.Background(), "not-a-valid-id"); !errors.Is(err, ErrRecordNotFound) {
		t.Fatalf("invalid get = %v", err)
	}
}

// TestYSQLUploadMetadataRecoveryQualification is enabled only by the YBC DR
// drill. It proves that a restored upload_metadata row remains usable by the
// application and still agrees with its separately restored object-store
// content.
func TestYSQLUploadMetadataRecoveryQualification(t *testing.T) {
	rawURL, objectDirectory := os.Getenv("EUBL21_TEST_DATABASE_URL"), os.Getenv("EUBL21_RECOVERY_OBJECT_DIR")
	if rawURL == "" || objectDirectory == "" {
		t.Skip("set EUBL21_TEST_DATABASE_URL and EUBL21_RECOVERY_OBJECT_DIR for recovery qualification")
	}
	cfg, err := database.ParseConfig(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	client, err := database.Connect(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	namespace := MustNamespace("recovery.qualification")
	metadata, err := NewYSQLUploadMetadataStore(client, namespace)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := metadata.Get(context.Background(), recoveryQualificationUploadID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Revision != 1 || loaded.Record.ID != recoveryQualificationUploadID || loaded.Record.DocumentType != "Invoice" || loaded.Record.Status != StatusEnqueuePending || loaded.Record.ReceivedAt != "2026-09-28T00:00:00Z" {
		t.Fatalf("restored upload metadata = %+v, revision=%d", loaded.Record, loaded.Revision)
	}
	objects, err := documentstore.New(documentstore.Options{Backend: documentstore.BackendFilesystem, Directory: objectDirectory})
	if err != nil {
		t.Fatal(err)
	}
	body, err := objects.Get(context.Background(), documentstore.ObjectKey(loaded.Record.ID))
	if err != nil {
		t.Fatal(err)
	}
	content, readErr := io.ReadAll(body)
	closeErr := body.Close()
	if readErr != nil || closeErr != nil {
		t.Fatalf("read restored object: %v %v", readErr, closeErr)
	}
	sum := sha256.Sum256(content)
	if loaded.Record.ContentSHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("restored object checksum does not match metadata: got %s", loaded.Record.ContentSHA256)
	}
	loaded.Record.Status = StatusPending
	if revision, err := metadata.Update(context.Background(), loaded.Record, loaded.Revision); err != nil || revision != 2 {
		t.Fatalf("resume restored upload metadata revision=%d err=%v", revision, err)
	}
}
