package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/svrl"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
)

const emptySVRL = `<svrl:schematron-output xmlns:svrl="http://purl.oclc.org/dsdl/svrl"/>`

func TestEN16931ValidatorReturnsTypedSemanticOutcomes(t *testing.T) {
	tests := []struct {
		name      string
		svrl      string
		wantValid bool
		wantKinds []queue.ValidationIssueKind
		wantRules []string
	}{
		{
			name:      "no failed assertions is valid",
			svrl:      emptySVRL,
			wantValid: true,
		},
		{
			name: "fatal failed assertion is invalid",
			svrl: `<svrl:schematron-output xmlns:svrl="http://purl.oclc.org/dsdl/svrl">
  <svrl:failed-assert id="BR-01" flag="fatal" location="/Invoice" test="cbc:ID">
    <svrl:text>Invoice identifier is required.</svrl:text>
  </svrl:failed-assert>
</svrl:schematron-output>`,
			wantValid: false,
			wantKinds: []queue.ValidationIssueKind{queue.ValidationIssueFailedAssert},
			wantRules: []string{"BR-01"},
		},
		{
			name: "warning assertion and successful report remain valid",
			svrl: `<svrl:schematron-output xmlns:svrl="http://purl.oclc.org/dsdl/svrl">
  <svrl:failed-assert id="BR-W-01" flag="warning"><svrl:text>Review this value.</svrl:text></svrl:failed-assert>
  <svrl:successful-report id="BR-R-01" flag="error"><svrl:text>Informational report.</svrl:text></svrl:successful-report>
</svrl:schematron-output>`,
			wantValid: true,
			wantKinds: []queue.ValidationIssueKind{queue.ValidationIssueFailedAssert, queue.ValidationIssueSuccessfulReport},
			wantRules: []string{"BR-W-01", "BR-R-01"},
		},
		{
			name:      "error failed assertion is invalid",
			svrl:      `<svrl:schematron-output xmlns:svrl="http://purl.oclc.org/dsdl/svrl"><svrl:failed-assert id="BR-02" role="error"><svrl:text>Bad total.</svrl:text></svrl:failed-assert></svrl:schematron-output>`,
			wantValid: false,
			wantKinds: []queue.ValidationIssueKind{queue.ValidationIssueFailedAssert},
			wantRules: []string{"BR-02"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := fakeSaxonRuntime(t, "cat >/dev/null\nprintf '%s' '"+test.svrl+"'")
			validator := newSaxonEN16931Validator(cfg, time.Second)
			instant := time.Date(2026, time.August, 25, 12, 0, 0, 0, time.FixedZone("test", 3600))
			calls := 0
			validator.now = func() time.Time {
				calls++
				if calls == 1 {
					return instant
				}
				return instant.Add(15 * time.Millisecond)
			}

			result, err := validator.Validate(context.Background(), strings.NewReader(`<Invoice/>`))
			if err != nil {
				t.Fatal(err)
			}
			if result.Valid != test.wantValid || result.Layer != queue.ValidationLayerEN16931 || result.Version != validationassets.EN16931().Version {
				t.Fatalf("result = %+v", result)
			}
			if result.ValidatedAt != "2026-08-25T11:00:00.015Z" || result.DurationMS != 15 {
				t.Fatalf("timing = %q/%dms", result.ValidatedAt, result.DurationMS)
			}
			if len(result.Issues) != len(test.wantKinds) {
				t.Fatalf("issues = %+v", result.Issues)
			}
			for index := range test.wantKinds {
				if result.Issues[index].Kind != test.wantKinds[index] || result.Issues[index].Rule != test.wantRules[index] || result.Issues[index].Layer != queue.ValidationLayerEN16931 {
					t.Fatalf("issue %d = %+v", index, result.Issues[index])
				}
			}
		})
	}
}

func TestEN16931ValidatorStreamsInputAndUsesRestrictedSaxonArguments(t *testing.T) {
	cfg := fakeSaxonRuntime(t, `
case "$*" in
  *-config:*" -ext:off -s:- -xsl:"*) ;;
  *) echo "missing secure Saxon arguments: $*" >&2; exit 9 ;;
esac
input=$(cat)
[ "$input" = '<Invoice><ID>streamed</ID></Invoice>' ] || { echo "unexpected stdin" >&2; exit 8; }
printf '%s' '`+emptySVRL+`'
`)
	validator := newSaxonEN16931Validator(cfg, time.Second)
	result, err := validator.Validate(context.Background(), strings.NewReader(`<Invoice><ID>streamed</ID></Invoice>`))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Valid {
		t.Fatalf("result = %+v", result)
	}
}

