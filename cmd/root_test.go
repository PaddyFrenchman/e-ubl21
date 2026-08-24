package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/spf13/viper"
)

func TestRootCommandBuildsAndPrintsHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := NewRootCmd(viper.New(), &stdout, &stderr)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("serve")) || !bytes.Contains(stdout.Bytes(), []byte("worker")) {
		t.Fatalf("help missing subcommands: %s", stdout.String())
	}
}

func TestLoadConfigReadsFileAndEnvironment(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	if err := os.WriteFile(configFile, []byte("host: 127.0.0.1\nport: 9090\nopenapi-path: /spec\n"), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	v := viper.New()
	configureViper(v)
	v.Set("config", configFile)
	t.Setenv("EUBL21_PORT", "9191")

	cfg := &Config{}
	if err := loadConfig(v, cfg); err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Host != "127.0.0.1" {
		t.Fatalf("host = %q, want %q", cfg.Host, "127.0.0.1")
	}

	if cfg.Port != 9191 {
		t.Fatalf("port = %d, want %d", cfg.Port, 9191)
	}

	if cfg.OpenAPIPath != "/spec" {
		t.Fatalf("openapi path = %q, want %q", cfg.OpenAPIPath, "/spec")
	}
}

func TestLoadConfigDefaultsQueueNamespaceToDefaultNamespace(t *testing.T) {
	v := viper.New()
	configureViper(v)

	cfg := &Config{}
	if err := loadConfig(v, cfg); err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.QueueNamespace != queue.DefaultNamespace {
		t.Fatalf("queue namespace = %q, want DefaultNamespace", cfg.QueueNamespace)
	}
}

func TestLoadConfigReadsQueueNamespaceFromEnvironment(t *testing.T) {
	v := viper.New()
	configureViper(v)
	t.Setenv("EUBL21_QUEUE_NAMESPACE", "eubl21.prod")

	cfg := &Config{}
	if err := loadConfig(v, cfg); err != nil {
		t.Fatalf("load config: %v", err)
	}

	if want := queue.MustNamespace("eubl21.prod"); cfg.QueueNamespace != want {
		t.Fatalf("queue namespace = %q, want %q", cfg.QueueNamespace, want)
	}
}

func TestLoadConfigRejectsInvalidQueueNamespace(t *testing.T) {
	v := viper.New()
	configureViper(v)
	t.Setenv("EUBL21_QUEUE_NAMESPACE", "eubl21--prod")

	cfg := &Config{}
	if err := loadConfig(v, cfg); err == nil {
		t.Fatal("load config: expected an error for a malformed queue namespace, got none")
	}
}

func TestLoadConfigReadsWorkerValidationTimeout(t *testing.T) {
	v := viper.New()
	configureViper(v)
	t.Setenv("EUBL21_WORKER_VALIDATION_TIMEOUT", "750ms")
	t.Setenv("EUBL21_WORKER_CONCURRENCY", "3")
	t.Setenv("EUBL21_WORKER_PREFETCH", "4")
	t.Setenv("EUBL21_WORKER_ACK_WAIT", "2s")
	t.Setenv("EUBL21_WORKER_MAX_DELIVER", "9")
	cfg := &Config{}
	if err := loadConfig(v, cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.WorkerValidationTimeout != 750*time.Millisecond {
		t.Fatalf("validation timeout = %s", cfg.WorkerValidationTimeout)
	}
	if cfg.WorkerConcurrency != 3 || cfg.WorkerPrefetch != 4 || cfg.WorkerAckWait != 2*time.Second || cfg.WorkerMaxDeliver != 9 {
		t.Fatalf("worker delivery config = concurrency %d, prefetch %d, ack wait %s, max deliver %d", cfg.WorkerConcurrency, cfg.WorkerPrefetch, cfg.WorkerAckWait, cfg.WorkerMaxDeliver)
	}
}
