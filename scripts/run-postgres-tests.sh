#!/usr/bin/env sh
set -eu
sh scripts/test-db-up.sh
DATABASE_URL=postgres://postgres:postgres@localhost:5433/go_lang_test?sslmode=disable go test ./...
