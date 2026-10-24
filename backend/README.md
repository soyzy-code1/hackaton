# Invoice Keeper — Backend (Go)

FastAPI-сервис переписан на **Go 1.22** (Gin + `database/sql` + SQLite по умолчанию).

## Локальный запуск

```bash
cd backend
cp .env.example .env
go mod download
go run ./cmd/server
```

- Swagger UI: http://localhost:8000/api/v1/docs
- OpenAPI JSON: http://localhost:8000/api/v1/openapi.json

При старте выполняются SQL-миграции (`internal/db/migrations`) и создаётся администратор из `ADMIN_*`.

## Docker

```bash
cd backend
cp .env.example .env
docker compose build
docker compose up -d
```

## Тесты

```bash
go test ./...
```

## Контракт для фронта

Список и оплата периодов: [`docs/frontend-payment-occurrences.md`](docs/frontend-payment-occurrences.md)  
Используй `GET/PATCH /api/v1/payment-occurrences?...&view=minimal` и в PATCH тело с `actual_payment_date`.

## Internal API (бот)

Заголовок: `X-Internal-Token: <INTERNAL_API_TOKEN>`

- `GET /api/v1/internal/due-occurrences`
- `POST /api/v1/internal/occurrences/{id}/mark-reminded`

`DATABASE_URL`: `sqlite:///./invoice_keeper.db` или `postgresql+psycopg://...` (драйвер `pgx`).

## SMTP и Webhook (доп. возможности кейса)

**SMTP** (`SMTP_ENABLED=true`): фоновый воркер раз в `NOTIFY_POLL_SECONDS` ищет периоды в окне `REMINDER_DAYS_BEFORE` и шлёт письмо на `SMTP_TO` (текст на русском). Дубли исключаются полем `email_reminder_sent_at`.

**Webhook** (`WEBHOOK_ENABLED=true`): `POST` JSON на `WEBHOOK_URL`:

| Событие | Когда |
|---------|--------|
| `payment.due_reminder` | Срок в окне напоминания (воркер) |
| `recurring_payment.created` | Создан регулярный платёж |
| `recurring_payment.deleted` | Soft-delete платежа |
| `payment_occurrence.paid` | Период отмечен оплаченным |

Заголовок `X-Webhook-Secret`, если задан `WEBHOOK_SECRET`. Тело: `{"event","timestamp","data"}`.
