package queue_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
	"github.com/nats-io/nats.go/jetstream"
)

func TestStageQueueProvisioningCreatesIndependentDurableTopology(t *testing.T) {
	server := queuetest.StartServer(t)
	client := connectStageInfraClient(t, server, queue.DefaultNamespace)
	ctx := context.Background()
	config := queue.DefaultStageQueueInfraConfig()
	config.Stages[0].RequestMaxAge = time.Hour
	config.Stages[0].ResultMaxAge = 2 * time.Hour
	config.Stages[0].DeadLetterMaxAge = 24 * time.Hour
	config.Stages[0].RequestDelivery = queue.StageDeliveryConfig{AckWait: 45 * time.Second, MaxAckPending: 11}
	config.Stages[0].ResultDelivery = queue.StageDeliveryConfig{AckWait: 15 * time.Second, MaxAckPending: 101}
	config.Stages[1].RequestMaxAge = 3 * time.Hour
	config.Stages[1].ResultMaxAge = 4 * time.Hour
	config.Stages[1].DeadLetterMaxAge = 48 * time.Hour
	config.Stages[1].RequestDelivery = queue.StageDeliveryConfig{AckWait: 90 * time.Second, MaxAckPending: 7}
	config.Stages[1].ResultDelivery = queue.StageDeliveryConfig{AckWait: 30 * time.Second, MaxAckPending: 202}

	if err := client.ProvisionStageQueueInfra(ctx, config); err != nil {
		t.Fatal(err)
	}
	if err := client.VerifyStageQueueInfra(ctx, config); err != nil {
		t.Fatal(err)
	}
	workerDelivery, err := client.VerifyStageWorkerInfra(ctx, queue.ValidationLayerUBL21XSD)
	if err != nil {
		t.Fatal(err)
	}
	if workerDelivery != config.Stages[0].RequestDelivery {
		t.Fatalf("XSD worker request delivery = %+v, want %+v", workerDelivery, config.Stages[0].RequestDelivery)
	}
	for _, stage := range config.Stages {
		if depth, err := client.StageRequestQueueDepth(ctx, stage.Stage); err != nil || depth != 0 {
			t.Fatalf("%s initial request depth = %d, %v", stage.Stage, depth, err)
		}
		names, err := client.Namespace.StageQueue(stage.Stage)
		if err != nil {
			t.Fatal(err)
		}
		requestStream, err := client.JS.Stream(ctx, names.RequestStream)
		if err != nil {
			t.Fatal(err)
		}
		requestInfo, err := requestStream.Info(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(requestInfo.Config.Subjects, []string{names.RequestSubject}) || requestInfo.Config.Retention != jetstream.WorkQueuePolicy || requestInfo.Config.Storage != jetstream.FileStorage || requestInfo.Config.MaxAge != stage.RequestMaxAge || requestInfo.Config.MaxMsgSize != 16<<10 {
			t.Fatalf("%s request stream config = %+v", stage.Stage, requestInfo.Config)
		}
		requestConsumer, err := client.StageRequestConsumer(ctx, stage.Stage)
		if err != nil {
			t.Fatal(err)
		}
		requestConsumerInfo, err := requestConsumer.Info(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if requestConsumerInfo.Config.FilterSubject != names.RequestSubject || requestConsumerInfo.Config.AckWait != stage.RequestDelivery.AckWait || requestConsumerInfo.Config.MaxAckPending != stage.RequestDelivery.MaxAckPending || requestConsumerInfo.Config.MaxDeliver != -1 {
			t.Fatalf("%s request consumer config = %+v", stage.Stage, requestConsumerInfo.Config)
		}

		resultStream, err := client.JS.Stream(ctx, names.ResultStream)
		if err != nil {
			t.Fatal(err)
		}
		resultInfo, err := resultStream.Info(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(resultInfo.Config.Subjects, []string{names.ResultSubject}) || resultInfo.Config.Retention != jetstream.WorkQueuePolicy || resultInfo.Config.Storage != jetstream.FileStorage || resultInfo.Config.MaxAge != stage.ResultMaxAge || resultInfo.Config.MaxMsgSize != 2<<20 {
			t.Fatalf("%s result stream config = %+v", stage.Stage, resultInfo.Config)
		}
		resultConsumer, err := client.StageResultConsumer(ctx, stage.Stage)
		if err != nil {
			t.Fatal(err)
		}
		resultConsumerInfo, err := resultConsumer.Info(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if resultConsumerInfo.Config.FilterSubject != names.ResultSubject || resultConsumerInfo.Config.AckWait != stage.ResultDelivery.AckWait || resultConsumerInfo.Config.MaxAckPending != stage.ResultDelivery.MaxAckPending || resultConsumerInfo.Config.MaxDeliver != -1 {
			t.Fatalf("%s result consumer config = %+v", stage.Stage, resultConsumerInfo.Config)
		}
		deadLetterStream, err := client.JS.Stream(ctx, names.DeadLetterStream)
		if err != nil {
			t.Fatal(err)
		}
		deadLetterInfo, err := deadLetterStream.Info(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(deadLetterInfo.Config.Subjects, []string{names.DeadLetterFilterSubject}) || deadLetterInfo.Config.Retention != jetstream.LimitsPolicy ||
			deadLetterInfo.Config.Storage != jetstream.FileStorage || deadLetterInfo.Config.MaxAge != stage.DeadLetterMaxAge || deadLetterInfo.Config.MaxMsgsPerSubject != 1 || !deadLetterInfo.Config.DiscardNewPerSubject {
			t.Fatalf("%s dead-letter stream config = %+v", stage.Stage, deadLetterInfo.Config)
		}
	}
	if _, err := client.StageRequestQueueDepth(ctx, queue.ValidationLayer("unknown")); err == nil {
		t.Fatal("unknown stage request depth was accepted")
	}
}

func TestStageQueuesRetainWorkAcrossScaleToZeroAndIsolateStageAndNamespace(t *testing.T) {
	server := queuetest.StartServer(t)
	dev := connectStageInfraClient(t, server, queue.MustNamespace("eubl21.dev"))
	prod := connectStageInfraClient(t, server, queue.MustNamespace("eubl21.prod"))
	ctx := context.Background()
	config := queue.DefaultStageQueueInfraConfig()
	if err := dev.ProvisionStageQueueInfra(ctx, config); err != nil {
		t.Fatal(err)
	}
	if err := prod.ProvisionStageQueueInfra(ctx, config); err != nil {
		t.Fatal(err)
	}
	devXSD, _ := dev.Namespace.StageQueue(queue.ValidationLayerUBL21XSD)
	devEN, _ := dev.Namespace.StageQueue(queue.ValidationLayerEN16931)
	prodXSD, _ := prod.Namespace.StageQueue(queue.ValidationLayerUBL21XSD)
	if _, err := dev.JS.Publish(ctx, devXSD.RequestSubject, []byte(`{"stage":"dev-xsd"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := dev.JS.Publish(ctx, devEN.RequestSubject, []byte(`{"stage":"dev-en"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := prod.JS.Publish(ctx, prodXSD.RequestSubject, []byte(`{"stage":"prod-xsd"}`)); err != nil {
		t.Fatal(err)
	}
	if depth, err := dev.StageRequestQueueDepth(ctx, queue.ValidationLayerUBL21XSD); err != nil || depth != 1 {
		t.Fatalf("dev XSD request depth = %d, %v; want 1", depth, err)
	}
	if depth, err := dev.StageRequestQueueDepth(ctx, queue.ValidationLayerEN16931); err != nil || depth != 1 {
		t.Fatalf("dev EN request depth = %d, %v; want 1", depth, err)
	}
	if _, err := dev.JS.Publish(ctx, devXSD.ResultSubject, []byte(`{"outcome":"passed"}`)); err != nil {
		t.Fatal(err)
	}

	// No consumer was opened while publishing. A later worker replica opens
	// the durable consumer and receives the retained work.
	runtimeDev := connectStageInfraClient(t, server, queue.MustNamespace("eubl21.dev"))
	if err := runtimeDev.VerifyStageQueueInfra(ctx, config); err != nil {
		t.Fatal(err)
	}
	assertOneStageMessage(t, ctx, runtimeDev, queue.ValidationLayerUBL21XSD, devXSD.RequestSubject, `{"stage":"dev-xsd"}`)
	assertOneStageMessage(t, ctx, runtimeDev, queue.ValidationLayerEN16931, devEN.RequestSubject, `{"stage":"dev-en"}`)
	assertOneStageMessage(t, ctx, prod, queue.ValidationLayerUBL21XSD, prodXSD.RequestSubject, `{"stage":"prod-xsd"}`)
	assertOneStageResult(t, ctx, runtimeDev, queue.ValidationLayerUBL21XSD, devXSD.ResultSubject, `{"outcome":"passed"}`)
}

func TestStageWorkerVerificationDoesNotRequireOrchestratorResultConsumer(t *testing.T) {
	server := queuetest.StartServer(t)
	client := connectStageInfraClient(t, server, queue.MustNamespace("workerrole"))
	ctx := context.Background()
	stage := queue.DefaultStageQueueInfraConfig().Stages[0]
	stage.RequestDelivery = queue.StageDeliveryConfig{AckWait: 90 * time.Second, MaxAckPending: 20}
	stage.ResultDelivery = queue.StageDeliveryConfig{AckWait: 120 * time.Second, MaxAckPending: 40}
	if err := client.ProvisionStageQueueInfra(ctx, queue.StageQueueInfraConfig{Stages: []queue.StageQueueConfig{stage}}); err != nil {
		t.Fatal(err)
	}
	names, err := client.Namespace.StageQueue(queue.ValidationLayerUBL21XSD)
	if err != nil {
		t.Fatal(err)
	}
	resultStream, err := client.JS.Stream(ctx, names.ResultStream)
	if err != nil {
		t.Fatal(err)
	}
	if err := resultStream.DeleteConsumer(ctx, names.ResultConsumer); err != nil {
		t.Fatal(err)
	}
	delivery, err := client.VerifyStageWorkerInfra(ctx, queue.ValidationLayerUBL21XSD)
	if err != nil {
		t.Fatalf("worker verification depended on orchestrator result consumer: %v", err)
	}
	if delivery != stage.RequestDelivery {
		t.Fatalf("worker request delivery = %+v, want %+v", delivery, stage.RequestDelivery)
	}
}

func TestStageRequestStreamRejectsNonCompactPayload(t *testing.T) {
	server := queuetest.StartServer(t)
	client := connectStageInfraClient(t, server, queue.MustNamespace("bounded"))
	ctx := context.Background()
	if err := client.ProvisionStageQueueInfra(ctx, queue.DefaultStageQueueInfraConfig()); err != nil {
		t.Fatal(err)
	}
	names, _ := client.Namespace.StageQueue(queue.ValidationLayerUBL21XSD)
	if _, err := client.JS.Publish(ctx, names.RequestSubject, make([]byte, (16<<10)+1)); err == nil {
		t.Fatal("stage request stream accepted a payload above the compact contract limit")
	}
}

func TestMaximumValidStageMessagesAreTransportable(t *testing.T) {
	server := queuetest.StartServer(t)
	client := connectStageInfraClient(t, server, queue.MustNamespace("maximum"))
	ctx := context.Background()
	if err := client.ProvisionStageQueueInfra(ctx, queue.DefaultStageQueueInfraConfig()); err != nil {
		t.Fatal(err)
	}

	plan := testFrozenPlan()
	plan.ID = strings.Repeat("a", queue.MaxStagePlanIDBytes)
	plan.Version = strings.Repeat("\x00", queue.MaxStageVersionBytes)
	plan.Stages[0].RulesetVersion = strings.Repeat("\x00", queue.MaxStageVersionBytes)
	intent := testInitialIntent(t, testID(t), plan, time.Now())
	intent.DocumentType = strings.Repeat("I", 128)
	execution, err := queue.NewValidationExecution(intent)
	if err != nil {
		t.Fatal(err)
	}
	request, err := queue.NewStageRequest(execution, 1, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	requestJSON, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if len(requestJSON) > 16<<10 {
		t.Fatalf("maximum-valid StageRequest serialized to %d bytes, above 16 KiB", len(requestJSON))
	}
	names, _ := client.Namespace.StageQueue(request.Stage)
	if _, err := client.JS.Publish(ctx, names.RequestSubject, requestJSON); err != nil {
		t.Fatalf("publish maximum-valid StageRequest (%d bytes): %v", len(requestJSON), err)
	}

	issues := make([]queue.ValidationIssue, 0, queue.MaxStageIssues)
	remainingText := queue.MaxStageIssueTextBytes
	for remainingText > 0 {
		messageBytes := queue.MaxStageIssueMessageBytes
		if messageBytes > remainingText-1 {
			messageBytes = remainingText - 1
		}
		issues = append(issues, queue.ValidationIssue{
			Kind: queue.ValidationIssueFailedAssert, Layer: request.Stage,
			Severity: "e", Message: strings.Repeat("\x00", messageBytes),
		})
		remainingText -= messageBytes + 1 // severity is part of aggregate text
	}
	startedAt := time.Now()
	result, err := queue.NewStageResult(
		request, queue.StageOutcomeStandardsInvalid, queue.StageReasonNone, issues,
		strings.Repeat("\x00", queue.MaxStageDiagnosticBytes),
		startedAt, startedAt.Add(time.Second), time.Second,
	)
	if err != nil {
		t.Fatal(err)
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if len(resultJSON) <= 512<<10 {
		t.Fatalf("escaping regression fixture serialized to only %d bytes; it must exercise the former 512 KiB failure", len(resultJSON))
	}
	if len(resultJSON) > 2<<20 {
		t.Fatalf("maximum-valid StageResult serialized to %d bytes, above 2 MiB", len(resultJSON))
	}
	if _, err := client.JS.Publish(ctx, names.ResultSubject, resultJSON); err != nil {
		t.Fatalf("publish maximum-valid StageResult (%d bytes): %v", len(resultJSON), err)
	}
}

func TestStageQueueProvisioningDoesNotMutateLegacyDocumentStream(t *testing.T) {
	server := queuetest.StartServer(t)
	client := connectStageInfraClient(t, server, queue.DefaultNamespace)
	ctx := context.Background()
	if err := client.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	id, err := queue.NewID()
	if err != nil {
		t.Fatal(err)
	}
	if err := client.PublishDocument(ctx, id, "Invoice", time.Now()); err != nil {
		t.Fatal(err)
	}
	legacy, err := client.JS.Stream(ctx, client.Namespace.DocumentsStreamName())
	if err != nil {
		t.Fatal(err)
	}
	before, err := legacy.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.ProvisionStageQueueInfra(ctx, queue.DefaultStageQueueInfraConfig()); err != nil {
		t.Fatal(err)
	}
	after, err := legacy.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if before.Config.Name != after.Config.Name || !reflect.DeepEqual(before.Config.Subjects, after.Config.Subjects) || before.Config.Retention != after.Config.Retention || before.Config.MaxAge != after.Config.MaxAge || after.State.Msgs != 1 {
		t.Fatalf("legacy document stream changed during stage provisioning: before=%+v after=%+v", before, after)
	}
}

func TestStageQueueRuntimeAccessDoesNotProvisionAndVerificationFailsClosed(t *testing.T) {
	server := queuetest.StartServer(t)
	client := connectStageInfraClient(t, server, queue.MustNamespace("runtime"))
	ctx := context.Background()
	if err := client.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.StageRequestConsumer(ctx, queue.ValidationLayerUBL21XSD); err == nil {
		t.Fatal("runtime accessor created or opened missing stage infrastructure")
	}
	names, _ := client.Namespace.StageQueue(queue.ValidationLayerUBL21XSD)
	if _, err := client.JS.Stream(ctx, names.RequestStream); err == nil {
		t.Fatal("runtime accessor provisioned a missing request stream")
	}

	stageConfig := queue.DefaultStageQueueInfraConfig().Stages[0]
	config := queue.StageQueueInfraConfig{Stages: []queue.StageQueueConfig{stageConfig}}
	if _, err := client.JS.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name: names.RequestStream, Subjects: []string{names.RequestSubject},
		Retention: jetstream.LimitsPolicy, Storage: jetstream.MemoryStorage,
	}); err != nil {
		t.Fatal(err)
	}
	if err := client.VerifyStageQueueInfra(ctx, config); err == nil {
		t.Fatal("runtime verification accepted incompatible stage infrastructure")
	}
}

func TestStageQueueConfigValidation(t *testing.T) {
	valid := queue.DefaultStageQueueInfraConfig()
	if err := validateStageQueueConfigByProvisioning(t, valid); err != nil {
		t.Fatalf("default config: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*queue.StageQueueInfraConfig)
	}{
		{"empty", func(config *queue.StageQueueInfraConfig) { config.Stages = nil }},
		{"unknown stage", func(config *queue.StageQueueInfraConfig) { config.Stages[0].Stage = "unknown" }},
		{"duplicate stage", func(config *queue.StageQueueInfraConfig) { config.Stages[1].Stage = config.Stages[0].Stage }},
		{"negative request age", func(config *queue.StageQueueInfraConfig) { config.Stages[0].RequestMaxAge = -time.Second }},
		{"negative result age", func(config *queue.StageQueueInfraConfig) { config.Stages[0].ResultMaxAge = -time.Second }},
		{"negative dead-letter age", func(config *queue.StageQueueInfraConfig) { config.Stages[0].DeadLetterMaxAge = -time.Second }},
		{"zero request ack wait", func(config *queue.StageQueueInfraConfig) { config.Stages[0].RequestDelivery.AckWait = 0 }},
		{"zero result ack wait", func(config *queue.StageQueueInfraConfig) { config.Stages[0].ResultDelivery.AckWait = 0 }},
		{"zero request pending", func(config *queue.StageQueueInfraConfig) { config.Stages[0].RequestDelivery.MaxAckPending = 0 }},
		{"zero result pending", func(config *queue.StageQueueInfraConfig) { config.Stages[0].ResultDelivery.MaxAckPending = 0 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := queue.DefaultStageQueueInfraConfig()
			test.mutate(&candidate)
			if err := validateStageQueueConfigByProvisioning(t, candidate); err == nil {
				t.Fatal("invalid stage queue config was accepted")
			}
		})
	}
	copyOne := queue.DefaultStageQueueInfraConfig()
	copyOne.Stages[0].RequestDelivery.MaxAckPending = 999
	copyTwo := queue.DefaultStageQueueInfraConfig()
	if copyTwo.Stages[0].RequestDelivery.MaxAckPending == 999 {
		t.Fatal("default topology returned shared mutable stage configuration")
	}
}

func connectStageInfraClient(t *testing.T, server queuetest.Server, namespace queue.Namespace) *queue.Client {
	t.Helper()
	client, err := queue.ConnectWithOptions(server.URL, namespace, server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return client
}

func assertOneStageMessage(t *testing.T, ctx context.Context, client *queue.Client, stage queue.ValidationLayer, wantSubject, wantPayload string) {
	t.Helper()
	consumer, err := client.StageRequestConsumer(ctx, stage)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := consumer.Fetch(1, jetstream.FetchMaxWait(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for message := range batch.Messages() {
		count++
		if message.Subject() != wantSubject || string(message.Data()) != wantPayload {
			t.Fatalf("stage message subject=%q payload=%q, want subject=%q payload=%q", message.Subject(), message.Data(), wantSubject, wantPayload)
		}
		if err := message.Ack(); err != nil {
			t.Fatal(err)
		}
	}
	if err := batch.Error(); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("received %d stage messages, want 1", count)
	}
}

func assertOneStageResult(t *testing.T, ctx context.Context, client *queue.Client, stage queue.ValidationLayer, wantSubject, wantPayload string) {
	t.Helper()
	consumer, err := client.StageResultConsumer(ctx, stage)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := consumer.Fetch(1, jetstream.FetchMaxWait(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for message := range batch.Messages() {
		count++
		if message.Subject() != wantSubject || string(message.Data()) != wantPayload {
			t.Fatalf("stage result subject=%q payload=%q, want subject=%q payload=%q", message.Subject(), message.Data(), wantSubject, wantPayload)
		}
		if err := message.Ack(); err != nil {
			t.Fatal(err)
		}
	}
	if err := batch.Error(); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("received %d stage results, want 1", count)
	}
}

func validateStageQueueConfigByProvisioning(t *testing.T, config queue.StageQueueInfraConfig) error {
	t.Helper()
	server := queuetest.StartServer(t)
	client := connectStageInfraClient(t, server, queue.MustNamespace("config"))
	return client.ProvisionStageQueueInfra(context.Background(), config)
}
