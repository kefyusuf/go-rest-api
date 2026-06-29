FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY cmd/ cmd/
COPY internal/ internal/
COPY docs/ docs/

RUN go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.22

RUN apk add --no-cache ca-certificates wget && adduser -D -u 10001 appuser

WORKDIR /app

COPY --from=builder /out/api /app/api

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=5s --start-period=10s --retries=5 \
  CMD wget -qO- http://127.0.0.1:8080/health/live > /dev/null || exit 1

ENTRYPOINT ["/app/api"]
