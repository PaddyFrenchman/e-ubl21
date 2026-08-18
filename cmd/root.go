package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type Config struct {
	ConfigFile    string
	Host          string
	Port          int
	OpenAPIPath   string
	OAuthIssuer   string
	OAuthAudience string
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
	rootCmd.PersistentFlags().String("oauth-issuer", "", "OAuth issuer URL")
	rootCmd.PersistentFlags().String("oauth-audience", "", "OAuth audience")

	mustBindFlag(v, rootCmd, "config")
	mustBindFlag(v, rootCmd, "host")
	mustBindFlag(v, rootCmd, "port")
	mustBindFlag(v, rootCmd, "openapi-path")
	mustBindFlag(v, rootCmd, "oauth-issuer")
	mustBindFlag(v, rootCmd, "oauth-audience")

	rootCmd.AddCommand(newServeCmd(cfg))
	return rootCmd
}

func mustBindFlag(v *viper.Viper, cmd *cobra.Command, name string) {
	if err := v.BindPFlag(name, cmd.PersistentFlags().Lookup(name)); err != nil {
		panic(fmt.Sprintf("bind flag %q: %v", name, err))
	}
}

func configureViper(v *viper.Viper) {
	v.SetDefault("host", "0.0.0.0")
	v.SetDefault("port", 8080)
	v.SetDefault("openapi-path", "/openapi.json")
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
	cfg.OAuthIssuer = v.GetString("oauth-issuer")
	cfg.OAuthAudience = v.GetString("oauth-audience")

	return nil
}
