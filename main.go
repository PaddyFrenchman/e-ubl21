package main

import (
	"fmt"
	"io"
	"os"

	"github.com/PaddyFrenchman/e-ubl21/cmd"
	"github.com/spf13/viper"
)

func main() {
	exitProcess(run(os.Args[1:], os.Stdout, os.Stderr))
}

var exitProcess = os.Exit

func run(args []string, stdout, stderr io.Writer) int {
	rootCmd := cmd.NewRootCmd(viper.New(), stdout, stderr)
	rootCmd.SetArgs(args)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	return 0
}
