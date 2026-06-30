package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "go-lang/docs"
	"go-lang/internal/auth"
	cacheimpl "go-lang/internal/cache"
	"go-lang/internal/config"
	"go-lang/internal/database"
	"go-lang/internal/events"
	"go-lang/internal/handler"
	"go-lang/internal/idempotency"
	"go-lang/internal/jobs"
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

	userStore, db, cleanup, pinger, err := buildUserStore(cfg, logger)
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

	blacklist, blacklistClose, err := buildBlacklist(cfg, logger)
	if err != nil {
		logger.Error("failed to build blacklist", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if blacklistClose != nil {
		defer blacklistClose()
	}

	resetTokenStore, resetTokenClose, err := buildResetTokenStore(cfg, logger)
	if err != nil {
		logger.Error("failed to build reset-token store", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if resetTokenClose != nil {
		defer resetTokenClose()
	}
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

	globalLimiter, globalLimiterClose, err := buildRateLimiter("global", cfg.RateLimitPerSecond, cfg.RateLimitBurst, cfg, logger)
	if err != nil {
		logger.Error("failed to build global rate limiter", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer globalLimiterClose()

	authLimiter, authLimiterClose, err := buildRateLimiter("auth", cfg.AuthRateLimitPerSecond, cfg.AuthRateLimitBurst, cfg, logger)
	if err != nil {
		logger.Error("failed to build auth rate limiter", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer authLimiterClose()

	idempStore, idempStoreClose, err := buildIdempotencyStore(cfg, logger)
	if err != nil {
		logger.Error("failed to build idempotency store", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if idempStoreClose != nil {
		defer idempStoreClose()
	}

	outbox, _, err := buildOutbox(db, logger)
	if err != nil {
		logger.Error("failed to build outbox", slog.String("error", err.Error()))
		os.Exit(1)
	}
	var publisher events.Publisher
	var kafkaClose func() error
	if len(cfg.KafkaBrokers) > 0 {
		kp, err := events.NewKafkaPublisher(events.KafkaConfig{
			Brokers:      cfg.KafkaBrokers,
			Topic:        cfg.KafkaTopic,
			WriteTimeout: cfg.KafkaWriteTimeout,
		}, logger)
		if err != nil {
			logger.Error("failed to build kafka publisher, falling back to logging", slog.String("error", err.Error()))
			publisher = events.NewLoggingPublisher(logger)
		} else {
			publisher = kp
			kafkaClose = kp.Close
			logger.Info("kafka publisher enabled",
				slog.String("topic", cfg.KafkaTopic),
				slog.Int("brokers", len(cfg.KafkaBrokers)))
		}
	} else {
		publisher = events.NewLoggingPublisher(logger)
		logger.Warn("KAFKA_BROKERS not set, using in-memory LoggingPublisher")
	}
	dispatcher := events.NewDispatcher(outbox, publisher, logger)
	dispatcherCtx, cancelDispatcher := context.WithCancel(context.Background())
	defer cancelDispatcher()
	go dispatcher.Run(dispatcherCtx)

	addr := ":" + cfg.Port
	app := server.New(cachedStore, logger, server.Options{
		MaxBodyBytes:    cfg.MaxBodyBytes,
		TokenIssuer:     issuer,
		RefreshIssuer:   refreshIssuer,
		Blacklist:       blacklist,
		BcryptCost:      cfg.BcryptCost,
		Metrics:         metrics,
		HealthProbes:    probes,
		DBPinger:        pinger,
		GlobalLimiter:   globalLimiter,
		AuthLimiter:     authLimiter,
		IdempotencyStore: idempStore,
		ResetTokens:     resetTokenStore,
		Outbox:           outbox,
		CORS: server.CORSConfig{
			AllowedOrigins: cfg.CORSAllowedOrigins,
		},
	})

	jobQueue, jobQueueClose, err := buildJobQueue(cfg, logger)
	if err != nil {
		logger.Error("failed to build job queue", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if jobQueueClose != nil {
		defer jobQueueClose()
	}

	jobDead, jobDeadClose, err := buildDeadLetter(cfg, logger)
	if err != nil {
		logger.Error("failed to build dead-letter list", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if jobDeadClose != nil {
		defer jobDeadClose()
	}
	jobReg := jobs.NewRegistry(jobQueue, jobDead, logger)
	jobReg.Register("welcome_email", jobs.HandlerFunc(func(_ context.Context, _ jobs.Job) error {
		logger.Info("welcome email job ran (mock)")
		return nil
	}))

	jobCtx, cancelJobs := context.WithCancel(context.Background())
	defer cancelJobs()
	jobReg.Start(jobCtx, 2)

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

	cancelJobs()
	jobReg.Stop()
	cancelDispatcher()
	_ = outbox.Close()
	if kafkaClose != nil {
		_ = kafkaClose()
	} else {
		_ = publisher.Close()
	}
}

func buildUserStore(cfg config.Config, logger *slog.Logger) (store.UserStore, *sql.DB, func(), observability.Pinger, error) {
	if cfg.DatabaseURL == "" {
		logger.Warn("DATABASE_URL not set, using in-memory user store")
		return store.NewMemoryUserStore(), nil, func() {}, nil, nil
	}

	db, err := database.OpenPostgres(cfg.DatabaseURL)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	if err := database.RunMigrations(db); err != nil {
		db.Close()
		return nil, nil, nil, nil, err
	}

	pinger := &sqlPinger{db: db}
	return store.NewPostgresUserStore(db), db, func() { db.Close() }, pinger, nil
}

// buildOutbox returns a database-backed outbox when DATABASE_URL is
// set, and the in-memory outbox when it is not. The DBOutbox is
// durable (events survive a process restart) and multi-replica
// safe (FOR UPDATE SKIP LOCKED prevents two dispatchers from
// picking the same row). The closer here is a no-op because the
// caller owns the *sql.DB.
func buildOutbox(db *sql.DB, logger *slog.Logger) (events.Outbox, func(), error) {
	if db == nil {
		return events.NewOutbox(), func() {}, nil
	}
	box := events.NewDBOutbox(db)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := box.EnsureSchema(ctx); err != nil {
		logger.Warn("failed to ensure outbox schema, falling back to in-memory outbox",
			slog.String("error", err.Error()))
		return events.NewOutbox(), func() {}, nil
	}
	logger.Info("database-backed outbox enabled")
	return box, func() {}, nil
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

// buildRateLimiter picks an in-memory or Redis-backed token-bucket
// limiter based on whether REDIS_URL is set. The Redis path returns
// the client so the caller can close it on shutdown.
func buildRateLimiter(name string, rate, burst float64, cfg config.Config, logger *slog.Logger) (ratelimit.Limiter, func(), error) {
	if cfg.RedisURL == "" {
		return ratelimit.New(rate, burst), func() {}, nil
	}
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid REDIS_URL for %s limiter: %w", name, err)
	}
	client := redis.NewClient(opts)
	// Light-touch ping; if Redis is unreachable we fall back to
	// the in-memory limiter so the API still serves traffic.
	pingCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		logger.Warn("REDIS_URL set but redis is unreachable, using in-memory limiter",
			slog.String("limiter", name),
			slog.String("error", err.Error()))
		_ = client.Close()
		return ratelimit.New(rate, burst), func() {}, nil
	}
	logger.Info("redis-backed rate limiter enabled",
		slog.String("limiter", name),
		slog.String("addr", opts.Addr))
	// We do not prefix with a per-name key here because the
	// caller passes a key; the RedisLimiter keyFor() adds the
	// 'ratelimit:' prefix. Use a unique bucket per limiter via
	// the key name (the HTTP middleware uses the client IP, so
	// we need a per-limiter prefix; left for the middleware to
	// handle — we use the name as a sub-namespace).
	rl := ratelimit.NewRedis(client, rate, burst)
	return rl, func() { _ = rl.Close() }, nil
}

// buildBlacklist returns the in-memory blacklist when REDIS_URL is
// empty, and a Redis-backed blacklist when it is set. The Redis
// implementation does not own its client; the closer here closes
// the client.
func buildBlacklist(cfg config.Config, logger *slog.Logger) (auth.Blacklist, func(), error) {
	if cfg.RedisURL == "" {
		return auth.NewBlacklist(), func() {}, nil
	}
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid REDIS_URL for blacklist: %w", err)
	}
	client := redis.NewClient(opts)
	pingCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		logger.Warn("REDIS_URL set but redis is unreachable, using in-memory blacklist",
			slog.String("error", err.Error()))
		_ = client.Close()
		return auth.NewBlacklist(), func() {}, nil
	}
	logger.Info("redis-backed token blacklist enabled", slog.String("addr", opts.Addr))
	bl := auth.NewRedisBlacklist(client)
	return bl, func() { _ = client.Close() }, nil
}

// buildJobQueue returns the in-memory queue when REDIS_URL is empty,
// and a Redis-Streams-backed queue when it is set. The Streams
// implementation gives the queue durability (jobs survive a process
// restart) and a natural multi-consumer fan-out via consumer
// groups. The closer here closes the underlying Redis client.
func buildJobQueue(cfg config.Config, logger *slog.Logger) (jobs.Queue, func(), error) {
	if cfg.RedisURL == "" {
		return jobs.NewMemoryQueue(), func() {}, nil
	}
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid REDIS_URL for job queue: %w", err)
	}
	client := redis.NewClient(opts)
	pingCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		logger.Warn("REDIS_URL set but redis is unreachable, using in-memory job queue",
			slog.String("error", err.Error()))
		_ = client.Close()
		return jobs.NewMemoryQueue(), func() {}, nil
	}
	logger.Info("redis-backed job queue enabled", slog.String("addr", opts.Addr))
	q, err := jobs.NewRedisQueue(client, jobs.RedisConfig{})
	if err != nil {
		_ = client.Close()
		return nil, nil, fmt.Errorf("build redis queue: %w", err)
	}
	return q, func() { _ = client.Close() }, nil
}

// buildDeadLetter returns the in-memory dead-letter list when
// REDIS_URL is empty, and a Redis-backed list when it is set.
// The Redis implementation does not own its client; the closer
// here closes the client.
func buildDeadLetter(cfg config.Config, logger *slog.Logger) (jobs.DeadLetter, func(), error) {
	if cfg.RedisURL == "" {
		return jobs.NewMemoryDeadLetter(), func() {}, nil
	}
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid REDIS_URL for dead-letter list: %w", err)
	}
	client := redis.NewClient(opts)
	pingCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		logger.Warn("REDIS_URL set but redis is unreachable, using in-memory dead-letter list",
			slog.String("error", err.Error()))
		_ = client.Close()
		return jobs.NewMemoryDeadLetter(), func() {}, nil
	}
	logger.Info("redis-backed dead-letter list enabled", slog.String("addr", opts.Addr))
	dl := jobs.NewRedisDeadLetter(client)
	return dl, func() { _ = client.Close() }, nil
}

// buildIdempotencyStore returns the in-memory store when REDIS_URL is
// empty, and a Redis-backed store when it is set. The Redis
// implementation does not own its client; the closer here closes
// the client.
func buildIdempotencyStore(cfg config.Config, logger *slog.Logger) (idempotency.Store, func(), error) {
	if cfg.RedisURL == "" {
		return idempotency.NewMemoryStore(cfg.IdempotencyTTL), func() {}, nil
	}
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid REDIS_URL for idempotency: %w", err)
	}
	client := redis.NewClient(opts)
	pingCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		logger.Warn("REDIS_URL set but redis is unreachable, using in-memory idempotency store",
			slog.String("error", err.Error()))
		_ = client.Close()
		return idempotency.NewMemoryStore(cfg.IdempotencyTTL), func() {}, nil
	}
	logger.Info("redis-backed idempotency store enabled", slog.String("addr", opts.Addr))
	store := idempotency.NewRedisStore(client, cfg.IdempotencyTTL)
	return store, func() { _ = client.Close() }, nil
}

// buildResetTokenStore returns the in-memory implementation when
// REDIS_URL is empty, and a Redis-backed implementation when it
// is set. The Redis implementation does not own its client; the
// closer here closes the client.
func buildResetTokenStore(cfg config.Config, logger *slog.Logger) (handler.TokenStore, func(), error) {
	if cfg.RedisURL == "" {
		return handler.NewMemoryResetTokenStore(), func() {}, nil
	}
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid REDIS_URL for reset-token store: %w", err)
	}
	client := redis.NewClient(opts)
	pingCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		logger.Warn("REDIS_URL set but redis is unreachable, using in-memory reset-token store",
			slog.String("error", err.Error()))
		_ = client.Close()
		return handler.NewMemoryResetTokenStore(), func() {}, nil
	}
	logger.Info("redis-backed reset-token store enabled", slog.String("addr", opts.Addr))
	store := handler.NewRedisResetTokenStore(client)
	return store, func() { _ = client.Close() }, nil
}
