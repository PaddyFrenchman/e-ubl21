package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	xsdschemas "github.com/PaddyFrenchman/e-ubl21/XSD"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
)

// validationContentSource reopens immutable upload content for each stage.
// This avoids retaining a full document in memory and gives every validator an
// independent forward-only stream.
type validationContentSource interface {
	OpenUploadContent(context.Context, string) (queue.UploadContentReader, error)
}

type xsdStreamValidator func(context.Context, string, io.Reader, time.Duration) error

var errValidationContentStream = errors.New("validation content stream failed")

type documentValidationResult struct {
	Status      queue.ValidationStatus
	Error       string
	Stages      queue.ValidationStages
	ValidatedAt time.Time
}

// documentValidationPipeline owns mandatory stage ordering and short-circuit
// policy. Invoice and CreditNote can never succeed without an EN validator.
type documentValidationPipeline struct {
	content        validationContentSource
	en16931        EN16931Validator
	xsdValidate    xsdStreamValidator
	overallTimeout time.Duration
	xsdTimeout     time.Duration
	now            func() time.Time
	logger         *slog.Logger
}

func newDocumentValidationPipeline(content validationContentSource, en16931 EN16931Validator, timeout time.Duration) *documentValidationPipeline {
	return newDocumentValidationPipelineWithTimeouts(content, en16931, timeout, timeout)
}

func newDocumentValidationPipelineWithTimeouts(content validationContentSource, en16931 EN16931Validator, overallTimeout, xsdTimeout time.Duration) *documentValidationPipeline {
	if overallTimeout <= 0 {
		overallTimeout = defaultValidationTimeout
	}
	if xsdTimeout <= 0 {
		xsdTimeout = overallTimeout
	}
	return &documentValidationPipeline{
		content:        content,
		en16931:        en16931,
		xsdValidate:    validateReaderAgainstSchema,
		overallTimeout: overallTimeout,
		xsdTimeout:     xsdTimeout,
		now:            time.Now,
	}
}

func (pipeline *documentValidationPipeline) Validate(ctx context.Context, uploadID, documentType, schemaPath string) (documentValidationResult, error) {
	return pipeline.ValidateProfile(ctx, uploadID, documentType, "", schemaPath)
}

