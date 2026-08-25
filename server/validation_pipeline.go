package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
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
// policy. A nil EN 16931 validator is supported only for direct-library staged
// rollout mode; the production worker CLI always supplies the verified runtime.
type documentValidationPipeline struct {
	content     validationContentSource
	en16931     EN16931Validator
	xsdValidate xsdStreamValidator
	timeout     time.Duration
	now         func() time.Time
}

func newDocumentValidationPipeline(content validationContentSource, en16931 EN16931Validator, timeout time.Duration) *documentValidationPipeline {
	if timeout <= 0 {
		timeout = defaultValidationTimeout
	}
	return &documentValidationPipeline{
		content:     content,
		en16931:     en16931,
		xsdValidate: validateReaderAgainstSchema,
		timeout:     timeout,
		now:         time.Now,
	}
}

func (pipeline *documentValidationPipeline) Validate(ctx context.Context, uploadID, documentType, schemaPath string) (documentValidationResult, error) {
	if pipeline == nil || pipeline.content == nil || pipeline.xsdValidate == nil {
		return documentValidationResult{}, fmt.Errorf("validation pipeline is not configured")
	}
	if ctx == nil {
		return documentValidationResult{}, fmt.Errorf("validation pipeline context is nil")
	}
	validationCtx, cancel := context.WithTimeout(ctx, pipeline.timeout)
	defer cancel()

	xsdStarted := pipeline.now()
	xsdErr := pipeline.withContent(validationCtx, uploadID, func(reader io.Reader) error {
		return pipeline.xsdValidate(validationCtx, schemaPath, reader, pipeline.timeout)
	})
	xsdCompleted := pipeline.now()
	xsdStage := queue.ValidationStageResult{
		Layer:       queue.ValidationLayerUBL21XSD,
		Version:     "2.1",
		Valid:       xsdErr == nil,
		ValidatedAt: xsdCompleted.UTC().Format(time.RFC3339Nano),
		DurationMS:  xsdCompleted.Sub(xsdStarted).Milliseconds(),
	}
	if xsdErr != nil {
		if xsdOperationalFailure(validationCtx, xsdErr) {
			return documentValidationResult{}, fmt.Errorf("UBL 2.1 XSD stage: %w", xsdErr)
		}
		xsdStage.Issues = []queue.ValidationIssue{{
			Kind:     queue.ValidationIssueXSDReport,
			Layer:    queue.ValidationLayerUBL21XSD,
			Severity: "error",
			Message:  xsdErr.Error(),
		}}
		return documentValidationResult{
			Status:      queue.StatusInvalid,
			Error:       xsdErr.Error(),
			Stages:      queue.ValidationStages{xsdStage},
			ValidatedAt: xsdCompleted,
		}, nil
	}

	stages := queue.ValidationStages{xsdStage}
	if requiresEN16931(documentType) && pipeline.en16931 != nil {
		var enStage queue.ValidationStageResult
		enErr := pipeline.withContent(validationCtx, uploadID, func(reader io.Reader) error {
			var err error
			enStage, err = pipeline.en16931.Validate(validationCtx, reader)
			return err
		})
		if enErr != nil {
			return documentValidationResult{}, fmt.Errorf("EN 16931 stage: %w", enErr)
		}
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
		return fmt.Errorf("%w: open upload content: %v", errValidationContentStream, err)
	}
	validationErr := validate(content)
	streamErr := content.Error()
	closeErr := content.Close()
	if streamErr == nil {
		streamErr = closeErr
	}
	if streamErr != nil {
		contentErr := fmt.Errorf("%w: stream upload content: %v", errValidationContentStream, streamErr)
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
