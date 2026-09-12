package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	xsdschemas "github.com/PaddyFrenchman/e-ubl21/XSD"
	"github.com/PaddyFrenchman/e-ubl21/observability"
	"github.com/PaddyFrenchman/e-ubl21/orchestrator"
	"github.com/PaddyFrenchman/e-ubl21/peppol"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
)

const splitValidationPlanVersion = "1"

// FrozenValidationPlanFor selects the complete immutable stage plan used by
// new split executions and by legacy-pending migration.
func FrozenValidationPlanFor(documentType string, profile queue.ValidationProfile) (queue.ValidationProfile, queue.FrozenValidationPlan, error) {
	if profile == "" {
		if requiresEN16931(documentType) {
			profile = queue.ValidationProfileUBL21EN16931
		} else {
			profile = queue.ValidationProfileUBL21
		}
	}
	if !queue.ValidValidationProfile(profile) {
		return "", queue.FrozenValidationPlan{}, fmt.Errorf("unsupported validation profile %q", profile)
	}
	plan, err := frozenValidationPlanForResolvedProfile(documentType, profile)
	if err != nil {
		return "", queue.FrozenValidationPlan{}, err
	}
	return profile, plan, nil
}

// ResolveFrozenValidationPlanFor resolves automatic or explicit Peppol
// applicability and builds the immutable concrete plan.
func ResolveFrozenValidationPlanFor(documentType string, requested queue.ValidationProfile, document []byte) (peppol.Resolution, queue.FrozenValidationPlan, error) {
	if requested == queue.ValidationProfilePeppolEnvelope {
		resolution, err := peppol.Resolve(documentType, queue.ValidationProfilePeppolBISBilling, document)
		if err != nil {
			return peppol.Resolution{}, queue.FrozenValidationPlan{}, err
		}
		resolution.RequestedProfile = requested
		resolution.ResolvedProfile = requested
		plan, err := frozenValidationPlanForResolvedProfile(documentType, requested)
		return resolution, plan, err
	}
	resolution, err := peppol.Resolve(documentType, requested, document)
	if err != nil {
		return peppol.Resolution{}, queue.FrozenValidationPlan{}, err
	}
	plan, err := frozenValidationPlanForResolvedProfile(documentType, resolution.ResolvedProfile)
	if err != nil {
		return peppol.Resolution{}, queue.FrozenValidationPlan{}, err
	}
	return resolution, plan, nil
}

func frozenValidationPlanForResolvedProfile(documentType string, profile queue.ValidationProfile) (queue.FrozenValidationPlan, error) {
	if !queue.ValidValidationProfile(profile) {
		return queue.FrozenValidationPlan{}, fmt.Errorf("unsupported resolved validation profile %q", profile)
	}
	if profile != queue.ValidationProfileUBL21 && !requiresEN16931(documentType) {
		return queue.FrozenValidationPlan{}, fmt.Errorf("validation profile %q is not applicable to %s", profile, documentType)
	}
	stages := []queue.FrozenValidationStage{{
		ID: queue.ValidationLayerUBL21XSD, Ordinal: 1,
		Requirement: queue.ValidationStageRequired, RulesetVersion: xsdschemas.Version,
	}}
	if profile == queue.ValidationProfileUBL21EN16931 || profile == queue.ValidationProfilePeppolBISBilling || profile == queue.ValidationProfilePeppolEnvelope {
		stages = append(stages, queue.FrozenValidationStage{
			ID: queue.ValidationLayerEN16931, Ordinal: 2,
			Requirement: queue.ValidationStageRequired, RulesetVersion: validationassets.EN16931().Version,
		})
	}
	if profile == queue.ValidationProfilePeppolBISBilling || profile == queue.ValidationProfilePeppolEnvelope {
		stages = append(stages, queue.FrozenValidationStage{
			ID: queue.ValidationLayerPeppolBISBilling, Ordinal: 3,
			Requirement: queue.ValidationStageRequired, RulesetVersion: validationassets.Peppol().ValidationArtefactsVersion,
		})
	}
	if profile == queue.ValidationProfilePeppolEnvelope {
		stages = append(stages, queue.FrozenValidationStage{
			ID: queue.ValidationLayerPeppolEnvelope, Ordinal: 4,
			Requirement: queue.ValidationStageRequired, RulesetVersion: validationassets.PeppolEnvelope().Version,
		})
	}
	return queue.FrozenValidationPlan{ID: string(profile), Version: splitValidationPlanVersion, Stages: stages}, nil
}

