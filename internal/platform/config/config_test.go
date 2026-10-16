package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

// env turns a map into a LookupFunc.
func env(vars map[string]string) LookupFunc {
	return func(key string) (string, bool) {
		v, ok := vars[key]
		return v, ok
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(env(map[string]string{"DATABASE_URL": "postgres://localhost/db"}))
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}

	want := Config{
		Env:             EnvDevelopment,
		HTTPAddr:        ":8080",
		DatabaseURL:     "postgres://localhost/db",
		LogLevel:        slog.LevelInfo,
		LogFormat:       "text",
		ShutdownTimeout: 15 * time.Second,
	}
	if cfg != want {
		t.Errorf("Load defaults:\n got %+v\nwant %+v", cfg, want)
	}
}

func TestLoadProductionDefaultsToJSONLogs(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"APP_ENV":      "production",
		"DATABASE_URL": "postgres://localhost/db",
	}))
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if cfg.LogFormat != "json" {
		t.Errorf("LogFormat = %q, want %q", cfg.LogFormat, "json")
	}
}

func TestLoadOverrides(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"APP_ENV":          "development",
		"HTTP_ADDR":        "127.0.0.1:9000",
		"DATABASE_URL":     "postgres://localhost/db",
		"LOG_LEVEL":        "debug",
		"LOG_FORMAT":       "json",
		"SHUTDOWN_TIMEOUT": "3s",
	}))
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if cfg.HTTPAddr != "127.0.0.1:9000" || cfg.LogLevel != slog.LevelDebug ||
		cfg.LogFormat != "json" || cfg.ShutdownTimeout != 3*time.Second {
		t.Errorf("overrides not applied: %+v", cfg)
	}
}

func TestLoadTreatsBlankValuesAsUnset(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"DATABASE_URL": "postgres://localhost/db",
		"HTTP_ADDR":    "   ",
	}))
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want default %q", cfg.HTTPAddr, ":8080")
	}
}

func TestLoadReportsAllErrorsAtOnce(t *testing.T) {
	_, err := Load(env(map[string]string{
		"APP_ENV":          "staging",
		"LOG_LEVEL":        "loud",
		"LOG_FORMAT":       "xml",
		"SHUTDOWN_TIMEOUT": "-1s",
	}))
	if err == nil {
		t.Fatal("Load: expected an error, got nil")
	}

	for _, key := range []string{"APP_ENV", "DATABASE_URL", "LOG_LEVEL", "LOG_FORMAT", "SHUTDOWN_TIMEOUT"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error does not mention %s:\n%v", key, err)
		}
	}
}
