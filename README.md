# Invoice Keeper — запуск окружения

## Windows без WSL

На Windows без WSL и Git Bash используйте `.bat`-версии:

```cmd
scripts\reset-demo.bat
scripts\seed-test-data.bat
scripts\check-stand.bat
```

На Linux/macOS используйте соответствующие `.sh`-версии:

```bash
./scripts/reset-demo.sh
./scripts/seed-test-data.sh
./scripts/check-stand.sh
```

## Известные ограничения окружения

- **PostgreSQL:** текущий Go-бэкенд использует SQLite (`AUTOINCREMENT`, плейсхолдеры `?`, `LastInsertId()`). Переключение на PostgreSQL требует правок Go-кода (`SERIAL`, `$1`, `RETURNING id`) и находится в зоне участника 2 (Backend). PostgreSQL-контейнер оставлен как подготовленный сервис, но текущим бэкендом не используется. Рабочие данные хранятся в volume `invoice_backenddata`.
- **Telegram API:** контейнер `bot` использует retry-цикл с интервалом 30 секунд; при сетевой ошибке контейнер остаётся `Up`, а после восстановления сети автоматически повторяет запуск. Если у жюри сеть блокирует Telegram, это ограничение сети, а не кода.
- **Токен бота:** в `.env` должен быть реальный `BOT_TOKEN`, полученный у [@BotFather](https://t.me/BotFather). Тестовые токены не использовать.
- **TELEGRAM_CHAT_ID:** значение должно быть реальным. `-1001234567890` — шаблон, с ним Telegram ответит `400 chat not found`. Перед демо добавьте бота в группу, получите chat ID через @userinfobot или getUpdates, впишите его в `.env` и выполните `docker compose restart bot`.
- **405 Not Allowed на `/api/`:** проверьте [`infra/nginx/nginx.conf`](infra/nginx/nginx.conf), что `location /api/` содержит `proxy_pass_request_body on`, `proxy_method $request_method`, не содержит `try_files`/`limit_except`, а CORS `OPTIONS` возвращает 204. После изменений выполните `docker compose build --no-cache frontend nginx && docker compose up -d`.

## Как проверить, что напоминание реально доходит

```bash
docker compose restart bot
./scripts/seed-test-data.sh
docker compose logs -f bot
```

Ожидается лог `reminder sent for occurrence_id=X, due_date=YYYY-MM-DD` и сообщение с кнопкой **✅ Оплачено**. После нажатия ожидается PATCH 200 в логах backend. `TelegramNetworkError` означает сетевую проблему (нужен HTTPS_PROXY), `400 chat not found` — неверный `TELEGRAM_CHAT_ID`.

## Диагностика Telegram-бота

```bash
docker compose ps bot
docker compose logs --tail=50 bot
docker compose exec bot python -c "import os; print('BOT_TOKEN set:', 'yes' if os.getenv('BOT_TOKEN') else 'no'); print('CHAT_ID:', os.getenv('TELEGRAM_CHAT_ID')); print('BACKEND_URL:', os.getenv('BACKEND_URL')); print('INTERNAL_TOKEN set:', 'yes' if os.getenv('INTERNAL_API_TOKEN') else 'no')"
docker compose exec bot python -c "import urllib.request; print('telegram:', urllib.request.urlopen('https://api.telegram.org', timeout=10).status)"
docker compose exec bot python -c "import urllib.request; print('backend:', urllib.request.urlopen('http://backend:8000/api/v1/openapi.json', timeout=10).status)"
docker compose exec bot python -c "import os,urllib.request; u='https://api.telegram.org/bot'+os.environ['BOT_TOKEN']+'/getMe'; print(urllib.request.urlopen(u,timeout=10).read(200).decode())"
```

## Стек

- **frontend** — React + TypeScript + Vite из [`Vatalone/aeza`](https://github.com/Vatalone/aeza), код размещён в [`frontend/`](frontend).
- **backend** — Go 1.25 + Gin из [`reznicenkodaniil5/hack`](https://github.com/reznicenkodaniil5/hack), код размещён в [`backend/`](backend).
- **bot** — Python 3.11 + aiogram + APScheduler, код размещён в [`bot/`](bot).
- **db** — PostgreSQL 16 как подготовленный инфраструктурный сервис.
- **Redis** — Redis 7, healthcheck и совместимость с общей сетью.
- **nginx** — reverse proxy и единая точка входа.

Исходный код `backend/*.go`, `bot/*.py` и `frontend/src/` не изменяется инфраструктурой.

## Требования

- Docker Engine / Docker Desktop.
- Docker Compose v2.
- Свободные порты `80`, `3000`, `8000`, `5432`, `6379`.

Проверка:

```bash
docker info
docker compose version
```

## Подготовка и запуск

```bash
cp .env.example .env
# Windows CMD: copy .env.example .env
# PowerShell: Copy-Item .env.example .env
```

Заполнить в `.env` реальные `BOT_TOKEN`, `TELEGRAM_CHAT_ID`, `ADMIN_LOGIN`, `ADMIN_PASSWORD` и `INTERNAL_API_TOKEN`. Для токена internal API можно использовать `openssl rand -hex 32`.

```bash
docker compose up --build -d
docker compose ps
./scripts/check-stand.sh
```

Основные адреса:

- приложение: <http://localhost/>;
- Swagger UI: <http://localhost/api/v1/docs>;
- OpenAPI: <http://localhost/api/v1/openapi.json>.

## Сценарий демонстрации

1. Открыть <http://localhost/> и войти как `admin` с паролем из `.env`.
2. Выполнить [`scripts/seed-test-data.sh`](scripts/seed-test-data.sh).
3. В UI показать аренду RUB, сервер USD и подписку EUR со сроком через 2 дня.
4. Дождаться планировщика и проверить `docker compose logs -f bot`.
5. В Telegram нажать **✅ Оплачено**; в логах backend проверить PATCH 200.
6. Убедиться, что повторное напоминание после `docker compose restart bot` не отправляется.

## Тестовые данные и идемпотентный seed

```bash
./scripts/seed-test-data.sh
```

Скрипт использует admin API и создаёт ровно три активных платежа с маркером `[DEMO]`:

- `[DEMO] Аренда офиса` — `50000 RUB`;
- `[DEMO] Сервер VPS` — `20 USD`;
- `[DEMO] Подписка Figma` — `15 EUR`.

Перед созданием старые `[DEMO]` записи удаляются через API DELETE. DELETE в Go-бэкенде — soft-delete: история периодов сохраняется, но удалённый платёж больше не активен и не должен попадать в напоминания. Повторный запуск не плодит активные дубли.

Контракт сверён с [`backend/openapi.yaml`](backend/openapi.yaml): `title`, `amount`, `currency`, `next_payment_date`, `periodicity`. Срок — через 2 дня, окно напоминания — `REMINDER_DAYS_BEFORE=3`.

Проверки:

```bash
docker compose exec bot python -c "import urllib.request; r=urllib.request.Request('http://backend:8000/api/v1/internal/due-occurrences',headers={'X-Internal-Token':'local_internal_token'}); print(urllib.request.urlopen(r).read().decode())"
docker compose logs --tail=30 backend
docker compose logs --tail=30 bot
```

## Проверка стенда

```bash
./scripts/check-stand.sh
```

Скрипт проверяет HTTP 200 frontend/Swagger/OpenAPI/login, статусы контейнеров, логи и наличие SQLite-файла бота. В конце он предупреждает, если `TELEGRAM_CHAT_ID` пустой или равен шаблонному `-1001234567890`.

## Управление

```bash
docker compose up --build -d
docker compose restart bot
docker compose logs -f bot
docker compose logs -f backend
docker compose down
docker compose down -v
```

`docker compose down -v` удаляет все данные, включая `invoice_backenddata` и `invoice_botdata`. Обычный restart бота сохраняет `/app/data/notifications.db` в volume `botdata`.

## Финальная диагностика

На текущем стенде проверены:

- `bot` — `Up (healthy)`;
- Telegram API из bot-контейнера — HTTP 200;
- `getMe` — `ok:true`;
- backend из bot-контейнера — HTTP 200;
- `check-stand.sh` — frontend, Swagger, OpenAPI и login возвращают 200;
- internal endpoint после сидинга возвращает due-occurrences.

Если internal endpoint возвращает больше трёх demo-периодов после повторного запуска, это блокер soft-delete/фильтрации в зоне Backend; Go-код инфраструктурой не изменяется.
