package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/PaddyFrenchman/e-ubl21/documentstore"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
)

func TestMigrateLegacyObjectsVerifiesChecksumAndRerun(t *testing.T) {
	ctx := context.Background()
	nats := queuetest.StartServer(t)
	client, err := queue.ConnectWithOptions(nats.URL, queue.MustNamespace("objectstore.migration"), nats.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	if err := client.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	legacy, err := client.JS.ObjectStore(ctx, client.Namespace.UploadContentBucketName())
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{"0123456789abcdef0123456789abcdef", "fedcba9876543210fedcba9876543210"}
	for _, id := range ids {
		if _, err := legacy.PutBytes(ctx, id, []byte("legacy-"+id)); err != nil {
			t.Fatal(err)
		}
	}
	objects, err := legacy.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	target, err := documentstore.New(documentstore.Options{Backend: documentstore.BackendFilesystem, Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	report, err := migrateLegacyObjects(ctx, target, nil, legacy, objects)
	if err != nil || report.Copied != len(ids) || report.Verified != len(ids) {
		t.Fatalf("first migration report=%+v err=%v", report, err)
	}
	report, err = migrateLegacyObjects(ctx, target, nil, legacy, objects)
	if err != nil || report.Skipped != len(ids) || report.Verified != len(ids) {
		t.Fatalf("rerun report=%+v err=%v", report, err)
	}
	for _, id := range ids {
		body, err := target.Get(ctx, documentstore.ObjectKey(id))
		if err != nil {
			t.Fatal(err)
		}
		got := new(bytes.Buffer)
		_, _ = got.ReadFrom(body)
		_ = body.Close()
		if got.String() != "legacy-"+id {
			t.Fatalf("target %s=%q", id, got.String())
		}
	}
	wrongID := "00112233445566778899aabbccddeeff"
	if _, err := legacy.PutBytes(ctx, wrongID, []byte("expected")); err != nil {
		t.Fatal(err)
	}
	if err := target.Put(ctx, documentstore.ObjectKey(wrongID), strings.NewReader("wrong")); err != nil {
		t.Fatal(err)
	}
	objects, err = legacy.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := migrateLegacyObjects(ctx, target, nil, legacy, objects); err == nil || !strings.Contains(err.Error(), "integrity failure") {
		t.Fatalf("mismatch error=%v", err)
	}
}
