package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	fixtures "github.com/PaddyFrenchman/e-ubl21/createsamples/en16931"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// BenchmarkWorkerValidationThroughput measures the complete production worker
// path: Object Store fetch, xmllint, optional Saxon EN 16931 validation, KV
// update, and result publication. The matrix makes XSD-only, semantic-valid,
// semantic-invalid, size, and concurrency costs directly comparable.
//
// Run through `make benchmark-en16931`; a fixed iteration count is required
// for meaningful concurrency comparisons.
func BenchmarkWorkerValidationThroughput(b *testing.B) {
	saxonRuntime, ok := localSaxonRuntimeConfig(b)
	if !ok {
		return
	}
	if err := ensureXMLLint(); err != nil {
		b.Skip(err)
	}
	validInvoice, err := fixtures.BuildInvoiceXML(fixtures.NewInvoiceVariantS())
	if err != nil {
		b.Fatal(err)
	}
	negative, err := fixtures.NegativeInvoiceFixtures()
	if err != nil {
		b.Fatal(err)
	}
	if len(negative) == 0 {
		b.Fatal("negative Invoice corpus is empty")
	}
	order, err := os.ReadFile(filepath.Join("..", "createsamples", "samples", "Order", "Order-sample-01.xml"))
	if err != nil {
		b.Fatal(err)
	}
	padInvoice := func(padding int) []byte {
		return []byte(strings.Replace(validInvoice, "</Invoice>", "<!--"+strings.Repeat("x", padding)+"--></Invoice>", 1))
	}
	cases := []struct {
		name         string
		documentType string
		body         []byte
		buildBody    func(*testing.B) ([]byte, int)
		wantStatus   queue.ValidationStatus
		concurrency  int
		prefetch     int
		lineCount    int
	}{
		{name: "xsd_only_order_112KiB/c1_p1", documentType: "Order", body: order, wantStatus: queue.StatusValid, concurrency: 1, prefetch: 1},
		{name: "invoice_valid_small/c1_p1", documentType: "Invoice", body: []byte(validInvoice), wantStatus: queue.StatusValid, concurrency: 1, prefetch: 1},
		{name: "invoice_invalid_small/c1_p1", documentType: "Invoice", body: []byte(negative[0].XML), wantStatus: queue.StatusInvalid, concurrency: 1, prefetch: 1},
		{name: "invoice_valid_byte_padded_1MiB/c1_p1", documentType: "Invoice", body: padInvoice(1 << 20), wantStatus: queue.StatusValid, concurrency: 1, prefetch: 1},
		{name: "invoice_valid_byte_padded_9MiB/c1_p1", documentType: "Invoice", body: padInvoice(9 << 20), wantStatus: queue.StatusValid, concurrency: 1, prefetch: 1},
		{name: "invoice_valid_semantic_100_lines/c1_p1", documentType: "Invoice", buildBody: func(b *testing.B) ([]byte, int) { return buildBenchmarkLineHeavyInvoice(b, 100), 100 }, wantStatus: queue.StatusValid, concurrency: 1, prefetch: 1},
		{name: "invoice_valid_semantic_1000_lines/c1_p1", documentType: "Invoice", buildBody: func(b *testing.B) ([]byte, int) { return buildBenchmarkLineHeavyInvoice(b, 1000), 1000 }, wantStatus: queue.StatusValid, concurrency: 1, prefetch: 1},
		{name: "invoice_valid_semantic_near_limit_2950_lines/c1_p1", documentType: "Invoice", buildBody: func(b *testing.B) ([]byte, int) {
			return buildBenchmarkLineHeavyInvoiceToSize(b, 2950, maxUploadBytes-(64<<10)), 2950
		}, wantStatus: queue.StatusValid, concurrency: 1, prefetch: 1},
		{name: "invoice_valid_small/c1_p4", documentType: "Invoice", body: []byte(validInvoice), wantStatus: queue.StatusValid, concurrency: 1, prefetch: 4},
		{name: "invoice_valid_small/c2_p1", documentType: "Invoice", body: []byte(validInvoice), wantStatus: queue.StatusValid, concurrency: 2, prefetch: 1},
		{name: "invoice_valid_small/c2_p2", documentType: "Invoice", body: []byte(validInvoice), wantStatus: queue.StatusValid, concurrency: 2, prefetch: 2},
		{name: "invoice_valid_small/c4_p4", documentType: "Invoice", body: []byte(validInvoice), wantStatus: queue.StatusValid, concurrency: 4, prefetch: 4},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			body, lineCount := tc.body, tc.lineCount
			if tc.buildBody != nil {
				body, lineCount = tc.buildBody(b)
			}
			benchmarkWorkerValidationThroughput(b, saxonRuntime, tc.documentType, body, tc.wantStatus, tc.concurrency, tc.prefetch, lineCount)
		})
	}
}

