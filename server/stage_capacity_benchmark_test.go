package server

import (
	"context"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	fixtures "github.com/PaddyFrenchman/e-ubl21/createsamples/en16931"
	"github.com/PaddyFrenchman/e-ubl21/orchestrator"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	defaultStageCapacitySamples = 100
	maxStageCapacitySamples     = 500
)

type stageCapacityCase struct {
	name        string
	body        []byte
	lines       int
	concurrency int
	wantOutcome queue.StageOutcome
}

type stageCapacityMessage struct {
	message   jetstream.Msg
	createdAt time.Time
}

// BenchmarkXSDStageCapacity measures the independently deployed XSD worker
// from durable broker arrival through authorization, Object Store read,
// xmllint execution, result publication and request acknowledgement.
func BenchmarkXSDStageCapacity(b *testing.B) {
	if err := ensureXMLLint(); err != nil {
		b.Skip(err)
	}
	catalog, err := loadValidationCatalog()
	if err != nil {
		b.Fatal(err)
	}
	order, err := os.ReadFile(filepath.Join("..", "createsamples", "samples", "Order", "Order-sample-01.xml"))
	if err != nil {
		b.Fatal(err)
	}
	invoice, err := fixtures.BuildInvoiceXML(fixtures.NewInvoiceVariantS())
	if err != nil {
		b.Fatal(err)
	}
	large := []byte(strings.Replace(invoice, "</Invoice>", "<!--"+strings.Repeat("x", 1<<20)+"--></Invoice>", 1))
	lineHeavy := buildBenchmarkLineHeavyInvoice(b, 1000)
	nearLimit := buildBenchmarkLineHeavyInvoiceToSize(b, 2950, maxUploadBytes-(64<<10))
	cases := []stageCapacityCase{
		{name: "valid_order_112KiB/c1", body: order, concurrency: 1, wantOutcome: queue.StageOutcomePassed},
		{name: "valid_invoice_small/c1", body: []byte(invoice), concurrency: 1, wantOutcome: queue.StageOutcomePassed},
		{name: "valid_invoice_small/c2", body: []byte(invoice), concurrency: 2, wantOutcome: queue.StageOutcomePassed},
		{name: "valid_invoice_small/c4", body: []byte(invoice), concurrency: 4, wantOutcome: queue.StageOutcomePassed},
		{name: "valid_invoice_1MiB/c2", body: large, concurrency: 2, wantOutcome: queue.StageOutcomePassed},
		{name: "valid_invoice_1000_lines/c2", body: lineHeavy, lines: 1000, concurrency: 2, wantOutcome: queue.StageOutcomePassed},
		{name: "valid_invoice_near_limit_2950_lines/c1", body: nearLimit, lines: 2950, concurrency: 1, wantOutcome: queue.StageOutcomePassed},
		{name: "invalid_xml/c2", body: []byte(`<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2">`), concurrency: 2, wantOutcome: queue.StageOutcomeStandardsInvalid},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			benchmarkXSDStageCase(b, catalog, tc)
		})
	}
}

// BenchmarkEN16931StageCapacity measures the independently deployed semantic
// worker. Every operation launches the current process-per-document Saxon JVM.
func BenchmarkEN16931StageCapacity(b *testing.B) {
	runtimeConfig, ok := localSaxonRuntimeConfig(b)
	if !ok {
		return
	}
	valid, err := fixtures.BuildInvoiceXML(fixtures.NewInvoiceVariantS())
	if err != nil {
		b.Fatal(err)
	}
	negative, err := fixtures.NegativeInvoiceFixtures()
	if err != nil || len(negative) == 0 {
		b.Fatalf("load negative EN 16931 fixtures: count=%d error=%v", len(negative), err)
	}
	lineHeavy := buildBenchmarkLineHeavyInvoice(b, 1000)
	nearLimit := buildBenchmarkLineHeavyInvoiceToSize(b, 2950, maxUploadBytes-(64<<10))
	cases := []stageCapacityCase{
		{name: "valid_invoice_small/c1", body: []byte(valid), concurrency: 1, wantOutcome: queue.StageOutcomePassed},
		{name: "valid_invoice_small/c2", body: []byte(valid), concurrency: 2, wantOutcome: queue.StageOutcomePassed},
		{name: "valid_invoice_small/c4", body: []byte(valid), concurrency: 4, wantOutcome: queue.StageOutcomePassed},
		{name: "valid_invoice_1000_lines/c1", body: lineHeavy, lines: 1000, concurrency: 1, wantOutcome: queue.StageOutcomePassed},
		{name: "valid_invoice_near_limit_2950_lines/c1", body: nearLimit, lines: 2950, concurrency: 1, wantOutcome: queue.StageOutcomePassed},
		{name: "invalid_semantics/c1", body: []byte(negative[0].XML), concurrency: 1, wantOutcome: queue.StageOutcomeStandardsInvalid},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			benchmarkENStageCase(b, runtimeConfig, tc)
		})
	}
}

