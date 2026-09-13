// Package database provides the application's vendor-neutral relational
// database configuration and connection-pool contract. The contract is
// qualified against YugabyteDB's YSQL API.
package database

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultPort                 = 5433
	DefaultDatabase             = "eubl21"
	DefaultSSLMode              = "require"
	DefaultConnectTimeout       = 5 * time.Second
	DefaultQueryTimeout         = 5 * time.Second
	DefaultMaxConnections int32 = 10
	DefaultMinConnections int32 = 0
)

// Settings controls bounded connection and query work performed by a Client.
type Settings struct {
	ConnectTimeout time.Duration
	QueryTimeout   time.Duration
	MaxConnections int32
	MinConnections int32
}

// DefaultSettings returns the product defaults for a database pool.
func DefaultSettings() Settings {
	return Settings{
		ConnectTimeout: DefaultConnectTimeout,
		QueryTimeout:   DefaultQueryTimeout,
		MaxConnections: DefaultMaxConnections,
		MinConnections: DefaultMinConnections,
	}
}

// Config is a validated database configuration. Its connection string is kept
// private so ordinary formatting cannot disclose a password.
type Config struct {
	host             string
	port             uint16
	database         string
	user             string
	sslMode          string
	connectTimeout   time.Duration
	queryTimeout     time.Duration
	maxConnections   int32
	minConnections   int32
	connectionString string
}

// ParseConfig parses a database URL using the product pool defaults.
func ParseConfig(rawURL string) (Config, error) {
	return ParseConfigWithSettings(rawURL, DefaultSettings())
}

// ParseConfigWithSettings parses and normalizes a database URL. A missing port,
// database, or sslmode receives the YSQL/product default.
func ParseConfigWithSettings(rawURL string, settings Settings) (Config, error) {
	if strings.TrimSpace(rawURL) == "" {
		return Config{}, configError("database-url", "is required")
	}
	if err := validateSettings(settings); err != nil {
		return Config{}, err
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return Config{}, configError("database-url", "is malformed")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return Config{}, configError("database-url", "must use the postgres or postgresql scheme")
	}
	if parsed.Fragment != "" {
		return Config{}, configError("database-url", "must not contain a fragment")
	}
	if parsed.User == nil || parsed.User.Username() == "" {
		return Config{}, configError("database-url", "must include a user")
	}
	if parsed.Hostname() == "" {
		return Config{}, configError("database-url", "must include a host")
	}

	port := DefaultPort
	if parsed.Port() != "" {
		parsedPort, parseErr := strconv.ParseUint(parsed.Port(), 10, 16)
		if parseErr != nil || parsedPort == 0 {
			return Config{}, configError("database-url port", "must be between 1 and 65535")
		}
		port = int(parsedPort)
	}
	parsed.Host = net.JoinHostPort(parsed.Hostname(), strconv.Itoa(port))

	databaseName := strings.TrimPrefix(parsed.EscapedPath(), "/")
	if databaseName == "" {
		databaseName = DefaultDatabase
		parsed.Path = "/" + databaseName
		parsed.RawPath = ""
	} else {
		if strings.Contains(databaseName, "/") {
			return Config{}, configError("database-url database", "must be a single path segment")
		}
		decodedName, decodeErr := url.PathUnescape(databaseName)
		if decodeErr != nil || decodedName == "" {
			return Config{}, configError("database-url database", "is malformed")
		}
		databaseName = decodedName
	}

	query := parsed.Query()
	if len(query["sslmode"]) > 1 {
		return Config{}, configError("database-url sslmode", "must be specified once")
	}
	sslMode := strings.ToLower(query.Get("sslmode"))
	if sslMode == "" {
		sslMode = DefaultSSLMode
	}
	if !validSSLMode(sslMode) {
		return Config{}, configError("database-url sslmode", "must be disable, allow, prefer, require, verify-ca, or verify-full")
	}
	query.Set("sslmode", sslMode)
	parsed.RawQuery = query.Encode()

	return Config{
		host:             parsed.Hostname(),
		port:             uint16(port),
		database:         databaseName,
		user:             parsed.User.Username(),
		sslMode:          sslMode,
		connectTimeout:   settings.ConnectTimeout,
		queryTimeout:     settings.QueryTimeout,
		maxConnections:   settings.MaxConnections,
		minConnections:   settings.MinConnections,
		connectionString: parsed.String(),
	}, nil
}

func validateSettings(settings Settings) error {
	if settings.ConnectTimeout <= 0 {
		return configError("database-connect-timeout", "must be greater than zero")
	}
	if settings.QueryTimeout <= 0 {
		return configError("database-query-timeout", "must be greater than zero")
	}
	if settings.MaxConnections <= 0 {
		return configError("database-max-connections", "must be greater than zero")
	}
	if settings.MinConnections < 0 {
		return configError("database-min-connections", "must not be negative")
	}
	if settings.MinConnections > settings.MaxConnections {
		return configError("database-min-connections", "must not exceed database-max-connections")
	}
	return nil
}

func validSSLMode(mode string) bool {
	switch mode {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
		return true
	default:
		return false
	}
}

func (c Config) Host() string                  { return c.host }
func (c Config) Port() uint16                  { return c.port }
func (c Config) Database() string              { return c.database }
func (c Config) User() string                  { return c.user }
func (c Config) SSLMode() string               { return c.sslMode }
func (c Config) ConnectTimeout() time.Duration { return c.connectTimeout }
func (c Config) QueryTimeout() time.Duration   { return c.queryTimeout }
func (c Config) MaxConnections() int32         { return c.maxConnections }
func (c Config) MinConnections() int32         { return c.minConnections }

// String returns a credential-safe diagnostic representation.
func (c Config) String() string {
	return fmt.Sprintf("database host=%q port=%d name=%q sslmode=%q connect_timeout=%s query_timeout=%s max_connections=%d min_connections=%d",
		c.host, c.port, c.database, c.sslMode, c.connectTimeout, c.queryTimeout, c.maxConnections, c.minConnections)
}

// GoString prevents %#v formatting from bypassing String and rendering the
// private connection string, which can contain a password.
func (c Config) GoString() string {
	return c.String()
}
