package orchestrator_test

import (
	"context"
	"math"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/orchestrator"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
)

const maxOrchestratorCapacityIterations = 24

type orchestratorCapacityOperation struct {
	run     func(context.Context) (queue.VersionedValidationExecution, error)
	want    queue.ValidationExecutionStatus
	ordinal uint32
}

// BenchmarkOrchestratorCapacity isolates durable start, advance, retry, and
// final-projection transitions. Each operation uses production JetStream KV,
// outbox publication, evidence storage, and compare-and-swap state changes.
func BenchmarkOrchestratorCapacity(b *testing.B) {
	for _, concurrency := range []int{1, 4} {
		b.Run(capacityConcurrencyName("start_dispatch", concurrency), func(b *testing.B) {
			benchmarkOrchestratorTransition(b, concurrency, prepareStartDispatch)
		})
		b.Run(capacityConcurrencyName("xsd_to_en", concurrency), func(b *testing.B) {
			benchmarkOrchestratorTransition(b, concurrency, prepareXSDToEN)
		})
		b.Run(capacityConcurrencyName("operational_retry", concurrency), func(b *testing.B) {
			benchmarkOrchestratorTransition(b, concurrency, prepareOperationalRetry)
		})
		b.Run(capacityConcurrencyName("terminal_projection", concurrency), func(b *testing.B) {
			benchmarkOrchestratorTransition(b, concurrency, prepareTerminalProjection)
		})
	}
	for _, historySize := range []int{100, 1000} {
		b.Run("reconciler_dispatch_scan/"+strconv.Itoa(historySize)+"_entries", func(b *testing.B) {
			benchmarkOrchestratorReconcileScan(b, historySize, func(ctx context.Context, machine *orchestrator.Machine) error {
				report, err := machine.ReconcilePendingDispatches(ctx)
				if err == nil && report.Scanned != 0 {
					b.Fatalf("dispatch reconciler found %d pending entries in steady-state history", report.Scanned)
				}
				return err
			})
		})
		b.Run("reconciler_final_scan/"+strconv.Itoa(historySize)+"_entries", func(b *testing.B) {
			benchmarkOrchestratorReconcileScan(b, historySize, func(ctx context.Context, machine *orchestrator.Machine) error {
				report, err := machine.ReconcilePendingFinalResults(ctx)
				if err == nil && report.Scanned != 0 {
					b.Fatalf("final-result reconciler found %d pending entries in nonterminal history", report.Scanned)
				}
				return err
			})
		})
	}
}

