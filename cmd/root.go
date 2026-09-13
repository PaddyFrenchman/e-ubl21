package cmd

import (
	"fmt"
	"io"
	"strings"
	"time"

	appdb "github.com/PaddyFrenchman/e-ubl21/database"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/server"
	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type Config struct {
	ConfigFile                         string
	Host                               string
	Port                               int
	OpenAPIPath                        string
	SwaggerPath                        string
	OAuthIssuer                        string
	OAuthAudience                      string
	OAuthHMACSecret                    string
	Database                           *appdb.Config
	NATSURL                            string
	NATSOptions                        []nats.Option
	QueueNamespace                     queue.Namespace
	RateLimitRPS                       float64
	RateLimitBurst                     int
	TLSCertFile                        string
	TLSKeyFile                         string
	HTTPReadHeaderTimeout              time.Duration
	HTTPReadTimeout                    time.Duration
	HTTPWriteTimeout                   time.Duration
	HTTPIdleTimeout                    time.Duration
	HTTPShutdownTimeout                time.Duration
	HTTPMaxHeaderBytes                 int
	UploadContentRetention             time.Duration
	UploadContentMaxBytes              int64
	WorkerConsumerName                 string
	WorkerSubjectFilter                string
	WorkerMetricsAddr                  string
	WorkerValidationTimeout            time.Duration
	WorkerXSDValidationTimeout         time.Duration
	WorkerEN16931ValidationTimeout     time.Duration
	WorkerConcurrency                  int
	WorkerPrefetch                     int
	WorkerAckWait                      time.Duration
	WorkerMaxDeliver                   int
	WorkerMaxAckPending                int
	WorkerJavaExecutable               string
	WorkerSaxonJARPath                 string
	WorkerXMLResolverJARPath           string
	WorkerSaxonConfigurationPath       string
	WorkerEN16931XSLTPath              string
	WorkerSaxonStartupTimeout          time.Duration
	XSDWorkerMetricsAddr               string
	XSDWorkerValidationTimeout         time.Duration
	XSDWorkerConcurrency               int
	XSDWorkerPrefetch                  int
	XSDWorkerProgressInterval          time.Duration
	ENWorkerMetricsAddr                string
	ENWorkerValidationTimeout          time.Duration
	ENWorkerConcurrency                int
	ENWorkerPrefetch                   int
	ENWorkerProgressInterval           time.Duration
	ENWorkerJavaExecutable             string
	ENWorkerSaxonJARPath               string
	ENWorkerXMLResolverJARPath         string
	ENWorkerSaxonConfigurationPath     string
	ENWorkerXSLTPath                   string
	ENWorkerSaxonStartupTimeout        time.Duration
	PeppolWorkerMetricsAddr            string
	PeppolWorkerValidationTimeout      time.Duration
	PeppolWorkerConcurrency            int
	PeppolWorkerPrefetch               int
	PeppolWorkerProgressInterval       time.Duration
	PeppolWorkerJavaExecutable         string
	PeppolWorkerSaxonJARPath           string
	PeppolWorkerXMLResolverJARPath     string
	PeppolWorkerSaxonConfigurationPath string
	PeppolWorkerXSLTPath               string
	PeppolWorkerSaxonStartupTimeout    time.Duration
	OrchestratorMetricsAddr            string
	OrchestratorRole                   string
	OrchestratorResultPrefetch         int
	OrchestratorReconcileInterval      time.Duration
	StageXSDAckWait                    time.Duration
	StageXSDMaxAckPending              int
	StageENAckWait                     time.Duration
	StageENMaxAckPending               int
	StagePeppolAckWait                 time.Duration
	StagePeppolMaxAckPending           int
}

func NewRootCmd(v *viper.Viper, stdout, stderr io.Writer) *cobra.Command {
	cfg := &Config{}

	rootCmd := &cobra.Command{
		Use:           "e-ubl21",
		Short:         "Run the e-UBL 2.1 validation service",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return loadConfig(v, cfg)
		},
	}

	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)

	configureViper(v)

	rootCmd.PersistentFlags().String("config", "", "configuration file path")
	rootCmd.PersistentFlags().String("host", "0.0.0.0", "HTTP bind host")
	rootCmd.PersistentFlags().Int("port", 8080, "HTTP bind port")
	rootCmd.PersistentFlags().String("openapi-path", "/openapi.json", "OpenAPI document path")
	rootCmd.PersistentFlags().String("swagger-path", "/swagger/", "Swagger UI path")
	rootCmd.PersistentFlags().String("oauth-issuer", "", "OAuth issuer URL")
	rootCmd.PersistentFlags().String("oauth-audience", "", "OAuth audience")
	rootCmd.PersistentFlags().String("oauth-hmac-secret", "", "Shared secret used to verify HS256 bearer token signatures (required for protected routes)")
	rootCmd.PersistentFlags().String("database-url", "", "Relational database URL (postgres/postgresql URL for YugabyteDB YSQL; optional until persistence is enabled)")
	rootCmd.PersistentFlags().Duration("database-connect-timeout", appdb.DefaultConnectTimeout, "Maximum time for a database connection or startup probe")
	rootCmd.PersistentFlags().Duration("database-query-timeout", appdb.DefaultQueryTimeout, "Maximum time for a database query or pool acquisition")
	rootCmd.PersistentFlags().Int32("database-max-connections", appdb.DefaultMaxConnections, "Maximum number of database pool connections")
	rootCmd.PersistentFlags().Int32("database-min-connections", appdb.DefaultMinConnections, "Minimum number of database pool connections")
	rootCmd.PersistentFlags().String("nats-url", "nats://localhost:4222", "NATS server URL used to queue uploads and read/write their status")
	rootCmd.PersistentFlags().String("queue-namespace", "", "Queue namespace scoping every JetStream stream/subject/KV bucket name (dot-separated lowercase alphanumeric segments, e.g. \"eubl21.prod\"), so multiple deployments can share one NATS cluster; empty preserves the original hard-coded names")
	rootCmd.PersistentFlags().Float64("rate-limit-rps", 5, "Per-client requests/second allowed before rate limiting")
	rootCmd.PersistentFlags().Int("rate-limit-burst", 10, "Per-client burst size allowed by the rate limiter")
	rootCmd.PersistentFlags().String("tls-cert-file", "", "TLS certificate file (enables TLS together with --tls-key-file)")
	rootCmd.PersistentFlags().String("tls-key-file", "", "TLS private key file (enables TLS together with --tls-cert-file)")
	rootCmd.PersistentFlags().Duration("http-read-header-timeout", 5*time.Second, "Maximum time to read HTTP request headers")
	rootCmd.PersistentFlags().Duration("http-read-timeout", 60*time.Second, "Maximum time to read an entire HTTP request, including an upload body")
	rootCmd.PersistentFlags().Duration("http-write-timeout", 60*time.Second, "Maximum time to write an HTTP response")
	rootCmd.PersistentFlags().Duration("http-idle-timeout", 2*time.Minute, "Maximum keep-alive idle time")
	rootCmd.PersistentFlags().Duration("http-shutdown-timeout", 15*time.Second, "Maximum graceful-shutdown drain time")
	rootCmd.PersistentFlags().Int("http-max-header-bytes", 64<<10, "Maximum total size of HTTP request headers")
	rootCmd.PersistentFlags().Duration("upload-content-retention", 30*24*time.Hour, "How long raw XML remains available after validation; 0 disables expiry")
	rootCmd.PersistentFlags().Int64("upload-content-max-bytes", 10<<30, "Maximum bytes allocated to the shared raw XML Object Store; 0 is unlimited")

	mustBindFlag(v, rootCmd, "config")
	mustBindFlag(v, rootCmd, "host")
	mustBindFlag(v, rootCmd, "port")
	mustBindFlag(v, rootCmd, "openapi-path")
	mustBindFlag(v, rootCmd, "swagger-path")
	mustBindFlag(v, rootCmd, "oauth-issuer")
	mustBindFlag(v, rootCmd, "oauth-audience")
	mustBindFlag(v, rootCmd, "oauth-hmac-secret")
	mustBindFlag(v, rootCmd, "database-url")
	mustBindFlag(v, rootCmd, "database-connect-timeout")
	mustBindFlag(v, rootCmd, "database-query-timeout")
	mustBindFlag(v, rootCmd, "database-max-connections")
	mustBindFlag(v, rootCmd, "database-min-connections")
	mustBindFlag(v, rootCmd, "nats-url")
	mustBindFlag(v, rootCmd, "queue-namespace")
	mustBindFlag(v, rootCmd, "rate-limit-rps")
	mustBindFlag(v, rootCmd, "rate-limit-burst")
	mustBindFlag(v, rootCmd, "tls-cert-file")
	mustBindFlag(v, rootCmd, "tls-key-file")
	mustBindFlag(v, rootCmd, "http-read-header-timeout")
	mustBindFlag(v, rootCmd, "http-read-timeout")
	mustBindFlag(v, rootCmd, "http-write-timeout")
	mustBindFlag(v, rootCmd, "http-idle-timeout")
	mustBindFlag(v, rootCmd, "http-shutdown-timeout")
	mustBindFlag(v, rootCmd, "http-max-header-bytes")
	mustBindFlag(v, rootCmd, "upload-content-retention")
	mustBindFlag(v, rootCmd, "upload-content-max-bytes")

	rootCmd.AddCommand(newServeCmd(cfg))
	rootCmd.AddCommand(newWorkerCmd(v, cfg))
	rootCmd.AddCommand(newXSDWorkerCmd(v, cfg))
	rootCmd.AddCommand(newEN16931WorkerCmd(v, cfg))
	rootCmd.AddCommand(newPeppolWorkerCmd(v, cfg))
	rootCmd.AddCommand(newOrchestratorCmd(v, cfg))
	rootCmd.AddCommand(newProvisionCmd(v, cfg))
	rootCmd.AddCommand(newMigrationCmd(cfg))
	rootCmd.AddCommand(newDeadLetterCmd(cfg))
	rootCmd.AddCommand(newMaintenanceCmd(cfg))
	rootCmd.AddCommand(newRegressionCmd(cfg))
	rootCmd.AddCommand(newIssue65Cmd(cfg))
	return rootCmd
}

