package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/PaddyFrenchman/e-ubl21/cmd"
	"github.com/PaddyFrenchman/e-ubl21/observability"
	"github.com/spf13/viper"
)

func main() {
	exitProcess(run(os.Args[1:], os.Stdout, os.Stderr))
}

var exitProcess = os.Exit

func run(args []string, stdout, stderr io.Writer) int {
	shutdownTracing, err := observability.ConfigureTracing(context.Background(), commandServiceName(args))
	if err != nil {
		fmt.Fprintln(stderr, "error: configure tracing:", err)
		return 1
	}
	defer func() {
		if err := shutdownTracing(context.Background()); err != nil {
			fmt.Fprintln(stderr, "error: shutdown tracing:", err)
		}
	}()
	rootCmd := cmd.NewRootCmd(viper.New(), stdout, stderr)
	rootCmd.SetArgs(args)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	return 0
}

func commandServiceName(args []string) string {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "e-ubl21"
	}
	switch args[0] {
	case "serve":
		return "e-ubl21-api"
	case "worker-xsd":
		return "e-ubl21-worker-xsd"
	case "worker-en16931":
		return "e-ubl21-worker-en16931"
	default:
		return "e-ubl21-" + strings.ReplaceAll(args[0], "_", "-")
	}
}