func TestEN16931ValidatorClassifiesOperationalFailures(t *testing.T) {
	t.Run("malformed SVRL", func(t *testing.T) {
		cfg := fakeSaxonRuntime(t, `cat >/dev/null; printf '%s' '<not-svrl/>'`)
		result, err := newSaxonEN16931Validator(cfg, time.Second).Validate(context.Background(), strings.NewReader(`<Invoice/>`))
		if err == nil || !strings.Contains(err.Error(), "parse Saxon SVRL") || !reflect.DeepEqual(result, queue.ValidationStageResult{}) {
			t.Fatalf("result/error = %+v / %v", result, err)
		}
	})

	t.Run("process crash with bounded stderr", func(t *testing.T) {
		cfg := fakeSaxonRuntime(t, `i=0; while [ "$i" -lt 10000 ]; do printf x >&2; i=$((i + 1)); done; exit 7`)
		result, err := newSaxonEN16931Validator(cfg, time.Second).Validate(context.Background(), strings.NewReader(`<Invoice/>`))
		if err == nil || !strings.Contains(err.Error(), "Saxon transform failed") || !strings.Contains(err.Error(), "[truncated]") || !reflect.DeepEqual(result, queue.ValidationStageResult{}) {
			t.Fatalf("result/error = %+v / %v", result, err)
		}
		if len(err.Error()) > maxValidationDiagnosticBytes+512 {
			t.Fatalf("diagnostic error is not bounded: %d bytes", len(err.Error()))
		}
	})

	t.Run("killed process", func(t *testing.T) {
		cfg := fakeSaxonRuntime(t, `kill -9 $$`)
		result, err := newSaxonEN16931Validator(cfg, time.Second).Validate(context.Background(), strings.NewReader(`<Invoice/>`))
		if err == nil || !strings.Contains(err.Error(), "Saxon transform failed") || !reflect.DeepEqual(result, queue.ValidationStageResult{}) {
			t.Fatalf("result/error = %+v / %v", result, err)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		cfg := fakeSaxonRuntime(t, `while :; do :; done`)
		result, err := newSaxonEN16931Validator(cfg, 10*time.Millisecond).Validate(context.Background(), strings.NewReader(`<Invoice/>`))
		if err == nil || !strings.Contains(err.Error(), "timed out") || !reflect.DeepEqual(result, queue.ValidationStageResult{}) {
			t.Fatalf("result/error = %+v / %v", result, err)
		}
	})

	t.Run("timeout is hard when XML reader blocks", func(t *testing.T) {
		cfg := fakeSaxonRuntime(t, `cat >/dev/null; printf '%s' '`+emptySVRL+`'`)
		validator := newSaxonEN16931Validator(cfg, 20*time.Millisecond)
		validator.waitDelay = 25 * time.Millisecond
		release := make(chan struct{})
		reader := &blockingValidationReader{started: make(chan struct{}), release: release}
		type outcome struct {
			result queue.ValidationStageResult
			err    error
		}
		done := make(chan outcome, 1)
		startedAt := time.Now()
		go func() {
			result, err := validator.Validate(context.Background(), reader)
			done <- outcome{result: result, err: err}
		}()
		select {
		case <-reader.started:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("validator never started reading XML input")
		}
		select {
		case got := <-done:
			close(release)
			if got.err == nil || !strings.Contains(got.err.Error(), "timed out") || !reflect.DeepEqual(got.result, queue.ValidationStageResult{}) {
				t.Fatalf("result/error = %+v / %v", got.result, got.err)
			}
			if elapsed := time.Since(startedAt); elapsed > 500*time.Millisecond {
				t.Fatalf("hard timeout returned after %s, want no more than timeout plus WaitDelay", elapsed)
			}
		case <-time.After(time.Second):
			close(release)
			t.Fatal("Validate remained blocked after timeout and WaitDelay")
		}
	})

	t.Run("missing Java", func(t *testing.T) {
		cfg := fakeSaxonRuntime(t, `exit 0`)
		cfg.javaExecutable = filepath.Join(t.TempDir(), "missing-java")
		_, err := newSaxonEN16931Validator(cfg, time.Second).Validate(context.Background(), strings.NewReader(`<Invoice/>`))
		if err == nil || !strings.Contains(err.Error(), "Java executable") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("missing Saxon JAR", func(t *testing.T) {
		cfg := fakeSaxonRuntime(t, `exit 0`)
		cfg.saxonJARPath = filepath.Join(t.TempDir(), "missing-saxon.jar")
		_, err := newSaxonEN16931Validator(cfg, time.Second).Validate(context.Background(), strings.NewReader(`<Invoice/>`))
		if err == nil || !strings.Contains(err.Error(), "read Saxon-HE JAR") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("missing XML Resolver JAR", func(t *testing.T) {
		cfg := fakeSaxonRuntime(t, `exit 0`)
		cfg.xmlResolverJARPath = filepath.Join(t.TempDir(), "missing-xmlresolver.jar")
		_, err := newSaxonEN16931Validator(cfg, time.Second).Validate(context.Background(), strings.NewReader(`<Invoice/>`))
		if err == nil || !strings.Contains(err.Error(), "read xmlresolver JAR") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("unreadable XSLT", func(t *testing.T) {
		cfg := fakeSaxonRuntime(t, `exit 0`)
		cfg.xsltPath = filepath.Join(t.TempDir(), "missing.xslt")
		_, err := newSaxonEN16931Validator(cfg, time.Second).Validate(context.Background(), strings.NewReader(`<Invoice/>`))
		if err == nil || !strings.Contains(err.Error(), "read EN 16931 XSLT") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("oversized SVRL", func(t *testing.T) {
		cfg := fakeSaxonRuntime(t, `cat >/dev/null; i=0; while [ "$i" -lt 200 ]; do printf x; i=$((i + 1)); done`)
		validator := newSaxonEN16931Validator(cfg, time.Second)
		validator.limits = svrl.DefaultLimits()
		validator.limits.MaxInputBytes = 64
		result, err := validator.Validate(context.Background(), strings.NewReader(`<Invoice/>`))
		if !errors.Is(err, errEN16931SVRLOutputLimit) || !reflect.DeepEqual(result, queue.ValidationStageResult{}) {
			t.Fatalf("result/error = %+v / %v", result, err)
		}
	})
}

func TestEN16931CancellationKillsJavaProcess(t *testing.T) {
	pidPath := filepath.Join(t.TempDir(), "java.pid")
	cfg := fakeSaxonRuntime(t, `printf '%s' "$$" > "`+pidPath+`"; while :; do :; done`)
	validator := newSaxonEN16931Validator(cfg, 5*time.Second)
	validator.waitDelay = 25 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := validator.Validate(ctx, strings.NewReader(`<Invoice/>`))
		done <- err
	}()
	var pid int
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		body, err := os.ReadFile(pidPath)
		if err == nil {
			pid, err = strconv.Atoi(string(body))
			if err != nil {
				t.Fatal(err)
			}
			break
		}
		time.Sleep(time.Millisecond)
	}
	if pid == 0 {
		t.Fatal("fake Java process did not start")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled validation error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("validation did not return after cancellation")
	}
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("Java process %d remains alive after cancellation", pid)
}

func TestPinnedEN16931ValidatorRejectsMalformedXSLT(t *testing.T) {
	cfg, ok := localSaxonRuntimeConfig(t)
	if !ok {
		return
	}
	path := filepath.Join(t.TempDir(), "malformed.xslt")
	if err := os.WriteFile(path, []byte(`<xsl:stylesheet`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.xsltPath = path
	result, err := newSaxonEN16931Validator(cfg, 5*time.Second).Validate(context.Background(), strings.NewReader(`<Invoice/>`))
	if err == nil || !strings.Contains(err.Error(), "Saxon transform failed") || !reflect.DeepEqual(result, queue.ValidationStageResult{}) {
		t.Fatalf("result/error = %+v / %v", result, err)
	}
}

type blockingValidationReader struct {
	started chan struct{}
	release <-chan struct{}
}

func (reader *blockingValidationReader) Read([]byte) (int, error) {
	close(reader.started)
	<-reader.release
	return 0, io.EOF
}

func TestPinnedEN16931ValidatorClassifiesOfficialAndNonCompliantInvoices(t *testing.T) {
	cfg, ok := localSaxonRuntimeConfig(t)
	if !ok {
		return
	}
	validator := newSaxonEN16931Validator(cfg, 15*time.Second)

	compliantPath := filepath.Join("..", "createsamples", "en16931", "testdata", "official", "guide-example1.xml")
	compliant, err := os.Open(compliantPath)
	if err != nil {
		t.Fatal(err)
	}
	result, err := validator.Validate(context.Background(), compliant)
	compliant.Close()
	if err != nil {
		t.Fatalf("validate official compliant invoice: %v", err)
	}
	if !result.Valid {
		t.Fatalf("official compliant invoice was invalid: %+v", result.Issues)
	}

	nonCompliant := bytes.NewBufferString(`<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"/>`)
	result, err = validator.Validate(context.Background(), nonCompliant)
	if err != nil {
		t.Fatalf("validate non-compliant invoice: %v", err)
	}
	if result.Valid || len(result.Issues) == 0 {
		t.Fatalf("non-compliant invoice result = %+v", result)
	}
	for _, issue := range result.Issues {
		if issue.Kind == queue.ValidationIssueFailedAssert {
			return
		}
	}
	t.Fatal("non-compliant invoice returned no failed assertions")
}
