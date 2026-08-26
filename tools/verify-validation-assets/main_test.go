package main

import (
	"flag"
	"os"
	"testing"
)

func TestMainVerifiesAndStagesAssets(t *testing.T) {
	originalArgs := os.Args
	originalFlags := flag.CommandLine
	t.Cleanup(func() {
		os.Args = originalArgs
		flag.CommandLine = originalFlags
	})

	for _, args := range [][]string{
		{"verify-validation-assets"},
		{"verify-validation-assets", "--stage-dir", t.TempDir()},
	} {
		flag.CommandLine = flag.NewFlagSet(args[0], flag.ContinueOnError)
		os.Args = args
		main()
	}
}
