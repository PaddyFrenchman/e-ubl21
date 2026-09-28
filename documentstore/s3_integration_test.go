package documentstore

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"
)

// TestS3CompatibleRoundTrip qualifies guarded S3 writes against a live
// endpoint. Static credentials are required so an S3-compatible endpoint
// never falls through to the AWS instance-metadata credential provider.
func TestS3CompatibleRoundTrip(t *testing.T) {
	endpoint, bucket := os.Getenv("EUBL21_S3_TEST_ENDPOINT"), os.Getenv("EUBL21_S3_TEST_BUCKET")
	accessKey, secretKey := os.Getenv("EUBL21_S3_TEST_ACCESS_KEY"), os.Getenv("EUBL21_S3_TEST_SECRET_KEY")
	if endpoint == "" || bucket == "" || accessKey == "" || secretKey == "" {
		t.Skip("set EUBL21_S3_TEST_ENDPOINT, EUBL21_S3_TEST_BUCKET, EUBL21_S3_TEST_ACCESS_KEY, and EUBL21_S3_TEST_SECRET_KEY to run S3 integration")
	}
	store, err := New(Options{Backend: BackendS3, Endpoint: endpoint, Bucket: bucket, AccessKey: accessKey, SecretKey: secretKey, UseTLS: os.Getenv("EUBL21_S3_TEST_USE_TLS") == "true", Immutability: ImmutabilityGuardedWrite})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	key := ObjectKey("qualification-" + time.Now().UTC().Format("20060102T150405.000000000"))
	if err := store.Put(ctx, key, bytes.NewReader([]byte("s3-qualification"))); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Delete(context.Background(), key) })
	r, err := store.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	body, err := io.ReadAll(r)
	if err != nil || string(body) != "s3-qualification" {
		t.Fatalf("body = %q, err = %v", body, err)
	}
	if err := store.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	exists, err := store.Exists(ctx, key)
	if err != nil || exists {
		t.Fatalf("exists after delete = %t, %v", exists, err)
	}
}
