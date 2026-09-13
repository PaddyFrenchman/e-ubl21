package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestDatabaseMigrateRequiresDatabaseConfiguration(t *testing.T) {
	var stdout bytes.Buffer
	root := NewRootCmd(viper.New(), &stdout, &stdout)
	root.SetArgs([]string{"database", "migrate"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "EUBL21_DATABASE_URL is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestDatabaseMigrateRejectsInvalidTimeout(t *testing.T) {
	var output bytes.Buffer
	root := NewRootCmd(viper.New(), &output, &output)
	root.SetArgs([]string{
		"--database-url", "postgresql://app:secret@127.0.0.1:1/eubl21?sslmode=disable",
		"--database-connect-timeout", "10ms",
		"database", "migrate", "--timeout", "0s",
	})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "migration timeout must be positive") {
		t.Fatalf("error = %v", err)
	}
}
