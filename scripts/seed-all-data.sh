#!/usr/bin/env bash
# Invoice Keeper — расширенный E2E seed.
# Покрывает пользователей, валюты, периодичность, статусы paid/unpaid/overdue,
# фильтры API, summary и разные даты. Запуск из корня: ./scripts/seed-all-data.sh
set -euo pipefail

BASE="${BASE_URL:-http://localhost}"
LOGIN="${ADMIN_LOGIN:-admin}"
PASSWORD="${ADMIN_PASSWORD:-admin}"

if command -v python >/dev/null 2>&1; then
  eval "$(python - <<'PY'
from datetime import date, timedelta
print(f'TODAY={date.today():%Y-%m-%d}')
print(f'PAST={date.today()-timedelta(days=5):%Y-%m-%d}')
print(f'NEAR={date.today()+timedelta(days=2):%Y-%m-%d}')
print(f'FUTURE={date.today()+timedelta(days=30):%Y-%m-%d}')
PY
)"
else
  TODAY=$(date -u +%Y-%m-%d)
  PAST=$(date -u -d '-5 days' +%Y-%m-%d)
  NEAR=$(date -u -d '+2 days' +%Y-%m-%d)
  FUTURE=$(date -u -d '+30 days' +%Y-%m-%d)
fi

json_value() { sed -n 's/.*"'"$1"'":"\([^"]*\)".*/\1/p'; }
request_json() { curl -fsS "$@"; }

login_response=$(request_json -X POST "$BASE/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"login\":\"$LOGIN\",\"password\":\"$PASSWORD\"}")
TOKEN=$(printf '%s' "$login_response" | json_value access_token)
[ -n "$TOKEN" ] || { echo 'LOGIN FAILED' >&2; exit 1; }
AUTH="Authorization: Bearer $TOKEN"
echo 'admin login: OK'

# Создаём двух обычных пользователей с разными профилями.
create_user() {
  local login="$1" password="$2" email="$3" full_name="$4"
  local code
  code=$(curl -sS -o /tmp/seed-user.json -w '%{http_code}' -X POST "$BASE/api/v1/users" \
    -H "$AUTH" -H 'Content-Type: application/json' \
    -d "{\"login\":\"$login\",\"password\":\"$password\",\"email\":\"$email\",\"full_name\":\"$full_name\",\"role\":\"user\"}")
  case "$code" in 201|409) echo "user $login: $code";; *) echo "user $login: FAILED $code"; cat /tmp/seed-user.json; exit 1;; esac
}
create_user seed_ivan 'IvanPass!2026' ivan@example.com 'Иван Петров'
create_user seed_anna 'AnnaPass!2026' anna@example.com 'Анна Смирнова'

create_payment() {
  local title="$1" amount="$2" currency="$3" due="$4"
  local code
  code=$(curl -sS -o /tmp/seed-payment.json -w '%{http_code}' -X POST "$BASE/api/v1/recurring-payments" \
    -H "$AUTH" -H 'Content-Type: application/json' \
    -d "{\"title\":\"[FULL-DEMO] $title\",\"amount\":$amount,\"currency\":\"$currency\",\"next_payment_date\":\"$due\",\"periodicity\":\"monthly\"}")
  [ "$code" = 201 ] || { echo "payment $title: FAILED $code"; cat /tmp/seed-payment.json; exit 1; }
  printf '%s\n' "payment [$currency] $title: OK"
  printf '%s' "$(cat /tmp/seed-payment.json)"
  printf '\n'
}

# Варианты валют, суммы и дат: overdue, near reminder, future.
create_payment 'Аренда офиса — просрочено' 50000 RUB "$PAST"
create_payment 'Аренда склада — скоро' 125000 RUB "$NEAR"
create_payment 'Сервер VPS — просрочено' 20 USD "$PAST"
create_payment 'Сервер backup — скоро' 35 USD "$NEAR"
create_payment 'Подписка Figma — скоро' 15 EUR "$NEAR"
create_payment 'Подписка облако — будущая' 99 EUR "$FUTURE"

# Проверяем списки и фильтры по валютам.
all=$(request_json "$BASE/api/v1/recurring-payments?is_active=true" -H "$AUTH")
echo "active recurring payments: $(printf '%s' "$all" | grep -o '"id"' | wc -l | tr -d ' ')"
for currency in RUB USD EUR; do
  filtered=$(request_json "$BASE/api/v1/recurring-payments?currency=$currency&is_active=true" -H "$AUTH")
  echo "currency $currency items: $(printf '%s' "$filtered" | grep -o '"id"' | wc -l | tr -d ' ')"
done

# Периоды: список и status-фильтры.
occ=$(request_json "$BASE/api/v1/payment-occurrences" -H "$AUTH")
echo "occurrences total: $(printf '%s' "$occ" | grep -o '"id"' | wc -l | tr -d ' ')"
for status in paid unpaid overdue; do
  filtered=$(request_json "$BASE/api/v1/payment-occurrences?status=$status" -H "$AUTH")
  echo "occurrences status=$status: $(printf '%s' "$filtered" | grep -o '"id"' | wc -l | tr -d ' ')"
done

summary=$(request_json "$BASE/api/v1/payments/summary" -H "$AUTH")
echo "summary: $summary"

echo 'full seed: OK'
