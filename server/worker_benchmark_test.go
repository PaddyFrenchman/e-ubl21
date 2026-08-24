package server

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
)

// BenchmarkWorkerValidationThroughput measures the complete bounded worker
// path: Object Store fetch, xmllint process, KV update and result publish.
// Run with a fixed benchtime (for example -benchtime=10x) when comparing
// concurrency/prefetch configurations.
func BenchmarkWorkerValidationThroughput(b *testing.B) {
	server := queuetest.StartServer(b)
	q, err := queue.ConnectWithOptions(server.URL, queue.DefaultNamespace, server.Options...)
	if err != nil {
		b.Fatal(err)
	}
	defer q.Close()
	ctx := context.Background()
	if err := q.EnsureInfra(ctx); err != nil {
		b.Fatal(err)
	}

	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- RunWorker(workerCtx, WorkerConfig{
			NATSURL:           server.URL,
			NATSOptions:       server.Options,
			ConsumerName:      "worker-benchmark",
			ValidationTimeout: 30 * time.Second,
			AckWait:           60 * time.Second,
			Concurrency:       2,
			Prefetch:          2,
		})
	}()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			b.Errorf("worker: %v", err)
		}
	}()

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := q.JS.Consumer(ctx, q.Namespace.DocumentsStreamName(), "worker-benchmark"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			b.Fatal("worker consumer did not become ready")
		}
		time.Sleep(10 * time.Millisecond)
	}

	ids := make([]string, b.N)
	b.ReportAllocs()
	b.ResetTimer()
	started := time.Now()
	for i := 0; i < b.N; i++ {
		id, _ := queue.NewID()
		ids[i] = id
		if err := q.PutUploadContent(ctx, id, []byte(validInvoiceXML)); err != nil {
			b.Fatal(err)
		}
		if err := q.PutUploadRecord(ctx, queue.UploadRecord{ID: id, DocumentType: "Invoice", Status: queue.StatusPending, ReceivedAt: time.Now().Format(time.RFC3339)}); err != nil {
			b.Fatal(err)
		}
		if err := q.PublishDocument(ctx, id, "Invoice", time.Now()); err != nil {
			b.Fatal(err)
		}
	}
	for _, id := range ids {
		for {
			record, err := q.GetUploadRecord(ctx, id)
			if err != nil {
				b.Fatal(err)
			}
			if record.Status != queue.StatusPending {
				if record.Status != queue.StatusValid {
					b.Fatal(fmt.Errorf("upload %s status %s: %s", id, record.Status, record.Error))
				}
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	elapsed := time.Since(started)
	b.StopTimer()
	b.ReportMetric(float64(b.N)/elapsed.Seconds(), "docs/s")
}
