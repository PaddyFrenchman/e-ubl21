package main

import (
	"fmt"
	"os"

	"github.com/PaddyFrenchman/e-ubl21/cmd"
	"github.com/spf13/viper"
)

func main() {
	rootCmd := cmd.NewRootCmd(viper.New(), os.Stdout, os.Stderr)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
