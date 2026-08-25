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
	if cfg.UploadContentRetention != 30*24*time.Hour || cfg.UploadContentMaxBytes != 10<<30 {
		t.Fatalf("default upload content limits = retention %s, max bytes %d", cfg.UploadContentRetention, cfg.UploadContentMaxBytes)
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
	t.Setenv("EUBL21_WORKER_MAX_ACK_PENDING", "12")
	cfg := &Config{}
	if err := loadConfig(v, cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.WorkerValidationTimeout != 750*time.Millisecond {
		t.Fatalf("validation timeout = %s", cfg.WorkerValidationTimeout)
	}
	if cfg.WorkerConcurrency != 3 || cfg.WorkerPrefetch != 4 || cfg.WorkerAckWait != 2*time.Second || cfg.WorkerMaxDeliver != 9 || cfg.WorkerMaxAckPending != 12 {
		t.Fatalf("worker delivery config = concurrency %d, prefetch %d, ack wait %s, max deliver %d, max ack pending %d", cfg.WorkerConcurrency, cfg.WorkerPrefetch, cfg.WorkerAckWait, cfg.WorkerMaxDeliver, cfg.WorkerMaxAckPending)
	}
}

func TestLoadConfigReadsHTTPResourceLimits(t *testing.T) {
	v := viper.New()
	configureViper(v)
	t.Setenv("EUBL21_HTTP_READ_HEADER_TIMEOUT", "2s")
	t.Setenv("EUBL21_HTTP_READ_TIMEOUT", "45s")
	t.Setenv("EUBL21_HTTP_WRITE_TIMEOUT", "30s")
	t.Setenv("EUBL21_HTTP_IDLE_TIMEOUT", "90s")
	t.Setenv("EUBL21_HTTP_SHUTDOWN_TIMEOUT", "7s")
	t.Setenv("EUBL21_HTTP_MAX_HEADER_BYTES", "32768")
	cfg := &Config{}
	if err := loadConfig(v, cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPReadHeaderTimeout != 2*time.Second || cfg.HTTPReadTimeout != 45*time.Second || cfg.HTTPWriteTimeout != 30*time.Second || cfg.HTTPIdleTimeout != 90*time.Second || cfg.HTTPShutdownTimeout != 7*time.Second || cfg.HTTPMaxHeaderBytes != 32768 {
		t.Fatalf("HTTP limits = %+v", cfg)
	}
}

func TestLoadConfigReadsUploadContentLimits(t *testing.T) {
	v := viper.New()
	configureViper(v)
	t.Setenv("EUBL21_UPLOAD_CONTENT_RETENTION", "168h")
	t.Setenv("EUBL21_UPLOAD_CONTENT_MAX_BYTES", "536870912")
	cfg := &Config{}
	if err := loadConfig(v, cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.UploadContentRetention != 7*24*time.Hour || cfg.UploadContentMaxBytes != 512<<20 {
		t.Fatalf("upload content limits = retention %s, max bytes %d", cfg.UploadContentRetention, cfg.UploadContentMaxBytes)
	}
}

func TestLoadConfigRejectsUnsafeUploadContentLimits(t *testing.T) {
	for _, tc := range []struct {
		key, value string
	}{
		{"EUBL21_UPLOAD_CONTENT_RETENTION", "30s"},
		{"EUBL21_UPLOAD_CONTENT_RETENTION", "-1s"},
		{"EUBL21_UPLOAD_CONTENT_MAX_BYTES", "1048576"},
		{"EUBL21_UPLOAD_CONTENT_MAX_BYTES", "-1"},
	} {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			v := viper.New()
			configureViper(v)
			t.Setenv(tc.key, tc.value)
			if err := loadConfig(v, &Config{}); err == nil {
				t.Fatal("unsafe upload-content configuration was accepted")
			}
		})
	}
}

func TestLoadConfigRejectsIncoherentHTTPResourceLimits(t *testing.T) {
	for _, tc := range []struct {
		name, key, value string
	}{
		{"read shorter than headers", "EUBL21_HTTP_READ_TIMEOUT", "2s"},
		{"impractical write timeout", "EUBL21_HTTP_WRITE_TIMEOUT", "1ms"},
		{"tiny header limit", "EUBL21_HTTP_MAX_HEADER_BYTES", "1024"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := viper.New()
			configureViper(v)
			if tc.name == "read shorter than headers" {
				t.Setenv("EUBL21_HTTP_READ_HEADER_TIMEOUT", "10s")
			}
			t.Setenv(tc.key, tc.value)
			if err := loadConfig(v, &Config{}); err == nil {
				t.Fatal("incoherent HTTP configuration was accepted")
			}
		})
	}
}
