FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
RUN go install github.com/swaggo/swag/cmd/swag@latest
COPY . .
RUN /go/bin/swag init -g main.go -d ./cmd/api,./internal/handler,./internal/model -o docs
RUN go build -o /app/bin/api ./cmd/api

FROM alpine:3.22
WORKDIR /app
RUN apk add --no-cache wget \
	&& adduser -D appuser
COPY --from=builder /app/bin/api /app/api
USER appuser
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=5s --start-period=10s --retries=5 \
  CMD wget -qO- http://127.0.0.1:8080/health/live > /dev/null || exit 1
EXPOSE 8080
EXPOSE 9090
ENTRYPOINT ["/app/api"]
