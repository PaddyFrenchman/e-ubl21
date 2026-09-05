package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	peppolprofile "github.com/PaddyFrenchman/e-ubl21/peppol"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/svrl"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
)

const defaultPeppolValidationTimeout = 30 * time.Second

var (
	errPeppolSVRLOutputLimit = errors.New("Peppol SVRL output limit exceeded")
	errPeppolRuntimeAsset    = errors.New("Peppol runtime asset failure")
	errPeppolRuntimeSaxon    = errors.New("Peppol Saxon process failure")
	errPeppolRuntimeSVRL     = errors.New("Peppol SVRL failure")
	errPeppolRuntimeInput    = errors.New("Peppol input stream failure")
)

// PeppolValidator executes the pinned Peppol BIS Billing ruleset for one UBL
// document. Standards failures are successful calls with Valid=false. Errors
// mean validation could not complete or its output could not be trusted and
// must therefore be handled as operational failures by a stage worker.
type PeppolValidator interface {
	Validate(ctx context.Context, xml io.Reader) (queue.ValidationStageResult, error)
}

var _ PeppolValidator = (*saxonPeppolValidator)(nil)

type saxonPeppolValidator struct {
	runtime     saxonRuntimeConfig
	timeout     time.Duration
	waitDelay   time.Duration
	limits      svrl.Limits
	now         func() time.Time
	identifiers peppolprofile.Identifiers
}

// newSaxonPeppolValidator expects the common Saxon assets and the Peppol XSLT
// in runtime to have been checksum-verified at process startup. Identifiers
// are supplied by the separate profile resolver; the optional form keeps the
// adapter useful in isolated ruleset tests where identity is not under test.
func newSaxonPeppolValidator(runtime saxonRuntimeConfig, timeout time.Duration) *saxonPeppolValidator {
	return newSaxonPeppolValidatorWithIdentifiers(runtime, timeout, peppolprofile.Identifiers{})
}

func newSaxonPeppolValidatorWithIdentifiers(runtime saxonRuntimeConfig, timeout time.Duration, identifiers peppolprofile.Identifiers) *saxonPeppolValidator {
	if timeout <= 0 {
		timeout = defaultPeppolValidationTimeout
	}
	return &saxonPeppolValidator{
		runtime: runtime, timeout: timeout,
		waitDelay: defaultEN16931ProcessWaitDelay,
		limits:    svrl.DefaultLimits(), now: time.Now,
		identifiers: identifiers,
	}
}

func (validator *saxonPeppolValidator) Validate(ctx context.Context, xmlInput io.Reader) (queue.ValidationStageResult, error) {
	if validator == nil {
		return queue.ValidationStageResult{}, fmt.Errorf("Peppol validation: validator is nil")
	}
	if ctx == nil {
		return queue.ValidationStageResult{}, fmt.Errorf("Peppol validation: context is nil")
	}
	if xmlInput == nil {
		return queue.ValidationStageResult{}, fmt.Errorf("Peppol validation: XML reader is nil")
	}
	run, err := (&saxonSVRLRunner{
		runtime: validator.runtime, stylesheetName: "Peppol XSLT",
		layer: queue.ValidationLayerPeppolBISBilling, timeout: validator.timeout,
		waitDelay: validator.waitDelay, limits: validator.limits, now: validator.now,
	}).run(ctx, xmlInput)
	if err != nil {
		return queue.ValidationStageResult{}, classifyPeppolRuntimeError(err)
	}
	profile := validationassets.Peppol()
	return queue.ValidationStageResult{
		Layer:      queue.ValidationLayerPeppolBISBilling,
		Version:    profile.ValidationArtefactsVersion,
		BISVersion: profile.Version, BISReleaseDate: profile.PublicationDate,
		BISMandatoryFrom: profile.MandatoryFrom,
		CustomizationID:  validator.identifiers.CustomizationID,
		ProfileID:        validator.identifiers.ProfileID,
		ProcessProfile:   validator.identifiers.ProcessProfile,
		Valid:            !queue.StageIssuesContainStandardsFailure(run.report.Issues),
		Issues:           run.report.Issues,
		ValidatedAt:      run.completedAt.UTC().Format(time.RFC3339Nano),
		DurationMS:       run.completedAt.Sub(run.startedAt).Milliseconds(),
	}, nil
}

func classifyPeppolRuntimeError(err error) error {
	if errors.Is(err, errSaxonSVRLOutputLimit) {
		return fmt.Errorf("Peppol validation operational failure: %w: %w: %w", errPeppolRuntimeSVRL, errPeppolSVRLOutputLimit, err)
	}
	var category error
	switch {
	case errors.Is(err, errSaxonRuntimeAsset):
		category = errPeppolRuntimeAsset
	case errors.Is(err, errSaxonRuntimeProcess):
		category = errPeppolRuntimeSaxon
	case errors.Is(err, errSaxonRuntimeSVRL):
		category = errPeppolRuntimeSVRL
	case errors.Is(err, errSaxonRuntimeInput):
		category = errPeppolRuntimeInput
	default:
		return fmt.Errorf("Peppol validation operational failure: %w", err)
	}
	return fmt.Errorf("Peppol validation operational failure: %w: %w", category, err)
}
