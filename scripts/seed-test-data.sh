#!/usr/bin/env bash
# Invoice Keeper — идемпотентные тестовые регулярные платежи для E2E-демо.
# Повторный запуск удаляет предыдущие [DEMO]-платежи через API (soft-delete),
# затем создаёт ровно один актуальный набор из RUB/USD/EUR.
set -euo pipefail

# Seed-файл и JSON-payload'ы должны передаваться в UTF-8 без преобразования.
export LC_ALL=C.UTF-8
export LANG=C.UTF-8

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
SEED_DIR="$SCRIPT_DIR/seed"
BASE="${BASE_URL:-http://localhost}"
ADMIN_LOGIN="${ADMIN_LOGIN:-admin}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin}"

if DUE="$(date -u -d '+2 days' +%Y-%m-%d 2>/dev/null)"; then
  :
elif command -v python >/dev/null 2>&1; then
  DUE="$(python -c "from datetime import datetime,timedelta,timezone; print((datetime.now(timezone.utc)+timedelta(days=2)).date())")"
elif command -v python3 >/dev/null 2>&1; then
  DUE="$(python3 -c "from datetime import datetime,timedelta,timezone; print((datetime.now(timezone.utc)+timedelta(days=2)).date())")"
else
  echo "Не найден GNU date или Python для вычисления даты" >&2
  exit 1
fi

json_value() {
  sed -n 's/.*"'"$1"'":"\([^"]*\)".*/\1/p'
}

extract_demo_ids() {
  # Backend возвращает массив объектов: [{"id":1,"title":"[DEMO] ..."}].
  # Для переносимости используем только grep/sed, без jq.
  printf '%s' "$1" \
    | grep -o '"id":[0-9]*,"title":"\[DEMO\][^"]*"' \
    | sed -n 's/.*"id":\([0-9][0-9]*\),.*/\1/p'
}

extract_legacy_ids() {
  # Удаляем только названия, созданные предыдущей версией seed без [DEMO].
  # Python используется только если доступен: stdlib JSON корректно обрабатывает
  # UTF-8 названия на Windows/Git Bash; fallback оставлен для Linux без Python.
  if command -v python >/dev/null 2>&1; then
    printf '%s' "$1" | python -c 'import json,sys
try:
    data=json.load(sys.stdin)
except Exception:
    sys.exit(0)
for item in data if isinstance(data,list) else data.get("items",[]):
    if item.get("title") in {"Аренда офиса","Сервер VPS","Подписка Figma"}:
        print(item.get("id", ""))'
  elif command -v python3 >/dev/null 2>&1; then
    printf '%s' "$1" | python3 -c 'import json,sys
try:
    data=json.load(sys.stdin)
except Exception:
    sys.exit(0)
for item in data if isinstance(data,list) else data.get("items",[]):
    if item.get("title") in {"Аренда офиса","Сервер VPS","Подписка Figma"}:
        print(item.get("id", ""))'
  fi
}

echo "== login =="
LOGIN_RESPONSE="$(curl -fsS -X POST "$BASE/api/v1/auth/login" \
  -H "Content-Type: application/json; charset=utf-8" \
  --data-binary "{\"login\":\"$ADMIN_LOGIN\",\"password\":\"$ADMIN_PASSWORD\"}")"
TOKEN="$(printf '%s' "$LOGIN_RESPONSE" | json_value access_token)"
if [ -z "$TOKEN" ]; then
  echo "LOGIN FAILED: access_token отсутствует" >&2
  printf '%s\n' "$LOGIN_RESPONSE" >&2
  exit 1
fi
echo "login: OK"

# Один GET получает и активные, и исторические записи. DELETE inactive может дать
# 404, поэтому ошибки старых уже удалённых записей не считаются фатальными.
EXISTING="$(curl -fsS "$BASE/api/v1/recurring-payments" \
  -H "Authorization: Bearer $TOKEN")"
DELETED=0
while IFS= read -r ID; do
  [ -z "$ID" ] && continue
  if curl -fsS -X DELETE "$BASE/api/v1/recurring-payments/$ID" \
      -H "Authorization: Bearer $TOKEN" -o /dev/null 2>/dev/null; then
    echo "deleted old [DEMO] id=$ID"
    DELETED=$((DELETED + 1))
  fi
done < <(extract_demo_ids "$EXISTING")
echo "old [DEMO] payments deleted: $DELETED"

# Одноразовая очистка ровно трёх legacy-записей от прежней версии скрипта.
# Остальные пользовательские платежи не затрагиваются.
LEGACY_DELETED=0
while IFS= read -r ID; do
  [ -z "$ID" ] && continue
  if curl -fsS -X DELETE "$BASE/api/v1/recurring-payments/$ID" \
      -H "Authorization: Bearer $TOKEN" -o /dev/null 2>/dev/null; then
    echo "deleted legacy demo id=$ID"
    LEGACY_DELETED=$((LEGACY_DELETED + 1))
  fi
done < <(extract_legacy_ids "$EXISTING")
echo "legacy demo payments deleted: $LEGACY_DELETED"

echo "due date: $DUE"
create_payment() {
  local currency="$1" payload_file="$SEED_DIR/$2" tmp_file response
  tmp_file="$(mktemp)"
  trap 'rm -f "$tmp_file"' RETURN
  sed "s/__DUE__/$DUE/g" "$payload_file" > "$tmp_file"
  response="$(curl -fsS -X POST "$BASE/api/v1/recurring-payments" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json; charset=utf-8" \
    --data-binary "@$tmp_file")"
  echo "$currency: OK id=$(printf '%s' "$response" | json_value id)"
}

create_payment RUB rub.json
create_payment USD usd.json
create_payment EUR eur.json

echo "== active demo payments =="
FINAL="$(curl -fsS "$BASE/api/v1/recurring-payments?is_active=true" \
  -H "Authorization: Bearer $TOKEN")"
DEMO_COUNT="$(extract_demo_ids "$FINAL" | sed '/^$/d' | wc -l | tr -d ' ')"
printf '%s\n' "$FINAL"
echo "active [DEMO] count: $DEMO_COUNT"
if [ "$DEMO_COUNT" -ne 3 ]; then
  echo "ERROR: expected exactly 3 active [DEMO] payments, got $DEMO_COUNT" >&2
  exit 1
fi

echo "== summary =="
curl -fsS "$BASE/api/v1/payments/summary" \
  -H "Authorization: Bearer $TOKEN"
printf '\n'
echo "seed: OK"
