package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
	errEN16931SVRLOutputLimit = errors.New("EN 16931 SVRL output limit exceeded")
	errEN16931RuntimeAsset    = errors.New("EN 16931 runtime asset failure")
	errEN16931RuntimeSaxon    = errors.New("EN 16931 Saxon process failure")
	errEN16931RuntimeSVRL     = errors.New("EN 16931 SVRL failure")
	errEN16931RuntimeInput    = errors.New("EN 16931 input stream failure")
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

	javaPath, err := exec.LookPath(validator.runtime.javaExecutable)
	if err != nil {
		return queue.ValidationStageResult{}, fmt.Errorf("%w: Java executable %q is unavailable: %v", errEN16931RuntimeAsset, validator.runtime.javaExecutable, err)
	}
	for _, asset := range []struct {
		name string
		path string
	}{
		{"Saxon-HE JAR", validator.runtime.saxonJARPath},
		{"xmlresolver JAR", validator.runtime.xmlResolverJARPath},
		{"Saxon configuration", validator.runtime.configurationPath},
		{"EN 16931 XSLT", validator.runtime.xsltPath},
	} {
		if err := requireReadableRegularFile(asset.path); err != nil {
			return queue.ValidationStageResult{}, fmt.Errorf("%w: read %s: %v", errEN16931RuntimeAsset, asset.name, err)
		}
	}

	startedAt := validator.now()
	validationCtx, cancel := context.WithTimeout(ctx, validator.timeout)
	defer cancel()
	waitDelay := validator.waitDelay
	if waitDelay <= 0 {
		waitDelay = defaultEN16931ProcessWaitDelay
	}

	classpath := validator.runtime.saxonJARPath + string(os.PathListSeparator) + validator.runtime.xmlResolverJARPath
	cmd := exec.CommandContext(validationCtx, javaPath,
		"-cp", classpath,
		saxonTransformClass,
		"-config:"+validator.runtime.configurationPath,
		"-ext:off",
		"-s:-",
		"-xsl:"+validator.runtime.xsltPath,
	)
	output := &boundedCommandOutput{limit: int(validator.limits.MaxInputBytes)}
	diagnostics := &boundedDiagnosticBuffer{}
	cmd.Stdout = output
	cmd.Stderr = diagnostics
	cmd.WaitDelay = waitDelay

	// Do not assign an arbitrary io.Reader directly to Cmd.Stdin. os/exec would
	// then own a copy goroutine and Wait could remain blocked forever inside the
	// reader even after CommandContext killed Saxon. An explicit pipe keeps that
	// goroutine outside Cmd.Wait, so the configured timeout bounds Validate even
	// if the source reader never returns. Closing a caller-owned reader remains
	// the caller's responsibility.
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return queue.ValidationStageResult{}, fmt.Errorf("%w: create Saxon stdin: %v", errEN16931RuntimeSaxon, err)
	}
	if err := cmd.Start(); err != nil {
		stdin.Close()
		return queue.ValidationStageResult{}, fmt.Errorf("%w: start Saxon transform: %v", errEN16931RuntimeSaxon, err)
	}
	stdinCopyDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(stdin, xmlInput)
		closeErr := stdin.Close()
		if copyErr == nil {
			copyErr = closeErr
		}
		stdinCopyDone <- copyErr
	}()

	commandErr := cmd.Wait()
	stdin.Close()
	if output.exceeded {
		return queue.ValidationStageResult{}, fmt.Errorf("%w: %w: maximum is %d bytes", errEN16931RuntimeSVRL, errEN16931SVRLOutputLimit, validator.limits.MaxInputBytes)
	}
	if errors.Is(validationCtx.Err(), context.DeadlineExceeded) {
		return queue.ValidationStageResult{}, fmt.Errorf("EN 16931 validation timed out after %s: %w", validator.timeout, validationCtx.Err())
	}
	if errors.Is(validationCtx.Err(), context.Canceled) {
		return queue.ValidationStageResult{}, fmt.Errorf("EN 16931 validation canceled: %w", validationCtx.Err())
	}
	if commandErr != nil {
		return queue.ValidationStageResult{}, fmt.Errorf("%w: EN 16931 Saxon transform failed: %v: %s", errEN16931RuntimeSaxon, commandErr, diagnostics.String())
	}
	var stdinCopyErr error
	select {
	case stdinCopyErr = <-stdinCopyDone:
	case <-time.After(waitDelay):
		return queue.ValidationStageResult{}, fmt.Errorf("%w: XML input reader did not finish after Saxon exited within %s", errEN16931RuntimeInput, waitDelay)
	}
	if stdinCopyErr != nil {
		return queue.ValidationStageResult{}, fmt.Errorf("%w: stream XML to Saxon: %w", errEN16931RuntimeInput, stdinCopyErr)
	}

	report, err := svrl.ParseWithLimits(bytes.NewReader(output.data), validator.limits)
	if err != nil {
		return queue.ValidationStageResult{}, fmt.Errorf("%w: parse Saxon SVRL: %w", errEN16931RuntimeSVRL, err)
	}
	valid := true
	for _, issue := range report.Issues {
		if issue.Kind == queue.ValidationIssueFailedAssert && en16931SeverityInvalid(issue.Severity) {
			valid = false
		}
	}

	completedAt := validator.now()
	return queue.ValidationStageResult{
		Layer:       queue.ValidationLayerEN16931,
		Version:     validationassets.EN16931().Version,
		Valid:       valid,
		Issues:      report.Issues,
		ValidatedAt: completedAt.UTC().Format(time.RFC3339Nano),
		DurationMS:  completedAt.Sub(startedAt).Milliseconds(),
	}, nil
}

func requireReadableRegularFile(path string) error {
	if path == "" {
		return fmt.Errorf("path is empty")
	}
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}
	return nil
}

// boundedCommandOutput deliberately returns an error once the maximum is
// crossed. os/exec then closes the child's stdout pipe instead of continuing
// to drain an attacker-controlled report into memory.
type boundedCommandOutput struct {
	data     []byte
	limit    int
	exceeded bool
}

func (output *boundedCommandOutput) Write(p []byte) (int, error) {
	remaining := output.limit - len(output.data)
	if remaining <= 0 {
		output.exceeded = true
		return 0, errEN16931SVRLOutputLimit
	}
	if len(p) > remaining {
		output.data = append(output.data, p[:remaining]...)
		output.exceeded = true
		return remaining, errEN16931SVRLOutputLimit
	}
	output.data = append(output.data, p...)
	return len(p), nil
}

func maxInt() int {
	return int(^uint(0) >> 1)
}