func mustBindFlag(v *viper.Viper, cmd *cobra.Command, name string) {
	if err := v.BindPFlag(name, cmd.PersistentFlags().Lookup(name)); err != nil {
		panic(fmt.Sprintf("bind flag %q: %v", name, err))
	}
}

// mustBindLocalFlag binds a flag local to cmd (not a persistent/root flag),
// used for flags specific to a single subcommand such as `worker`.
func mustBindLocalFlag(v *viper.Viper, cmd *cobra.Command, name string) {
	if err := v.BindPFlag(name, cmd.Flags().Lookup(name)); err != nil {
		panic(fmt.Sprintf("bind flag %q: %v", name, err))
	}
}

func configureViper(v *viper.Viper) {
	v.SetDefault("host", "0.0.0.0")
	v.SetDefault("port", 8080)
	v.SetDefault("openapi-path", "/openapi.json")
	v.SetDefault("swagger-path", "/swagger/")
	v.SetDefault("database-url", "")
	v.SetDefault("database-connect-timeout", appdb.DefaultConnectTimeout)
	v.SetDefault("database-query-timeout", appdb.DefaultQueryTimeout)
	v.SetDefault("database-max-connections", appdb.DefaultMaxConnections)
	v.SetDefault("database-min-connections", appdb.DefaultMinConnections)
	v.SetDefault("nats-url", "nats://localhost:4222")
	v.SetDefault("queue-namespace", "")
	v.SetDefault("rate-limit-rps", 5)
	v.SetDefault("rate-limit-burst", 10)
	v.SetDefault("http-read-header-timeout", 5*time.Second)
	v.SetDefault("http-read-timeout", 60*time.Second)
	v.SetDefault("http-write-timeout", 60*time.Second)
	v.SetDefault("http-idle-timeout", 2*time.Minute)
	v.SetDefault("http-shutdown-timeout", 15*time.Second)
	v.SetDefault("http-max-header-bytes", 64<<10)
	v.SetDefault("upload-content-retention", 30*24*time.Hour)
	v.SetDefault("upload-content-max-bytes", int64(10<<30))
	v.SetDefault("worker-consumer-name", "ubl-validator")
	v.SetDefault("worker-subject-filter", "")
	v.SetDefault("worker-metrics-addr", ":9091")
	v.SetDefault("worker-validation-timeout", 30*time.Second)
	v.SetDefault("worker-xsd-validation-timeout", time.Duration(0))
	v.SetDefault("worker-en16931-validation-timeout", time.Duration(0))
	v.SetDefault("worker-concurrency", 1)
	v.SetDefault("worker-prefetch", 1)
	v.SetDefault("worker-ack-wait", 60*time.Second)
	v.SetDefault("worker-max-deliver", 5)
	v.SetDefault("worker-max-ack-pending", 0)
	v.SetDefault("worker-java-executable", "java")
	v.SetDefault("worker-saxon-startup-timeout", 30*time.Second)
	v.SetDefault("worker-xsd-metrics-addr", ":9092")
	v.SetDefault("worker-xsd-timeout", 30*time.Second)
	v.SetDefault("worker-xsd-concurrency", 1)
	v.SetDefault("worker-xsd-prefetch", 1)
	v.SetDefault("worker-xsd-progress-interval", 20*time.Second)
	v.SetDefault("worker-en16931-metrics-addr", ":9093")
	v.SetDefault("worker-en16931-timeout", 30*time.Second)
	v.SetDefault("worker-en16931-concurrency", 1)
	v.SetDefault("worker-en16931-prefetch", 1)
	v.SetDefault("worker-en16931-progress-interval", 20*time.Second)
	v.SetDefault("worker-en16931-java-executable", "java")
	v.SetDefault("worker-en16931-saxon-startup-timeout", 30*time.Second)
	v.SetDefault("worker-peppol-metrics-addr", ":9095")
	v.SetDefault("worker-peppol-timeout", 30*time.Second)
	v.SetDefault("worker-peppol-concurrency", 1)
	v.SetDefault("worker-peppol-prefetch", 1)
	v.SetDefault("worker-peppol-progress-interval", 20*time.Second)
	v.SetDefault("worker-peppol-java-executable", "java")
	v.SetDefault("worker-peppol-saxon-startup-timeout", 30*time.Second)
	v.SetDefault("orchestrator-metrics-addr", ":9094")
	v.SetDefault("orchestrator-role", "combined")
	v.SetDefault("orchestrator-result-prefetch", 8)
	v.SetDefault("orchestrator-reconcile-interval", 10*time.Second)
	v.SetDefault("stage-xsd-ack-wait", 90*time.Second)
	v.SetDefault("stage-xsd-max-ack-pending", 64)
	v.SetDefault("stage-en16931-ack-wait", 120*time.Second)
	v.SetDefault("stage-en16931-max-ack-pending", 32)
	v.SetDefault("stage-peppol-ack-wait", 120*time.Second)
	v.SetDefault("stage-peppol-max-ack-pending", 32)
	v.SetEnvPrefix("EUBL21")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()
}

