package main

import (
	"bytes"
	"os"
	"testing"
)

func TestMainDelegatesExitCode(t *testing.T) {
	oldArgs := os.Args
	oldExit := exitProcess
	t.Cleanup(func() {
		os.Args = oldArgs
		exitProcess = oldExit
	})
	os.Args = []string{"e-ubl21", "--help"}
	got := -1
	exitProcess = func(code int) { got = code }
	main()
	if got != 0 {
		t.Fatalf("exit code = %d", got)
	}
}

func TestRunPrintsRootHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("e-ubl21")) {
		t.Fatalf("unexpected help output: %s", stdout.String())
	}
}

func TestRunReportsCommandErrors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--definitely-invalid"}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit code = %d", code)
	}
}

func TestCommandServiceNames(t *testing.T) {
	for name, test := range map[string]struct {
		args []string
		want string
	}{
		"empty": {nil, "e-ubl21"},
		"flag":  {[]string{"--help"}, "e-ubl21"},
		"api":   {[]string{"serve"}, "e-ubl21-api"},
		"xsd":   {[]string{"worker-xsd"}, "e-ubl21-worker-xsd"},
		"en":    {[]string{"worker-en16931"}, "e-ubl21-worker-en16931"},
		"other": {[]string{"custom_worker"}, "e-ubl21-custom-worker"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := commandServiceName(test.args); got != test.want {
				t.Fatalf("service name = %q, want %q", got, test.want)
			}
		})
	}
}
