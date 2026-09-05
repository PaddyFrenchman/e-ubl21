package server

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/svrl"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
)

const (
	defaultEN16931ValidationTimeout = 30 * time.Second
	defaultEN16931ProcessWaitDelay  = time.Second
)

var (
	errEN16931SVRLOutputLimit = errSaxonSVRLOutputLimit
	errEN16931RuntimeAsset    = errSaxonRuntimeAsset
	errEN16931RuntimeSaxon    = errSaxonRuntimeProcess
	errEN16931RuntimeSVRL     = errSaxonRuntimeSVRL
	errEN16931RuntimeInput    = errSaxonRuntimeInput
)

// EN16931Validator executes the pinned semantic ruleset for one UBL document.
// A non-compliant document is returned as a successful result with Valid=false;
// errors are reserved for failures to complete or interpret the validation.
// The caller retains ownership of xml and must close it after Validate returns
// when the concrete reader implements io.Closer.
type EN16931Validator interface {
	Validate(ctx context.Context, xml io.Reader) (queue.ValidationStageResult, error)
}

// saxonEN16931Validator is intentionally independent of the worker pipeline.
// TODO-EN-16931-007 composes it after the UBL XSD stage.
type saxonEN16931Validator struct {
	runtime   saxonRuntimeConfig
	timeout   time.Duration
	waitDelay time.Duration
	limits    svrl.Limits
	now       func() time.Time
}

// newSaxonEN16931Validator expects runtime to have passed ensureSaxonRuntime
// during worker startup. That check pins the hashes of all four runtime assets;
// per-validation checks below only defend against assets becoming unreadable.
func newSaxonEN16931Validator(runtime saxonRuntimeConfig, timeout time.Duration) *saxonEN16931Validator {
	if timeout <= 0 {
		timeout = defaultEN16931ValidationTimeout
	}
	return &saxonEN16931Validator{
		runtime:   runtime,
		timeout:   timeout,
		waitDelay: defaultEN16931ProcessWaitDelay,
		limits:    svrl.DefaultLimits(),
		now:       time.Now,
	}
}

func (validator *saxonEN16931Validator) Validate(ctx context.Context, xmlInput io.Reader) (queue.ValidationStageResult, error) {
	if validator == nil {
		return queue.ValidationStageResult{}, fmt.Errorf("EN 16931 validation: validator is nil")
	}
	if ctx == nil {
		return queue.ValidationStageResult{}, fmt.Errorf("EN 16931 validation: context is nil")
	}
	if xmlInput == nil {
		return queue.ValidationStageResult{}, fmt.Errorf("EN 16931 validation: XML reader is nil")
	}
	if validator.limits.MaxInputBytes <= 0 || validator.limits.MaxInputBytes > int64(maxInt()) {
		return queue.ValidationStageResult{}, fmt.Errorf("%w: invalid SVRL output limit %d", errEN16931RuntimeSVRL, validator.limits.MaxInputBytes)
	}

	run, err := (&saxonSVRLRunner{
		runtime: validator.runtime, stylesheetName: "EN 16931 XSLT",
		layer: queue.ValidationLayerEN16931, timeout: validator.timeout,
		waitDelay: validator.waitDelay, limits: validator.limits, now: validator.now,
	}).run(ctx, xmlInput)
	if err != nil {
		return queue.ValidationStageResult{}, fmt.Errorf("EN 16931 validation: %w", err)
	}
	valid := true
	for _, issue := range run.report.Issues {
		if issue.Kind == queue.ValidationIssueFailedAssert && en16931SeverityInvalid(issue.Severity) {
			valid = false
		}
	}

	return queue.ValidationStageResult{
		Layer:       queue.ValidationLayerEN16931,
		Version:     validationassets.EN16931().Version,
		Valid:       valid,
		Issues:      run.report.Issues,
		ValidatedAt: run.completedAt.UTC().Format(time.RFC3339Nano),
		DurationMS:  run.completedAt.Sub(run.startedAt).Milliseconds(),
	}, nil
}
