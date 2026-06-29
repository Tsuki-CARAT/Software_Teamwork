package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("AUTH_HTTP_ADDR", "")
	t.Setenv("AUTH_SERVICE_VERSION", "")
	t.Setenv("AUTH_ENV", "")
	t.Setenv("AUTH_DATABASE_URL", "")
	t.Setenv("AUTH_SHUTDOWN_TIMEOUT", "")
	t.Setenv("AUTH_READINESS_TIMEOUT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTPAddr != DefaultHTTPAddr {
		t.Fatalf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if cfg.ServiceVersion != DefaultServiceVersion {
		t.Fatalf("ServiceVersion = %q", cfg.ServiceVersion)
	}
	if cfg.Environment != DefaultEnvironment {
		t.Fatalf("Environment = %q", cfg.Environment)
	}
	if cfg.ShutdownTimeout != DefaultShutdownTimeout {
		t.Fatalf("ShutdownTimeout = %s", cfg.ShutdownTimeout)
	}
	if cfg.ReadinessTimeout != DefaultReadinessTimeout {
		t.Fatalf("ReadinessTimeout = %s", cfg.ReadinessTimeout)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("AUTH_HTTP_ADDR", ":9100")
	t.Setenv("AUTH_SERVICE_VERSION", "0.2.0")
	t.Setenv("AUTH_ENV", "test")
	t.Setenv("AUTH_DATABASE_URL", "postgres://auth:auth@localhost:5432/auth?sslmode=disable")
	t.Setenv("AUTH_SHUTDOWN_TIMEOUT", "5s")
	t.Setenv("AUTH_READINESS_TIMEOUT", "3")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTPAddr != ":9100" || cfg.ServiceVersion != "0.2.0" || cfg.Environment != "test" {
		t.Fatalf("cfg = %+v", cfg)
	}
	if cfg.DatabaseURL == "" {
		t.Fatalf("DatabaseURL is empty")
	}
	if cfg.ShutdownTimeout != 5*time.Second {
		t.Fatalf("ShutdownTimeout = %s", cfg.ShutdownTimeout)
	}
	if cfg.ReadinessTimeout != 3*time.Second {
		t.Fatalf("ReadinessTimeout = %s", cfg.ReadinessTimeout)
	}
}

func TestLoadRejectsInvalidDuration(t *testing.T) {
	t.Setenv("AUTH_SHUTDOWN_TIMEOUT", "nope")

	if _, err := Load(); err == nil {
		t.Fatalf("Load() error = nil")
	}
}
