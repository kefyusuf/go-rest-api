@echo off
setlocal EnableDelayedExpansion
docker compose -f docker-compose.test.yml up -d

set "CONTAINER=go-lang-postgres-test"
for /L %%i in (1,1,30) do (
  for /F "usebackq delims=" %%s in (`docker inspect -f "{{.State.Health.Status}}" %CONTAINER% 2^>nul`) do set "HEALTH=%%s"
  if "!HEALTH!"=="healthy" exit /B 0
  timeout /T 2 /NOBREAK >nul
)

echo postgres test database did not become healthy
docker compose -f docker-compose.test.yml ps
exit /B 1
