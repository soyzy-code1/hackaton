#!/usr/bin/env bash
# =============================================================================
#  Invoice Keeper — проверка Telegram-бота и фоновых задач напоминаний
# -----------------------------------------------------------------------------
#  Сценарий:
#    1) убедиться, что контейнер бота жив и в логах нет ошибок подключения;
#    2) проверить, что бот видит Redis (очереди фоновых задач);
#    3) проверить, что бот читает данные из PostgreSQL (счета к оплате);
#    4) проверить, что TELEGRAM_CHAT_ID валиден и токен принят Telegram API.
#
#  Использование:
#      bash scripts/check-bot.sh
#      bash scripts/check-bot.sh --send-test   # дополнительно отправить тестовое сообщение
# =============================================================================
set -uo pipefail

SEND_TEST=0
[ "${1:-}" = "--send-test" ] && SEND_TEST=1

GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'; BLUE='\033[0;36m'; NC='\033[0m'
PASS=0; FAIL=0

ok()   { printf "  ${GREEN}[OK]${NC}   %s\n" "$1"; PASS=$((PASS + 1)); }
err()  { printf "  ${RED}[FAIL]${NC} %s\n" "$1"; FAIL=$((FAIL + 1)); }
warn() { printf "  ${YELLOW}[WARN]${NC} %s\n" "$1"; }
sec()  { printf "\n${BLUE}== %s ==${NC}\n" "$1"; }

# Значения берём из .env, если он есть
if [ -f .env ]; then
    # shellcheck disable=SC1091
    set -a; . ./.env; set +a
fi
TOKEN="${TELEGRAM_BOT_TOKEN:-}"
CHAT_ID="${TELEGRAM_CHAT_ID:-}"

# -----------------------------------------------------------------------------
sec "1. Контейнер бота"
# -----------------------------------------------------------------------------
state="$(docker inspect -f '{{.State.Status}}' invoice_bot 2>/dev/null || echo missing)"
if [ "$state" = "running" ]; then
    ok "Контейнер invoice_bot — running"
else
    err "Контейнер invoice_bot — ${state}"
fi

restart_count="$(docker inspect -f '{{.RestartCount}}' invoice_bot 2>/dev/null || echo 0)"
if [ "${restart_count:-0}" -gt 3 ]; then
    warn "Контейнер перезапускался ${restart_count} раз — смотрите логи"
else
    ok "Контейнер стабилен (перезапусков: ${restart_count})"
fi

# -----------------------------------------------------------------------------
sec "2. Redis (очереди фоновых задач)"
# -----------------------------------------------------------------------------
if docker exec invoice_redis redis-cli ping 2>/dev/null | grep -q PONG; then
    ok "Redis доступен из сети Docker"

    # Проверяем, что бот записал что-то в Redis (регистрация в очереди/кеше)
    bot_keys="$(docker exec invoice_redis redis-cli --scan 2>/dev/null | grep -viE '^$' | wc -l | tr -d ' ')"
    ok "Ключей в Redis: ${bot_keys}"

    echo "  Ключи (первые 20):"
    docker exec invoice_redis redis-cli --scan 2>/dev/null | head -n 20 | sed 's/^/    - /'
else
    err "Redis недоступен"
fi

# -----------------------------------------------------------------------------
sec "3. PostgreSQL (данные для напоминаний)"
# -----------------------------------------------------------------------------
if docker exec invoice_db pg_isready -U "${POSTGRES_USER:-invoice_user}" >/dev/null 2>&1; then
    ok "PostgreSQL доступен"

    # Ищем таблицы со счетами/арендой/подписками (названия зависят от модели проекта)
    found=0
    for t in invoices invoice bills rental rentals subscription subscriptions payments payment server servers; do
        n="$(docker exec invoice_db psql -U "${POSTGRES_USER:-invoice_user}" -d "${POSTGRES_DB:-invoice_keeper}" \
                -tAc "SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name='${t}';" 2>/dev/null | tr -d ' \r')"
        if [ "${n:-0}" = "1" ]; then
            ok "Найдена таблица '${t}'"
            found=1
        fi
    done
    [ "$found" -eq 0 ] && warn "Стандартные таблицы не найдены — проверьте модели бэкенда (docker exec -it invoice_db psql -U \$POSTGRES_USER -d \$POSTGRES_DB -c '\dt')"
else
    err "PostgreSQL недоступен"
fi

# -----------------------------------------------------------------------------
sec "4. Telegram API"
# -----------------------------------------------------------------------------
if [ -z "$TOKEN" ] || [ "$TOKEN" = "1234567890:AAExampleTokenReplaceMe" ]; then
    err "TELEGRAM_BOT_TOKEN не заполнен в .env"
elif [ -z "$CHAT_ID" ]; then
    warn "TELEGRAM_CHAT_ID не заполнен — бот не сможет слать уведомления"
else
    # getMe — проверяет, что токен валиден
    me="$(curl -s --max-time 15 "https://api.telegram.org/bot${TOKEN}/getMe" 2>/dev/null)"
    if echo "$me" | grep -q '"ok":true'; then
        username="$(echo "$me" | sed -n 's/.*"username":"\([^"]*\)".*/\1/p')"
        ok "Токен валиден — бот @${username}"
    else
        err "Telegram API не принял токен: $(echo "$me" | head -c 200)"
    fi

    # getChat — проверяет доступ к чату
    chat="$(curl -s --max-time 15 "https://api.telegram.org/bot${TOKEN}/getChat?chat_id=${CHAT_ID}" 2>/dev/null)"
    if echo "$chat" | grep -q '"ok":true'; then
        title="$(echo "$chat" | sed -n 's/.*"title":"\([^"]*\)".*/\1/p')"
        [ -z "$title" ] && title="$(echo "$chat" | sed -n 's/.*"username":"\([^"]*\)".*/\1/p')"
        ok "Чат ${CHAT_ID} доступен боту ${title:+(${title})}"
    else
        err "Бот не может писать в чат ${CHAT_ID}: добавьте бота в группу / напишите ему в ЛС"
    fi

    # Отправка тестового сообщения (по запросу)
    if [ "$SEND_TEST" -eq 1 ]; then
        msg="$(curl -s --max-time 15 -X POST "https://api.telegram.org/bot${TOKEN}/sendMessage" \
                -d chat_id="${CHAT_ID}" \
                -d text="Invoice Keeper: тест инфраструктуры (проверка бота и фоновых задач Redis) ✅" 2>/dev/null)"
        if echo "$msg" | grep -q '"ok":true'; then
            ok "Тестовое сообщение отправлено в чат ${CHAT_ID}"
        else
            err "Не удалось отправить тестовое сообщение: $(echo "$msg" | head -c 200)"
        fi
    fi
fi

# -----------------------------------------------------------------------------
sec "5. Логи бота (последние 30 строк)"
# -----------------------------------------------------------------------------
docker logs --tail 30 invoice_bot 2>&1 | sed 's/^/    /'
echo "----------------------------------------------------"

sec "Итог"
printf "  Успешно: %s${GREEN}   Провалено: %s${RED}${NC}\n" "$PASS" "$FAIL"
echo "Логи бота в реальном времени:  docker compose logs -f bot"
echo "Фоновые задачи в Redis:       docker exec -it invoice_redis redis-cli monitor"
echo

[ "$FAIL" -gt 0 ] && exit 1
exit 0
