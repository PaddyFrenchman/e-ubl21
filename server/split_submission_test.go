package server

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
	"time"

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
