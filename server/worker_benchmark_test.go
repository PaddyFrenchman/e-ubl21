package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
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

// BenchmarkWorkerDeepBacklogMemory is an opt-in stress benchmark for peak
// live heap under a deep descriptor backlog. All descriptors reference the
// same immutable near-limit object: this models the worker's relevant memory
// behavior without creating roughly 100 GiB of duplicate test objects.
//
// Run the full scenario explicitly:
//
//	EUBL21_BACKLOG_BENCHMARK_DOCUMENTS=10000 go test ./server \
//	  -run '^$' -bench '^BenchmarkWorkerDeepBacklogMemory$' -benchtime=1x
func BenchmarkWorkerDeepBacklogMemory(b *testing.B) {
	count, err := strconv.Atoi(os.Getenv("EUBL21_BACKLOG_BENCHMARK_DOCUMENTS"))
	if err != nil || count <= 0 {
		b.Skip("set EUBL21_BACKLOG_BENCHMARK_DOCUMENTS to run the deep-backlog stress benchmark")
	}

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

	id, _ := queue.NewID()
	body := strings.Replace(validInvoiceXML, "</Invoice>", "<!--"+strings.Repeat("x", 9<<20)+"--></Invoice>", 1)
	if err := q.PutUploadContent(ctx, id, []byte(body)); err != nil {
		b.Fatal(err)
	}
	if err := q.PutUploadRecord(ctx, queue.UploadRecord{ID: id, DocumentType: "Invoice", Status: queue.StatusPending, ReceivedAt: time.Now().Format(time.RFC3339)}); err != nil {
		b.Fatal(err)
	}
	receivedAt := time.Now().UTC().Format(time.RFC3339)
	payload, err := json.Marshal(queue.DocumentWorkItem{ID: id, DocumentType: "Invoice", ReceivedAt: receivedAt})
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < count; i++ {
		msg := nats.NewMsg(q.Namespace.DocumentSubject("Invoice"))
		msg.Data = payload
		msg.Header.Set(queue.HeaderUploadID, id)
		msg.Header.Set(queue.HeaderDocumentType, "Invoice")
		msg.Header.Set(queue.HeaderReceivedAt, receivedAt)
		msg.Header.Set(nats.MsgIdHdr, fmt.Sprintf("%s-%d", id, i))
		if _, err := q.JS.PublishMsg(ctx, msg); err != nil {
			b.Fatal(err)
		}
	}

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	var peak atomic.Uint64
	peak.Store(before.HeapAlloc)
	sampleDone := make(chan struct{})
	go samplePeakHeap(sampleDone, &peak)

	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- RunWorker(workerCtx, WorkerConfig{
			NATSURL: natsServer.URL, NATSOptions: natsServer.Options,
			ConsumerName: "deep-backlog-benchmark", ValidationTimeout: 30 * time.Second,
			AckWait: 60 * time.Second, Concurrency: 2, Prefetch: 2,
			Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		})
	}()

	consumer := waitForBenchmarkConsumer(b, ctx, q, "deep-backlog-benchmark")
	deadline := time.Now().Add(time.Hour)
	for {
		info, err := consumer.Info(ctx)
		if err != nil {
			b.Fatal(err)
		}
		if info.NumPending == 0 && info.NumAckPending == 0 {
			break
		}
		if time.Now().After(deadline) {
			b.Fatalf("backlog did not drain within one hour: pending=%d ackPending=%d", info.NumPending, info.NumAckPending)
		}
		time.Sleep(100 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		b.Fatal(err)
	}
	close(sampleDone)
	b.ReportMetric(float64(peak.Load()), "peak_heap_bytes")
	b.ReportMetric(float64(peak.Load()-before.HeapAlloc), "heap_growth_bytes")
	b.ReportMetric(float64(count), "documents")
}

func samplePeakHeap(done <-chan struct{}, peak *atomic.Uint64) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			var stats runtime.MemStats
			runtime.ReadMemStats(&stats)
			for current := peak.Load(); stats.HeapAlloc > current && !peak.CompareAndSwap(current, stats.HeapAlloc); current = peak.Load() {
			}
		case <-done:
			return
		}
	}
}

func waitForBenchmarkConsumer(b *testing.B, ctx context.Context, q *queue.Client, name string) jetstream.Consumer {
	b.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		consumer, err := q.JS.Consumer(ctx, q.Namespace.DocumentsStreamName(), name)
		if err == nil {
			return consumer
		}
		if time.Now().After(deadline) {
			b.Fatal("worker consumer did not become ready")
		}
		time.Sleep(10 * time.Millisecond)
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
