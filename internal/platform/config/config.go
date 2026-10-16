// Package config loads and validates runtime configuration from environment
// variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Env is the deployment environment the process runs in.
type Env string

const (
	EnvDevelopment Env = "development"
	EnvProduction  Env = "production"
)

// Config holds every runtime setting. It is built once at startup and passed
// down explicitly; no package reads environment variables on its own.
type Config struct {
	Env             Env
	HTTPAddr        string
	DatabaseURL     string // contains a password: never log it
	LogLevel        slog.Level
	LogFormat       string // "text" or "json"
	ShutdownTimeout time.Duration
}

// LookupFunc matches os.LookupEnv. Tests pass a map-backed function instead,
// so they never touch the real process environment.
type LookupFunc func(key string) (string, bool)

// Load reads and validates the configuration. It collects every problem
// before returning, so a misconfigured deployment can be fixed in one pass
// instead of one restart per mistake.
func Load(lookup LookupFunc) (Config, error) {
	get := func(key, fallback string) string {
		if v, ok := lookup(key); ok {
			if v = strings.TrimSpace(v); v != "" {
				return v
			}
		}
		return fallback
	}

	var errs []error
	cfg := Config{
		HTTPAddr:    get("HTTP_ADDR", ":8080"),
		DatabaseURL: get("DATABASE_URL", ""),
	}

	switch env := Env(get("APP_ENV", string(EnvDevelopment))); env {
	case EnvDevelopment, EnvProduction:
		cfg.Env = env
	default:
		errs = append(errs, fmt.Errorf("APP_ENV must be %q or %q, got %q", EnvDevelopment, EnvProduction, env))
	}

	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}

	if err := cfg.LogLevel.UnmarshalText([]byte(get("LOG_LEVEL", "info"))); err != nil {
		errs = append(errs, fmt.Errorf("LOG_LEVEL: %w", err))
	}

	// Humans read text logs locally; log collectors parse JSON in production.
	defaultFormat := "text"
	if cfg.Env == EnvProduction {
		defaultFormat = "json"
	}
	switch format := get("LOG_FORMAT", defaultFormat); format {
	case "text", "json":
		cfg.LogFormat = format
	default:
		errs = append(errs, fmt.Errorf("LOG_FORMAT must be \"text\" or \"json\", got %q", format))
	}

	raw := get("SHUTDOWN_TIMEOUT", "15s")
	if d, err := time.ParseDuration(raw); err != nil || d <= 0 {
		errs = append(errs, fmt.Errorf("SHUTDOWN_TIMEOUT must be a positive duration such as 15s, got %q", raw))
	} else {
		cfg.ShutdownTimeout = d
	}

	if err := errors.Join(errs...); err != nil {
		return Config{}, fmt.Errorf("invalid configuration:\n%w", err)
	}
	return cfg, nil
}
