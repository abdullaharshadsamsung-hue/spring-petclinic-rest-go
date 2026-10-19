// Package config loads the PetClinic runtime configuration from environment
// variables, with command-line flag overrides.
//
// Environment variables (Spring property equivalents in parentheses):
//
//	PETCLINIC_PORT              listen port (server.port), default 9966
//	PETCLINIC_CONTEXT_PATH      context path (server.servlet.context-path), default /petclinic
//	PETCLINIC_DB                sqlite | postgres (spring.profiles.active), default sqlite
//	POSTGRES_URL                PostgreSQL URL (spring.datasource.url)
//	POSTGRES_USER               PostgreSQL user (spring.datasource.username)
//	POSTGRES_PASS               PostgreSQL password (spring.datasource.password)
//	PETCLINIC_SECURITY_ENABLE   true | false (petclinic.security.enable), default false
//	PETCLINIC_LOG_LEVEL         debug | info | warn | error, default info
//
// MIGRATION_NOTE (deliberate deviation, see README): when PETCLINIC_PORT is
// unset, PORT is honoured; when POSTGRES_URL is unset, DATABASE_URL is used,
// and when PETCLINIC_DB is unset but DATABASE_URL is set, postgres is chosen.
package config

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// Supported database drivers.
const (
	DriverSQLite   = "sqlite"
	DriverPostgres = "postgres"
)

// DefaultSQLiteDSN is an in-memory SQLite database with foreign keys enabled.
const DefaultSQLiteDSN = "file::memory:?_pragma=foreign_keys(1)"

// Config is the application configuration.
type Config struct {
	Addr            string
	ContextPath     string
	DBDriver        string
	DSN             string
	SecurityEnabled bool
	LogLevel        string
}

// Load reads the configuration from the environment and then applies flag
// overrides parsed from args (usually os.Args[1:]).
func Load(args []string) (*Config, error) {
	return load(os.Getenv, args)
}

func load(getenv func(string) string, args []string) (*Config, error) {
	port := firstNonEmpty(getenv("PETCLINIC_PORT"), getenv("PORT"), "9966")
	driver := strings.ToLower(strings.TrimSpace(getenv("PETCLINIC_DB")))
	if driver == "" {
		if getenv("DATABASE_URL") != "" {
			driver = DriverPostgres
		} else {
			driver = DriverSQLite
		}
	}
	cfg := &Config{
		Addr:            ":" + strings.TrimPrefix(port, ":"),
		ContextPath:     firstNonEmpty(getenv("PETCLINIC_CONTEXT_PATH"), "/petclinic"),
		DBDriver:        driver,
		SecurityEnabled: strings.EqualFold(strings.TrimSpace(getenv("PETCLINIC_SECURITY_ENABLE")), "true"),
		LogLevel:        firstNonEmpty(getenv("PETCLINIC_LOG_LEVEL"), "info"),
	}

	fs := flag.NewFlagSet("petclinic", flag.ContinueOnError)
	addr := fs.String("addr", cfg.Addr, "listen address")
	ctxPath := fs.String("context-path", cfg.ContextPath, "context path")
	db := fs.String("db", cfg.DBDriver, "database driver: sqlite|postgres")
	dsn := fs.String("dsn", "", "database DSN (overrides POSTGRES_URL / default SQLite DSN)")
	sec := fs.Bool("security", cfg.SecurityEnabled, "enable HTTP Basic security")
	lvl := fs.String("log-level", cfg.LogLevel, "log level")
	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("parse flags: %w", err)
	}
	cfg.Addr = *addr
	cfg.ContextPath = *ctxPath
	cfg.DBDriver = strings.ToLower(*db)
	cfg.SecurityEnabled = *sec
	cfg.LogLevel = *lvl

	switch cfg.DBDriver {
	case DriverSQLite:
		cfg.DSN = DefaultSQLiteDSN
	case DriverPostgres:
		pg, err := postgresDSN(getenv)
		if err != nil {
			return nil, err
		}
		cfg.DSN = pg
	default:
		return nil, fmt.Errorf("unsupported database driver %q (want sqlite or postgres)", cfg.DBDriver)
	}
	if *dsn != "" {
		cfg.DSN = *dsn
	}
	return cfg, nil
}

// postgresDSN builds a pgx URL from POSTGRES_URL (or DATABASE_URL),
// POSTGRES_USER and POSTGRES_PASS. A Spring-style "jdbc:" prefix is removed.
// Credentials already present in the URL win.
func postgresDSN(getenv func(string) string) (string, error) {
	raw := firstNonEmpty(getenv("POSTGRES_URL"), getenv("DATABASE_URL"), "postgres://localhost/petclinic")
	raw = strings.TrimPrefix(raw, "jdbc:")
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse postgres url: %w", err)
	}
	if u.User == nil || u.User.Username() == "" {
		user := firstNonEmpty(getenv("POSTGRES_USER"), "petclinic")
		pass := firstNonEmpty(getenv("POSTGRES_PASS"), "petclinic")
		u.User = url.UserPassword(user, pass)
	}
	return u.String(), nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
