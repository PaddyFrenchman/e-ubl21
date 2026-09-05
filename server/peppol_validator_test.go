package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	peppolprofile "github.com/PaddyFrenchman/e-ubl21/peppol"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
)

func TestPeppolValidatorReturnsStructuredSemanticOutcomes(t *testing.T) {
	tests := []struct {
		name, svrl string
		wantValid  bool
		wantRules  []string
	}{
		{name: "empty report passes", svrl: emptySVRL, wantValid: true},
		{name: "official fatal rule fails", svrl: `<svrl:schematron-output xmlns:svrl="http://purl.oclc.org/dsdl/svrl"><svrl:failed-assert id="PEPPOL-EN16931-R001" flag="fatal"><svrl:text>Business process is required.</svrl:text></svrl:failed-assert></svrl:schematron-output>`, wantRules: []string{"PEPPOL-EN16931-R001"}},
		{name: "warnings and reports remain visible but pass", svrl: `<svrl:schematron-output xmlns:svrl="http://purl.oclc.org/dsdl/svrl"><svrl:failed-assert id="PEPPOL-WARN" flag="warning"><svrl:text>Review.</svrl:text></svrl:failed-assert><svrl:successful-report id="PEPPOL-REPORT" flag="fatal"><svrl:text>Observed.</svrl:text></svrl:successful-report></svrl:schematron-output>`, wantValid: true, wantRules: []string{"PEPPOL-WARN", "PEPPOL-REPORT"}},
	}
	identifiers := peppolprofile.Identifiers{CustomizationID: peppolprofile.CustomizationIDBase, ProfileID: peppolprofile.ProfileID01, ProcessProfile: queue.PeppolProcessProfile01}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := fakeSaxonRuntime(t, "cat >/dev/null\nprintf '%s' '"+test.svrl+"'")
			validator := newSaxonPeppolValidatorWithIdentifiers(cfg, time.Second, identifiers)
			instant := time.Date(2026, time.September, 5, 12, 0, 0, 0, time.FixedZone("test", 3600))
			calls := 0
			validator.now = func() time.Time { calls++; return instant.Add(time.Duration(calls-1) * 12 * time.Millisecond) }
			result, err := validator.Validate(context.Background(), strings.NewReader(`<Invoice/>`))
			if err != nil {
				t.Fatal(err)
			}
			profile := validationassets.Peppol()
			if result.Valid != test.wantValid || result.Layer != queue.ValidationLayerPeppolBISBilling ||
				result.Version != profile.ValidationArtefactsVersion || result.BISVersion != profile.Version ||
				result.BISReleaseDate != profile.PublicationDate || result.BISMandatoryFrom != profile.MandatoryFrom ||
				result.CustomizationID != identifiers.CustomizationID || result.ProfileID != identifiers.ProfileID || result.ProcessProfile != identifiers.ProcessProfile {
				t.Fatalf("result = %+v", result)
			}
			if result.ValidatedAt != "2026-09-05T11:00:00.012Z" || result.DurationMS != 12 {
				t.Fatalf("timing = %q/%dms", result.ValidatedAt, result.DurationMS)
			}
			if len(result.Issues) != len(test.wantRules) {
				t.Fatalf("issues = %+v", result.Issues)
			}
			for index, rule := range test.wantRules {
				if result.Issues[index].Rule != rule || result.Issues[index].Layer != queue.ValidationLayerPeppolBISBilling {
					t.Fatalf("issue %d = %+v", index, result.Issues[index])
				}
			}
		})
	}
}

func TestPeppolValidatorStreamsInputWithRestrictedSaxonArguments(t *testing.T) {
	cfg := fakeSaxonRuntime(t, `
case "$*" in
  *-config:*" -ext:off -s:- -xsl:"*) ;;
  *) echo "missing secure Saxon arguments: $*" >&2; exit 9 ;;
esac
input=$(cat)
[ "$input" = '<Invoice><ID>streamed</ID></Invoice>' ] || exit 8
printf '%s' '`+emptySVRL+`'
`)
	result, err := newSaxonPeppolValidator(cfg, time.Second).Validate(context.Background(), strings.NewReader(`<Invoice><ID>streamed</ID></Invoice>`))
	if err != nil || !result.Valid {
		t.Fatalf("result/error = %+v / %v", result, err)
	}
}

