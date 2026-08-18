package main

import (
	"os"

	"github.com/PaddyFrenchman/e-ubl21/cmd"
	"github.com/spf13/viper"
)

func main() {
	rootCmd := cmd.NewRootCmd(viper.New(), os.Stdout, os.Stderr)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
