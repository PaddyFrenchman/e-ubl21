package queue_test

import (
	"context"
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
	if err := client.PutUploadContent(ctx, "bad", nil); err == nil {
		t.Error("invalid content ID accepted")
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
	if _, err := client.GetUploadRecord(ctx, id); err == nil {
		t.Error("record read on closed connection succeeded")
	}
	if err := client.PutUploadContent(ctx, id, nil); err == nil {
		t.Error("content write on closed connection succeeded")
	}
	if _, err := client.GetUploadContent(ctx, id); err == nil {
		t.Error("content read on closed connection succeeded")
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
