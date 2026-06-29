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
	"strings"
	"time"
)

type Config struct {
	Port            string
	DatabaseURL     string
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	BcryptCost      int
	Environment     string

	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int

	MaxBodyBytes int64

	ShutdownTimeout time.Duration

	RedisURL  string
	UserCacheTTL time.Duration

	KafkaBrokers      []string
	KafkaTopic        string
	KafkaWriteTimeout time.Duration

	RateLimitPerSecond float64
	RateLimitBurst     float64
	AuthRateLimitPerSecond float64
	AuthRateLimitBurst     float64

	CORSAllowedOrigins []string
	CORSAllowedMethods []string
	CORSAllowedHeaders []string

	IdempotencyTTL time.Duration
}

const (
	defaultPort              = "8080"
	defaultEnv               = "development"
	defaultAccessTokenTTL    = 15 * time.Minute
	defaultRefreshTokenTTL   = 7 * 24 * time.Hour
	defaultBcryptCost        = 10
	defaultReadHeaderTimeout = 5 * time.Second
	defaultReadTimeout       = 15 * time.Second
	defaultWriteTimeout      = 15 * time.Second
	defaultIdleTimeout       = 60 * time.Second
	defaultMaxHeaderBytes    = 1 << 20
	defaultMaxBodyBytes      = 1 << 20
	defaultShutdownTimeout   = 15 * time.Second
	defaultUserCacheTTL       = 5 * time.Minute
	defaultRateLimitPerSecond  = 20
	defaultRateLimitBurst      = 40
	defaultAuthRateLimitPerSecond = 5
	defaultAuthRateLimitBurst     = 10
	defaultIdempotencyTTL          = 24 * time.Hour
)