func benchmarkXSDStageCase(b *testing.B, catalog *validationCatalog, tc stageCapacityCase) {
	requireSingleCapacityBenchmarkIteration(b)
	samples := stageCapacitySampleCount(b)
	client, ctx := newStageCapacityClient(b)
	prepareStageCapacityRequests(b, ctx, client, tc.body, queue.ValidationLayerUBL21XSD, samples)
	worker := &xsdStageWorker{
		queue: client, catalog: catalog, validate: validateReaderAgainstSchema,
		timeout: 30 * time.Second, logger: discardCapacityLogger(), now: time.Now,
		publishResult: client.PublishStageResult, publishDeadLetter: client.PublishStageDeadLetter,
		nakDelay: xsdWorkerNAKDelay,
	}
	runStageCapacityCase(b, ctx, client, queue.ValidationLayerUBL21XSD, tc, samples, worker.handle)
}

func benchmarkENStageCase(b *testing.B, runtimeConfig saxonRuntimeConfig, tc stageCapacityCase) {
	requireSingleCapacityBenchmarkIteration(b)
	samples := stageCapacitySampleCount(b)
	client, ctx := newStageCapacityClient(b)
	prepareStageCapacityRequests(b, ctx, client, tc.body, queue.ValidationLayerEN16931, samples)
	worker := &enStageWorker{
		queue: client, validator: newSaxonEN16931Validator(runtimeConfig, 45*time.Second),
		timeout: 45 * time.Second, logger: discardCapacityLogger(), now: time.Now,
		publishResult: client.PublishStageResult, publishDeadLetter: client.PublishStageDeadLetter,
		nakDelay: enWorkerNAKDelay,
	}
	runStageCapacityCase(b, ctx, client, queue.ValidationLayerEN16931, tc, samples, worker.handle)
}