func TestPeppolValidatorClassifiesOperationalFailures(t *testing.T) {
	tests := []struct {
		name, script string
		configure    func(*saxonPeppolValidator)
		want         error
		contains     string
	}{
		{name: "malformed SVRL", script: `cat >/dev/null; printf '%s' '<not-svrl/>'`, want: errPeppolRuntimeSVRL, contains: "parse Saxon SVRL"},
		{name: "process crash", script: `i=0; while [ "$i" -lt 10000 ]; do printf x >&2; i=$((i + 1)); done; exit 7`, want: errPeppolRuntimeSaxon, contains: "[truncated]"},
		{name: "timeout", script: `while :; do :; done`, configure: func(v *saxonPeppolValidator) { v.timeout = 10 * time.Millisecond }, want: context.DeadlineExceeded, contains: "timed out"},
		{name: "oversized SVRL", script: `cat >/dev/null; i=0; while [ "$i" -lt 200 ]; do printf x; i=$((i + 1)); done`, configure: func(v *saxonPeppolValidator) { v.limits.MaxInputBytes = 64 }, want: errPeppolSVRLOutputLimit, contains: "maximum is 64 bytes"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			validator := newSaxonPeppolValidator(fakeSaxonRuntime(t, test.script), time.Second)
			if test.configure != nil {
				test.configure(validator)
			}
			result, err := validator.Validate(context.Background(), strings.NewReader(`<Invoice/>`))
			if err == nil || !errors.Is(err, test.want) || !strings.Contains(err.Error(), test.contains) || !reflect.DeepEqual(result, queue.ValidationStageResult{}) {
				t.Fatalf("result/error = %+v / %v", result, err)
			}
		})
	}

	t.Run("missing Peppol stylesheet", func(t *testing.T) {
		cfg := fakeSaxonRuntime(t, `exit 0`)
		cfg.xsltPath = filepath.Join(t.TempDir(), "missing.xslt")
		_, err := newSaxonPeppolValidator(cfg, time.Second).Validate(context.Background(), strings.NewReader(`<Invoice/>`))
		if !errors.Is(err, errPeppolRuntimeAsset) || !strings.Contains(err.Error(), "read Peppol XSLT") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("cancellation", func(t *testing.T) {
		validator := newSaxonPeppolValidator(fakeSaxonRuntime(t, `while :; do :; done`), time.Second)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := validator.Validate(ctx, strings.NewReader(`<Invoice/>`))
		if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "operational failure") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestPinnedPeppolValidatorMatchesReferenceManifest(t *testing.T) {
	cfg, ok := localSaxonRuntimeConfig(t)
	if !ok {
		return
	}
	profile := validationassets.Peppol()
	cfg.xsltPath = filepath.Join("..", "validation-assets", filepath.FromSlash(profile.EmbeddedXSLTPath))
	cfg.xsltSHA256 = profile.XSLTSHA256
	if err := ensurePeppolSaxonRuntime(context.Background(), cfg); err != nil {
		t.Fatalf("verify pinned Peppol runtime: %v", err)
	}
	validator := newSaxonPeppolValidator(cfg, 20*time.Second)
	manifest, err := validationassets.PeppolReference()
	if err != nil {
		t.Fatal(err)
	}
	for _, testCase := range manifest.Cases {
		testCase := testCase
		t.Run(testCase.ID, func(t *testing.T) {
			input, err := os.Open(filepath.Join("..", filepath.FromSlash(testCase.Path)))
			if err != nil {
				t.Fatal(err)
			}
			result, validationErr := validator.Validate(context.Background(), input)
			input.Close()
			if validationErr != nil {
				t.Fatalf("Peppol validation: %v", validationErr)
			}
			wantValid := testCase.Outcome == "passed"
			if result.Valid != wantValid || len(result.Issues) != len(testCase.Issues) {
				t.Fatalf("result valid/issues = %v/%+v, want %v/%+v", result.Valid, result.Issues, wantValid, testCase.Issues)
			}
			for index, want := range testCase.Issues {
				got := result.Issues[index]
				if string(got.Kind) != want.Kind || got.Rule != want.RuleID || got.Severity != want.Severity ||
					got.Layer != queue.ValidationLayerPeppolBISBilling {
					t.Fatalf("issue %d = %+v, want %+v", index, got, want)
				}
			}
		})
	}
}