func Load() (Config, error) {
	cfg := Config{
		Port:             getenv("PORT", defaultPort),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		JWTSecret:        os.Getenv("JWTSecret"),
		AccessTokenTTL:   defaultAccessTokenTTL,
		RefreshTokenTTL:  defaultRefreshTokenTTL,
		BcryptCost:       defaultBcryptCost,
		Environment:      getenv("APP_ENV", defaultEnv),
		ReadHeaderTimeout: defaultReadHeaderTimeout,
		ReadTimeout:       defaultReadTimeout,
		WriteTimeout:      defaultWriteTimeout,
		IdleTimeout:       defaultIdleTimeout,
		MaxHeaderBytes:    defaultMaxHeaderBytes,
		MaxBodyBytes:      defaultMaxBodyBytes,
		ShutdownTimeout:   defaultShutdownTimeout,
		RedisURL:          os.Getenv("REDIS_URL"),
		UserCacheTTL:      defaultUserCacheTTL,
		KafkaBrokers:      nil,
		KafkaTopic:        "go-rest-api.events",
		KafkaWriteTimeout: 10 * time.Second,
		RateLimitPerSecond: defaultRateLimitPerSecond,
		RateLimitBurst:     defaultRateLimitBurst,
		AuthRateLimitPerSecond: defaultAuthRateLimitPerSecond,
		AuthRateLimitBurst:     defaultAuthRateLimitBurst,
		IdempotencyTTL:         defaultIdempotencyTTL,
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

	if v := os.Getenv("REFRESH_TOKEN_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid REFRESH_TOKEN_TTL: %w", err)
		}
		if d <= 0 {
			return Config{}, errors.New("REFRESH_TOKEN_TTL must be positive")
		}
		cfg.RefreshTokenTTL = d
	}

	if v := os.Getenv("USER_CACHE_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid USER_CACHE_TTL: %w", err)
		}
		if d <= 0 {
			return Config{}, errors.New("USER_CACHE_TTL must be positive")
		}
		cfg.UserCacheTTL = d
	}

	if v := os.Getenv("RATE_LIMIT_PER_SECOND"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return Config{}, fmt.Errorf("invalid RATE_LIMIT_PER_SECOND: %w", err)
		}
		if f <= 0 {
			return Config{}, errors.New("RATE_LIMIT_PER_SECOND must be positive")
		}
		cfg.RateLimitPerSecond = f
	}

	if v := os.Getenv("RATE_LIMIT_BURST"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return Config{}, fmt.Errorf("invalid RATE_LIMIT_BURST: %w", err)
		}
		if f <= 0 {
			return Config{}, errors.New("RATE_LIMIT_BURST must be positive")
		}
		cfg.RateLimitBurst = f
	}

	if v := os.Getenv("AUTH_RATE_LIMIT_PER_SECOND"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return Config{}, fmt.Errorf("invalid AUTH_RATE_LIMIT_PER_SECOND: %w", err)
		}
		if f <= 0 {
			return Config{}, errors.New("AUTH_RATE_LIMIT_PER_SECOND must be positive")
		}
		cfg.AuthRateLimitPerSecond = f
	}

	if v := os.Getenv("AUTH_RATE_LIMIT_BURST"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return Config{}, fmt.Errorf("invalid AUTH_RATE_LIMIT_BURST: %w", err)
		}
		if f <= 0 {
			return Config{}, errors.New("AUTH_RATE_LIMIT_BURST must be positive")
		}
		cfg.AuthRateLimitBurst = f
	}

	if v := os.Getenv("CORS_ALLOWED_ORIGINS"); v != "" {
		for _, o := range strings.Split(v, ",") {
			if o = strings.TrimSpace(o); o != "" {
				cfg.CORSAllowedOrigins = append(cfg.CORSAllowedOrigins, o)
			}
		}
	}

	if v := os.Getenv("KAFKA_BROKERS"); v != "" {
		for _, b := range strings.Split(v, ",") {
			if b = strings.TrimSpace(b); b != "" {
				cfg.KafkaBrokers = append(cfg.KafkaBrokers, b)
			}
		}
	}
	if v := os.Getenv("KAFKA_TOPIC"); v != "" {
		cfg.KafkaTopic = v
	}
	if v := os.Getenv("KAFKA_WRITE_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid KAFKA_WRITE_TIMEOUT: %w", err)
		}
		if d <= 0 {
			return Config{}, errors.New("KAFKA_WRITE_TIMEOUT must be positive")
		}
		cfg.KafkaWriteTimeout = d
	}

	if v := os.Getenv("IDEMPOTENCY_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid IDEMPOTENCY_TTL: %w", err)
		}
		if d <= 0 {
			return Config{}, errors.New("IDEMPOTENCY_TTL must be positive")
		}
		cfg.IdempotencyTTL = d
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

	if v := os.Getenv("READ_HEADER_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid READ_HEADER_TIMEOUT: %w", err)
		}
		if d < 0 {
			return Config{}, errors.New("READ_HEADER_TIMEOUT must be non-negative")
		}
		cfg.ReadHeaderTimeout = d
	}

	if v := os.Getenv("READ_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid READ_TIMEOUT: %w", err)
		}
		if d < 0 {
			return Config{}, errors.New("READ_TIMEOUT must be non-negative")
		}
		cfg.ReadTimeout = d
	}

	if v := os.Getenv("WRITE_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid WRITE_TIMEOUT: %w", err)
		}
		if d < 0 {
			return Config{}, errors.New("WRITE_TIMEOUT must be non-negative")
		}
		cfg.WriteTimeout = d
	}

	if v := os.Getenv("IDLE_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid IDLE_TIMEOUT: %w", err)
		}
		if d < 0 {
			return Config{}, errors.New("IDLE_TIMEOUT must be non-negative")
		}
		cfg.IdleTimeout = d
	}

	if v := os.Getenv("MAX_HEADER_BYTES"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid MAX_HEADER_BYTES: %w", err)
		}
		if n <= 0 {
			return Config{}, errors.New("MAX_HEADER_BYTES must be positive")
		}
		cfg.MaxHeaderBytes = n
	}

	if v := os.Getenv("MAX_BODY_BYTES"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("invalid MAX_BODY_BYTES: %w", err)
		}
		if n <= 0 {
			return Config{}, errors.New("MAX_BODY_BYTES must be positive")
		}
		cfg.MaxBodyBytes = n
	}

	if v := os.Getenv("SHUTDOWN_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid SHUTDOWN_TIMEOUT: %w", err)
		}
		if d <= 0 {
			return Config{}, errors.New("SHUTDOWN_TIMEOUT must be positive")
		}
		cfg.ShutdownTimeout = d
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func (c Config) Validate() error {
	if c.AccessTokenTTL <= 0 {
		return errors.New("AccessTokenTTL must be positive")
	}
	if c.RefreshTokenTTL <= 0 {
		return errors.New("RefreshTokenTTL must be positive")
	}
	if c.BcryptCost < 4 || c.BcryptCost > 31 {
		return errors.New("BcryptCost must be between 4 and 31")
	}
	if len(c.JWTSecret) < 32 {
		return errors.New("JWTSecret must be at least 32 bytes")
	}
	return nil
}
