#!/usr/bin/env bash
# =============================================================================
#  Invoice Keeper — init-db.sh
# -----------------------------------------------------------------------------
#  Назначение:
#    1) дождаться готовности PostgreSQL (db:5432) и Redis (redis:6379);
#    2) убедиться, что Redis действительно отвечает (а не просто открыт порт);
#    3) (опционально) применить миграции/инициализацию схемы.
#
#  Запускается:
#    * автоматически из entrypoint-backend.sh и entrypoint-bot.sh;
#    * вручную:  docker compose run --rm backend /scripts/init-db.sh
#
#  Переменные окружения:
#    DB_HOST, DB_PORT, DB_NAME, DB_USER, DB_PASSWORD
#    REDIS_HOST, REDIS_PORT, REDIS_URL, DATABASE_URL
#    WAIT_TIMEOUT  — сколько секунд ждать (по умолчанию 120)
#    RUN_MIGRATIONS — 1 (по умолчанию) / 0
# =============================================================================
set -euo pipefail

DB_HOST="${DB_HOST:-db}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-${POSTGRES_DB:-invoice_keeper}}"
DB_USER="${DB_USER:-${POSTGRES_USER:-invoice_user}}"
DB_PASSWORD="${DB_PASSWORD:-${POSTGRES_PASSWORD:-}}"

REDIS_HOST="${REDIS_HOST:-redis}"
REDIS_PORT="${REDIS_PORT:-6379}"
REDIS_URL="${REDIS_URL:-redis://${REDIS_HOST}:${REDIS_PORT}/0}"

WAIT_TIMEOUT="${WAIT_TIMEOUT:-120}"
RUN_MIGRATIONS="${RUN_MIGRATIONS:-1}"

log()  { printf '[init-db] %s %s\n' "$(date '+%H:%M:%S')" "$*"; }
fail() { printf '[init-db] ОШИБКА: %s\n' "$*" >&2; exit 1; }

# -----------------------------------------------------------------------------
# 1. Ожидание TCP-готовности PostgreSQL
# -----------------------------------------------------------------------------
wait_for_tcp() {
    local host="$1" port="$2" name="$3" waited=0
    log "Ожидание готовности ${name} (${host}:${port}), таймаут ${WAIT_TIMEOUT}s..."
    while true; do
        if python - "$host" "$port" <<'PY' 2>/dev/null
import socket, sys
host, port = sys.argv[1], int(sys.argv[2])
s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
s.settimeout(3)
try:
    s.connect((host, port))
except Exception:
    sys.exit(1)
finally:
    s.close()
PY
        then
            log "${name}: TCP соединение установлено"
            return 0
        fi
        sleep 2
        waited=$((waited + 2))
        if [ "$waited" -ge "$WAIT_TIMEOUT" ]; then
            fail "${name} не ответил за ${WAIT_TIMEOUT}s (${host}:${port})"
        fi
    done
}

# -----------------------------------------------------------------------------
# 2. Проверка, что PostgreSQL действительно готов принимать запросы
#    (открытый порт ещё не значит, что сервер закончил инициализацию)
# -----------------------------------------------------------------------------
wait_for_postgres() {
    local waited=0
    log "Проверка готовности PostgreSQL (pg_isready-эквивалент)..."
    while true; do
        if python - "$DB_HOST" "$DB_PORT" "$DB_USER" "$DB_NAME" <<'PY' 2>/dev/null
import os, socket, struct, sys

host, port, user, dbname = sys.argv[1], int(sys.argv[2]), sys.argv[3], sys.argv[4]
password = os.environ.get("DB_PASSWORD", "") or os.environ.get("PGPASSWORD", "")

s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
s.settimeout(5)
try:
    s.connect((host, port))

    # StartupMessage (протокол PostgreSQL 3.0)
    params = b""
    for k, v in (("user", user), ("database", dbname), ("client_encoding", "UTF8")):
        params += k.encode() + b"\x00" + v.encode() + b"\x00"
    params += b"\x00"
    body = struct.pack("!i", 196608) + params
    s.sendall(struct.pack("!i", len(body) + 4) + body)

    resp = s.recv(1)
    if resp == b"E":
        sys.exit(1)
    # 'R' — аутентификация, 'S'/'K' — параметры сервера: сервер жив
    if resp in (b"R", b"S", b"K"):
        sys.exit(0)
    sys.exit(1)
except Exception:
    sys.exit(1)
finally:
    try:
        s.close()
    except Exception:
        pass
PY
        then
            log "PostgreSQL: сервер готов принимать соединения"
            return 0
        fi
        sleep 2
        waited=$((waited + 2))
        if [ "$waited" -ge "$WAIT_TIMEOUT" ]; then
            fail "PostgreSQL не готов за ${WAIT_TIMEOUT}s"
        fi
    done
}

