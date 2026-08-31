package orchestrator_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/orchestrator"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
	"github.com/nats-io/nats.go/jetstream"
)

func TestChaosGateDuplicateENResultsProduceOneFinalResult(t *testing.T) {
	machine, client, ctx := newMachine(t)
	if _, err := machine.Start(ctx, testIntent(t, testPlan())); err != nil {
		t.Fatal(err)
	}
	xsdRequest := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	if _, err := machine.HandleStageResult(ctx, resultFor(t, xsdRequest, queue.StageOutcomePassed, queue.StageReasonNone)); err != nil {
		t.Fatal(err)
	}
	enRequest := takeRequest(t, ctx, client, queue.ValidationLayerEN16931)
	enResult := resultFor(t, enRequest, queue.StageOutcomePassed, queue.StageReasonNone)

	const replicas = 8
	start := make(chan struct{})
	errorsSeen := make(chan error, replicas)
	var group sync.WaitGroup
	for range replicas {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			replica := orchestrator.New(client)
			replica.Now = func() time.Time { return time.Date(2026, 8, 31, 15, 0, 0, 0, time.UTC) }
			if _, err := replica.HandleStageResult(context.Background(), enResult); err != nil {
				errorsSeen <- err
			}
		}()
	}
	close(start)
	group.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		t.Errorf("concurrent duplicate EN result: %v", err)
	}

	stored, err := client.GetValidationExecution(ctx, enResult.UploadID, enResult.ValidationID, enResult.Generation)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Orchestration.Status != queue.ExecutionStatusValid || len(stored.Orchestration.Stages) != 2 || len(stored.Orchestration.Stages[1].Attempts) != 1 {
		t.Fatalf("duplicate EN terminal state = %+v", stored.Orchestration)
	}
	assertPermanentResultCount(t, ctx, client, 1)
}