func buildBenchmarkLineHeavyInvoice(b *testing.B, lineCount int) []byte {
	b.Helper()
	document, err := fixtures.NewLineHeavyInvoiceVariantS(lineCount)
	if err != nil {
		b.Fatal(err)
	}
	xml, err := fixtures.BuildInvoiceXML(document)
	if err != nil {
		b.Fatal(err)
	}
	return []byte(xml)
}

func buildBenchmarkLineHeavyInvoiceToSize(b *testing.B, lineCount, targetBytes int) []byte {
	b.Helper()
	if targetBytes <= 0 || targetBytes >= maxUploadBytes {
		b.Fatalf("invalid line-heavy target size %d", targetBytes)
	}
	document, err := fixtures.NewLineHeavyInvoiceVariantS(lineCount)
	if err != nil {
		b.Fatal(err)
	}
	baseline, err := fixtures.BuildInvoiceXML(document)
	if err != nil {
		b.Fatal(err)
	}
	padding := targetBytes - len(baseline)
	if padding < lineCount {
		b.Fatalf("%d-line invoice is already %d bytes; cannot pad to %d", lineCount, len(baseline), targetBytes)
	}
	perLine, remainder := padding/lineCount, padding%lineCount
	for index := range document.Lines {
		extra := perLine
		if index < remainder {
			extra++
		}
		document.Lines[index].ItemName += strings.Repeat("x", extra)
	}
	xml, err := fixtures.BuildInvoiceXML(document)
	if err != nil {
		b.Fatal(err)
	}
	if len(xml) != targetBytes {
		b.Fatalf("near-limit line-heavy invoice is %d bytes, want %d", len(xml), targetBytes)
	}
	return []byte(xml)
}

