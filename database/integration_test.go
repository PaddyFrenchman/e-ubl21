package database_test

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	appdb "github.com/PaddyFrenchman/e-ubl21/database"
)

func TestYugabyteDBIntegration(t *testing.T) {
	rawURL := os.Getenv("EUBL21_TEST_DATABASE_URL")
	if rawURL == "" {
		t.Skip("set EUBL21_TEST_DATABASE_URL to run against a real YugabyteDB instance")
	}

	t.Run("connect and query", func(t *testing.T) {
		settings := appdb.DefaultSettings()
		settings.MaxConnections = 1
		config, err := appdb.ParseConfigWithSettings(rawURL, settings)
		if err != nil {
			t.Fatal(err)
		}
		connectDeadline := time.Now().Add(2 * time.Minute)
		var client *appdb.Client
		for {
			client, err = appdb.Connect(context.Background(), config)
			if err == nil {
				break
			}
			if time.Now().After(connectDeadline) {
				t.Fatalf("same pool could not establish its initial connection: %v", err)
			}
			time.Sleep(500 * time.Millisecond)
		}
		defer client.Close()
		var preFailoverBackend int
		if err := client.ScanRow(context.Background(), "SELECT pg_backend_pid()", nil, &preFailoverBackend); err != nil {
			t.Fatal(err)
		}

		var one int
		if err := client.ScanRow(context.Background(), "SELECT 1", nil, &one); err != nil {
			t.Fatal(err)
		}
		if one != 1 {
			t.Fatalf("SELECT 1 = %d", one)
		}
		var version string
		if err := client.ScanRow(context.Background(), "SELECT version()", nil, &version); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(version, "YugabyteDB") && !strings.Contains(version, "-YB-") {
			t.Fatalf("integration target is not YugabyteDB: %q", version)
		}
		if os.Getenv("EUBL21_TEST_DATABASE_CATALOG_OPERATION") == "1" {
			catalogDeadline := time.Now().Add(2 * time.Minute)
			for {
				_, _ = client.Exec(context.Background(), "DROP TABLE IF EXISTS eubl21_ha_quorum_probe", nil)
				_, createErr := client.Exec(context.Background(), "CREATE TABLE eubl21_ha_quorum_probe (id int PRIMARY KEY)", nil)
				if createErr == nil {
					_, dropErr := client.Exec(context.Background(), "DROP TABLE eubl21_ha_quorum_probe", nil)
					if dropErr == nil {
						break
					}
					createErr = dropErr
				}
				if time.Now().After(catalogDeadline) {
					t.Fatalf("master quorum did not accept catalog write: %v", createErr)
				}
				time.Sleep(500 * time.Millisecond)
			}
		}
		var expiredLease appdb.Connection
		if err := client.WithConn(context.Background(), func(connection appdb.Connection) error {
			expiredLease = connection
			return connection.ScanRow("SELECT 1", nil, &one)
		}); err != nil {
			t.Fatal(err)
		}
		if err := expiredLease.ScanRow("SELECT 1", nil, &one); !errors.Is(err, context.Canceled) {
			t.Fatalf("connection remained usable after callback: %v", err)
		}
		client.Close()
	})

	t.Run("same pool reconnects after failover", func(t *testing.T) {
		readyFile := os.Getenv("EUBL21_TEST_DATABASE_FAILOVER_READY_FILE")
		doneFile := os.Getenv("EUBL21_TEST_DATABASE_FAILOVER_DONE_FILE")
		if readyFile == "" || doneFile == "" {
			t.Skip("set failover ready and done files for a coordinated live failover test")
		}
		settings := appdb.DefaultSettings()
		settings.MaxConnections = 1
		config, err := appdb.ParseConfigWithSettings(rawURL, settings)
		if err != nil {
			t.Fatal(err)
		}
		connectDeadline := time.Now().Add(2 * time.Minute)
		var client *appdb.Client
		for {
			client, err = appdb.Connect(context.Background(), config)
			if err == nil {
				break
			}
			if time.Now().After(connectDeadline) {
				t.Fatalf("same pool could not establish its initial connection: %v", err)
			}
			time.Sleep(500 * time.Millisecond)
		}
		defer client.Close()
		if err := os.WriteFile(readyFile, []byte("pool-ready\n"), 0600); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(2 * time.Minute)
		failoverDone := false
		observedFailure := false
		for {
			var one int
			err = client.ScanRow(context.Background(), "SELECT 1", nil, &one)
			if _, statErr := os.Stat(doneFile); statErr == nil {
				failoverDone = true
			}
			if err != nil {
				observedFailure = true
			}
			if failoverDone && observedFailure && err == nil && one == 1 {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("same pool did not recover after failover: %v", err)
			}
			time.Sleep(500 * time.Millisecond)
		}
	})

	t.Run("pool exhaustion is bounded", func(t *testing.T) {
		settings := appdb.DefaultSettings()
		settings.MaxConnections = 1
		settings.QueryTimeout = 200 * time.Millisecond
		config, err := appdb.ParseConfigWithSettings(rawURL, settings)
		if err != nil {
			t.Fatal(err)
		}
		client, err := appdb.Connect(context.Background(), config)
		if err != nil {
			t.Fatal(err)
		}
		defer client.Close()
		started := time.Now()
		err = client.WithConn(context.Background(), func(appdb.Connection) error {
			return client.WithConn(context.Background(), func(appdb.Connection) error {
				t.Fatal("second connection unexpectedly acquired")
				return nil
			})
		})
		if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > time.Second {
			t.Fatalf("pool timeout was not bounded: duration=%s error=%v", time.Since(started), err)
		}
	})

	t.Run("wrong credentials fail safely", func(t *testing.T) {
		wrongURL, err := url.Parse(rawURL)
		if err != nil {
			t.Fatal(err)
		}
		const wrongPassword = "known-wrong-secret"
		wrongURL.User = url.UserPassword(wrongURL.User.Username(), wrongPassword)
		config, err := appdb.ParseConfig(wrongURL.String())
		if err != nil {
			t.Fatal(err)
		}
		_, err = appdb.Connect(context.Background(), config)
		if err == nil {
			t.Fatal("wrong credentials unexpectedly connected")
		}
		if strings.Contains(err.Error(), wrongPassword) {
			t.Fatalf("error disclosed password: %v", err)
		}
	})

	t.Run("unavailable host is bounded", func(t *testing.T) {
		settings := appdb.DefaultSettings()
		settings.ConnectTimeout = 300 * time.Millisecond
		config, err := appdb.ParseConfigWithSettings("postgres://eubl21:secret@127.0.0.1:1/eubl21?sslmode=disable", settings)
		if err != nil {
			t.Fatal(err)
		}
		started := time.Now()
		_, err = appdb.Connect(context.Background(), config)
		if err == nil {
			t.Fatal("unavailable endpoint unexpectedly connected")
		}
		if time.Since(started) > 2*time.Second {
			t.Fatalf("unavailable endpoint failure was not bounded: %s", time.Since(started))
		}
	})

	t.Run("TLS policy is enforced", func(t *testing.T) {
		tlsURL := os.Getenv("EUBL21_TEST_DATABASE_TLS_URL")
		if tlsURL == "" {
			parsed, err := url.Parse(rawURL)
			if err != nil {
				t.Fatal(err)
			}
			query := parsed.Query()
			if query.Get("sslmode") != "disable" {
				t.Skip("set EUBL21_TEST_DATABASE_TLS_URL to exercise certificate verification")
			}
			query.Set("sslmode", "require")
			parsed.RawQuery = query.Encode()
			config, err := appdb.ParseConfig(parsed.String())
			if err != nil {
				t.Fatal(err)
			}
			if client, connectErr := appdb.Connect(context.Background(), config); connectErr == nil {
				client.Close()
				t.Fatal("plaintext topology unexpectedly accepted required TLS")
			}
			return
		}
		parsed, err := url.Parse(tlsURL)
		if err != nil {
			t.Fatal(err)
		}
		mode := parsed.Query().Get("sslmode")
		if mode != "verify-ca" && mode != "verify-full" {
			t.Fatalf("TLS integration URL must use verify-ca or verify-full, got %q", mode)
		}
		config, err := appdb.ParseConfig(tlsURL)
		if err != nil {
			t.Fatal(err)
		}
		client, err := appdb.Connect(context.Background(), config)
		if err != nil {
			t.Fatal(err)
		}
		client.Close()
	})
}
