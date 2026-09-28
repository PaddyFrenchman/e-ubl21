package documentstore

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestFilesystemStoreUsesBackendNeutralImmutableKeys(t *testing.T) {
	store, err := New(Options{Backend: BackendFilesystem, Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	key := ObjectKey("0123456789abcdef0123456789abcdef")
	if key != "uploads/0123456789abcdef0123456789abcdef/document.xml" {
		t.Fatalf("key = %q", key)
	}
	if err := store.Put(context.Background(), key, strings.NewReader("first")); err != nil {
		t.Fatal(err)
	}
	if size, err := store.Size(context.Background()); err != nil || size != uint64(len("first")) {
		t.Fatalf("size = %d, %v", size, err)
	}
	if err := store.Put(context.Background(), key, strings.NewReader("second")); err == nil {
		t.Fatal("immutable object was overwritten")
	}
	r, err := store.Get(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	body, err := io.ReadAll(r)
	if err != nil || string(body) != "first" {
		t.Fatalf("body = %q, %v", body, err)
	}
	if err := store.Delete(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	ok, err := store.Exists(context.Background(), key)
	if err != nil || ok {
		t.Fatalf("exists = %t, %v", ok, err)
	}
}

func TestNewRejectsIncompleteConfiguration(t *testing.T) {
	for _, cfg := range []Options{
		{},
		{Backend: BackendFilesystem},
		{Backend: BackendS3},
		{Backend: BackendS3, Bucket: "documents"},
		{Backend: BackendS3, Bucket: "documents", Endpoint: "s3.example.test", AccessKey: "access"},
	} {
		if _, err := New(cfg); err == nil {
			t.Fatalf("New(%+v) succeeded", cfg)
		}
	}
	if _, err := New(Options{Backend: "other"}); err == nil {
		t.Fatal("unsupported backend succeeded")
	}
}
