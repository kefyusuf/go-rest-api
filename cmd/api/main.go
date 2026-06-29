package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	_ "go-lang/docs"
	"go-lang/internal/auth"
	cacheimpl "go-lang/internal/cache"
	"go-lang/internal/config"
	"go-lang/internal/database"
	"go-lang/internal/observability"
	"go-lang/internal/ratelimit"
	"go-lang/internal/server"
	"go-lang/internal/store"
	"fmt"

	"github.com/redis/go-redis/v9"
)

//go:generate swag init -g main.go -d .,../../internal/handler,../../internal/model -o ../../docs

// @title Go API Starter
// @version 1.0
// @description A clean and beginner-friendly Go API skeleton.
// @host localhost:8080
// @BasePath /
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if err := cfg.Validate(); err != nil {
		logger.Error("invalid config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	issuer, err := auth.NewTokenIssuer(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.Environment, auth.KindAccess)
	if err != nil {
		logger.Error("failed to build token issuer", slog.String("error", err.Error()))
		os.Exit(1)
	}

	refreshIssuer, err := auth.NewTokenIssuer(cfg.JWTSecret, cfg.RefreshTokenTTL, cfg.Environment, auth.KindRefresh)
	if err != nil {
		logger.Error("failed to build refresh token issuer", slog.String("error", err.Error()))
		os.Exit(1)
	}

	userStore, cleanup, pinger, err := buildUserStore(cfg, logger)
	if err != nil {
		logger.Error("failed to build user store", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer cleanup()

	cacheImpl, cacheClose, err := buildCache(cfg, logger)
	if err != nil {
		logger.Error("failed to build cache", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if cacheClose != nil {
		defer cacheClose()
	}
	userCache := cacheimpl.NewUserCache(cacheImpl, cfg.UserCacheTTL)
	cachedStore := store.NewCachedUserStore(userStore, userCache, cfg.UserCacheTTL)

	metrics := observability.NewMetrics("go-rest-api", cfg.Environment)
	probes := observability.NewHealthProbes("go-rest-api", "1.0.0", cfg.Environment)

	globalLimiter := ratelimit.New(cfg.RateLimitPerSecond, cfg.RateLimitBurst)
	authLimiter := ratelimit.New(cfg.AuthRateLimitPerSecond, cfg.AuthRateLimitBurst)

	addr := ":" + cfg.Port
	app := server.New(cachedStore, logger, server.Options{
		MaxBodyBytes:  cfg.MaxBodyBytes,
		TokenIssuer:   issuer,
		RefreshIssuer: refreshIssuer,
		Blacklist:     auth.NewBlacklist(),
		BcryptCost:    cfg.BcryptCost,
		Metrics:       metrics,
		HealthProbes:  probes,
		DBPinger:      pinger,
		GlobalLimiter: globalLimiter,
		AuthLimiter:   authLimiter,
		CORS: server.CORSConfig{
			AllowedOrigins: cfg.CORSAllowedOrigins,
		},
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           app,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("server starting", slog.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case err := <-serverErr:
		if err != nil {
			logger.Error("server stopped with error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining connections")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		logger.Info("server stopped cleanly")
	}
}

func buildUserStore(cfg config.Config, logger *slog.Logger) (store.UserStore, func(), observability.Pinger, error) {
	if cfg.DatabaseURL == "" {
		logger.Warn("DATABASE_URL not set, using in-memory user store")
		return store.NewMemoryUserStore(), func() {}, nil, nil
	}

	db, err := database.OpenPostgres(cfg.DatabaseURL)
	if err != nil {
		return nil, nil, nil, err
	}

	if err := database.RunMigrations(db); err != nil {
		db.Close()
		return nil, nil, nil, err
	}

	pinger := &sqlPinger{db: db}
	return store.NewPostgresUserStore(db), func() {
			db.Close()
		}, pinger, nil
}

type sqlPinger struct {
	db pingerDB
}

type pingerDB interface {
	PingContext(ctx context.Context) error
}

func (p *sqlPinger) PingContext(ctx context.Context) error {
	return p.db.PingContext(ctx)
}

func buildCache(cfg config.Config, logger *slog.Logger) (cacheimpl.Cache, func(), error) {
	if cfg.RedisURL == "" {
		logger.Warn("REDIS_URL not set, using in-memory cache (per-instance, lost on restart)")
		return cacheimpl.NewMemoryCache(), nil, nil
	}
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid REDIS_URL: %w", err)
	}
	rc := cacheimpl.NewRedisCache(opts.Addr, opts.Password, opts.DB)
	return rc, func() { _ = rc.Close() }, nil
}