func benchmarkOrchestratorReconcileScan(b *testing.B, historySize int, scan func(context.Context, *orchestrator.Machine) error) {
	b.Helper()
	if b.N > maxOrchestratorCapacityIterations {
		b.Fatalf("orchestrator capacity benchmarks require fixed -benchtime no greater than %dx; got %d", maxOrchestratorCapacityIterations, b.N)
	}
	server := queuetest.StartServer(b)
	client, err := queue.ConnectWithOptions(server.URL, queue.MustNamespace("orchestrator.reconciler.capacity"), server.Options...)
	if err != nil {
		b.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := client.EnsureInfra(ctx); err != nil {
		b.Fatal(err)
	}
	if err := client.ProvisionValidationExecutionInfra(ctx); err != nil {
		b.Fatal(err)
	}
	if err := client.ProvisionStageQueueInfra(ctx, queue.DefaultStageQueueInfraConfig()); err != nil {
		b.Fatal(err)
	}
	machine := orchestrator.New(client)
	for range historySize {
		if _, err := machine.Start(ctx, orchestratorCapacityIntent(b, orchestratorCapacityPlan(false))); err != nil {
			b.Fatal(err)
		}
	}
	runtime.GC()
	cpuBefore := orchestratorCPU(b)
	durations := make([]time.Duration, b.N)
	b.ReportAllocs()
	b.ResetTimer()
	started := time.Now()
	for index := range b.N {
		operationStarted := time.Now()
		if err := scan(ctx, machine); err != nil {
			b.Fatal(err)
		}
		durations[index] = time.Since(operationStarted)
	}
	elapsed := time.Since(started)
	b.StopTimer()
	cpuAfter := orchestratorCPU(b)
	b.ReportMetric(float64(historySize*b.N)/elapsed.Seconds(), "history_entries/s")
	b.ReportMetric(orchestratorPercentile(durations, 0.50), "service_p50_ms")
	b.ReportMetric(orchestratorPercentile(durations, 0.95), "service_p95_ms")
	b.ReportMetric(orchestratorPercentile(durations, 0.99), "service_p99_ms")
	b.ReportMetric((cpuAfter-cpuBefore)*1000/float64(b.N), "cpu_ms/scan")
	b.ReportMetric(float64(historySize), "history_entries")
}

func benchmarkOrchestratorTransition(b *testing.B, concurrency int, prepare func(*testing.B, context.Context, *queue.Client, int) []orchestratorCapacityOperation) {
	b.Helper()
	if b.N > maxOrchestratorCapacityIterations {
		b.Fatalf("orchestrator capacity benchmarks require fixed -benchtime no greater than %dx; got %d", maxOrchestratorCapacityIterations, b.N)
	}
	server := queuetest.StartServer(b)
	client, err := queue.ConnectWithOptions(server.URL, queue.MustNamespace("orchestrator.capacity"), server.Options...)
	if err != nil {
		b.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := client.EnsureInfra(ctx); err != nil {
		b.Fatal(err)
	}
	if err := client.ProvisionValidationExecutionInfra(ctx); err != nil {
		b.Fatal(err)
	}
	if err := client.ProvisionStageQueueInfra(ctx, queue.DefaultStageQueueInfraConfig()); err != nil {
		b.Fatal(err)
	}
	operations := prepare(b, ctx, client, b.N)
	runtime.GC()
	cpuBefore := orchestratorCPU(b)
	durations := make([]time.Duration, len(operations))
	states := make([]queue.VersionedValidationExecution, len(operations))
	errorsByIndex := make([]error, len(operations))
	jobs := make(chan int)
	var workers sync.WaitGroup
	b.ReportAllocs()
	b.ResetTimer()
	started := time.Now()
	for range concurrency {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				operationStarted := time.Now()
				states[index], errorsByIndex[index] = operations[index].run(ctx)
				durations[index] = time.Since(operationStarted)
			}
		}()
	}
	for index := range operations {
		jobs <- index
	}
	close(jobs)
	workers.Wait()
	if err := client.NC.Flush(); err != nil {
		b.Fatal(err)
	}
	elapsed := time.Since(started)
	b.StopTimer()
	cpuAfter := orchestratorCPU(b)
	for index, operation := range operations {
		if errorsByIndex[index] != nil {
			b.Fatal(errorsByIndex[index])
		}
		if states[index].Execution.Orchestration.Status != operation.want {
			b.Fatalf("operation %d status = %q, want %q", index, states[index].Execution.Orchestration.Status, operation.want)
		}
		if operation.ordinal != 0 && states[index].Execution.Orchestration.CurrentStageOrdinal != operation.ordinal {
			b.Fatalf("operation %d ordinal = %d, want %d", index, states[index].Execution.Orchestration.CurrentStageOrdinal, operation.ordinal)
		}
	}
	count := float64(len(operations))
	b.ReportMetric(count/elapsed.Seconds(), "transitions/s")
	b.ReportMetric(orchestratorPercentile(durations, 0.50), "service_p50_ms")
	b.ReportMetric(orchestratorPercentile(durations, 0.95), "service_p95_ms")
	b.ReportMetric(orchestratorPercentile(durations, 0.99), "service_p99_ms")
	b.ReportMetric((cpuAfter-cpuBefore)*1000/count, "cpu_ms/transition")
	b.ReportMetric(float64(concurrency), "concurrency")
}

func prepareStartDispatch(b *testing.B, _ context.Context, client *queue.Client, count int) []orchestratorCapacityOperation {
	b.Helper()
	operations := make([]orchestratorCapacityOperation, count)
	for index := range operations {
		intent := orchestratorCapacityIntent(b, orchestratorCapacityPlan(true))
		operations[index] = orchestratorCapacityOperation{
			run: func(ctx context.Context) (queue.VersionedValidationExecution, error) {
				return orchestrator.New(client).Start(ctx, intent)
			},
			want: queue.ExecutionStatusStageQueued, ordinal: 1,
		}
	}
	return operations
}

func prepareXSDToEN(b *testing.B, ctx context.Context, client *queue.Client, count int) []orchestratorCapacityOperation {
	b.Helper()
	operations := make([]orchestratorCapacityOperation, count)
	for index := range operations {
		machine := orchestrator.New(client)
		state, err := machine.Start(ctx, orchestratorCapacityIntent(b, orchestratorCapacityPlan(true)))
		if err != nil {
			b.Fatal(err)
		}
		request := orchestratorCapacityRequest(b, state.Execution)
		result := orchestratorCapacityResult(b, request, queue.StageOutcomePassed, queue.StageReasonNone)
		operations[index] = orchestratorCapacityOperation{
			run: func(ctx context.Context) (queue.VersionedValidationExecution, error) {
				return orchestrator.New(client).HandleStageResult(ctx, result)
			},
			want: queue.ExecutionStatusStageQueued, ordinal: 2,
		}
	}
	return operations
}

