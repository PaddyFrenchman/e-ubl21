package queue

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nats-io/nats.go/jetstream"
)

// ValidationExecutionResultID returns the deterministic permanent-result
// identity for one exact validation lineage generation.
func ValidationExecutionResultID(validationID string, generation uint64) (string, error) {
	if !ValidID(validationID) || generation == 0 {
		return "", fmt.Errorf("invalid validation result execution identity %q generation %d", validationID, generation)
	}
	digest := sha256.Sum256([]byte("e-ubl21.validation-result.v1\x00" + validationID + "\x00" + strconv.FormatUint(generation, 10)))
	return hex.EncodeToString(digest[:16]), nil
}

// ValidateExecutionResult validates the execution-scoped canonical result
// contract. Legacy upload-scoped results intentionally do not use this method.
func (result ValidationResult) ValidateExecutionResult() error {
	if !ValidID(result.ID) {
		return fmt.Errorf("invalid validation result upload id %q", result.ID)
	}
	wantResultID, err := ValidationExecutionResultID(result.ValidationID, result.Generation)
	if err != nil {
		return err
	}
	if result.ResultID != wantResultID {
		return fmt.Errorf("validation result id %q does not match deterministic id %q", result.ResultID, wantResultID)
	}
	if !validationDocumentTypePattern.MatchString(result.DocumentType) {
		return fmt.Errorf("invalid validation result document type %q", result.DocumentType)
	}
	if !ValidValidationProfile(result.Profile) {
		return fmt.Errorf("invalid validation result profile %q", result.Profile)
	}
	plan := FrozenValidationPlan{ID: result.Plan, Version: result.PlanVersion, Stages: result.PlanStages}
	if err := plan.validate(); err != nil {
		return err
	}
	if _, err := parseCanonicalStageTime("validation result validatedAt", result.ValidatedAt); err != nil {
		return err
	}
	if !utf8.ValidString(result.Error) || len(result.Error) > MaxStageDiagnosticBytes {
		return fmt.Errorf("validation result error exceeds %d bytes or is not UTF-8", MaxStageDiagnosticBytes)
	}
	if len(result.ValidationStages) == 0 {
		return errors.New("validation result requires terminal stage evidence")
	}
	seen := make(map[ValidationLayer]struct{}, len(result.ValidationStages))
	for index, stage := range result.ValidationStages {
		if index >= len(result.PlanStages) || stage.Layer != result.PlanStages[index].ID || stage.Version != result.PlanStages[index].RulesetVersion {
			return fmt.Errorf("validation result stage %d does not match frozen plan", index)
		}
		if _, duplicate := seen[stage.Layer]; duplicate {
			return fmt.Errorf("validation result stage %d duplicates layer %q", index, stage.Layer)
		}
		seen[stage.Layer] = struct{}{}
		if err := validateStageVersion("validation result stage version", stage.Version); err != nil {
			return fmt.Errorf("validation result stage %d: %w", index, err)
		}
		if _, err := parseCanonicalStageTime("validation result stage validatedAt", stage.ValidatedAt); err != nil {
			return fmt.Errorf("validation result stage %d: %w", index, err)
		}
		if stage.DurationMS < 0 {
			return fmt.Errorf("validation result stage %d durationMs must not be negative", index)
		}
		if !utf8.ValidString(stage.Diagnostic) || len(stage.Diagnostic) > MaxStageDiagnosticBytes {
			return fmt.Errorf("validation result stage %d diagnostic exceeds %d bytes or is not UTF-8", index, MaxStageDiagnosticBytes)
		}
		if err := validateValidationStageIdentity(stage); err != nil {
			return fmt.Errorf("validation result stage %d: %w", index, err)
		}
		if err := ValidateStageIssues(stage.Layer, stage.Issues); err != nil {
			return fmt.Errorf("validation result stage %d: %w", index, err)
		}
		switch stage.Outcome {
		case StageOutcomePassed:
			if !stage.Valid || stage.Reason != StageReasonNone || StageIssuesContainStandardsFailure(stage.Issues) {
				return fmt.Errorf("validation result stage %d has inconsistent passed evidence", index)
			}
		case StageOutcomeStandardsInvalid:
			if stage.Valid || stage.Reason != StageReasonNone || (!StageIssuesContainStandardsFailure(stage.Issues) && strings.TrimSpace(stage.Diagnostic) == "") {
				return fmt.Errorf("validation result stage %d has inconsistent standards-invalid evidence", index)
			}
		case StageOutcomeOperationalFailure:
			if stage.Valid || !validStageOperationalReason(stage.Reason) || len(stage.Issues) != 0 {
				return fmt.Errorf("validation result stage %d has inconsistent operational-failure evidence", index)
			}
		default:
			return fmt.Errorf("validation result stage %d has invalid outcome %q", index, stage.Outcome)
		}
		if index < len(result.ValidationStages)-1 && stage.Outcome != StageOutcomePassed {
			return fmt.Errorf("validation result advanced beyond non-passing stage %d", index)
		}
	}
	last := result.ValidationStages[len(result.ValidationStages)-1]
	switch result.Status {
	case StatusValid:
		if strings.TrimSpace(result.Error) != "" {
			return errors.New("valid validation result cannot contain an error")
		}
		for _, stage := range result.ValidationStages {
			if stage.Outcome != StageOutcomePassed {
				return errors.New("valid validation result requires every persisted stage to pass")
			}
		}
		if len(result.ValidationStages) != len(result.PlanStages) {
			return errors.New("valid validation result requires every frozen-plan stage")
		}
	case StatusInvalid:
		if last.Outcome != StageOutcomeStandardsInvalid {
			return errors.New("invalid validation result requires terminal standards-invalid evidence")
		}
	case StatusProcessingFailed:
		if last.Outcome != StageOutcomeOperationalFailure || strings.TrimSpace(result.Error) == "" {
			return errors.New("processing-failed validation result requires explicit operational evidence")
		}
	default:
		return fmt.Errorf("validation result has non-terminal status %q", result.Status)
	}
	return nil
}

