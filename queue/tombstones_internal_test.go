package queue

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
)

func TestTombstoneHelpersRejectMaximumSequenceAndClassifyStatuses(t *testing.T) {
	if err := purgeUploadContentMetadataThrough(context.Background(), nil, "subject", math.MaxUint64); err == nil {
		t.Fatal("maximum tombstone sequence accepted")
	}
	for _, test := range []struct {
		record UploadRecord
		want   bool
	}{
		{UploadRecord{Status: StatusValid, ContentExpiredAt: "now"}, true},
		{UploadRecord{Status: StatusInvalid}, false},
		{UploadRecord{Status: StatusAcceptanceFailed}, true},
		{UploadRecord{Status: StatusPending, ContentExpiredAt: "now"}, false},
	} {
		if got := uploadContentTombstonePurgeable(test.record); got != test.want {
			t.Errorf("purgeable(%+v) = %v, want %v", test.record, got, test.want)
		}
	}
}

func TestTombstonePurgeSkipsUnsafeInventoryAndCountsCorruption(t *testing.T) {
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
	makeObject := func(deleted bool) string {
		t.Helper()
		id, _ := NewID()
		if err := client.PutUploadContent(ctx, id, []byte("content")); err != nil {
			t.Fatal(err)
		}
		if deleted {
			if err := client.DeleteUploadContent(ctx, id); err != nil {
				t.Fatal(err)
			}
		}
		return id
	}
	_ = makeObject(false)
	missingRecordID := makeObject(true)
	corruptID := makeObject(true)
	nonterminalID := makeObject(true)
	kv, err := client.uploadsBucket(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kv.Put(ctx, corruptID, []byte("{")); err != nil {
		t.Fatal(err)
	}
	record := UploadRecord{ID: nonterminalID, DocumentType: "Invoice", Status: StatusPending, ReceivedAt: time.Now().UTC().Format(time.RFC3339)}
	body, _ := json.Marshal(record)
	if _, err := kv.Put(ctx, nonterminalID, body); err != nil {
		t.Fatal(err)
	}

	report, err := client.PurgeUploadContentTombstones(ctx, 0, 20, false, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if report.Scanned != 3 || report.Eligible != 0 || report.Failed != 1 {
		t.Fatalf("report = %+v (missing record %s)", report, missingRecordID)
	}

	young, err := client.PurgeUploadContentTombstones(ctx, time.Hour, 20, false, time.Now())
	if err != nil || young.Eligible != 0 {
		t.Fatalf("young report = %+v, %v", young, err)
	}
}

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