func newStageCapacityClient(b *testing.B) (*queue.Client, context.Context) {
	b.Helper()
	server := queuetest.StartServer(b)
	namespace := queue.MustNamespace("stage.capacity")
	client, err := queue.ConnectWithOptions(server.URL, namespace, server.Options...)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(client.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	b.Cleanup(cancel)
	if err := client.EnsureInfra(ctx); err != nil {
		b.Fatal(err)
	}
	if err := client.ProvisionValidationExecutionInfra(ctx); err != nil {
		b.Fatal(err)
	}
	if err := client.ProvisionStageQueueInfra(ctx, queue.DefaultStageQueueInfraConfig()); err != nil {
		b.Fatal(err)
	}
	return client, ctx
}

func prepareStageCapacityRequests(b *testing.B, ctx context.Context, client *queue.Client, body []byte, stage queue.ValidationLayer, count int) {
	b.Helper()
	machine := orchestrator.New(client)
	uploadID := capacityID(b)
	if err := client.PutUploadContent(ctx, uploadID, body); err != nil {
		b.Fatal(err)
	}
	var initialReference *queue.ValidationExecutionReference
	for index := range count {
		plan := capacityPlan(stage)
		plan.Version = strconv.Itoa(index + 1)
		intent := queue.ValidationExecutionIntent{
			IntentID: capacityID(b), UploadID: uploadID, DocumentType: "Invoice",
			Trigger: queue.ValidationTriggerInitial, RequestedProfile: queue.ValidationProfileUBL21EN16931,
			Plan: plan, RetryPolicy: queue.DefaultFrozenValidationRetryPolicy(), CreatedAt: time.Now().UTC(),
		}
		if index > 0 {
			intent.Trigger = queue.ValidationTriggerRevalidation
			intent.RevalidationOf = initialReference
		}
		if stage == queue.ValidationLayerUBL21XSD && bytesLookLikeOrder(body) {
			intent.DocumentType = "Order"
			intent.RequestedProfile = queue.ValidationProfileUBL21
		}
		state, err := machine.Start(ctx, intent)
		if err != nil {
			b.Fatal(err)
		}
		if index == 0 {
			initialReference = &queue.ValidationExecutionReference{ValidationID: state.Execution.ValidationID, Generation: state.Execution.Generation}
		}
		if stage == queue.ValidationLayerEN16931 {
			xsdRequest := takeCapacityRequest(b, ctx, client, queue.ValidationLayerUBL21XSD)
			xsdResult, err := queue.NewStageResult(xsdRequest, queue.StageOutcomePassed, queue.StageReasonNone, nil, "", time.Now().UTC(), time.Now().UTC(), 0)
			if err != nil {
				b.Fatal(err)
			}
			if state, err = machine.HandleStageResult(ctx, xsdResult); err != nil {
				b.Fatal(err)
			}
			_ = state
		}
	}
}

func runStageCapacityCase(b *testing.B, ctx context.Context, client *queue.Client, stage queue.ValidationLayer, tc stageCapacityCase, count int, handle func(context.Context, jetstream.Msg) error) {
	b.Helper()
	consumer, err := client.StageRequestConsumer(ctx, stage)
	if err != nil {
		b.Fatal(err)
	}
	runtime.GC()
	var memoryBefore runtime.MemStats
	runtime.ReadMemStats(&memoryBefore)
	cpuBefore := capacityCPU(b)
	service := make([]time.Duration, count)
	queueWait := make([]time.Duration, count)
	errorsByIndex := make([]error, count)
	type indexedMessage struct {
		index int
		stageCapacityMessage
	}
	jobs := make(chan indexedMessage)
	completed := make(chan int)
	var workers sync.WaitGroup
	b.SetBytes(int64(len(tc.body) * count))
	b.ResetTimer()
	started := time.Now()
	for range tc.concurrency {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for job := range jobs {
				serviceStarted := time.Now()
				queueWait[job.index] = serviceStarted.Sub(job.createdAt)
				errorsByIndex[job.index] = handle(ctx, job.message)
				service[job.index] = time.Since(serviceStarted)
				completed <- job.index
			}
		}()
	}
	received := 0
	for received < count {
		waveSize := min(tc.concurrency, count-received)
		batch, err := consumer.Fetch(waveSize, jetstream.FetchMaxWait(30*time.Second))
		if err != nil {
			b.Fatal(err)
		}
		waveReceived := 0
		for message := range batch.Messages() {
			var request queue.StageRequest
			if err := request.UnmarshalJSON(message.Data()); err != nil {
				b.Fatal(err)
			}
			createdAt, err := time.Parse(time.RFC3339Nano, request.CreatedAt)
			if err != nil {
				b.Fatal(err)
			}
			jobs <- indexedMessage{index: received, stageCapacityMessage: stageCapacityMessage{message: message, createdAt: createdAt}}
			received++
			waveReceived++
		}
		if err := batch.Error(); err != nil {
			b.Fatal(err)
		}
		if waveReceived != waveSize {
			b.Fatalf("received %d %s requests in wave, want %d", waveReceived, stage, waveSize)
		}
		for range waveReceived {
			<-completed
		}
	}
	close(jobs)
	workers.Wait()
	if received != count {
		b.Fatalf("received %d %s requests, want %d", received, stage, count)
	}
	if err := client.NC.Flush(); err != nil {
		b.Fatal(err)
	}
	elapsed := time.Since(started)
	b.StopTimer()
	cpuAfter := capacityCPU(b)
	var memoryAfter runtime.MemStats
	runtime.ReadMemStats(&memoryAfter)
	for _, err := range errorsByIndex {
		if err != nil {
			b.Fatal(err)
		}
	}
	verifyCapacityResults(b, ctx, client, stage, count, tc.wantOutcome)
	reportCapacityMetrics(b, service, queueWait, elapsed, cpuAfter-cpuBefore, memoryAfter.TotalAlloc-memoryBefore.TotalAlloc, memoryAfter.Mallocs-memoryBefore.Mallocs, tc.concurrency, len(tc.body), tc.lines)
}

func takeCapacityRequest(b *testing.B, ctx context.Context, client *queue.Client, stage queue.ValidationLayer) queue.StageRequest {
	b.Helper()
	consumer, err := client.StageRequestConsumer(ctx, stage)
	if err != nil {
		b.Fatal(err)
	}
	batch, err := consumer.Fetch(1, jetstream.FetchMaxWait(5*time.Second))
	if err != nil {
		b.Fatal(err)
	}
	var request queue.StageRequest
	count := 0
	for message := range batch.Messages() {
		count++
		if err := request.UnmarshalJSON(message.Data()); err != nil {
			b.Fatal(err)
		}
		if err := message.Ack(); err != nil {
			b.Fatal(err)
		}
	}
	if err := batch.Error(); err != nil {
		b.Fatal(err)
	}
	if count != 1 {
		b.Fatalf("received %d %s requests, want 1", count, stage)
	}
	return request
}

