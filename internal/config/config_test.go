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
}

func TestLoadHonorsOverrides(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://user:pass@host:5432/db")
	t.Setenv("ACCESS_TOKEN_TTL", "30m")
	t.Setenv("BcryptCost", "12")
	t.Setenv("APP_ENV", "staging")

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
