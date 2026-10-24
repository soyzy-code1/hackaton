#!/usr/bin/env bash
# =============================================================================
#  Invoice Keeper — healthcheck стенда для жюри
#  Проверяет: контейнеры, HTTP-эндпоинты, PostgreSQL, Redis, фоновые задачи.
#
#  Использование:
#      bash scripts/healthcheck.sh          # полная проверка
#      bash scripts/healthcheck.sh --quick  # без обращений к БД/Redis
# =============================================================================
set -uo pipefail

QUICK=0
[ "${1:-}" = "--quick" ] && QUICK=1

PASS=0
FAIL=0
SKIP=0

GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'; BLUE='\033[0;36m'; NC='\033[0m'

ok()   { printf "  ${GREEN}[OK]${NC}   %s\n" "$1"; PASS=$((PASS + 1)); }
err()  { printf "  ${RED}[FAIL]${NC} %s\n" "$1"; FAIL=$((FAIL + 1)); }
warn() { printf "  ${YELLOW}[WARN]${NC} %s\n" "$1"; SKIP=$((SKIP + 1)); }
sec()  { printf "\n${BLUE}== %s ==${NC}\n" "$1"; }

docker_cmd() {
    if docker compose version >/dev/null 2>&1; then
        docker compose "$@"
    else
        docker-compose "$@"
    fi
}

# -----------------------------------------------------------------------------
sec "1. Состояние контейнеров"
# -----------------------------------------------------------------------------
CONTAINERS=(invoice_db invoice_redis invoice_backend invoice_frontend invoice_bot invoice_nginx)
for c in "${CONTAINERS[@]}"; do
    state="$(docker inspect -f '{{.State.Status}}' "$c" 2>/dev/null || echo 'missing')"
    health="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}n/a{{end}}' "$c" 2>/dev/null || echo 'n/a')"
    if [ "$state" = "running" ]; then
        ok "$c — running (health: $health)"
    else
        err "$c — $state"
    fi
done

# -----------------------------------------------------------------------------
sec "2. HTTP-эндпоинты"
# -----------------------------------------------------------------------------
BASE="${BASE_URL:-http://localhost}"
FRONT="${FRONTEND_URL:-http://localhost:3000}"

check_http() {
    local url="$1" expect="$2" label="$3"
    local code
    code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$url" 2>/dev/null || echo 000)"
    if [ "$code" = "$expect" ] || { [ "$expect" = "2xx" ] && [[ "$code" == 2* ]]; } \
       || { [ "$expect" = "3xx" ] && [[ "$code" == 3* ]]; }; then
        ok "$label — HTTP $code ($url)"
    else
        err "$label — HTTP $code, ожидался $expect ($url)"
    fi
}

check_http "$BASE/healthz"        "200"   "nginx healthz"
check_http "$FRONT/"             "200"   "frontend (SPA)"
check_http "$BASE/api/v1/docs"    "200"   "Swagger UI (/api/v1/docs)"
check_http "$BASE/api/v1/openapi.json" "2xx" "OpenAPI schema"
check_http "$BASE/api/"           "2xx"   "API root (/api/)"

# -----------------------------------------------------------------------------
if [ "$QUICK" -eq 0 ]; then
    # -------------------------------------------------------------------------
    sec "3. PostgreSQL"
    # -------------------------------------------------------------------------
    if docker exec invoice_db pg_isready -U "${POSTGRES_USER:-invoice_user}" >/dev/null 2>&1; then
        ok "PostgreSQL отвечает на pg_isready"
        # Список таблиц — признак, что миграции применены
        tables="$(docker exec invoice_db psql -U "${POSTGRES_USER:-invoice_user}" -d "${POSTGRES_DB:-invoice_keeper}" \
                    -tAc "SELECT count(*) FROM information_schema.tables WHERE table_schema='public';" 2>/dev/null || echo 0)"
        if [ "${tables:-0}" -gt 0 ]; then
            ok "В схеме public найдено таблиц: ${tables} (миграции применены)"
        else
            warn "В схеме public нет таблиц — миграции не применены или используются другие схемы"
        fi
    else
        err "PostgreSQL не отвечает"
    fi

    # -------------------------------------------------------------------------
    sec "4. Redis и фоновые задачи"
    # -------------------------------------------------------------------------
    if docker exec invoice_redis redis-cli ping 2>/dev/null | grep -q PONG; then
        ok "Redis отвечает на PING"
        keys="$(docker exec invoice_redis redis-cli dbsize 2>/dev/null | tr -d '\r' || echo 0)"
        ok "Redis: ключей в БД — ${keys}"

        # Проверка наличия очередей/планировщика (имена зависят от реализации бота)
        found_queue=0
        for pat in 'celery*' 'rq:*' 'arq*' 'reminder*' 'schedule*' 'queue*' 'tasks*'; do
            n="$(docker exec invoice_redis redis-cli --scan --pattern "${pat}" 2>/dev/null | head -n 5 | wc -l | tr -d ' ')"
            if [ "${n:-0}" -gt 0 ]; then
                ok "Найдены ключи очередей/задач по паттерну '${pat}' (${n})"
                found_queue=1
            fi
        done
        [ "$found_queue" -eq 0 ] && warn "Ключей очередей не найдено — бот ещё не создал задачи (или используется APScheduler/внутрипроцессный планировщик)"
    else
        err "Redis не отвечает на PING"
    fi

    # -------------------------------------------------------------------------
    sec "5. Логи бота (последние строки)"
    # -------------------------------------------------------------------------
    echo "--- docker logs --tail 15 invoice_bot ---"
    docker logs --tail 15 invoice_bot 2>&1 | sed 's/^/    /'
    echo "--------------------------------------------"
fi

# -----------------------------------------------------------------------------
sec "Итог"
# -----------------------------------------------------------------------------
printf "  Успешно: %s${GREEN}   Провалено: %s${RED}   Предупреждений: %s${YELLOW}${NC}\n" "$PASS" "$FAIL" "$SKIP"
echo

if [ "$FAIL" -gt 0 ]; then
    echo "Подробности логов:  docker compose logs -f backend bot nginx"
    exit 1
fi
exit 0
