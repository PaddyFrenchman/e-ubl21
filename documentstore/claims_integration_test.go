package documentstore

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/database"
)

// TestGuardedWriteClaims exercises the linearization and recovery boundaries
// against YugabyteDB. Run with EUBL21_TEST_DATABASE_URL after `database
// migrate`; the local YugabyteDB harness supplies that environment.
func TestGuardedWriteClaims(t *testing.T) {
	rawURL := os.Getenv("EUBL21_TEST_DATABASE_URL")
	if rawURL == "" {
		t.Skip("set EUBL21_TEST_DATABASE_URL to run guarded-write integration")
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
	claims, err := NewClaimStore(client)
	if err != nil {
		t.Fatal(err)
	}
	store, err := New(Options{Backend: BackendFilesystem, Directory: t.TempDir(), Immutability: ImmutabilityGuardedWrite})
	if err != nil {
		t.Fatal(err)
	}
	key := ObjectKey(fmt.Sprintf("%032x", time.Now().UnixNano()))
	digest := checksum([]byte("first"))

	t.Run("same key and content converges", func(t *testing.T) {
		_, write, err := claims.Claim(context.Background(), key, digest)
		if err != nil || !write {
			t.Fatalf("first claim write=%t err=%v", write, err)
		}
		if err := store.Put(context.Background(), key, strings.NewReader("first")); err != nil {
			t.Fatal(err)
		}
		if err := claims.Complete(context.Background(), key, digest); err != nil {
			t.Fatal(err)
		}
		claim, write, err := claims.Claim(context.Background(), key, digest)
		if err != nil || write || claim.State != Complete {
			t.Fatalf("retry=%+v write=%t err=%v", claim, write, err)
		}
	})

	t.Run("completed claim rechecks storage", func(t *testing.T) {
		k, body := ObjectKey(fmt.Sprintf("%032x", time.Now().UnixNano()+99)), []byte("durable")
		if err := claims.Put(context.Background(), store, k, body); err != nil {
			t.Fatal(err)
		}
		if err := store.Delete(context.Background(), k); err != nil {
			t.Fatal(err)
		}
		if err := claims.Put(context.Background(), store, k, body); err == nil {
			t.Fatal("completed claim accepted missing object")
		}
	})

	t.Run("different content is immutable conflict", func(t *testing.T) {
		_, _, err := claims.Claim(context.Background(), key, checksum([]byte("different")))
		if err == nil {
			t.Fatal("different-content claim succeeded")
		}
	})

	for index, scenario := range []struct {
		name  string
		body  []byte
		setup func(string, string) error
		want  error
	}{
		{"claim before write releases", []byte("missing"), func(k, d string) error { _, _, e := claims.Claim(context.Background(), k, d); return e }, nil},
		{"write before completion completes", []byte("written"), func(k, d string) error {
			_, _, e := claims.Claim(context.Background(), k, d)
			if e != nil {
				return e
			}
			return store.Put(context.Background(), k, strings.NewReader("written"))
		}, nil},
		{"checksum mismatch hard fails", []byte("expected"), func(k, d string) error {
			_, _, e := claims.Claim(context.Background(), k, d)
			if e != nil {
				return e
			}
			return store.Put(context.Background(), k, strings.NewReader("actual"))
		}, ErrIntegrity},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			k, d := ObjectKey(fmt.Sprintf("%032x", time.Now().UnixNano()+int64(index)+1)), checksum(scenario.body)
			if err := scenario.setup(k, d); err != nil {
				t.Fatal(err)
			}
			claim, _, err := claims.Claim(context.Background(), k, d)
			if err != nil {
				t.Fatal(err)
			}
			report, err := claims.recoverClaims(context.Background(), store, []WriteClaim{claim})
			if !errors.Is(err, scenario.want) {
				t.Fatalf("recover report=%+v err=%v want=%v", report, err, scenario.want)
			}
			if scenario.want == nil && report.Deferred+report.Completed == 0 {
				t.Fatalf("report=%+v", report)
			}
		})
	}

	t.Run("concurrent writers have one owner", func(t *testing.T) {
		k, d := key+"concurrent", checksum([]byte("concurrent"))
		var wg sync.WaitGroup
		writes := make(chan bool, 8)
		errs := make(chan error, 8)
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, write, err := claims.Claim(context.Background(), k, d)
				writes <- write
				errs <- err
			}()
		}
		wg.Wait()
		close(writes)
		close(errs)
		owners := 0
		for write := range writes {
			if write {
				owners++
			}
		}
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		if owners != 1 {
			t.Fatalf("owners=%d, want 1", owners)
		}
	})

	t.Run("explicit absent release requires the exact abandoned claim", func(t *testing.T) {
		k, d := ObjectKey(fmt.Sprintf("%032x", time.Now().UnixNano()+100)), checksum([]byte("abandoned"))
		if _, write, err := claims.Claim(context.Background(), k, d); err != nil || !write {
			t.Fatalf("claim write=%t err=%v", write, err)
		}
		time.Sleep(time.Millisecond)
		if err := claims.ReleaseAbsent(context.Background(), store, k, d, time.Nanosecond); err != nil {
			t.Fatal(err)
		}
		if _, write, err := claims.Claim(context.Background(), k, d); err != nil || !write {
			t.Fatalf("reclaimed write=%t err=%v", write, err)
		}
		if err := store.Put(context.Background(), k, strings.NewReader("abandoned")); err != nil {
			t.Fatal(err)
		}
		if err := claims.ReleaseAbsent(context.Background(), store, k, d, time.Nanosecond); err == nil {
			t.Fatal("released claim despite existing object")
		}
	})
}

func checksum(body []byte) string { return fmt.Sprintf("%x", sha256.Sum256(body)) }
