@echo off
call scripts\test-db-up.bat
set "DATABASE_URL=postgres://postgres:postgres@localhost:5433/go_lang_test?sslmode=disable"
go test ./...