func verifyCapacityResults(b *testing.B, ctx context.Context, client *queue.Client, stage queue.ValidationLayer, count int, want queue.StageOutcome) {
	b.Helper()
	consumer, err := client.StageResultConsumer(ctx, stage)
	if err != nil {
		b.Fatal(err)
	}
	seen := 0
	for seen < count {
		batchSize := min(count-seen, 64)
		batch, err := consumer.Fetch(batchSize, jetstream.FetchMaxWait(30*time.Second))
		if err != nil {
			b.Fatal(err)
		}
		before := seen
		for message := range batch.Messages() {
			var result queue.StageResult
			if err := result.UnmarshalJSON(message.Data()); err != nil {
				b.Fatal(err)
			}
			if result.Outcome != want {
				b.Fatalf("%s result outcome = %q, want %q", stage, result.Outcome, want)
			}
			seen++
			if err := message.Ack(); err != nil {
				b.Fatal(err)
			}
		}
		if err := batch.Error(); err != nil {
			b.Fatal(err)
		}
		if seen == before {
			b.Fatalf("received %d %s results, want %d", seen, stage, count)
		}
	}
}

func reportCapacityMetrics(b *testing.B, service, queueWait []time.Duration, elapsed time.Duration, cpuSeconds float64, allocatedBytes, allocations uint64, concurrency, inputBytes, lines int) {
	b.Helper()
	count := float64(len(service))
	b.ReportMetric(count/elapsed.Seconds(), "docs/s")
	b.ReportMetric(durationPercentile(service, 0.50), "service_p50_ms")
	b.ReportMetric(durationPercentile(service, 0.95), "service_p95_ms")
	b.ReportMetric(durationPercentile(service, 0.99), "service_p99_ms")
	b.ReportMetric(durationPercentile(queueWait, 0.50), "queue_p50_ms")
	b.ReportMetric(durationPercentile(queueWait, 0.95), "queue_p95_ms")
	b.ReportMetric(durationPercentile(queueWait, 0.99), "queue_p99_ms")
	b.ReportMetric(cpuSeconds*1000/count, "cpu_ms/doc")
	b.ReportMetric(float64(allocatedBytes)/count, "alloc_B/doc")
	b.ReportMetric(float64(allocations)/count, "allocs/doc")
	b.ReportMetric(float64(concurrency), "concurrency")
	b.ReportMetric(float64(inputBytes), "input_B/doc")
	b.ReportMetric(float64(lines), "semantic_lines")
}

func requireSingleCapacityBenchmarkIteration(b *testing.B) {
	b.Helper()
	if b.N != 1 {
		b.Fatalf("stage capacity benchmarks require -benchtime=1x; got %d Go benchmark iterations", b.N)
	}
}

func stageCapacitySampleCount(b *testing.B) int {
	b.Helper()
	value := os.Getenv("EUBL21_CAPACITY_SAMPLES")
	if value == "" {
		return defaultStageCapacitySamples
	}
	count, err := strconv.Atoi(value)
	if err != nil || count < 1 || count > maxStageCapacitySamples {
		b.Fatalf("EUBL21_CAPACITY_SAMPLES must be between 1 and %d; got %q", maxStageCapacitySamples, value)
	}
	return count
}

func durationPercentile(values []time.Duration, quantile float64) float64 {
	copyValues := append([]time.Duration(nil), values...)
	sort.Slice(copyValues, func(i, j int) bool { return copyValues[i] < copyValues[j] })
	index := int(math.Ceil(float64(len(copyValues))*quantile)) - 1
	return float64(copyValues[index]) / float64(time.Millisecond)
}

func capacityCPU(b *testing.B) float64 {
	b.Helper()
	var self, children syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &self); err != nil {
		b.Fatal(err)
	}
	if err := syscall.Getrusage(syscall.RUSAGE_CHILDREN, &children); err != nil {
		b.Fatal(err)
	}
	return timevalSeconds(self.Utime) + timevalSeconds(self.Stime) + timevalSeconds(children.Utime) + timevalSeconds(children.Stime)
}

func timevalSeconds(value syscall.Timeval) float64 {
	return float64(value.Sec) + float64(value.Usec)/1e6
}

func capacityPlan(finalStage queue.ValidationLayer) queue.FrozenValidationPlan {
	stages := []queue.FrozenValidationStage{{ID: queue.ValidationLayerUBL21XSD, Ordinal: 1, Requirement: queue.ValidationStageRequired, RulesetVersion: "2.1"}}
	profile := "ubl21-xsd"
	if finalStage == queue.ValidationLayerEN16931 {
		profile = "ubl21-en16931"
		stages = append(stages, queue.FrozenValidationStage{ID: queue.ValidationLayerEN16931, Ordinal: 2, Requirement: queue.ValidationStageRequired, RulesetVersion: "1.3.16"})
	}
	return queue.FrozenValidationPlan{ID: profile, Version: "1", Stages: stages}
}

func bytesLookLikeOrder(body []byte) bool {
	return strings.Contains(string(body[:min(len(body), 512)]), "<Order")
}

func capacityID(b *testing.B) string {
	b.Helper()
	id, err := queue.NewID()
	if err != nil {
		b.Fatal(err)
	}
	return id
}

func discardCapacityLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
