#!/usr/bin/env bash
# Temporary API probe (will be deleted).
BASE=http://localhost:3000/api/v1
TOKEN=$(curl -s -X POST "$BASE/auth/login" -H 'Content-Type: application/json' \
  -d '{"login":"admin","password":"admin"}' | sed -E 's/.*"access_token":"([^"]+)".*/\1/')
echo "TOKEN=${TOKEN:0:15}"
for p in recurring-payments payment-occurrences "notifications?page=1&page_size=5" payments/summary users; do
  echo
  echo "== $p"
  curl -s -o /tmp/r -w '%{http_code}\n' -H "Authorization: Bearer $TOKEN" "$BASE/$p"
  head -c 900 /tmp/r
  echo
done
