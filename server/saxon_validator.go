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
)

var (
	errSaxonSVRLOutputLimit = errors.New("Saxon SVRL output limit exceeded")
	errSaxonRuntimeAsset    = errors.New("Saxon runtime asset failure")
	errSaxonRuntimeProcess  = errors.New("Saxon process failure")
	errSaxonRuntimeSVRL     = errors.New("Saxon SVRL failure")
	errSaxonRuntimeInput    = errors.New("Saxon input stream failure")
)

// saxonSVRLRunner is the common, security-sensitive process boundary shared
// by semantic validators. Profile adapters supply only their pinned stylesheet,
// result layer and policy; process control and resource bounds stay identical.
type saxonSVRLRunner struct {
	runtime        saxonRuntimeConfig
	stylesheetName string
	layer          queue.ValidationLayer
	timeout        time.Duration
	waitDelay      time.Duration
	limits         svrl.Limits
	now            func() time.Time
}

type saxonSVRLRun struct {
	report      svrl.Report
	startedAt   time.Time
	completedAt time.Time
}

func (runner *saxonSVRLRunner) run(ctx context.Context, xmlInput io.Reader) (saxonSVRLRun, error) {
	if err := ctx.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return saxonSVRLRun{}, fmt.Errorf("validation timed out before Saxon started: %w", err)
		}
		return saxonSVRLRun{}, fmt.Errorf("validation canceled before Saxon started: %w", err)
	}
	if runner.limits.MaxInputBytes <= 0 || runner.limits.MaxInputBytes > int64(maxInt()) {
		return saxonSVRLRun{}, fmt.Errorf("%w: invalid SVRL output limit %d", errSaxonRuntimeSVRL, runner.limits.MaxInputBytes)
	}
	javaPath, err := exec.LookPath(runner.runtime.javaExecutable)
	if err != nil {
		return saxonSVRLRun{}, fmt.Errorf("%w: Java executable %q is unavailable: %v", errSaxonRuntimeAsset, runner.runtime.javaExecutable, err)
	}
	for _, asset := range []struct{ name, path string }{
		{"Saxon-HE JAR", runner.runtime.saxonJARPath},
		{"xmlresolver JAR", runner.runtime.xmlResolverJARPath},
		{"Saxon configuration", runner.runtime.configurationPath},
		{runner.stylesheetName, runner.runtime.xsltPath},
	} {
		if err := requireReadableRegularFile(asset.path); err != nil {
			return saxonSVRLRun{}, fmt.Errorf("%w: read %s: %v", errSaxonRuntimeAsset, asset.name, err)
		}
	}

	startedAt := runner.now()
	validationCtx, cancel := context.WithTimeout(ctx, runner.timeout)
	defer cancel()
	waitDelay := runner.waitDelay
	if waitDelay <= 0 {
		waitDelay = defaultEN16931ProcessWaitDelay
	}
	classpath := runner.runtime.saxonJARPath + string(os.PathListSeparator) + runner.runtime.xmlResolverJARPath
	cmd := exec.CommandContext(validationCtx, javaPath,
		"-cp", classpath, saxonTransformClass,
		"-config:"+runner.runtime.configurationPath, "-ext:off", "-s:-", "-xsl:"+runner.runtime.xsltPath,
	)
	output := &boundedCommandOutput{limit: int(runner.limits.MaxInputBytes)}
	diagnostics := &boundedDiagnosticBuffer{}
	cmd.Stdout, cmd.Stderr, cmd.WaitDelay = output, diagnostics, waitDelay

	// Keep the input copy outside Cmd.Wait. CommandContext can then enforce a
	// hard bound even when a caller-provided reader never returns.
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return saxonSVRLRun{}, fmt.Errorf("%w: create Saxon stdin: %v", errSaxonRuntimeProcess, err)
	}
	if err := cmd.Start(); err != nil {
		stdin.Close()
		if validationCtx.Err() != nil {
			return saxonSVRLRun{}, fmt.Errorf("validation canceled before Saxon started: %w", validationCtx.Err())
		}
		return saxonSVRLRun{}, fmt.Errorf("%w: start Saxon transform: %v", errSaxonRuntimeProcess, err)
	}
	stdinCopyDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(stdin, xmlInput)
		if closeErr := stdin.Close(); copyErr == nil {
			copyErr = closeErr
		}
		stdinCopyDone <- copyErr
	}()

	commandErr := cmd.Wait()
	stdin.Close()
	if output.exceeded {
		return saxonSVRLRun{}, fmt.Errorf("%w: %w: maximum is %d bytes", errSaxonRuntimeSVRL, errSaxonSVRLOutputLimit, runner.limits.MaxInputBytes)
	}
	if errors.Is(validationCtx.Err(), context.DeadlineExceeded) {
		return saxonSVRLRun{}, fmt.Errorf("validation timed out after %s: %w", runner.timeout, validationCtx.Err())
	}
	if errors.Is(validationCtx.Err(), context.Canceled) {
		return saxonSVRLRun{}, fmt.Errorf("validation canceled: %w", validationCtx.Err())
	}
	if commandErr != nil {
		return saxonSVRLRun{}, fmt.Errorf("%w: Saxon transform failed: %v: %s", errSaxonRuntimeProcess, commandErr, diagnostics.String())
	}
	select {
	case copyErr := <-stdinCopyDone:
		if copyErr != nil {
			return saxonSVRLRun{}, fmt.Errorf("%w: stream XML to Saxon: %w", errSaxonRuntimeInput, copyErr)
		}
	case <-time.After(waitDelay):
		return saxonSVRLRun{}, fmt.Errorf("%w: XML input reader did not finish after Saxon exited within %s", errSaxonRuntimeInput, waitDelay)
	}

	report, err := svrl.ParseWithLimitsForLayer(bytes.NewReader(output.data), runner.limits, runner.layer)
	if err != nil {
		return saxonSVRLRun{}, fmt.Errorf("%w: parse Saxon SVRL: %w", errSaxonRuntimeSVRL, err)
	}
	return saxonSVRLRun{report: report, startedAt: startedAt, completedAt: runner.now()}, nil
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

// boundedCommandOutput stops draining stdout as soon as the maximum is
// crossed, preventing attacker-controlled SVRL from growing memory usage.
type boundedCommandOutput struct {
	data     []byte
	limit    int
	exceeded bool
}

func (output *boundedCommandOutput) Write(p []byte) (int, error) {
	remaining := output.limit - len(output.data)
	if remaining <= 0 {
		output.exceeded = true
		return 0, errSaxonSVRLOutputLimit
	}
	if len(p) > remaining {
		output.data = append(output.data, p[:remaining]...)
		output.exceeded = true
		return remaining, errSaxonSVRLOutputLimit
	}
	output.data = append(output.data, p...)
	return len(p), nil
}

func maxInt() int { return int(^uint(0) >> 1) }
