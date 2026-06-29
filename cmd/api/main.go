package main

import (
	"log/slog"
	"net/http"
	"os"

	_ "go-lang/docs"
	"go-lang/internal/config"
	"go-lang/internal/database"
	"go-lang/internal/server"
	"go-lang/internal/store"
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

	userStore, cleanup, err := buildUserStore(cfg, logger)
	if err != nil {
		logger.Error("failed to build user store", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer cleanup()

	addr := ":" + cfg.Port
	app := server.New(userStore, logger)

	logger.Info("server starting", slog.String("addr", addr))

	if err := http.ListenAndServe(addr, app); err != nil {
		logger.Error("server stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func buildUserStore(cfg config.Config, logger *slog.Logger) (store.UserStore, func(), error) {
	if cfg.DatabaseURL == "" {
		logger.Warn("DATABASE_URL not set, using in-memory user store")
		return store.NewMemoryUserStore(), func() {}, nil
	}

	db, err := database.OpenPostgres(cfg.DatabaseURL)
	if err != nil {
		return nil, nil, err
	}

	if err := database.RunMigrations(db); err != nil {
		db.Close()
		return nil, nil, err
	}

	return store.NewPostgresUserStore(db), func() {
		db.Close()
	}, nil
}
