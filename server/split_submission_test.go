package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/orchestrator"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
)

func TestMigrateLegacyPendingUploadsRestartsCompletePlanAndPreservesTerminal(t *testing.T) {
	client, ctx := newMigrationTestClient(t)
	terminalID, _ := queue.NewID()
	terminal := queue.UploadRecord{
		ID: terminalID, DocumentType: "Invoice", Status: queue.StatusValid,
		ReceivedAt:  time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
		ValidatedAt: "legacy", ValidationStages: queue.ValidationStages{{Layer: queue.ValidationLayerUBL21XSD, Valid: true}},
	}
	if err := client.PutUploadRecord(ctx, terminal); err != nil {
		t.Fatal(err)
	}
	pendingID, _ := queue.NewID()
	pending := queue.UploadRecord{
		ID: pendingID, DocumentType: "Invoice", Status: queue.StatusPending,
		ReceivedAt:       time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
		ValidationStages: queue.ValidationStages{{Layer: queue.ValidationLayerUBL21XSD, Valid: true}}, ProcessingAttempts: 2,
	}
	if err := client.PutUploadRecord(ctx, pending); err != nil {
		t.Fatal(err)
	}
	if err := client.PutUploadContent(ctx, pendingID, []byte(validInvoiceXML)); err != nil {
		t.Fatal(err)
	}

	report, err := MigrateLegacyPendingUploads(ctx, client)
	if err != nil {
		t.Fatal(err)
	}
	if report.Started != 1 || report.Preserved != 1 || report.Errors != 0 {
		t.Fatalf("migration report = %+v", report)
	}
	storedTerminal, err := client.GetUploadRecord(ctx, terminalID)
	if err != nil || !reflect.DeepEqual(storedTerminal, terminal) {
		t.Fatalf("terminal changed: got=%+v err=%v want=%+v", storedTerminal, err, terminal)
	}
	storedPending, err := client.GetUploadRecord(ctx, pendingID)
	if err != nil {
		t.Fatal(err)
	}
	if storedPending.Status != queue.StatusPending || storedPending.Profile != queue.ValidationProfileUBL21EN16931 || len(storedPending.ValidationStages) != 0 || storedPending.ProcessingAttempts != 0 {
		t.Fatalf("migrated pending record = %+v", storedPending)
	}
	executions, err := client.ValidationExecutionsForUpload(ctx, pendingID)
	if err != nil || len(executions) != 1 {
		t.Fatalf("executions = %+v, err=%v", executions, err)
	}
	if got := executions[0].Plan.Stages; len(got) != 2 || got[0].ID != queue.ValidationLayerUBL21XSD || got[1].ID != queue.ValidationLayerEN16931 {
		t.Fatalf("migrated plan = %+v", got)
	}
}

