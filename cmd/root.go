package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type Config struct {
	ConfigFile          string
	Host                string
	Port                int
	OpenAPIPath         string
	SwaggerPath         string
	OAuthIssuer         string
	OAuthAudience       string
	OAuthHMACSecret     string
	NATSURL             string
	QueueNamespace      queue.Namespace
	RateLimitRPS        float64
	RateLimitBurst      int
	TLSCertFile         string
	TLSKeyFile          string
	WorkerConsumerName  string
	WorkerSubjectFilter string
	WorkerMetricsAddr   string
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
	rootCmd.PersistentFlags().String("nats-url", "nats://localhost:4222", "NATS server URL used to queue uploads and read/write their status")
	rootCmd.PersistentFlags().String("queue-namespace", "", "Queue namespace scoping every JetStream stream/subject/KV bucket name (dot-separated lowercase alphanumeric segments, e.g. \"eubl21.prod\"), so multiple deployments can share one NATS cluster; empty preserves the original hard-coded names")
	rootCmd.PersistentFlags().Float64("rate-limit-rps", 5, "Per-client requests/second allowed before rate limiting")
	rootCmd.PersistentFlags().Int("rate-limit-burst", 10, "Per-client burst size allowed by the rate limiter")
	rootCmd.PersistentFlags().String("tls-cert-file", "", "TLS certificate file (enables TLS together with --tls-key-file)")
	rootCmd.PersistentFlags().String("tls-key-file", "", "TLS private key file (enables TLS together with --tls-cert-file)")

	mustBindFlag(v, rootCmd, "config")
	mustBindFlag(v, rootCmd, "host")
	mustBindFlag(v, rootCmd, "port")
	mustBindFlag(v, rootCmd, "openapi-path")
	mustBindFlag(v, rootCmd, "swagger-path")
	mustBindFlag(v, rootCmd, "oauth-issuer")
	mustBindFlag(v, rootCmd, "oauth-audience")
	mustBindFlag(v, rootCmd, "oauth-hmac-secret")
	mustBindFlag(v, rootCmd, "nats-url")
	mustBindFlag(v, rootCmd, "queue-namespace")
	mustBindFlag(v, rootCmd, "rate-limit-rps")
	mustBindFlag(v, rootCmd, "rate-limit-burst")
	mustBindFlag(v, rootCmd, "tls-cert-file")
	mustBindFlag(v, rootCmd, "tls-key-file")

	rootCmd.AddCommand(newServeCmd(cfg))
	rootCmd.AddCommand(newWorkerCmd(v, cfg))
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
	v.SetDefault("nats-url", "nats://localhost:4222")
	v.SetDefault("queue-namespace", "")
	v.SetDefault("rate-limit-rps", 5)
	v.SetDefault("rate-limit-burst", 10)
	v.SetDefault("worker-consumer-name", "ubl-validator")
	v.SetDefault("worker-subject-filter", "")
	v.SetDefault("worker-metrics-addr", ":9091")
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
	cfg.WorkerConsumerName = v.GetString("worker-consumer-name")
	cfg.WorkerSubjectFilter = v.GetString("worker-subject-filter")
	cfg.WorkerMetricsAddr = v.GetString("worker-metrics-addr")

	return nil
}
