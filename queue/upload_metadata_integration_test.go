package queue

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/database"
)

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