func startSplitValidation(ctx context.Context, client *queue.Client, record queue.UploadRecord, receivedAt time.Time) error {
	profile, plan, err := FrozenValidationPlanFor(record.DocumentType, record.Profile)
	if err != nil {
		return err
	}
	ctx = observability.RestoreTrace(ctx, record.EnqueueTrace)
	_, err = orchestrator.New(client).Start(ctx, queue.ValidationExecutionIntent{
		IntentID: record.ID, UploadID: record.ID, DocumentType: record.DocumentType,
		Trigger: queue.ValidationTriggerInitial, RequestedProfile: profile,
		Plan: plan, RetryPolicy: queue.DefaultFrozenValidationRetryPolicy(), CreatedAt: receivedAt,
	})
	return err
}

func deploymentMode(ctx context.Context, client *queue.Client) (queue.DeploymentMode, error) {
	current, err := client.DeploymentGeneration(ctx)
	if errors.Is(err, queue.ErrDeploymentGenerationNotProvisioned) {
		return queue.DeploymentModeLegacy, nil
	}
	if err != nil {
		return "", err
	}
	return current.State.Mode, nil
}

func dispatchAcceptedUpload(ctx context.Context, client *queue.Client, record queue.UploadRecord, receivedAt time.Time) error {
	mode, err := deploymentMode(ctx, client)
	if err != nil {
		return err
	}
	switch mode {
	case queue.DeploymentModeLegacy:
		return client.PublishDocument(ctx, record.ID, record.DocumentType, receivedAt)
	case queue.DeploymentModeDraining, queue.DeploymentModeActivating:
		return queue.ErrDeploymentDraining
	case queue.DeploymentModeSplit:
		return startSplitValidation(ctx, client, record, receivedAt)
	default:
		return fmt.Errorf("unsupported deployment mode %q", mode)
	}
}

// LegacyPendingMigrationReport accounts for every upload inspected during the
// one-time split activation reconciliation.
type LegacyPendingMigrationReport struct {
	Scanned, Started, Preserved, Deferred, AcceptanceFailed, Errors int
}

// MigrateLegacyPendingUploads re-runs every ambiguous active upload from the
// first stage of a newly frozen plan. Existing terminal records are immutable.
func MigrateLegacyPendingUploads(ctx context.Context, client *queue.Client) (LegacyPendingMigrationReport, error) {
	var report LegacyPendingMigrationReport
	keys, err := client.UploadRecordKeys(ctx)
	if err != nil {
		return report, err
	}
	for _, id := range keys {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		versioned, err := client.GetUploadRecordVersion(ctx, id)
		if err != nil {
			report.Errors++
			continue
		}
		record := versioned.Record
		report.Scanned++
		switch record.Status {
		case queue.StatusValid, queue.StatusInvalid, queue.StatusAcceptanceFailed, queue.StatusProcessingFailed:
			report.Preserved++
			continue
		case queue.StatusCleanupPending:
			report.Preserved++
			continue
		case queue.StatusDeadLetterPending:
			report.Errors++
			continue
		case queue.StatusAccepting, queue.StatusEnqueuePending, queue.StatusPending:
		default:
			report.Errors++
			continue
		}

		hasContent, contentErr := client.HasUploadContent(ctx, record.ID)
		if contentErr != nil {
			report.Errors++
			continue
		}
		if !hasContent {
			if record.Status == queue.StatusAccepting {
				// This may be an HTTP request between acceptance-record creation
				// and Object Store persistence. Never turn that live request into
				// a terminal failure; leave it untouched and block activation.
				report.Deferred++
				continue
			}
			if record.Status == queue.StatusPending {
				report.Errors++
				continue
			}
			record.Status = queue.StatusAcceptanceFailed
			record.Error = "upload content was not durably persisted before split migration"
			if _, err := client.UpdateUploadRecord(ctx, record, versioned.Revision); err != nil {
				report.Errors++
			} else {
				report.AcceptanceFailed++
			}
			continue
		}
		receivedAt, err := time.Parse(time.RFC3339, record.ReceivedAt)
		if err != nil {
			report.Errors++
			continue
		}
		profile, _, err := FrozenValidationPlanFor(record.DocumentType, record.Profile)
		if err != nil {
			report.Errors++
			continue
		}
		record.Profile = profile
		record.ResolvedProfile = profile
		if record.RequestedProfile == "" {
			record.RequestedProfile = profile
		}
		record.Status = queue.StatusPending
		record.Error = ""
		record.ValidatedAt = ""
		record.ValidationStages = nil
		record.ResultPending = false
		record.LastError = ""
		record.ProcessingAttempts = 0
		record.ProcessingFailedAt = ""
		if _, err := client.UpdateUploadRecord(ctx, record, versioned.Revision); err != nil {
			report.Errors++
			continue
		}
		if err := startSplitValidation(ctx, client, record, receivedAt); err != nil {
			report.Errors++
			continue
		}
		report.Started++
	}
	if report.Errors != 0 || report.Deferred != 0 {
		return report, fmt.Errorf("legacy pending reconciliation blocked by %d errors and %d in-flight accepting uploads", report.Errors, report.Deferred)
	}
	return report, nil
}
