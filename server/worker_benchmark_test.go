package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
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
	cases := []struct {
		name    string
		padding int
	}{
		{name: "small"},
		{name: "medium_1MiB", padding: 1 << 20},
		{name: "near_limit_9MiB", padding: 9 << 20},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			benchmarkWorkerValidationThroughput(b, tc.padding)
		})
	}
}

func benchmarkWorkerValidationThroughput(b *testing.B, padding int) {
	natsServer := queuetest.StartServer(b)
	q, err := queue.ConnectWithOptions(natsServer.URL, queue.DefaultNamespace, natsServer.Options...)
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
			NATSURL:           natsServer.URL,
			NATSOptions:       natsServer.Options,
			ConsumerName:      "worker-benchmark",
			ValidationTimeout: 30 * time.Second,
			AckWait:           60 * time.Second,
			Concurrency:       2,
			Prefetch:          2,
			Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
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

	body := validInvoiceXML
	if padding > 0 {
		body = strings.Replace(body, "</Invoice>", "<!--"+strings.Repeat("x", padding)+"--></Invoice>", 1)
	}
	ids := make([]string, b.N)
	b.ReportAllocs()
	b.SetBytes(int64(len(body)))
	b.ResetTimer()
	started := time.Now()
	for i := 0; i < b.N; i++ {
		id, _ := queue.NewID()
		ids[i] = id
		if err := q.PutUploadContent(ctx, id, []byte(body)); err != nil {
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