func prepareOperationalRetry(b *testing.B, ctx context.Context, client *queue.Client, count int) []orchestratorCapacityOperation {
	b.Helper()
	operations := make([]orchestratorCapacityOperation, count)
	for index := range operations {
		machine := orchestrator.New(client)
		state, err := machine.Start(ctx, orchestratorCapacityIntent(b, orchestratorCapacityPlan(false)))
		if err != nil {
			b.Fatal(err)
		}
		request := orchestratorCapacityRequest(b, state.Execution)
		result := orchestratorCapacityResult(b, request, queue.StageOutcomeOperationalFailure, queue.StageReasonRuntimeFailure)
		operations[index] = orchestratorCapacityOperation{
			run: func(ctx context.Context) (queue.VersionedValidationExecution, error) {
				return orchestrator.New(client).HandleStageResult(ctx, result)
			},
			want: queue.ExecutionStatusStageQueued, ordinal: 1,
		}
	}
	return operations
}

func prepareTerminalProjection(b *testing.B, ctx context.Context, client *queue.Client, count int) []orchestratorCapacityOperation {
	b.Helper()
	operations := make([]orchestratorCapacityOperation, count)
	for index := range operations {
		machine := orchestrator.New(client)
		state, err := machine.Start(ctx, orchestratorCapacityIntent(b, orchestratorCapacityPlan(false)))
		if err != nil {
			b.Fatal(err)
		}
		request := orchestratorCapacityRequest(b, state.Execution)
		result := orchestratorCapacityResult(b, request, queue.StageOutcomePassed, queue.StageReasonNone)
		operations[index] = orchestratorCapacityOperation{
			run: func(ctx context.Context) (queue.VersionedValidationExecution, error) {
				return orchestrator.New(client).HandleStageResult(ctx, result)
			},
			want: queue.ExecutionStatusValid, ordinal: 1,
		}
	}
	return operations
}

func orchestratorCapacityIntent(b *testing.B, plan queue.FrozenValidationPlan) queue.ValidationExecutionIntent {
	b.Helper()
	return queue.ValidationExecutionIntent{
		IntentID: orchestratorCapacityID(b), UploadID: orchestratorCapacityID(b), DocumentType: "Invoice",
		Trigger: queue.ValidationTriggerInitial, RequestedProfile: queue.ValidationProfileUBL21EN16931,
		Plan: plan, RetryPolicy: queue.DefaultFrozenValidationRetryPolicy(), CreatedAt: time.Now().UTC(),
	}
}

func orchestratorCapacityPlan(withEN bool) queue.FrozenValidationPlan {
	stages := []queue.FrozenValidationStage{{ID: queue.ValidationLayerUBL21XSD, Ordinal: 1, Requirement: queue.ValidationStageRequired, RulesetVersion: "2.1"}}
	if withEN {
		stages = append(stages, queue.FrozenValidationStage{ID: queue.ValidationLayerEN16931, Ordinal: 2, Requirement: queue.ValidationStageRequired, RulesetVersion: "1.3.16"})
	}
	return queue.FrozenValidationPlan{ID: "capacity", Version: "1", Stages: stages}
}

func orchestratorCapacityRequest(b *testing.B, execution queue.ValidationExecution) queue.StageRequest {
	b.Helper()
	stage := execution.Orchestration.Stages[execution.Orchestration.CurrentStageOrdinal-1]
	createdAt, err := time.Parse(time.RFC3339Nano, stage.RequestCreatedAt)
	if err != nil {
		b.Fatal(err)
	}
	request, err := queue.NewStageRequest(execution, execution.Orchestration.CurrentStageOrdinal, stage.AuthorizedAttempt, createdAt)
	if err != nil {
		b.Fatal(err)
	}
	return request
}

func orchestratorCapacityResult(b *testing.B, request queue.StageRequest, outcome queue.StageOutcome, reason queue.StageOperationalReason) queue.StageResult {
	b.Helper()
	now := time.Now().UTC()
	result, err := queue.NewStageResult(request, outcome, reason, nil, "capacity benchmark", now, now, 0)
	if err != nil {
		b.Fatal(err)
	}
	return result
}

func orchestratorCapacityID(b *testing.B) string {
	b.Helper()
	id, err := queue.NewID()
	if err != nil {
		b.Fatal(err)
	}
	return id
}

func capacityConcurrencyName(operation string, concurrency int) string {
	return operation + "/c" + strconv.Itoa(concurrency)
}

func orchestratorPercentile(values []time.Duration, quantile float64) float64 {
	copyValues := append([]time.Duration(nil), values...)
	sort.Slice(copyValues, func(i, j int) bool { return copyValues[i] < copyValues[j] })
	index := int(math.Ceil(float64(len(copyValues))*quantile)) - 1
	return float64(copyValues[index]) / float64(time.Millisecond)
}

func orchestratorCPU(b *testing.B) float64 {
	b.Helper()
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		b.Fatal(err)
	}
	return float64(usage.Utime.Sec+usage.Stime.Sec) + float64(usage.Utime.Usec+usage.Stime.Usec)/1e6
}
