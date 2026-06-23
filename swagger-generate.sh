#!/usr/bin/env sh
set -eu
swag init -g main.go -d ./cmd/api,./internal/handler,./internal/model -o ./docs