func loadConfig(v *viper.Viper, cfg *Config) error {
	if cfgFile := v.GetString("config"); cfgFile != "" {
		v.SetConfigFile(cfgFile)
		if err := v.ReadInConfig(); err != nil {
			return fmt.Errorf("read config: %w", err)
		}
	}

	cfg.ConfigFile = v.GetString("config")
	cfg.Host = v.GetString("host")
	cfg.Port = v.GetInt("port")
	cfg.OpenAPIPath = v.GetString("openapi-path")
	cfg.SwaggerPath = v.GetString("swagger-path")
	cfg.OAuthIssuer = v.GetString("oauth-issuer")
	cfg.OAuthAudience = v.GetString("oauth-audience")
	cfg.OAuthHMACSecret = v.GetString("oauth-hmac-secret")
	cfg.Database = nil
	if databaseURL := v.GetString("database-url"); databaseURL != "" {
		databaseConfig, databaseErr := appdb.ParseConfigWithSettings(databaseURL, appdb.Settings{
			ConnectTimeout: v.GetDuration("database-connect-timeout"),
			QueryTimeout:   v.GetDuration("database-query-timeout"),
			MaxConnections: v.GetInt32("database-max-connections"),
			MinConnections: v.GetInt32("database-min-connections"),
		})
		if databaseErr != nil {
			return fmt.Errorf("load config: %w", databaseErr)
		}
		cfg.Database = &databaseConfig
	}
	cfg.NATSURL = v.GetString("nats-url")
	namespace, err := queue.NewNamespace(v.GetString("queue-namespace"))
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	cfg.QueueNamespace = namespace
	cfg.RateLimitRPS = v.GetFloat64("rate-limit-rps")
	cfg.RateLimitBurst = v.GetInt("rate-limit-burst")
	cfg.TLSCertFile = v.GetString("tls-cert-file")
	cfg.TLSKeyFile = v.GetString("tls-key-file")
	cfg.HTTPReadHeaderTimeout = v.GetDuration("http-read-header-timeout")
	cfg.HTTPReadTimeout = v.GetDuration("http-read-timeout")
	cfg.HTTPWriteTimeout = v.GetDuration("http-write-timeout")
	cfg.HTTPIdleTimeout = v.GetDuration("http-idle-timeout")
	cfg.HTTPShutdownTimeout = v.GetDuration("http-shutdown-timeout")
	cfg.HTTPMaxHeaderBytes = v.GetInt("http-max-header-bytes")
	if err := validateHTTPConfig(cfg); err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	cfg.UploadContentRetention = v.GetDuration("upload-content-retention")
	cfg.UploadContentMaxBytes = v.GetInt64("upload-content-max-bytes")
	if err := validateUploadContentConfig(cfg); err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	cfg.WorkerConsumerName = v.GetString("worker-consumer-name")
	cfg.WorkerSubjectFilter = v.GetString("worker-subject-filter")
	cfg.WorkerMetricsAddr = v.GetString("worker-metrics-addr")
	cfg.WorkerValidationTimeout = v.GetDuration("worker-validation-timeout")
	cfg.WorkerXSDValidationTimeout = v.GetDuration("worker-xsd-validation-timeout")
	cfg.WorkerEN16931ValidationTimeout = v.GetDuration("worker-en16931-validation-timeout")
	cfg.WorkerConcurrency = v.GetInt("worker-concurrency")
	cfg.WorkerPrefetch = v.GetInt("worker-prefetch")
	cfg.WorkerAckWait = v.GetDuration("worker-ack-wait")
	cfg.WorkerMaxDeliver = v.GetInt("worker-max-deliver")
	cfg.WorkerMaxAckPending = v.GetInt("worker-max-ack-pending")
	cfg.WorkerJavaExecutable = v.GetString("worker-java-executable")
	cfg.WorkerSaxonJARPath = v.GetString("worker-saxon-jar-path")
	cfg.WorkerXMLResolverJARPath = v.GetString("worker-xmlresolver-jar-path")
	cfg.WorkerSaxonConfigurationPath = v.GetString("worker-saxon-configuration-path")
	cfg.WorkerEN16931XSLTPath = v.GetString("worker-en16931-xslt-path")
	cfg.WorkerSaxonStartupTimeout = v.GetDuration("worker-saxon-startup-timeout")
	cfg.XSDWorkerMetricsAddr = v.GetString("worker-xsd-metrics-addr")
	cfg.XSDWorkerValidationTimeout = v.GetDuration("worker-xsd-timeout")
	cfg.XSDWorkerConcurrency = v.GetInt("worker-xsd-concurrency")
	cfg.XSDWorkerPrefetch = v.GetInt("worker-xsd-prefetch")
	cfg.XSDWorkerProgressInterval = v.GetDuration("worker-xsd-progress-interval")
	cfg.ENWorkerMetricsAddr = v.GetString("worker-en16931-metrics-addr")
	cfg.ENWorkerValidationTimeout = v.GetDuration("worker-en16931-timeout")
	cfg.ENWorkerConcurrency = v.GetInt("worker-en16931-concurrency")
	cfg.ENWorkerPrefetch = v.GetInt("worker-en16931-prefetch")
	cfg.ENWorkerProgressInterval = v.GetDuration("worker-en16931-progress-interval")
	cfg.ENWorkerJavaExecutable = v.GetString("worker-en16931-java-executable")
	cfg.ENWorkerSaxonJARPath = v.GetString("worker-en16931-saxon-jar-path")
	cfg.ENWorkerXMLResolverJARPath = v.GetString("worker-en16931-xmlresolver-jar-path")
	cfg.ENWorkerSaxonConfigurationPath = v.GetString("worker-en16931-saxon-configuration-path")
	cfg.ENWorkerXSLTPath = v.GetString("worker-en16931-runtime-xslt-path")
	cfg.ENWorkerSaxonStartupTimeout = v.GetDuration("worker-en16931-saxon-startup-timeout")
	cfg.PeppolWorkerMetricsAddr = v.GetString("worker-peppol-metrics-addr")
	cfg.PeppolWorkerValidationTimeout = v.GetDuration("worker-peppol-timeout")
	cfg.PeppolWorkerConcurrency = v.GetInt("worker-peppol-concurrency")
	cfg.PeppolWorkerPrefetch = v.GetInt("worker-peppol-prefetch")
	cfg.PeppolWorkerProgressInterval = v.GetDuration("worker-peppol-progress-interval")
	cfg.PeppolWorkerJavaExecutable = v.GetString("worker-peppol-java-executable")
	cfg.PeppolWorkerSaxonJARPath = v.GetString("worker-peppol-saxon-jar-path")
	cfg.PeppolWorkerXMLResolverJARPath = v.GetString("worker-peppol-xmlresolver-jar-path")
	cfg.PeppolWorkerSaxonConfigurationPath = v.GetString("worker-peppol-saxon-configuration-path")
	cfg.PeppolWorkerXSLTPath = v.GetString("worker-peppol-runtime-xslt-path")
	cfg.PeppolWorkerSaxonStartupTimeout = v.GetDuration("worker-peppol-saxon-startup-timeout")
	cfg.OrchestratorMetricsAddr = v.GetString("orchestrator-metrics-addr")
	cfg.OrchestratorRole = v.GetString("orchestrator-role")
	cfg.OrchestratorResultPrefetch = v.GetInt("orchestrator-result-prefetch")
	cfg.OrchestratorReconcileInterval = v.GetDuration("orchestrator-reconcile-interval")
	cfg.StageXSDAckWait = v.GetDuration("stage-xsd-ack-wait")
	cfg.StageXSDMaxAckPending = v.GetInt("stage-xsd-max-ack-pending")
	cfg.StageENAckWait = v.GetDuration("stage-en16931-ack-wait")
	cfg.StageENMaxAckPending = v.GetInt("stage-en16931-max-ack-pending")
	cfg.StagePeppolAckWait = v.GetDuration("stage-peppol-ack-wait")
	cfg.StagePeppolMaxAckPending = v.GetInt("stage-peppol-max-ack-pending")

	return nil
}

