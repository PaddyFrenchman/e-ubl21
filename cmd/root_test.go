package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestRootCommandBuildsAndPrintsHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := NewRootCmd(viper.New(), &stdout, &stderr)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("serve")) || !bytes.Contains(stdout.Bytes(), []byte("worker")) || !bytes.Contains(stdout.Bytes(), []byte("worker-xsd")) || !bytes.Contains(stdout.Bytes(), []byte("worker-en16931")) || !bytes.Contains(stdout.Bytes(), []byte("worker-peppol")) || !bytes.Contains(stdout.Bytes(), []byte("orchestrator")) || !bytes.Contains(stdout.Bytes(), []byte("provision")) || !bytes.Contains(stdout.Bytes(), []byte("dead-letter")) || !bytes.Contains(stdout.Bytes(), []byte("maintenance")) || !bytes.Contains(stdout.Bytes(), []byte("regression")) {
		t.Fatalf("help missing subcommands: %s", stdout.String())
	}
}

func TestPeppolWorkerCommandExposesOnlyPeppolRuntimeControls(t *testing.T) {
	command := newPeppolWorkerCmd(viper.New(), &Config{})
	for _, name := range []string{
		"worker-peppol-metrics-addr", "worker-peppol-timeout", "worker-peppol-concurrency",
		"worker-peppol-prefetch", "worker-peppol-progress-interval", "worker-peppol-java-executable",
		"worker-peppol-saxon-jar-path", "worker-peppol-xmlresolver-jar-path",
		"worker-peppol-saxon-configuration-path", "worker-peppol-runtime-xslt-path",
		"worker-peppol-saxon-startup-timeout",
	} {
		if command.Flags().Lookup(name) == nil {
			t.Fatalf("worker-peppol flag %q is missing", name)
		}
	}
	if command.Flags().Lookup("worker-en16931-timeout") != nil || command.Flags().Lookup("worker-xsd-timeout") != nil {
		t.Fatal("worker-peppol exposes another stage's runtime controls")
	}
}

func TestWorkerCommandMakesEN16931MandatoryAndDescribesOverallTimeout(t *testing.T) {
	worker := newWorkerCmd(viper.New(), &Config{})
	if flag := worker.Flags().Lookup("worker-en16931-mandatory"); flag != nil {
		t.Fatal("worker still exposes an EN 16931 disable switch")
	}
	timeout := worker.Flags().Lookup("worker-validation-timeout")
	if timeout == nil || timeout.Usage != "Maximum total validation time per document" {
		t.Fatalf("validation timeout help = %#v", timeout)
	}
	if worker.Flags().Lookup("worker-xsd-validation-timeout") == nil || worker.Flags().Lookup("worker-en16931-validation-timeout") == nil {
		t.Fatal("worker does not expose both stage-specific validation timeouts")
	}
}

func TestXSDWorkerCommandExposesOnlyXSDRuntimeControls(t *testing.T) {
	command := newXSDWorkerCmd(viper.New(), &Config{})
	for _, name := range []string{"worker-xsd-metrics-addr", "worker-xsd-timeout", "worker-xsd-concurrency", "worker-xsd-prefetch", "worker-xsd-progress-interval"} {
		if command.Flags().Lookup(name) == nil {
			t.Fatalf("worker-xsd flag %q is missing", name)
		}
	}
	if command.Flags().Lookup("worker-en16931-validation-timeout") != nil || command.Flags().Lookup("worker-java-executable") != nil {
		t.Fatal("worker-xsd exposes EN 16931 runtime controls")
	}
}

func TestEN16931WorkerCommandExposesOnlySemanticRuntimeControls(t *testing.T) {
	command := newEN16931WorkerCmd(viper.New(), &Config{})
	for _, name := range []string{
		"worker-en16931-metrics-addr", "worker-en16931-timeout", "worker-en16931-concurrency",
		"worker-en16931-prefetch", "worker-en16931-progress-interval", "worker-en16931-java-executable",
		"worker-en16931-saxon-jar-path", "worker-en16931-xmlresolver-jar-path",
		"worker-en16931-saxon-configuration-path", "worker-en16931-runtime-xslt-path",
		"worker-en16931-saxon-startup-timeout",
	} {
		if command.Flags().Lookup(name) == nil {
			t.Fatalf("worker-en16931 flag %q is missing", name)
		}
	}
	if command.Flags().Lookup("worker-xsd-timeout") != nil {
		t.Fatal("worker-en16931 exposes XSD runtime controls")
	}
}

