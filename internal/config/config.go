// Package config centralises process configuration.
//
// It is the single place where environment variables are read and
// validated. Handlers and stores never call os.Getenv directly; they
// receive a Config value at startup.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port           string
	DatabaseURL    string
	JWTSecret      string
	AccessTokenTTL time.Duration
	BcryptCost     int
	Environment    string
}

func Load() (Config, error) {
	cfg := Config{
		Port:           getenv("PORT", "8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWTSecret"),
		AccessTokenTTL: 15 * time.Minute,
		BcryptCost:     10,
		Environment:    getenv("APP_ENV", "development"),
	}

	if v := os.Getenv("ACCESS_TOKEN_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid ACCESS_TOKEN_TTL: %w", err)
		}
		if d <= 0 {
			return Config{}, errors.New("ACCESS_TOKEN_TTL must be positive")
		}
		cfg.AccessTokenTTL = d
	}

	if v := os.Getenv("BcryptCost"); v != "" {
		c, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid BcryptCost: %w", err)
		}
		if c < 4 || c > 31 {
			return Config{}, errors.New("BcryptCost must be between 4 and 31")
		}
		cfg.BcryptCost = c
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