// ValidateProfile executes the explicitly selected durable profile. An empty
// profile preserves the legacy document-type-derived behavior.
func (pipeline *documentValidationPipeline) ValidateProfile(ctx context.Context, uploadID, documentType string, profile queue.ValidationProfile, schemaPath string) (documentValidationResult, error) {
	if pipeline == nil || pipeline.content == nil || pipeline.xsdValidate == nil {
		return documentValidationResult{}, fmt.Errorf("validation pipeline is not configured")
	}
	if ctx == nil {
		return documentValidationResult{}, fmt.Errorf("validation pipeline context is nil")
	}
	validationCtx, cancel := context.WithTimeout(ctx, pipeline.overallTimeout)
	defer cancel()

	xsdStarted := pipeline.now()
	xsdErr := pipeline.withContent(validationCtx, uploadID, func(reader io.Reader) error {
		return pipeline.xsdValidate(validationCtx, schemaPath, reader, pipeline.xsdTimeout)
	})
	xsdCompleted := pipeline.now()
	xsdStage := queue.ValidationStageResult{
		Layer:       queue.ValidationLayerUBL21XSD,
		Version:     xsdschemas.Version,
		Valid:       xsdErr == nil,
		ValidatedAt: xsdCompleted.UTC().Format(time.RFC3339Nano),
		DurationMS:  xsdCompleted.Sub(xsdStarted).Milliseconds(),
	}
	if xsdErr != nil {
		if xsdOperationalFailure(validationCtx, xsdErr) {
			if ctx.Err() == nil {
				pipeline.observeStage(uploadID, documentType, xsdStage, "error", xsdCompleted.Sub(xsdStarted), xsdErr)
			}
			return documentValidationResult{}, fmt.Errorf("UBL 2.1 XSD stage: %w", xsdErr)
		}
		xsdStage.Issues = []queue.ValidationIssue{{
			Kind:     queue.ValidationIssueXSDReport,
			Layer:    queue.ValidationLayerUBL21XSD,
			Severity: "error",
			Message:  xsdErr.Error(),
		}}
		pipeline.observeStage(uploadID, documentType, xsdStage, "invalid", xsdCompleted.Sub(xsdStarted), nil)
		return documentValidationResult{
			Status:      queue.StatusInvalid,
			Error:       xsdErr.Error(),
			Stages:      queue.ValidationStages{xsdStage},
			ValidatedAt: xsdCompleted,
		}, nil
	}
	pipeline.observeStage(uploadID, documentType, xsdStage, "valid", xsdCompleted.Sub(xsdStarted), nil)

	stages := queue.ValidationStages{xsdStage}
	withEN16931 := requiresEN16931(documentType)
	if profile != "" {
		withEN16931 = profile == queue.ValidationProfileUBL21EN16931
	}
	if withEN16931 {
		enStarted := pipeline.now()
		if pipeline.en16931 == nil {
			enErr := fmt.Errorf("%w: EN 16931 validator is required for %s", errEN16931RuntimeAsset, documentType)
			pipeline.observeStage(uploadID, documentType, queue.ValidationStageResult{
				Layer: queue.ValidationLayerEN16931, Version: validationassets.EN16931().Version,
			}, "error", pipeline.now().Sub(enStarted), enErr)
			return documentValidationResult{}, enErr
		}
		var enStage queue.ValidationStageResult
		enErr := pipeline.withContent(validationCtx, uploadID, func(reader io.Reader) error {
			var err error
			enStage, err = pipeline.en16931.Validate(validationCtx, reader)
			return err
		})
		enCompleted := pipeline.now()
		if enErr != nil {
			if ctx.Err() == nil {
				pipeline.observeStage(uploadID, documentType, queue.ValidationStageResult{
					Layer: queue.ValidationLayerEN16931, Version: validationassets.EN16931().Version,
				}, "error", enCompleted.Sub(enStarted), enErr)
			}
			return documentValidationResult{}, fmt.Errorf("EN 16931 stage: %w", enErr)
		}
		enOutcome := "valid"
		if !enStage.Valid {
			enOutcome = "invalid"
		}
		pipeline.observeStage(uploadID, documentType, enStage, enOutcome, enCompleted.Sub(enStarted), nil)
		stages = append(stages, enStage)
		if !enStage.Valid {
			return documentValidationResult{
				Status:      queue.StatusInvalid,
				Error:       en16931InvalidMessage(enStage),
				Stages:      stages,
				ValidatedAt: pipeline.now(),
			}, nil
		}
	}

	return documentValidationResult{
		Status:      queue.StatusValid,
		Stages:      stages,
		ValidatedAt: pipeline.now(),
	}, nil
}

func (pipeline *documentValidationPipeline) withContent(ctx context.Context, uploadID string, validate func(io.Reader) error) error {
	content, err := pipeline.content.OpenUploadContent(ctx, uploadID)
	if err != nil {
		return fmt.Errorf("%w: open upload content: %w", errValidationContentStream, err)
	}
	validationErr := validate(content)
	streamErr := content.Error()
	closeErr := content.Close()
	if streamErr == nil {
		streamErr = closeErr
	}
	if streamErr != nil {
		contentErr := fmt.Errorf("%w: stream upload content: %w", errValidationContentStream, streamErr)
		if validationErr != nil {
			return errors.Join(validationErr, contentErr)
		}
		return contentErr
	}
	return validationErr
}

func xsdOperationalFailure(ctx context.Context, err error) bool {
	if ctx.Err() != nil || errors.Is(err, errValidationContentStream) {
		return true
	}
	return !errors.Is(err, errSchemaStandardsInvalid)
}

func en16931InvalidMessage(stage queue.ValidationStageResult) string {
	for _, issue := range stage.Issues {
		if issue.Kind != queue.ValidationIssueFailedAssert || !en16931SeverityInvalid(issue.Severity) {
			continue
		}
		switch {
		case issue.Rule != "" && issue.Message != "":
			return fmt.Sprintf("EN 16931 rule %s failed: %s", issue.Rule, issue.Message)
		case issue.Rule != "":
			return fmt.Sprintf("EN 16931 rule %s failed", issue.Rule)
		case issue.Message != "":
			return "EN 16931 validation failed: " + issue.Message
		}
	}
	return "EN 16931 validation failed"
}
