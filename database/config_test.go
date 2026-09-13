package database

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseConfigAppliesYSQLDefaults(t *testing.T) {
	config, err := ParseConfig("postgresql://app:secret@db.example")
	if err != nil {
		t.Fatal(err)
	}
	if config.Host() != "db.example" || config.Port() != DefaultPort || config.Database() != DefaultDatabase {
		t.Fatalf("endpoint defaults = host %q port %d database %q", config.Host(), config.Port(), config.Database())
	}
	if config.User() != "app" || config.SSLMode() != DefaultSSLMode {
		t.Fatalf("identity/TLS defaults = user %q sslmode %q", config.User(), config.SSLMode())
	}
	if config.ConnectTimeout() != DefaultConnectTimeout || config.QueryTimeout() != DefaultQueryTimeout || config.MaxConnections() != DefaultMaxConnections || config.MinConnections() != DefaultMinConnections {
		t.Fatalf("pool defaults = %+v", config)
	}
	if strings.Contains(fmt.Sprintf("%+v", config), "secret") || strings.Contains(config.String(), "secret") {
		t.Fatal("formatted config disclosed the password")
	}
}

func TestParseConfigPreservesExplicitValues(t *testing.T) {
	settings := Settings{
		ConnectTimeout: 2 * time.Second,
		QueryTimeout:   3 * time.Second,
		MaxConnections: 23,
		MinConnections: 4,
	}
	config, err := ParseConfigWithSettings("postgres://alice:p%40ss@[2001:db8::1]:15433/invoices?sslmode=verify-full", settings)
	if err != nil {
		t.Fatal(err)
	}
	if config.Host() != "2001:db8::1" || config.Port() != 15433 || config.Database() != "invoices" || config.User() != "alice" || config.SSLMode() != "verify-full" {
		t.Fatalf("parsed config = %+v", config)
	}
	if config.ConnectTimeout() != 2*time.Second || config.QueryTimeout() != 3*time.Second || config.MaxConnections() != 23 || config.MinConnections() != 4 {
		t.Fatalf("settings = %+v", config)
	}
	if !strings.Contains(config.connectionString, "@[2001:db8::1]:15433") {
		t.Fatal("normalized connection string is missing the explicit/default port")
	}
}

func TestParseConfigAcceptsSupportedSSLModes(t *testing.T) {
	for _, mode := range []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"} {
		t.Run(mode, func(t *testing.T) {
			config, err := ParseConfig("postgres://app@localhost/eubl21?sslmode=" + mode)
			if err != nil {
				t.Fatal(err)
			}
			if config.SSLMode() != mode {
				t.Fatalf("sslmode = %q", config.SSLMode())
			}
		})
	}
}

func TestParseConfigNormalizesSSLModeInEffectiveURL(t *testing.T) {
	config, err := ParseConfig("postgres://app@localhost/eubl21?sslmode=REQUIRE")
	if err != nil {
		t.Fatal(err)
	}
	if config.SSLMode() != "require" || !strings.Contains(config.connectionString, "sslmode=require") {
		t.Fatalf("sslmode was not normalized consistently: %v", config)
	}
}

func TestConfigHasNoExternallyMutableFields(t *testing.T) {
	configType := reflect.TypeOf(Config{})
	for index := 0; index < configType.NumField(); index++ {
		field := configType.Field(index)
		if field.IsExported() {
			t.Fatalf("Config field %q is externally mutable", field.Name)
		}
	}
}

func TestParseConfigRejectsMissingAndMalformedValues(t *testing.T) {
	tests := map[string]string{
		"empty":              "",
		"missing user":       "postgres://localhost/eubl21",
		"missing host":       "postgres://app@/eubl21",
		"unsupported scheme": "mysql://app@localhost/eubl21",
		"bad port":           "postgres://app@localhost:70000/eubl21",
		"nested database":    "postgres://app@localhost/one/two",
		"bad sslmode":        "postgres://app@localhost/eubl21?sslmode=sometimes",
		"duplicate sslmode":  "postgres://app@localhost/eubl21?sslmode=require&sslmode=disable",
		"fragment":           "postgres://app@localhost/eubl21#secret",
	}
	for name, rawURL := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := ParseConfig(rawURL)
			if err == nil {
				t.Fatal("expected an error")
			}
			var structured *Error
			if !errors.As(err, &structured) || structured.Kind != ErrorConfiguration {
				t.Fatalf("error = %#v", err)
			}
		})
	}
}

func TestParseConfigRejectsInvalidPoolSettings(t *testing.T) {
	tests := map[string]Settings{
		"connect timeout": {ConnectTimeout: 0, QueryTimeout: time.Second, MaxConnections: 1},
		"query timeout":   {ConnectTimeout: time.Second, QueryTimeout: 0, MaxConnections: 1},
		"max connections": {ConnectTimeout: time.Second, QueryTimeout: time.Second, MaxConnections: 0},
		"min connections": {ConnectTimeout: time.Second, QueryTimeout: time.Second, MaxConnections: 1, MinConnections: -1},
		"pool range":      {ConnectTimeout: time.Second, QueryTimeout: time.Second, MaxConnections: 1, MinConnections: 2},
	}
	for name, settings := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := ParseConfigWithSettings("postgres://app@localhost/eubl21", settings)
			if err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestParseErrorsDoNotLeakCredentials(t *testing.T) {
	const password = "never-log-this-password"
	_, err := ParseConfig("postgres://app:" + password + "@localhost:bad/eubl21")
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), password) || strings.Contains(fmt.Sprintf("%+v", err), password) {
		t.Fatalf("error disclosed password: %v", err)
	}
}
