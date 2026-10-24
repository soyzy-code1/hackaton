#!/usr/bin/env bash
# Invoice Keeper — единая диагностика перед демонстрацией.
# Запуск из корня проекта: ./scripts/check-stand.sh
set -euo pipefail

echo "== compose ps =="
docker compose ps

echo "== frontend =="
curl -sf -o /dev/null -w "frontend: %{http_code}\n" http://localhost/

echo "== swagger =="
curl -sf -o /dev/null -w "swagger:  %{http_code}\n" http://localhost/api/v1/docs

echo "== openapi =="
curl -sf -o /dev/null -w "openapi:  %{http_code}\n" http://localhost/api/v1/openapi.json

echo "== login =="
# В .env используется ADMIN_LOGIN/ADMIN_PASSWORD, а Go-бэкенд создаёт admin
# при первой миграции в SQLite volume invoice_backenddata.
curl -sf -X POST http://localhost/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d "{\"login\":\"${ADMIN_LOGIN:-admin}\",\"password\":\"${ADMIN_PASSWORD:-admin}\"}" \
  -o /dev/null -w "login:    %{http_code}\n" || echo "login: FAILED"

echo "== bot logs (last 20) =="
docker compose logs --tail=20 bot

echo "== backend logs (last 20) =="
docker compose logs --tail=20 backend

echo "== botdata =="
# Бот может быть перезапускаемым при блокировке Telegram API, поэтому проверка
# volume не делает диагностику fail-fast.
docker compose exec -T bot sh -c 'ls -la /app/data 2>/dev/null || true; ls -la /data 2>/dev/null || true'

echo "== bot status =="
docker compose ps bot
echo "== bot last 10 lines =="
docker compose logs --tail=10 bot

echo "== telegram chat validation =="
CHAT_ID="$(docker compose exec -T bot sh -c 'printf "%s" "$TELEGRAM_CHAT_ID"')"
case "$CHAT_ID" in
  -1001234567890|""|0)
    echo "WARNING: TELEGRAM_CHAT_ID is a placeholder or empty — reminders will fail" ;;
  *)
    echo "chat_id set: yes" ;;
esac
