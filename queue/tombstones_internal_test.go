package queue

import (
	"context"
	"testing"

	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
)

func TestSequenceBoundedTombstonePurgePreservesRacingLiveMetadata(t *testing.T) {
	server := queuetest.StartServer(t)
	client, err := ConnectWithOptions(server.URL, DefaultNamespace, server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	ctx := context.Background()
	if err := client.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	id, _ := NewID()
	if err := client.PutUploadContent(ctx, id, []byte("old")); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteUploadContent(ctx, id); err != nil {
		t.Fatal(err)
	}
	stream, err := client.JS.Stream(ctx, uploadContentObjectStreamName(client.Namespace.UploadContentBucketName()))
	if err != nil {
		t.Fatal(err)
	}
	metadataSubject := uploadContentMetadataSubject(client.Namespace.UploadContentBucketName(), id)
	tombstone, err := stream.GetLastMsgForSubject(ctx, metadataSubject)
	if err != nil {
		t.Fatal(err)
	}

	// Recreate the object after the maintenance read but before its purge.
	if err := client.PutUploadContent(ctx, id, []byte("new-live-content")); err != nil {
		t.Fatal(err)
	}
	liveMetadata, err := stream.GetLastMsgForSubject(ctx, metadataSubject)
	if err != nil {
		t.Fatal(err)
	}
	if liveMetadata.Sequence <= tombstone.Sequence {
		t.Fatalf("live metadata sequence %d is not after tombstone %d", liveMetadata.Sequence, tombstone.Sequence)
	}

	if err := purgeUploadContentMetadataThrough(ctx, stream, metadataSubject, tombstone.Sequence); err != nil {
		t.Fatal(err)
	}
	content, err := client.GetUploadContent(ctx, id)
	if err != nil || string(content) != "new-live-content" {
		t.Fatalf("sequence-bounded purge removed racing live object: content=%q err=%v", content, err)
	}
	remaining, err := stream.GetLastMsgForSubject(ctx, metadataSubject)
	if err != nil {
		t.Fatal(err)
	}
	if remaining.Sequence != liveMetadata.Sequence {
		t.Fatalf("remaining metadata sequence=%d, want live sequence %d", remaining.Sequence, liveMetadata.Sequence)
	}
}
