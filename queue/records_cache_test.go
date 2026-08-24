package queue

import (
	"context"
	"testing"

	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
)

func TestEnsureInfraCachesUploadHandles(t *testing.T) {
	server := queuetest.StartServer(t)
	client, err := ConnectWithOptions(server.URL, DefaultNamespace, server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.EnsureInfra(context.Background()); err != nil {
		t.Fatal(err)
	}
	if client.cachedUploadsKV == nil || client.cachedUploadContentStore == nil {
		t.Fatal("EnsureInfra did not cache both upload handles")
	}
	kv, err := client.uploadsBucket(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	store, err := client.uploadContentStore(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if kv != client.cachedUploadsKV || store != client.cachedUploadContentStore {
		t.Fatal("hot-path accessors did not return cached handles")
	}
}

func BenchmarkUploadHandleLookup(b *testing.B) {
	server := queuetest.StartServer(b)
	client, err := ConnectWithOptions(server.URL, DefaultNamespace, server.Options...)
	if err != nil {
		b.Fatal(err)
	}
	defer client.Close()
	ctx := context.Background()
	if err := client.EnsureInfra(ctx); err != nil {
		b.Fatal(err)
	}

	b.Run("cached", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := client.uploadsBucket(ctx); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("control_plane_lookup", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := client.JS.KeyValue(ctx, client.Namespace.UploadsBucketName()); err != nil {
				b.Fatal(err)
			}
		}
	})
}
