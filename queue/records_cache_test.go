package queue

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
	"github.com/nats-io/nats.go/jetstream"
)

func TestValidationExecutionInfraSeparatesProvisioningFromRuntimeVerification(t *testing.T) {
	server := queuetest.StartServer(t)
	provisioner, err := ConnectWithOptions(server.URL, DefaultNamespace, server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	defer provisioner.Close()
	ctx := context.Background()
	if err := provisioner.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := provisioner.JS.KeyValue(ctx, DefaultNamespace.ValidationExecutionsBucketName()); !errors.Is(err, jetstream.ErrBucketNotFound) {
		t.Fatalf("runtime EnsureInfra execution bucket lookup = %v, want %v", err, jetstream.ErrBucketNotFound)
	}
	if err := provisioner.ProvisionValidationExecutionInfra(ctx); err != nil {
		t.Fatal(err)
	}

	runtimeClient, err := ConnectWithOptions(server.URL, DefaultNamespace, server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	defer runtimeClient.Close()
	if err := runtimeClient.VerifyValidationExecutionInfra(ctx); err != nil {
		t.Fatal(err)
	}
	if runtimeClient.cachedValidationExecutionsKV == nil {
		t.Fatal("runtime verification did not cache the execution bucket")
	}
}

func TestValidationExecutionInfraVerificationRejectsIncompatibleBucket(t *testing.T) {
	server := queuetest.StartServer(t)
	client, err := ConnectWithOptions(server.URL, MustNamespace("incompatible"), server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx := context.Background()
	if _, err := client.JS.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: client.Namespace.ValidationExecutionsBucketName(), Storage: jetstream.MemoryStorage, MaxValueSize: 1024,
	}); err != nil {
		t.Fatal(err)
	}
	if err := client.VerifyValidationExecutionInfra(ctx); err == nil {
		t.Fatal("runtime verification accepted incompatible execution infrastructure")
	}
}

type countingJetStream struct {
	jetstream.JetStream
	keyValueCalls    atomic.Int32
	objectStoreCalls atomic.Int32
}

func (c *countingJetStream) KeyValue(ctx context.Context, bucket string) (jetstream.KeyValue, error) {
	c.keyValueCalls.Add(1)
	return c.JetStream.KeyValue(ctx, bucket)
}

func (c *countingJetStream) ObjectStore(ctx context.Context, bucket string) (jetstream.ObjectStore, error) {
	c.objectStoreCalls.Add(1)
	return c.JetStream.ObjectStore(ctx, bucket)
}

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
	if client.cachedValidationExecutionsKV != nil {
		t.Fatal("runtime EnsureInfra provisioned or opened deployer-owned validation execution infrastructure")
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

func TestColdHandleDiscoveryIsCoalesced(t *testing.T) {
	server := queuetest.StartServer(t)
	creator, err := ConnectWithOptions(server.URL, DefaultNamespace, server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	defer creator.Close()
	if err := creator.EnsureInfra(context.Background()); err != nil {
		t.Fatal(err)
	}

	client, err := ConnectWithOptions(server.URL, DefaultNamespace, server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	counting := &countingJetStream{JetStream: client.JS}
	client.JS = counting

	const callers = 32
	start := make(chan struct{})
	var callersDone sync.WaitGroup
	for i := 0; i < callers; i++ {
		callersDone.Add(2)
		go func() {
			defer callersDone.Done()
			<-start
			if _, err := client.uploadsBucket(context.Background()); err != nil {
				t.Errorf("uploadsBucket: %v", err)
			}
		}()
		go func() {
			defer callersDone.Done()
			<-start
			if _, err := client.uploadContentStore(context.Background()); err != nil {
				t.Errorf("uploadContentStore: %v", err)
			}
		}()
	}
	close(start)
	callersDone.Wait()
	if got := counting.keyValueCalls.Load(); got != 1 {
		t.Fatalf("KeyValue discovery calls = %d, want 1", got)
	}
	if got := counting.objectStoreCalls.Load(); got != 1 {
		t.Fatalf("ObjectStore discovery calls = %d, want 1", got)
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

	b.Run("key_value", func(b *testing.B) {
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
	})
	b.Run("object_store", func(b *testing.B) {
		b.Run("cached", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := client.uploadContentStore(ctx); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("control_plane_lookup", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := client.JS.ObjectStore(ctx, client.Namespace.UploadContentBucketName()); err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}