const maxPeppolIdentifierBytes = 1024

func validateValidationStageIdentity(stage ValidationStageResult) error {
	hasPeppolIdentity := stage.BISVersion != "" || stage.BISReleaseDate != "" ||
		stage.BISMandatoryFrom != "" || stage.CustomizationID != "" ||
		stage.ProfileID != "" || stage.ProcessProfile != ""
	if stage.Layer != ValidationLayerPeppolBISBilling {
		if hasPeppolIdentity {
			return errors.New("non-Peppol validation layer cannot contain Peppol identity")
		}
		return nil
	}
	if err := validateStageVersion("Peppol BIS version", stage.BISVersion); err != nil {
		return err
	}
	if stage.BISVersion != strings.TrimSpace(stage.BISVersion) {
		return errors.New("Peppol BIS version is not normalized")
	}
	releaseDate, err := parsePeppolDate("Peppol BIS release date", stage.BISReleaseDate)
	if err != nil {
		return err
	}
	mandatoryFrom, err := parsePeppolDate("Peppol BIS mandatory-from date", stage.BISMandatoryFrom)
	if err != nil {
		return err
	}
	if mandatoryFrom.Before(releaseDate) {
		return errors.New("Peppol BIS mandatory-from date precedes release date")
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "customizationId", value: stage.CustomizationID},
		{name: "profileId", value: stage.ProfileID},
	} {
		name, value := field.name, field.value
		if !utf8.ValidString(value) || len(value) > maxPeppolIdentifierBytes {
			return fmt.Errorf("Peppol %s exceeds %d bytes or is not UTF-8", name, maxPeppolIdentifierBytes)
		}
		if value != strings.TrimSpace(value) {
			return fmt.Errorf("Peppol %s is not normalized", name)
		}
	}
	switch stage.ProcessProfile {
	case "", PeppolProcessProfile01, PeppolProcessProfile02:
	default:
		return fmt.Errorf("invalid Peppol process profile %q", stage.ProcessProfile)
	}
	if stage.ProcessProfile != "" && stage.ProfileID == "" {
		return errors.New("resolved Peppol process profile requires profileId")
	}
	if stage.Outcome == StageOutcomePassed &&
		(stage.CustomizationID == "" || stage.ProfileID == "" || stage.ProcessProfile == "") {
		return errors.New("passed Peppol stage requires resolved customization, profile, and process identifiers")
	}
	return nil
}

func parsePeppolDate(name, value string) (time.Time, error) {
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil || value != parsed.Format(time.DateOnly) {
		return time.Time{}, fmt.Errorf("%s %q is not a canonical date", name, value)
	}
	return parsed, nil
}

// SemanticallyEqualValidationResults compares complete canonical payloads.
func SemanticallyEqualValidationResults(left, right ValidationResult) bool {
	return reflect.DeepEqual(left, right)
}

// HasValidationExecutionResult reports whether the permanent first-result
// slot for one exact validation execution is occupied.
func (c *Client) HasValidationExecutionResult(ctx context.Context, validationID string, generation uint64) (bool, error) {
	resultID, err := ValidationExecutionResultID(validationID, generation)
	if err != nil {
		return false, err
	}
	stream, err := c.JS.Stream(ctx, c.Namespace.ResultsStreamName())
	if err != nil {
		return false, fmt.Errorf("open validation results stream: %w", err)
	}
	if _, err := stream.GetLastMsgForSubject(ctx, c.Namespace.ResultSubject(resultID)); err != nil {
		if errors.Is(err, jetstream.ErrMsgNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("inspect validation result %s: %w", resultID, err)
	}
	return true, nil
}

// GetValidationExecutionResult retrieves and validates one exact permanent
// execution result.
func (c *Client) GetValidationExecutionResult(ctx context.Context, validationID string, generation uint64) (ValidationResult, error) {
	resultID, err := ValidationExecutionResultID(validationID, generation)
	if err != nil {
		return ValidationResult{}, err
	}
	stream, err := c.JS.Stream(ctx, c.Namespace.ResultsStreamName())
	if err != nil {
		return ValidationResult{}, fmt.Errorf("open validation results stream: %w", err)
	}
	message, err := stream.GetLastMsgForSubject(ctx, c.Namespace.ResultSubject(resultID))
	if err != nil {
		return ValidationResult{}, fmt.Errorf("get validation result %s: %w", resultID, err)
	}
	var result ValidationResult
	if err := json.Unmarshal(message.Data, &result); err != nil {
		return ValidationResult{}, fmt.Errorf("decode validation result %s: %w", resultID, err)
	}
	if err := result.ValidateExecutionResult(); err != nil {
		return ValidationResult{}, fmt.Errorf("validate stored validation result %s: %w", resultID, err)
	}
	if result.ResultID != resultID || result.ValidationID != validationID || result.Generation != generation {
		return ValidationResult{}, errors.New("stored validation result identity does not match subject")
	}
	return result, nil
}
