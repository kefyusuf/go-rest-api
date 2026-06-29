#!/usr/bin/env sh
set -eu
docker compose -f docker-compose.test.yml up -d

container=go-lang-postgres-test
attempt=0
while [ "$attempt" -lt 30 ]; do
  health="$(docker inspect -f '{{.State.Health.Status}}' "$container" 2>/dev/null || true)"
  if [ "$health" = "healthy" ]; then
    exit 0
  fi
  attempt=$((attempt + 1))
  sleep 2
done

echo "postgres test database did not become healthy" >&2
docker compose -f docker-compose.test.yml ps
exit 1
