package queue_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
)

func TestDocumentWorkItemAndClientValidationErrors(t *testing.T) {
	if _, err := queue.Connect("://bad-url", queue.DefaultNamespace); err == nil {
		t.Error("malformed NATS URL accepted")
	}
	id, err := queue.NewID()
	if err != nil || !queue.ValidID(id) {
		t.Fatalf("NewID = %q, %v", id, err)
	}
	inputs := [][]byte{
		[]byte(`{`),
		[]byte(`{"id":"bad","documentType":"Invoice","receivedAt":"now"}`),
		[]byte(`{"id":"00000000000000000000000000000000","receivedAt":"now"}`),
		[]byte(`{"id":"00000000000000000000000000000000","documentType":"Invoice"}`),
	}
	for _, input := range inputs {
		if _, err := queue.DecodeDocumentWorkItem(input); err == nil {
			t.Errorf("DecodeDocumentWorkItem(%s) unexpectedly succeeded", input)
		}
	}

	client := newTestClient(t)
	ctx := context.Background()
	if err := client.EnsureInfraWithConfig(ctx, queue.InfraConfig{UploadContentMaxBytes: -1}); err == nil {
		t.Error("negative upload content byte limit accepted")
	}
	if err := client.PublishDocument(ctx, "bad", "Invoice", time.Now()); err == nil {
		t.Error("invalid publish ID accepted")
	}
	if err := client.PublishDocument(ctx, id, "", time.Now()); err == nil {
		t.Error("empty document type accepted")
	}
	if _, err := client.DocumentsConsumer(ctx, "bad-filter", "outside.documents.Invoice"); err == nil {
		t.Error("foreign filter accepted")
	}
	if err := client.PutUploadRecord(ctx, queue.UploadRecord{ID: "bad"}); err == nil {
		t.Error("invalid record ID accepted")
	}
	if _, err := client.CreateUploadRecord(ctx, queue.UploadRecord{ID: "bad"}); err == nil {
		t.Error("invalid create ID accepted")
	}
	if _, err := client.UpdateUploadRecord(ctx, queue.UploadRecord{ID: "bad"}, 1); err == nil {
		t.Error("invalid update ID accepted")
	}
	if _, err := client.GetUploadRecordVersion(ctx, "bad"); !errors.Is(err, queue.ErrRecordNotFound) {
		t.Errorf("invalid record lookup error = %v", err)
	}
	if keys, err := client.UploadRecordKeys(ctx); err != nil || len(keys) != 0 {
		t.Errorf("empty record keys = %v, %v", keys, err)
	}
	if err := client.PutUploadContent(ctx, "bad", nil); err == nil {
		t.Error("invalid content ID accepted")
	}
	if _, err := client.OpenUploadContent(ctx, "bad"); !errors.Is(err, queue.ErrRecordNotFound) {
		t.Errorf("invalid content open error = %v", err)
	}
	if err := client.DeleteUploadContent(ctx, "bad"); err == nil {
		t.Error("invalid content delete ID accepted")
	}
	if exists, err := client.HasUploadContent(ctx, "bad"); err != nil || exists {
		t.Errorf("invalid content existence = %v, %v", exists, err)
	}
	if err := client.PublishDeadLetter(ctx, queue.DeadLetter{ID: "bad"}); err == nil {
		t.Error("invalid dead-letter ID accepted")
	}
	if depth, err := client.DeadLetterDepth(ctx); err != nil || depth != 0 {
		t.Errorf("empty dead-letter depth = %d, %v", depth, err)
	}
	if err := client.PublishResult(ctx, queue.ValidationResult{ID: "bad"}); err == nil {
		t.Error("invalid result ID accepted")
	}

	client.Close()
	if err := client.EnsureInfra(ctx); err == nil {
		t.Error("EnsureInfra on closed connection succeeded")
	}
	if err := client.PublishDocument(ctx, id, "Invoice", time.Now()); err == nil {
		t.Error("publish on closed connection succeeded")
	}
	if err := client.PutUploadRecord(ctx, queue.UploadRecord{ID: id}); err == nil {
		t.Error("record write on closed connection succeeded")
	}
	if _, err := client.CreateUploadRecord(ctx, queue.UploadRecord{ID: id}); err == nil {
		t.Error("record create on closed connection succeeded")
	}
	if _, err := client.UpdateUploadRecord(ctx, queue.UploadRecord{ID: id}, 1); err == nil {
		t.Error("record update on closed connection succeeded")
	}
	if _, err := client.UploadRecordKeys(ctx); err == nil {
		t.Error("record key scan on closed connection succeeded")
	}
	if _, err := client.GetUploadRecord(ctx, id); err == nil {
		t.Error("record read on closed connection succeeded")
	}
	if err := client.PutUploadContent(ctx, id, nil); err == nil {
		t.Error("content write on closed connection succeeded")
	}
	if _, err := client.GetUploadContent(ctx, id); err == nil {
		t.Error("content read on closed connection succeeded")
	}
	if err := client.DeleteUploadContent(ctx, id); err == nil {
		t.Error("content delete on closed connection succeeded")
	}
	if _, err := client.HasUploadContent(ctx, id); err == nil {
		t.Error("content inspection on closed connection succeeded")
	}
	if _, err := client.DeadLetterDepth(ctx); err == nil {
		t.Error("dead-letter depth on closed connection succeeded")
	}
	if err := client.ReplayProcessingFailure(ctx, id); err == nil {
		t.Error("replay on closed connection succeeded")
	}
	if err := client.PublishResult(ctx, queue.ValidationResult{ID: id}); err == nil {
		t.Error("result publish on closed connection succeeded")
	}
}

func TestMustNamespacePanicsForInvalidValue(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("MustNamespace did not panic")
		}
	}()
	queue.MustNamespace("BAD")
}