func TestEN16931WorkerFlagsDoNotRebindTransitionalWorkerConfiguration(t *testing.T) {
	v := viper.New()
	root := NewRootCmd(v, &bytes.Buffer{}, &bytes.Buffer{})
	var transitional *cobra.Command
	for _, command := range root.Commands() {
		if command.Name() == "worker" {
			transitional = command
			break
		}
	}
	if transitional == nil {
		t.Fatal("transitional worker command is missing")
	}
	if err := transitional.Flags().Set("worker-en16931-xslt-path", "/transitional/en.xslt"); err != nil {
		t.Fatal(err)
	}
	if got := v.GetString("worker-en16931-xslt-path"); got != "/transitional/en.xslt" {
		t.Fatalf("transitional EN XSLT flag was rebound by split worker: %q", got)
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
	if cfg.Database != nil {
		t.Fatalf("database should remain optional, got %v", cfg.Database)
	}
	if cfg.UploadContentRetention != 30*24*time.Hour || cfg.UploadContentMaxBytes != 10<<30 {
		t.Fatalf("default upload content limits = retention %s, max bytes %d", cfg.UploadContentRetention, cfg.UploadContentMaxBytes)
	}
	if cfg.WorkerJavaExecutable != "java" || cfg.WorkerSaxonStartupTimeout != 30*time.Second {
		t.Fatalf("default worker EN 16931 runtime config = java %q, startup timeout %s", cfg.WorkerJavaExecutable, cfg.WorkerSaxonStartupTimeout)
	}
	if cfg.WorkerValidationTimeout != 30*time.Second || cfg.WorkerXSDValidationTimeout != 0 || cfg.WorkerEN16931ValidationTimeout != 0 {
		t.Fatalf("default worker validation timeouts = overall %s XSD %s EN %s", cfg.WorkerValidationTimeout, cfg.WorkerXSDValidationTimeout, cfg.WorkerEN16931ValidationTimeout)
	}
	if cfg.XSDWorkerMetricsAddr != ":9092" || cfg.XSDWorkerValidationTimeout != 30*time.Second || cfg.XSDWorkerConcurrency != 1 || cfg.XSDWorkerPrefetch != 1 || cfg.XSDWorkerProgressInterval != 20*time.Second {
		t.Fatalf("default worker-xsd config = metrics %q timeout %s concurrency %d prefetch %d progress %s", cfg.XSDWorkerMetricsAddr, cfg.XSDWorkerValidationTimeout, cfg.XSDWorkerConcurrency, cfg.XSDWorkerPrefetch, cfg.XSDWorkerProgressInterval)
	}
	if cfg.ENWorkerMetricsAddr != ":9093" || cfg.ENWorkerValidationTimeout != 30*time.Second || cfg.ENWorkerConcurrency != 1 || cfg.ENWorkerPrefetch != 1 || cfg.ENWorkerProgressInterval != 20*time.Second || cfg.ENWorkerJavaExecutable != "java" || cfg.ENWorkerSaxonStartupTimeout != 30*time.Second {
		t.Fatalf("default worker-en16931 config = %+v", cfg)
	}
	if cfg.PeppolWorkerMetricsAddr != ":9095" || cfg.PeppolWorkerValidationTimeout != 30*time.Second || cfg.PeppolWorkerConcurrency != 1 || cfg.PeppolWorkerPrefetch != 1 || cfg.PeppolWorkerProgressInterval != 20*time.Second || cfg.PeppolWorkerJavaExecutable != "java" || cfg.PeppolWorkerSaxonStartupTimeout != 30*time.Second {
		t.Fatalf("default worker-peppol config = %+v", cfg)
	}
	if cfg.OrchestratorMetricsAddr != ":9094" || cfg.OrchestratorRole != "combined" || cfg.OrchestratorResultPrefetch != 8 || cfg.OrchestratorReconcileInterval != 10*time.Second {
		t.Fatalf("default orchestrator config = %+v", cfg)
	}
	if cfg.StageXSDAckWait != 90*time.Second || cfg.StageXSDMaxAckPending != 64 || cfg.StageENAckWait != 120*time.Second || cfg.StageENMaxAckPending != 32 || cfg.StagePeppolAckWait != 120*time.Second || cfg.StagePeppolMaxAckPending != 32 {
		t.Fatalf("default stage provisioning config = %+v", cfg)
	}
}

func TestLoadConfigReadsVendorNeutralDatabaseSettings(t *testing.T) {
	v := viper.New()
	configureViper(v)
	t.Setenv("EUBL21_DATABASE_URL", "postgres://app:secret@yb.example:15433/accounting?sslmode=verify-full")
	t.Setenv("EUBL21_DATABASE_CONNECT_TIMEOUT", "2s")
	t.Setenv("EUBL21_DATABASE_QUERY_TIMEOUT", "3s")
	t.Setenv("EUBL21_DATABASE_MAX_CONNECTIONS", "17")
	t.Setenv("EUBL21_DATABASE_MIN_CONNECTIONS", "2")

	cfg := &Config{}
	if err := loadConfig(v, cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Database == nil {
		t.Fatal("database config was not loaded")
	}
	if cfg.Database.Host() != "yb.example" || cfg.Database.Port() != 15433 || cfg.Database.Database() != "accounting" || cfg.Database.User() != "app" || cfg.Database.SSLMode() != "verify-full" {
		t.Fatalf("database endpoint config = %+v", cfg.Database)
	}
	if cfg.Database.ConnectTimeout() != 2*time.Second || cfg.Database.QueryTimeout() != 3*time.Second || cfg.Database.MaxConnections() != 17 || cfg.Database.MinConnections() != 2 {
		t.Fatalf("database pool config = %+v", cfg.Database)
	}
}

func TestLoadConfigRejectsDatabaseSettingsWithoutLeakingPassword(t *testing.T) {
	v := viper.New()
	configureViper(v)
	const password = "do-not-print-this"
	t.Setenv("EUBL21_DATABASE_URL", "postgres://app:"+password+"@localhost:bad/eubl21")

	err := loadConfig(v, &Config{})
	if err == nil {
		t.Fatal("expected malformed database configuration to fail")
	}
	if strings.Contains(err.Error(), password) {
		t.Fatalf("configuration error leaked password: %v", err)
	}
}

func TestLoadConfigReadsIndependentOrchestratorAndStageProvisioningSettings(t *testing.T) {
	v := viper.New()
	configureViper(v)
	t.Setenv("EUBL21_ORCHESTRATOR_METRICS_ADDR", ":9494")
	t.Setenv("EUBL21_ORCHESTRATOR_ROLE", "reconciler")
	t.Setenv("EUBL21_ORCHESTRATOR_RESULT_PREFETCH", "17")
	t.Setenv("EUBL21_ORCHESTRATOR_RECONCILE_INTERVAL", "3s")
	t.Setenv("EUBL21_STAGE_XSD_ACK_WAIT", "41s")
	t.Setenv("EUBL21_STAGE_XSD_MAX_ACK_PENDING", "19")
	t.Setenv("EUBL21_STAGE_EN16931_ACK_WAIT", "73s")
	t.Setenv("EUBL21_STAGE_EN16931_MAX_ACK_PENDING", "11")
	t.Setenv("EUBL21_STAGE_PEPPOL_ACK_WAIT", "89s")
	t.Setenv("EUBL21_STAGE_PEPPOL_MAX_ACK_PENDING", "7")

	cfg := &Config{}
	if err := loadConfig(v, cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.OrchestratorMetricsAddr != ":9494" || cfg.OrchestratorRole != "reconciler" || cfg.OrchestratorResultPrefetch != 17 || cfg.OrchestratorReconcileInterval != 3*time.Second {
		t.Fatalf("orchestrator settings = %+v", cfg)
	}
	if cfg.StageXSDAckWait != 41*time.Second || cfg.StageXSDMaxAckPending != 19 || cfg.StageENAckWait != 73*time.Second || cfg.StageENMaxAckPending != 11 || cfg.StagePeppolAckWait != 89*time.Second || cfg.StagePeppolMaxAckPending != 7 {
		t.Fatalf("stage provisioning settings = %+v", cfg)
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
	t.Setenv("EUBL21_WORKER_XSD_VALIDATION_TIMEOUT", "250ms")
	t.Setenv("EUBL21_WORKER_EN16931_VALIDATION_TIMEOUT", "500ms")
	t.Setenv("EUBL21_WORKER_CONCURRENCY", "3")
	t.Setenv("EUBL21_WORKER_PREFETCH", "4")
	t.Setenv("EUBL21_WORKER_ACK_WAIT", "2s")
	t.Setenv("EUBL21_WORKER_MAX_DELIVER", "9")
	t.Setenv("EUBL21_WORKER_MAX_ACK_PENDING", "12")
	t.Setenv("EUBL21_WORKER_JAVA_EXECUTABLE", "/test/java")
	t.Setenv("EUBL21_WORKER_SAXON_STARTUP_TIMEOUT", "4s")
	cfg := &Config{}
	if err := loadConfig(v, cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.WorkerValidationTimeout != 750*time.Millisecond || cfg.WorkerXSDValidationTimeout != 250*time.Millisecond || cfg.WorkerEN16931ValidationTimeout != 500*time.Millisecond {
		t.Fatalf("validation timeouts = overall %s XSD %s EN %s", cfg.WorkerValidationTimeout, cfg.WorkerXSDValidationTimeout, cfg.WorkerEN16931ValidationTimeout)
	}
	if cfg.WorkerConcurrency != 3 || cfg.WorkerPrefetch != 4 || cfg.WorkerAckWait != 2*time.Second || cfg.WorkerMaxDeliver != 9 || cfg.WorkerMaxAckPending != 12 {
		t.Fatalf("worker delivery config = concurrency %d, prefetch %d, ack wait %s, max deliver %d, max ack pending %d", cfg.WorkerConcurrency, cfg.WorkerPrefetch, cfg.WorkerAckWait, cfg.WorkerMaxDeliver, cfg.WorkerMaxAckPending)
	}
	if cfg.WorkerJavaExecutable != "/test/java" || cfg.WorkerSaxonStartupTimeout != 4*time.Second {
		t.Fatalf("worker EN 16931 runtime config = java %q, startup timeout %s", cfg.WorkerJavaExecutable, cfg.WorkerSaxonStartupTimeout)
	}
}

func TestLoadConfigReadsXSDWorkerSettings(t *testing.T) {
	v := viper.New()
	configureViper(v)
	t.Setenv("EUBL21_WORKER_XSD_METRICS_ADDR", ":9292")
	t.Setenv("EUBL21_WORKER_XSD_TIMEOUT", "9s")
	t.Setenv("EUBL21_WORKER_XSD_CONCURRENCY", "5")
	t.Setenv("EUBL21_WORKER_XSD_PREFETCH", "6")
	t.Setenv("EUBL21_WORKER_XSD_PROGRESS_INTERVAL", "7s")
	cfg := &Config{}
	if err := loadConfig(v, cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.XSDWorkerMetricsAddr != ":9292" || cfg.XSDWorkerValidationTimeout != 9*time.Second || cfg.XSDWorkerConcurrency != 5 || cfg.XSDWorkerPrefetch != 6 || cfg.XSDWorkerProgressInterval != 7*time.Second {
		t.Fatalf("worker-xsd settings = %+v", cfg)
	}
}

func TestLoadConfigReadsEN16931WorkerSettings(t *testing.T) {
	v := viper.New()
	configureViper(v)
	t.Setenv("EUBL21_WORKER_EN16931_METRICS_ADDR", ":9393")
	t.Setenv("EUBL21_WORKER_EN16931_TIMEOUT", "11s")
	t.Setenv("EUBL21_WORKER_EN16931_CONCURRENCY", "5")
	t.Setenv("EUBL21_WORKER_EN16931_PREFETCH", "6")
	t.Setenv("EUBL21_WORKER_EN16931_PROGRESS_INTERVAL", "7s")
	t.Setenv("EUBL21_WORKER_EN16931_JAVA_EXECUTABLE", "/test/java")
	t.Setenv("EUBL21_WORKER_EN16931_SAXON_JAR_PATH", "/test/saxon.jar")
	t.Setenv("EUBL21_WORKER_EN16931_XMLRESOLVER_JAR_PATH", "/test/resolver.jar")
	t.Setenv("EUBL21_WORKER_EN16931_SAXON_CONFIGURATION_PATH", "/test/saxon.xml")
	t.Setenv("EUBL21_WORKER_EN16931_RUNTIME_XSLT_PATH", "/test/en.xslt")
	t.Setenv("EUBL21_WORKER_EN16931_SAXON_STARTUP_TIMEOUT", "8s")
	cfg := &Config{}
	if err := loadConfig(v, cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.ENWorkerMetricsAddr != ":9393" || cfg.ENWorkerValidationTimeout != 11*time.Second || cfg.ENWorkerConcurrency != 5 || cfg.ENWorkerPrefetch != 6 || cfg.ENWorkerProgressInterval != 7*time.Second || cfg.ENWorkerJavaExecutable != "/test/java" || cfg.ENWorkerSaxonJARPath != "/test/saxon.jar" || cfg.ENWorkerXMLResolverJARPath != "/test/resolver.jar" || cfg.ENWorkerSaxonConfigurationPath != "/test/saxon.xml" || cfg.ENWorkerXSLTPath != "/test/en.xslt" || cfg.ENWorkerSaxonStartupTimeout != 8*time.Second {
		t.Fatalf("worker-en16931 settings = %+v", cfg)
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