func TestChaosGateRestartDrainsThousandsOfCompletedTransitions(t *testing.T) {
	const transitionCount = 1000
	server := queuetest.StartServer(t)
	client, err := queue.ConnectWithOptions(server.URL, queue.MustNamespace("orchestrator.chaos.backlog"), server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := client.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	if err := client.ProvisionValidationExecutionInfra(ctx); err != nil {
		t.Fatal(err)
	}
	if err := client.ProvisionStageQueueInfra(ctx, queue.DefaultStageQueueInfraConfig()); err != nil {
		t.Fatal(err)
	}

	plan := testPlan()
	plan.Stages = plan.Stages[:1]
	producer := orchestrator.New(client)
	producer.Now = func() time.Time { return time.Date(2026, 8, 31, 16, 0, 0, 0, time.UTC) }
	for range transitionCount {
		if _, err := producer.Start(ctx, testIntent(t, plan)); err != nil {
			t.Fatal(err)
		}
	}

	requestConsumer, err := client.StageRequestConsumer(ctx, queue.ValidationLayerUBL21XSD)
	if err != nil {
		t.Fatal(err)
	}
	requests := make([]queue.StageRequest, 0, transitionCount)
	for len(requests) < transitionCount {
		remaining := transitionCount - len(requests)
		batchSize := min(remaining, 64)
		batch, fetchErr := requestConsumer.Fetch(batchSize, jetstream.FetchMaxWait(5*time.Second))
		if fetchErr != nil {
			t.Fatal(fetchErr)
		}
		for message := range batch.Messages() {
			var request queue.StageRequest
			if err := request.UnmarshalJSON(message.Data()); err != nil {
				t.Fatal(err)
			}
			if err := message.Ack(); err != nil {
				t.Fatal(err)
			}
			requests = append(requests, request)
		}
		if err := batch.Error(); err != nil {
			t.Fatal(err)
		}
	}
	for _, request := range requests {
		if err := client.PublishStageResult(ctx, resultFor(t, request, queue.StageOutcomePassed, queue.StageReasonNone)); err != nil {
			t.Fatal(err)
		}
	}

	// No orchestrator consumes results until every completed transition is
	// durable. A fresh machine then represents the restarted process.
	restarted := orchestrator.New(client)
	restarted.Now = func() time.Time { return time.Date(2026, 8, 31, 16, 1, 0, 0, time.UTC) }
	resultConsumer, err := client.StageResultConsumer(ctx, queue.ValidationLayerUBL21XSD)
	if err != nil {
		t.Fatal(err)
	}
	advanced := 0
	for advanced < transitionCount {
		remaining := transitionCount - advanced
		batchSize := min(remaining, 64)
		batch, fetchErr := resultConsumer.Fetch(batchSize, jetstream.FetchMaxWait(5*time.Second))
		if fetchErr != nil {
			t.Fatal(fetchErr)
		}
		for message := range batch.Messages() {
			if err := restarted.HandleStageResultMessage(ctx, message); err != nil {
				t.Fatal(err)
			}
			advanced++
		}
		if err := batch.Error(); err != nil {
			t.Fatal(err)
		}
	}
	if advanced != transitionCount {
		t.Fatalf("advanced transitions = %d, want %d", advanced, transitionCount)
	}
	assertPermanentResultCount(t, ctx, client, transitionCount)
	for _, request := range requests {
		execution, err := client.GetValidationExecution(ctx, request.UploadID, request.ValidationID, request.Generation)
		if err != nil {
			t.Fatal(err)
		}
		if execution.Orchestration.Status != queue.ExecutionStatusValid || execution.Orchestration.FinalResultStatus != queue.FinalResultPublished || len(execution.Orchestration.Stages[0].Attempts) != 1 {
			t.Fatalf("restarted transition %s = %+v", request.ValidationID, execution.Orchestration)
		}
	}
}

func TestChaosGateConcurrentReplayCommandsConverge(t *testing.T) {
	machine, client, ctx := newMachine(t)
	plan := testPlan()
	plan.Stages = plan.Stages[:1]
	started, err := machine.Start(ctx, testIntent(t, plan, 1))
	if err != nil {
		t.Fatal(err)
	}
	request := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	failure := resultFor(t, request, queue.StageOutcomeOperationalFailure, queue.StageReasonRuntimeFailure)
	dead, err := queue.NewStageDeadLetter(failure, 1, time.Date(2026, 8, 31, 17, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if err := client.PublishStageDeadLetter(ctx, dead); err != nil {
		t.Fatal(err)
	}
	failed, err := machine.HandleStageResult(ctx, failure)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Execution.Orchestration.Status != queue.ExecutionStatusProcessingFailed {
		t.Fatalf("failed generation = %+v", failed.Execution.Orchestration)
	}

	const replicas = 16
	start := make(chan struct{})
	successes := make(chan queue.VersionedValidationExecution, replicas)
	errorsSeen := make(chan error, replicas)
	var group sync.WaitGroup
	for range replicas {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			replica := orchestrator.New(client)
			replica.Now = func() time.Time { return time.Date(2026, 8, 31, 17, 1, 0, 0, time.UTC) }
			replayed, replayErr := replica.ReplayStage(context.Background(), dead)
			if replayErr != nil {
				errorsSeen <- replayErr
				return
			}
			successes <- replayed
		}()
	}
	close(start)
	group.Wait()
	close(successes)
	close(errorsSeen)

	successCount := 0
	for replayed := range successes {
		successCount++
		if replayed.Execution.Generation != started.Execution.Generation+1 || replayed.Execution.CreationIntentID != dead.DeadLetterID {
			t.Errorf("concurrent replay execution = %+v", replayed.Execution)
		}
	}
	if successCount == 0 {
		t.Fatal("no concurrent replay command succeeded")
	}
	for replayErr := range errorsSeen {
		if !errors.Is(replayErr, jetstream.ErrMsgNotFound) {
			t.Errorf("concurrent replay error = %v", replayErr)
		}
	}

	executions, err := client.ValidationExecutionsForUpload(ctx, started.Execution.UploadID)
	if err != nil {
		t.Fatal(err)
	}
	if len(executions) != 2 || executions[1].Generation != 2 || executions[1].CreationIntentID != dead.DeadLetterID {
		t.Fatalf("concurrent replay generations = %+v", executions)
	}
	replayRequest := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	finisher := orchestrator.New(client)
	finisher.Now = func() time.Time { return time.Date(2026, 8, 31, 17, 2, 0, 0, time.UTC) }
	terminal, err := finisher.HandleStageResult(ctx, resultFor(t, replayRequest, queue.StageOutcomePassed, queue.StageReasonNone))
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Execution.Orchestration.Status != queue.ExecutionStatusValid {
		t.Fatalf("replayed terminal state = %+v", terminal.Execution.Orchestration)
	}
	assertPermanentResultCount(t, ctx, client, 2)
}

func assertPermanentResultCount(t *testing.T, ctx context.Context, client *queue.Client, want uint64) {
	t.Helper()
	stream, err := client.JS.Stream(ctx, client.Namespace.ResultsStreamName())
	if err != nil {
		t.Fatal(err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.State.Msgs != want {
		t.Fatalf("permanent final results = %d, want %d", info.State.Msgs, want)
	}
}