func TestApplicationMigrationModesControlAdmissionAndDispatch(t *testing.T) {
	for _, test := range []struct {
		name       string
		mode       queue.DeploymentMode
		wantStatus int
		wantExec   int
	}{
		{name: "draining", mode: queue.DeploymentModeDraining, wantStatus: http.StatusServiceUnavailable},
		{name: "split", mode: queue.DeploymentModeSplit, wantStatus: http.StatusAccepted, wantExec: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := testConfig(t, nil)
			seed, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			if err := seed.EnsureInfra(ctx); err != nil {
				t.Fatal(err)
			}
			if err := seed.ProvisionValidationExecutionInfra(ctx); err != nil {
				t.Fatal(err)
			}
			if err := seed.ProvisionStageQueueInfra(ctx, queue.DefaultStageQueueInfraConfig()); err != nil {
				t.Fatal(err)
			}
			legacy, err := seed.ProvisionDeploymentGenerationInfra(ctx, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			current, err := seed.TransitionDeploymentMode(ctx, legacy, queue.DeploymentModeDraining, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if test.mode == queue.DeploymentModeSplit {
				current, err = seed.TransitionDeploymentMode(ctx, current, queue.DeploymentModeSplit, time.Now())
				if err != nil {
					t.Fatal(err)
				}
			}
			_ = current
			seed.Close()

			handler := newApplicationHandler(t, cfg)
			token := signedJWT(t, cfg.OAuthIssuer, cfg.OAuthAudience, testSecret, time.Hour)
			response := doRequest(handler, http.MethodPost, "/api/v1/ubl/upload", token, validInvoiceXML, "application/xml")
			if response.Code != test.wantStatus {
				t.Fatalf("upload status = %d, body=%s", response.Code, response.Body.String())
			}
			if test.wantExec == 0 {
				return
			}
			var payload map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			id, _ := payload["id"].(string)
			applicationClient, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(applicationClient.Close)
			executions, err := applicationClient.ValidationExecutionsForUpload(ctx, id)
			if err != nil || len(executions) != test.wantExec {
				t.Fatalf("split executions = %+v, err=%v", executions, err)
			}
			drain, err := applicationClient.InspectLegacyDrain(ctx, defaultWorkerConsumerName)
			if err != nil || drain.StreamMessages != 0 {
				t.Fatalf("legacy queue received split upload: %+v, err=%v", drain, err)
			}
		})
	}
}

func TestSplitCanonicalResultProjectsToUploadStatusAndRetention(t *testing.T) {
	client, ctx := newMigrationTestClient(t)
	legacy, err := client.ProvisionDeploymentGenerationInfra(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	draining, err := client.TransitionDeploymentMode(ctx, legacy, queue.DeploymentModeDraining, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.TransitionDeploymentMode(ctx, draining, queue.DeploymentModeSplit, time.Now()); err != nil {
		t.Fatal(err)
	}

	id, _ := queue.NewID()
	receivedAt := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Second)
	record := queue.UploadRecord{
		ID: id, DocumentType: "Invoice", Profile: queue.ValidationProfileUBL21EN16931,
		Status: queue.StatusPending, ReceivedAt: receivedAt.Format(time.RFC3339),
	}
	if err := client.PutUploadRecord(ctx, record); err != nil {
		t.Fatal(err)
	}
	if err := client.PutUploadContent(ctx, id, []byte(validInvoiceXML)); err != nil {
		t.Fatal(err)
	}

	_, plan, err := FrozenValidationPlanFor(record.DocumentType, record.Profile)
	if err != nil {
		t.Fatal(err)
	}
	var requests []queue.StageRequest
	machine := orchestrator.New(client)
	machine.PublishStageRequest = func(_ context.Context, request queue.StageRequest) error {
		requests = append(requests, request)
		return nil
	}
	if _, err := machine.Start(ctx, queue.ValidationExecutionIntent{
		IntentID: id, UploadID: id, DocumentType: record.DocumentType,
		Trigger: queue.ValidationTriggerInitial, RequestedProfile: record.Profile,
		Plan: plan, RetryPolicy: queue.DefaultFrozenValidationRetryPolicy(), CreatedAt: receivedAt,
	}); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 2; index++ {
		if len(requests) != index+1 {
			t.Fatalf("published requests = %d, want %d", len(requests), index+1)
		}
		started := receivedAt.Add(time.Duration(index+1) * time.Minute)
		result, err := queue.NewStageResult(requests[index], queue.StageOutcomePassed, queue.StageReasonNone, nil, "", started, started.Add(time.Second), time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := machine.HandleStageResult(ctx, result); err != nil {
			t.Fatal(err)
		}
	}

	stored, err := client.GetUploadRecord(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != queue.StatusPending {
		t.Fatalf("acceptance metadata was mutated to %q", stored.Status)
	}
	projected, err := client.ProjectUploadRecord(ctx, stored)
	if err != nil {
		t.Fatal(err)
	}
	if projected.Status != queue.StatusValid || len(projected.ValidationStages) != 2 || projected.ValidatedAt == "" {
		executions, listErr := client.ValidationExecutionsForUpload(ctx, id)
		t.Fatalf("split upload projection = %+v; executions=%+v; listErr=%v", projected, executions, listErr)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ubl/"+id, nil)
	request.SetPathValue("id", id)
	response := httptest.NewRecorder()
	(&uploadHandlers{queue: client, projectUpload: client.ProjectUploadRecord}).handleGetUpload(response, request)
	var statusPayload map[string]any
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &statusPayload) != nil || statusPayload["status"] != string(queue.StatusValid) {
		t.Fatalf("projected HTTP status = %d body=%s", response.Code, response.Body.String())
	}
	executions, err := client.ValidationExecutionsForUpload(ctx, id)
	if err != nil || len(executions) != 1 {
		t.Fatalf("initial executions = %+v, err=%v", executions, err)
	}
	replayIntentID, _ := queue.NewID()
	replay, err := queue.NextValidationGeneration(executions[0], replayIntentID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.CreateValidationExecution(ctx, replay); err != nil {
		t.Fatal(err)
	}
	replayVersion, err := client.GetValidationExecutionVersion(ctx, replay.UploadID, replay.ValidationID, replay.Generation)
	if err != nil {
		t.Fatal(err)
	}
	requests = nil
	if _, err := machine.EnsureDispatch(ctx, replayVersion); err != nil {
		t.Fatal(err)
	}
	invalidResult, err := queue.NewStageResult(requests[0], queue.StageOutcomeStandardsInvalid, queue.StageReasonNone, nil, "replay found invalid XML", time.Now(), time.Now().Add(time.Second), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.HandleStageResult(ctx, invalidResult); err != nil {
		t.Fatal(err)
	}
	projected, err = client.ProjectUploadRecord(ctx, stored)
	if err != nil || projected.Status != queue.StatusInvalid || len(projected.ValidationStages) != 1 {
		t.Fatalf("replay generation projection = %+v, err=%v", projected, err)
	}
	report, err := client.ExpireUploadContent(ctx, time.Minute, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if report.Expired != 1 || report.Reclaimed != 1 {
		t.Fatalf("split retention report = %+v", report)
	}
	expired, err := client.GetUploadRecord(ctx, id)
	if err != nil || expired.ContentExpiredAt == "" {
		t.Fatalf("split retention metadata = %+v, err=%v", expired, err)
	}
}

func newMigrationTestClient(t *testing.T) (*queue.Client, context.Context) {
	t.Helper()
	server := queuetest.StartServer(t)
	client, err := queue.ConnectWithOptions(server.URL, queue.MustNamespace("migration.pending"), server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	ctx := context.Background()
	if err := client.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	if err := client.ProvisionValidationExecutionInfra(ctx); err != nil {
		t.Fatal(err)
	}
	if err := client.ProvisionStageQueueInfra(ctx, queue.DefaultStageQueueInfraConfig()); err != nil {
		t.Fatal(err)
	}
	return client, ctx
}
