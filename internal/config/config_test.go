package config

import (
	"testing"
	"time"
)

func TestLoadAppliesDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("ACCESS_TOKEN_TTL", "")
	t.Setenv("BcryptCost", "")
	t.Setenv("READ_HEADER_TIMEOUT", "")
	t.Setenv("READ_TIMEOUT", "")
	t.Setenv("WRITE_TIMEOUT", "")
	t.Setenv("IDLE_TIMEOUT", "")
	t.Setenv("MAX_HEADER_BYTES", "")
	t.Setenv("MAX_BODY_BYTES", "")
	t.Setenv("SHUTDOWN_TIMEOUT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != "8080" {
		t.Fatalf("expected default port 8080, got %q", cfg.Port)
	}
	if cfg.AccessTokenTTL != 15*time.Minute {
		t.Fatalf("expected default 15m access token TTL, got %s", cfg.AccessTokenTTL)
	}
	if cfg.BcryptCost != 10 {
		t.Fatalf("expected default bcrypt cost 10, got %d", cfg.BcryptCost)
	}
	if cfg.Environment != "development" {
		t.Fatalf("expected default environment development, got %q", cfg.Environment)
	}
	if cfg.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("expected default read header timeout 5s, got %s", cfg.ReadHeaderTimeout)
	}
	if cfg.ReadTimeout != 15*time.Second {
		t.Fatalf("expected default read timeout 15s, got %s", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 15*time.Second {
		t.Fatalf("expected default write timeout 15s, got %s", cfg.WriteTimeout)
	}
	if cfg.IdleTimeout != 60*time.Second {
		t.Fatalf("expected default idle timeout 60s, got %s", cfg.IdleTimeout)
	}
	if cfg.MaxHeaderBytes != 1<<20 {
		t.Fatalf("expected default max header bytes 1MiB, got %d", cfg.MaxHeaderBytes)
	}
	if cfg.MaxBodyBytes != 1<<20 {
		t.Fatalf("expected default max body bytes 1MiB, got %d", cfg.MaxBodyBytes)
	}
	if cfg.ShutdownTimeout != 15*time.Second {
		t.Fatalf("expected default shutdown timeout 15s, got %s", cfg.ShutdownTimeout)
	}
}

func TestLoadHonorsOverrides(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://user:pass@host:5432/db")
	t.Setenv("ACCESS_TOKEN_TTL", "30m")
	t.Setenv("BcryptCost", "12")
	t.Setenv("APP_ENV", "staging")
	t.Setenv("READ_HEADER_TIMEOUT", "2s")
	t.Setenv("READ_TIMEOUT", "10s")
	t.Setenv("WRITE_TIMEOUT", "20s")
	t.Setenv("IDLE_TIMEOUT", "2m")
	t.Setenv("MAX_HEADER_BYTES", "4096")
	t.Setenv("MAX_BODY_BYTES", "65536")
	t.Setenv("SHUTDOWN_TIMEOUT", "30s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != "9090" {
		t.Fatalf("expected port 9090, got %q", cfg.Port)
	}
	if cfg.DatabaseURL != "postgres://user:pass@host:5432/db" {
		t.Fatalf("unexpected database url, got %q", cfg.DatabaseURL)
	}
	if cfg.AccessTokenTTL != 30*time.Minute {
		t.Fatalf("expected 30m access token TTL, got %s", cfg.AccessTokenTTL)
	}
	if cfg.BcryptCost != 12 {
		t.Fatalf("expected bcrypt cost 12, got %d", cfg.BcryptCost)
	}
	if cfg.Environment != "staging" {
		t.Fatalf("expected staging environment, got %q", cfg.Environment)
	}
	if cfg.ReadHeaderTimeout != 2*time.Second {
		t.Fatalf("expected read header timeout 2s, got %s", cfg.ReadHeaderTimeout)
	}
	if cfg.ReadTimeout != 10*time.Second {
		t.Fatalf("expected read timeout 10s, got %s", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 20*time.Second {
		t.Fatalf("expected write timeout 20s, got %s", cfg.WriteTimeout)
	}
	if cfg.IdleTimeout != 2*time.Minute {
		t.Fatalf("expected idle timeout 2m, got %s", cfg.IdleTimeout)
	}
	if cfg.MaxHeaderBytes != 4096 {
		t.Fatalf("expected max header bytes 4096, got %d", cfg.MaxHeaderBytes)
	}
	if cfg.MaxBodyBytes != 65536 {
		t.Fatalf("expected max body bytes 65536, got %d", cfg.MaxBodyBytes)
	}
	if cfg.ShutdownTimeout != 30*time.Second {
		t.Fatalf("expected shutdown timeout 30s, got %s", cfg.ShutdownTimeout)
	}
}

func TestLoadRejectsInvalidAccessTokenTTL(t *testing.T) {
	t.Setenv("ACCESS_TOKEN_TTL", "not-a-duration")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid ACCESS_TOKEN_TTL, got nil")
	}
}

func TestLoadRejectsInvalidBcryptCost(t *testing.T) {
	t.Setenv("BcryptCost", "99")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for out-of-range BcryptCost, got nil")
	}
}

func TestLoadRejectsNonPositiveMaxHeaderBytes(t *testing.T) {
	t.Setenv("MAX_HEADER_BYTES", "0")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for non-positive MAX_HEADER_BYTES, got nil")
	}
}

func TestLoadRejectsNonPositiveMaxBodyBytes(t *testing.T) {
	t.Setenv("MAX_BODY_BYTES", "0")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for non-positive MAX_BODY_BYTES, got nil")
	}
}

func TestLoadRejectsNonPositiveShutdownTimeout(t *testing.T) {
	t.Setenv("SHUTDOWN_TIMEOUT", "0s")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for non-positive SHUTDOWN_TIMEOUT, got nil")
	}
}
