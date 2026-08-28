package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestRegressionEvidenceCommandWritesCoreEvidence(t *testing.T) {
	output := t.TempDir()
	var stdout, stderr bytes.Buffer
	root := NewRootCmd(viper.New(), &stdout, &stderr)
	root.SetArgs([]string{
		"regression", "collect-evidence",
		"--manifest", filepath.Join("..", "createsamples", "samples-en16931", "manifest.json"),
		"--output", output,
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v stderr=%s", err, stderr.String())
	}
	for _, name := range []string{"manifest-used.json", "environment.json", "collection.json"} {
		if _, err := os.Stat(filepath.Join(output, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
	if !bytes.Contains(stdout.Bytes(), []byte("failed=0")) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestParseNamedEvidenceSources(t *testing.T) {
	parsed, err := parseNamedEvidenceSources([]string{"api=/tmp/api.log", "worker=http://worker/metrics"})
	if err != nil || parsed["api"] != "/tmp/api.log" || parsed["worker"] != "http://worker/metrics" {
		t.Fatalf("parsed=%v err=%v", parsed, err)
	}
	for _, values := range [][]string{{"missing-separator"}, {"api="}, {"api=one", "api=two"}} {
		if _, err := parseNamedEvidenceSources(values); err == nil {
			t.Errorf("accepted invalid sources %v", values)
		}
	}
}

func TestRegressionEvidenceCommandExposesKubernetesExecutables(t *testing.T) {
	root := NewRootCmd(viper.New(), &bytes.Buffer{}, &bytes.Buffer{})
	command, _, err := root.Find([]string{"regression", "collect-evidence"})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"kubectl-executable", "minikube-executable"} {
		if command.Flags().Lookup(name) == nil {
			t.Errorf("missing --%s", name)
		}
	}
}
