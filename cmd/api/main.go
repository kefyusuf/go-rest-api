package main

import (
	"log"
	"net/http"
	"os"

	_ "go-lang/docs"
	"go-lang/internal/database"
	"go-lang/internal/server"
	"go-lang/internal/store"
)

//go:generate C:/Users/yukonit/go/bin/swag init -g main.go -d .,../../internal/handler,../../internal/model -o ../../docs

// @title Go API Starter
// @version 1.0
// @description A clean and beginner-friendly Go API skeleton.
// @host localhost:8080
// @BasePath /
func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	userStore, cleanup, err := buildUserStore()
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()

	addr := ":" + port
	app := server.New(userStore)

	log.Printf("server starting on %s", addr)

	if err := http.ListenAndServe(addr, app); err != nil {
		log.Fatal(err)
	}
}

func buildUserStore() (store.UserStore, func(), error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Println("DATABASE_URL not set, using in-memory user store")
		return store.NewMemoryUserStore(), func() {}, nil
	}

	db, err := database.OpenPostgres()
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