func validateUploadContentConfig(cfg *Config) error {
	return server.ValidateUploadContentLimits(cfg.UploadContentRetention, cfg.UploadContentMaxBytes)
}

const minimumHTTPTimeout = time.Second
const minimumHTTPMaxHeaderBytes = 4 << 10

func validateHTTPConfig(cfg *Config) error {
	for _, setting := range []struct {
		name  string
		value time.Duration
	}{
		{"http-read-header-timeout", cfg.HTTPReadHeaderTimeout},
		{"http-read-timeout", cfg.HTTPReadTimeout},
		{"http-write-timeout", cfg.HTTPWriteTimeout},
		{"http-idle-timeout", cfg.HTTPIdleTimeout},
		{"http-shutdown-timeout", cfg.HTTPShutdownTimeout},
	} {
		if setting.value < minimumHTTPTimeout {
			return fmt.Errorf("%s must be at least %s", setting.name, minimumHTTPTimeout)
		}
	}
	if cfg.HTTPReadTimeout < cfg.HTTPReadHeaderTimeout {
		return fmt.Errorf("http-read-timeout (%s) must be at least http-read-header-timeout (%s)", cfg.HTTPReadTimeout, cfg.HTTPReadHeaderTimeout)
	}
	if cfg.HTTPMaxHeaderBytes < minimumHTTPMaxHeaderBytes {
		return fmt.Errorf("http-max-header-bytes must be at least %d", minimumHTTPMaxHeaderBytes)
	}
	return nil
}