# -----------------------------------------------------------------------------
# 3. Проверка Redis: PING -> PONG
# -----------------------------------------------------------------------------
wait_for_redis() {
    local waited=0
    log "Проверка готовности Redis (PING/PONG)..."
    while true; do
        if python - "$REDIS_HOST" "$REDIS_PORT" <<'PY' 2>/dev/null
import socket, sys
host, port = sys.argv[1], int(sys.argv[2])
s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
s.settimeout(5)
try:
    s.connect((host, port))
    s.sendall(b"PING\r\n")
    data = s.recv(64)
    sys.exit(0 if data.startswith(b"+PONG") else 1)
except Exception:
    sys.exit(1)
finally:
    try:
        s.close()
    except Exception:
        pass
PY
        then
            log "Redis: ответил на PING"
            return 0
        fi
        sleep 2
        waited=$((waited + 2))
        if [ "$waited" -ge "$WAIT_TIMEOUT" ]; then
            fail "Redis не ответил на PING за ${WAIT_TIMEOUT}s"
        fi
    done
}

# -----------------------------------------------------------------------------
# 4. Миграции / инициализация схемы (если есть чем мигрировать)
# -----------------------------------------------------------------------------
apply_migrations() {
    if [ "$RUN_MIGRATIONS" != "1" ]; then
        log "RUN_MIGRATIONS=0 — миграции пропущены"
        return 0
    fi

    cd /app 2>/dev/null || cd /app/backend 2>/dev/null || true

    # Alembic
    if [ -f alembic.ini ] || [ -f /app/backend/alembic.ini ] || [ -f /app/alembic.ini ]; then
        if command -v alembic >/dev/null 2>&1; then
            log "Применяю миграции Alembic..."
            alembic upgrade head || fail "Миграции Alembic завершились с ошибкой"
            return 0
        fi
    fi

    # Django
    if [ -f manage.py ] || [ -f /app/backend/manage.py ]; then
        if command -v python >/dev/null 2>&1; then
            log "Применяю миграции Django..."
            (cd /app/backend 2>/dev/null || cd /app; python manage.py migrate --noinput) \
                || fail "Миграции Django завершились с ошибкой"
            return 0
        fi
    fi

    log "Механизм миграций не обнаружен (alembic/manage.py) — пропускаю."
    log "Подсказка: если у бэкенда свой механизм миграций, запустите его в entrypoint-backend.sh."
    return 0
}

# -----------------------------------------------------------------------------
# MAIN
# -----------------------------------------------------------------------------
main() {
    log "=== Инициализация окружения Invoice Keeper ==="
    log "PostgreSQL : ${DB_HOST}:${DB_PORT} (db=${DB_NAME}, user=${DB_USER})"
    log "Redis      : ${REDIS_HOST}:${REDIS_PORT}"
    log "WAIT_TIMEOUT=${WAIT_TIMEOUT}s"

    wait_for_tcp   "${DB_HOST}" "${DB_PORT}" "PostgreSQL"
    wait_for_postgres
    wait_for_tcp   "${REDIS_HOST}" "${REDIS_PORT}" "Redis"
    wait_for_redis

    apply_migrations

    log "=== Окружение готово ==="
}

main "$@"