// BenchmarkEN16931ProcessCost quantifies the per-document process model. The
// Java case is a launch-only baseline; the Saxon case includes JVM launch,
// stylesheet compilation, transformation, bounded SVRL capture, and parsing.
func BenchmarkEN16931ProcessCost(b *testing.B) {
	saxonRuntime, ok := localSaxonRuntimeConfig(b)
	if !ok {
		return
	}
	validInvoice, err := fixtures.BuildInvoiceXML(fixtures.NewInvoiceVariantS())
	if err != nil {
		b.Fatal(err)
	}
	b.Run("java_startup", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			cmd := exec.Command(saxonRuntime.javaExecutable, "-version")
			cmd.Stdout = io.Discard
			cmd.Stderr = io.Discard
			if err := cmd.Run(); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("saxon_transform", func(b *testing.B) {
		validator := newSaxonEN16931Validator(saxonRuntime, 30*time.Second)
		b.ReportAllocs()
		b.SetBytes(int64(len(validInvoice)))
		for i := 0; i < b.N; i++ {
			result, err := validator.Validate(context.Background(), strings.NewReader(validInvoice))
			if err != nil || !result.Valid {
				b.Fatalf("Saxon validation result/error = %+v / %v", result, err)
			}
		}
	})
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

func benchmarkWorkerValidationThroughput(b *testing.B, saxonRuntime saxonRuntimeConfig, documentType string, body []byte, wantStatus queue.ValidationStatus, concurrency, prefetch, lineCount int) {
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
			NATSURL:                  natsServer.URL,
			NATSOptions:              natsServer.Options,
			ConsumerName:             "worker-benchmark",
			ValidationTimeout:        30 * time.Second,
			XSDValidationTimeout:     30 * time.Second,
			EN16931ValidationTimeout: 30 * time.Second,
			AckWait:                  60 * time.Second,
			Concurrency:              concurrency,
			Prefetch:                 prefetch,
			JavaExecutable:           saxonRuntime.javaExecutable,
			SaxonJARPath:             saxonRuntime.saxonJARPath,
			XMLResolverJARPath:       saxonRuntime.xmlResolverJARPath,
			SaxonConfigurationPath:   saxonRuntime.configurationPath,
			EN16931XSLTPath:          saxonRuntime.xsltPath,
			SaxonStartupTimeout:      30 * time.Second,
			Logger:                   slog.New(slog.NewTextHandler(io.Discard, nil)),
		})
	}()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			b.Errorf("worker: %v", err)
		}
	}()

	consumer := waitForBenchmarkConsumer(b, ctx, q, "worker-benchmark")

	ids := make([]string, b.N)
	b.ReportAllocs()
	b.SetBytes(int64(len(body)))
	b.ResetTimer()
	started := time.Now()
	for i := 0; i < b.N; i++ {
		id, _ := queue.NewID()
		ids[i] = id
		if err := q.PutUploadContent(ctx, id, body); err != nil {
			b.Fatal(err)
		}
		if err := q.PutUploadRecord(ctx, queue.UploadRecord{ID: id, DocumentType: documentType, Status: queue.StatusPending, ReceivedAt: time.Now().Format(time.RFC3339)}); err != nil {
			b.Fatal(err)
		}
		if err := q.PublishDocument(ctx, id, documentType, time.Now()); err != nil {
			b.Fatal(err)
		}
	}
	for _, id := range ids {
		deadline := time.Now().Add(5 * time.Minute)
		for {
			record, err := q.GetUploadRecord(ctx, id)
			if err != nil {
				b.Fatal(err)
			}
			if record.Status != queue.StatusPending && record.Status != queue.StatusEnqueuePending && record.Status != wantStatus {
				b.Fatal(fmt.Errorf("upload %s status %s, want %s: %s", id, record.Status, wantStatus, record.Error))
			}
			if record.Status == wantStatus && !record.ResultPending {
				published, err := q.HasValidationResult(ctx, id)
				if err != nil {
					b.Fatal(err)
				}
				if benchmarkDocumentResultComplete(record, published, wantStatus) {
					break
				}
			}
			if time.Now().After(deadline) {
				b.Fatalf("upload %s did not durably publish its validation result: status=%s resultPending=%t", id, record.Status, record.ResultPending)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	ackDeadline := time.Now().Add(5 * time.Second)
	for {
		info, err := consumer.Info(ctx)
		if err != nil {
			b.Fatal(err)
		}
		if info.NumAckPending == 0 {
			break
		}
		if time.Now().After(ackDeadline) {
			b.Fatalf("worker left %d benchmark messages awaiting acknowledgement", info.NumAckPending)
		}
		time.Sleep(5 * time.Millisecond)
	}
	elapsed := time.Since(started)
	b.StopTimer()
	b.ReportMetric(float64(b.N)/elapsed.Seconds(), "docs/s")
	b.ReportMetric(elapsed.Seconds()*1000/float64(b.N), "wall_ms/doc")
	b.ReportMetric(float64(concurrency), "workers")
	b.ReportMetric(float64(prefetch), "prefetch")
	if lineCount > 0 {
		b.ReportMetric(float64(lineCount), "invoice_lines")
	}
}

func benchmarkDocumentResultComplete(record queue.UploadRecord, published bool, wantStatus queue.ValidationStatus) bool {
	return record.Status == wantStatus && !record.ResultPending && published
}

func TestBenchmarkCompletionRequiresDurableResultPublication(t *testing.T) {
	complete := queue.UploadRecord{Status: queue.StatusValid}
	tests := []struct {
		name      string
		record    queue.UploadRecord
		published bool
		want      bool
	}{
		{name: "complete", record: complete, published: true, want: true},
		{name: "result pending", record: queue.UploadRecord{Status: queue.StatusValid, ResultPending: true}, published: true},
		{name: "result absent", record: complete},
		{name: "status pending", record: queue.UploadRecord{Status: queue.StatusPending}, published: true},
		{name: "wrong terminal status", record: queue.UploadRecord{Status: queue.StatusInvalid}, published: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := benchmarkDocumentResultComplete(test.record, test.published, queue.StatusValid); got != test.want {
				t.Fatalf("complete = %t, want %t for record %+v published=%t", got, test.want, test.record, test.published)
			}
		})
	}
}
