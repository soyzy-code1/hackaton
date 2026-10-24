# invoice-keeper

Учёт регулярных платежей команды: график платежей, напоминания в Telegram (за 3 дня до срока и
позже), привязка Telegram-аккаунтов, Mini App и ассистент «спроси про платежи» на естественном языке.

Подробные описания: `docs/TELEGRAM_BOT.md` (бот и напоминания), `docs/NL_QUERIES.md` (ассистент),
`docs/ENV.md` (все переменные окружения), `nlq/README.md` (Python-часть ассистента).

## Состав репозитория

```
backend/    Go API (chi, PostgreSQL): cmd, internal, migrations, swagger (backend/docs)
frontend/   Nuxt-панель (пока заготовка: одна страница)
miniapp/    Telegram Mini App (Vue 3 + Vite, статичный образ, раздаётся по /tg/)
nlq/        Python: MCP-сервер (read-only SQL), агент (OpenRouter), dbinit, keygen
nginx/      Образ nginx с шаблоном конфига (TLS, прокси на backend, frontend и miniapp)
scripts/    cert.sh: выпуск и продление сертификата Let's Encrypt
docker-compose.local.yaml   локальный стек (сборка из исходников)
docker-compose.yaml         продовый стек (образы из GHCR)
```

## Архитектура

```
                      Telegram (Bot API)
                        ^           |
        sendMessage /   |           | webhook (прод)
        getUpdates      |           v   или long polling (локально)
                        |
  Браузер ---> nginx :443 ---> backend (Go, :8080) <---------------+
  Mini App       | | |           |   |   |                          |
                 | | |           |   |   +-- подписанный actor-JWT  |
                 | | +--/api/--->+   |            (Ed25519)         |
                 | +--/tg/--> miniapp (Vue+Vite, nginx :8080)       |
                 +--/ (SSR)--> frontend (Nuxt :3000)                |
                                     |                              |
                                     v                              |
                              PostgreSQL <---- read-only роль       |
                                     ^         nlq_reader           |
                                     |                              |
                               nlq-mcp (:8900) <--- nlq-agent (:8090)
                          (4 read-only инструмента)   |  (цикл с LLM)
                                                      v
                                                  OpenRouter
```

- Backend единственная публичная точка входа: аутентификация, лимиты, роли, бот, напоминания.
- Ассистент опциональный: приложение работает и при `NLQ_ENABLED=false`, и без сервисов `nlq-*`.
- В проде nlq-mcp сидит во внутренних сетях без выхода в интернет, наружу ходит только nlq-agent
  (к OpenRouter).

## Требования

- Docker с плагином Compose (`docker compose`), `make`.
- Для запуска backend и тестов вне Docker: Go (версия из `backend/go.mod`).
- Для тестов и утилит nlq вне Docker: [`uv`](https://docs.astral.sh/uv/), Python 3.
- Для Telegram: аккаунт Telegram и бот от @BotFather.
- Для прода: сервер с публичным IP или доменом, открытые порты 80 и 443, доступ к GHCR.

## Быстрый локальный старт

1. Скопируйте пример окружения и задайте секреты:

   ```bash
   cp .env.example .env
   ```

   Для локали достаточно значений по умолчанию, но смените `JWT_ACCESS_SECRET`, `JWT_REFRESH_SECRET`
   и `ADMIN_PASSWORD` (минимум 8 символов). Все переменные описаны в `docs/ENV.md`.

2. (Только если нужен ассистент.) Сгенерируйте пару ключей actor-JWT и впишите обе строки в `.env`:

   ```bash
   docker compose -f docker-compose.local.yaml run --rm --no-deps nlq-agent nlq.keygen
   # или без Docker:  cd nlq && uv run python -m nlq.keygen
   ```

   Вывод содержит `NLQ_ACTOR_PRIVATE_KEY=...` (только backend) и `NLQ_ACTOR_PUBLIC_KEY=...`
   (agent и mcp). Затем задайте в `.env`: `NLQ_ENABLED=true` и `NLQ_DB_PASSWORD=<любой пароль>`
   (если оставить пустым, локальный compose подставит `nlq_local_password`).

3. (Только если нужен режим LLM.) Ключ OpenRouter хранится отдельно:

   ```bash
   cp nlq/.env.nlq.example .env.nlq    # файл в корне репозитория, в .gitignore
   # впишите OPENROUTER_API_KEY=sk-or-...
   ```

4. Поднимите стек:

   ```bash
   docker compose -f docker-compose.local.yaml up -d --build
   ```

   Ассистент (nlq-db-init, nlq-mcp, nlq-agent) поднимается вместе со стеком. `make up` делает то же самое. Миграции накатывает сервис `migrate`
   автоматически перед стартом backend.

5. Проверьте:

   ```bash
   curl http://localhost:8080/api/v1/ping      # pong
   docker compose -f docker-compose.local.yaml ps
   ```

   Войти можно под `ADMIN_LOGIN` / `ADMIN_PASSWORD` из `.env`: `POST /api/v1/auth/login`.

### Что где находится локально

| Что | Адрес |
|---|---|
| API | `http://localhost:8080/api/v1/` (порт `LOCAL_API_PORT`) |
| Swagger UI (только `DEBUG=true`) | `http://localhost:8080/api/v1/docs/index.html` |
| pgAdmin | `http://localhost:5050` (`LOCAL_PGADMIN_PORT`; логин и пароль `PGADMIN_DEFAULT_EMAIL` / `PGADMIN_DEFAULT_PASSWORD`, по умолчанию `ad@ad.com` / `admin`) |
| PostgreSQL | `localhost:5432` (`LOCAL_POSTGRES_PORT`) |
| nlq-agent (профиль `nlq`) | `http://127.0.0.1:8090`, проверка `/healthz` |
| nlq-mcp (профиль `nlq`) | `http://127.0.0.1:8900`, проверка `/healthz`, MCP: `/mcp` |

Swagger-спецификация лежит в `backend/docs/`, обновляется командой `make swagger`.

Полезные команды: `make logs` (логи backend), `make ps`, `make down`. Backend без Docker:
`make run` (читает `.env`, нужен Postgres на `POSTGRES_HOST:POSTGRES_PORT`; для запуска на хосте
оставьте `POSTGRES_HOST=localhost`).

### Проверка ассистента

```bash
curl http://localhost:8090/healthz                           # агент
curl http://localhost:8900/healthz                           # mcp
# после логина (cookie или Bearer):
curl http://localhost:8080/api/v1/assistant/status
```

Режимы «мозга» задаются `NLQ_LLM`:

| Режим | Что делает | Нужен ключ |
|---|---|---|
| `openrouter` | боевой: модели из `NLQ_MODELS` через OpenRouter | да, `OPENROUTER_API_KEY` в `.env.nlq` |
| `rules` | детерминированный офлайн-разбор вопросов, без LLM | нет |
| `fake` | воспроизводит JSON-скрипт `NLQ_FAKE_SCRIPT`, для тестов | нет |

При `NLQ_RULES_FALLBACK=true` (по умолчанию) агент сам переходит на `rules`, если LLM недоступна. Без
ключа OpenRouter удобно ставить `NLQ_LLM=rules`.

## Telegram-бот

### 1. Создать бота

1. В Telegram откройте @BotFather, команда `/newbot`, задайте имя и username (например `my_invoice_bot`).
2. Скопируйте токен. Впишите в `.env`:

   ```
   TELEGRAM_BOT_TOKEN=<токен>
   TELEGRAM_BOT_USERNAME=my_invoice_bot      # без @
   ```

3. Выберите режим приёма апдейтов (см. ниже) через `TELEGRAM_MODE` и перезапустите backend:
   `docker compose -f docker-compose.local.yaml up -d backend`.

Пустой токен или `TELEGRAM_MODE=off` выключает бота, приложение при этом работает.

### 2. Добавить в группу и назначить командный чат

1. Добавьте бота в группу команды. Он напишет «Админ, отправьте /bind».
2. Администратор (пользователь приложения с ролью `admin`, привязанный к Telegram) отправляет `/bind`
   в группе. Эта группа становится командным чатом. Повторный `/bind` в другой группе заменяет прежний
   чат. `/unbind` отключает. `/chatid` показывает ID чата.
3. Команды бота: `/start`, `/help`, `/today`, `/week`, `/overdue`, `/ask`, `/unlink`.
4. `/bind` нельзя отправлять от имени анонимного админа или канала: отправьте от своего имени.

Начальный ID командного чата можно задать `TELEGRAM_TEAM_CHAT_ID` (применяется только при
`REMINDER_SOURCE=db`, дефолт).

### 3. Привязать аккаунт из веб-панели

Авторизованный пользователь вызывает `POST /api/v1/me/telegram/link-code` (в панели это кнопка
в профиле). Ответ содержит `deep_link` вида `https://t.me/<bot>?start=link_<код>` (живёт 10 минут).
Откройте ссылку в Telegram и нажмите Start: бот привяжет аккаунт. Статус: `GET /api/v1/me/telegram`.
Для работы нужен `TELEGRAM_BOT_USERNAME`.

### 4. Polling или webhook

| | polling | webhook |
|---|---|---|
| Где | локально | прод |
| `TELEGRAM_MODE` | `polling` | `webhook` |
| Нужен публичный HTTPS | нет | да |
| Доп. переменные | — | `TELEGRAM_WEBHOOK_URL=https://<DOMAIN>/api/v1/telegram/webhook`, `TELEGRAM_WEBHOOK_SECRET` (от 32 символов) |

В режиме webhook backend сам вызывает `setWebhook` при старте.

**Важно: бот один на токен.** Один токен нельзя одновременно слушать двумя способами. Если запустить
локальный polling с продовым токеном, апдейты прода пропадут, а продовый webhook собьётся. Заведите
**отдельного dev-бота** для локали (второй `/newbot`) и никогда не копируйте продовый токен в
локальный `.env`. Если у токена стоит webhook, polling получит ошибку 409 (см. «Проблемы»).

### 5. Mini App

1. В @BotFather: `/newapp` (или `/mybots` -> Bot Settings -> Menu Button), укажите URL
   `https://<DOMAIN>/tg/` (со слэшем) и получите `short_name`.
2. Mini App требует **HTTPS** на вашем домене (см. «Деплой»).
3. Впишите в `.env`:

   ```
   MINIAPP_URL=https://<DOMAIN>/tg/
   TELEGRAM_MINIAPP_SHORT_NAME=<short_name>   # если создано через /newapp
   ```

4. Вход: Mini App шлёт `initData` на `POST /api/v1/telegram/miniapp/auth`; для первой привязки
   используется `POST /api/v1/telegram/miniapp/link` (логин и пароль вводятся один раз).
   `initData` проверяется по токену бота и живёт `TELEGRAM_INITDATA_MAX_AGE` (10 минут).

Mini App — это отдельное приложение `miniapp/` (Vue 3 + Vite, без SSR), собираемое в
статический образ `…-miniapp`. Главный nginx раздаёт его по `/tg/` (проксирует на контейнер
`miniapp`); это не Nuxt-фронтенд из `frontend/`.

#### Локальная разработка Mini App

```
cd miniapp && npm ci && npm run dev   # http://localhost:5173/tg/
```

- Запросы к `/api` проксируются на `DEV_API_TARGET` (по умолчанию `http://localhost:8080`).
- Переменные dev-режима — в `miniapp/.env.development.local` (не коммитится):
  - `DEV_TG_BOT_TOKEN` — должен совпадать с backend `TELEGRAM_BOT_TOKEN`, и бот должен быть включён,
    иначе `/telegram/miniapp/*` отвечает 404;
  - `DEV_TG_USER_ID`, `DEV_TG_FIRST_NAME`, `DEV_TG_USERNAME`;
  - `DEV_TG_START_PARAM` — например `occ_<hex32>` для открытия карточки периода напрямую.
- Первый вход показывает экран привязки (логин и пароль от Invoice Keeper).
- Telegram-мок и подписант `initData` работают только в dev и не попадают в прод-бандл.
- Тесты: `npm test` (unit, vitest) и `npm run e2e` (Playwright, прогон локально).

Зона приложения вшивается в образ на этапе сборки через build-arg `VITE_APP_TIMEZONE`
(по умолчанию `Europe/Moscow`, должна совпадать с backend `APP_TIMEZONE`).

## Платежи: регулярные платежи и периоды

Админ заводит шаблон регулярного платежа (сумма, валюта RUB/USD/EUR, ближайшая дата оплаты
`start_date`, периодичность `monthly`/`weekly`, ответственный). `PaymentScheduler` генерирует из него
периоды (`payments`): у каждого свой `due_date` и отметка оплаты с фактической датой.

Эндпоинты (`/api/v1`):

| Метод | Путь | Доступ | Назначение |
|---|---|---|---|
| GET | `/recurring-payments/` | любой | список шаблонов команды (с остановленными) |
| POST | `/recurring-payments/` | админ | создать шаблон + назначить ответственного |
| PATCH | `/recurring-payments/{id}` | админ | сумма/валюта/описание/ответственный (график неизменяем) |
| DELETE | `/recurring-payments/{id}` | админ | мягкая остановка (история сохраняется) |
| GET | `/payment-occurrences/` | любой | периоды команды; `?assignee=me` — свои |
| GET | `/payment-occurrences/{id}` | любой | один период |
| PATCH | `/payment-occurrences/{id}` | ответственный/админ | `action=confirm`/`cancel`, правка полей — только админ |
| GET | `/payments/summary` | любой | суммы по валютам (`?from&to`) |

Правила кейса в API:
- **Статус (D6)** вычисляется из `paid_date`, `due_date` и «сегодня» в `APP_TIMEZONE`: `paid` → иначе
  `overdue` при `due_date < today` → иначе `pending`; `paid_late = paid_date > due_date`. Срок (`due_date`)
  при исправлении фактической даты не меняется. Несуществующий день месяца → последний день месяца.
- **Остановка (D7)** проставляет `stopped_at`, удаляет неоплаченные будущие периоды, историю и
  оплаченные периоды сохраняет; напоминания прекращаются.
- **Права (D10):** чтение — вся команда; шаблоны — только админ; отметка оплаты/даты — ответственный
  или админ (проверяется в сервисе, общем для веба и бота). Валюты суммируются по отдельности.

## Демо-данные

Модуль платежей (`recurrent_payments`, `payments`) интегрирован; дефолт — работа по базе:

```
REMINDER_SOURCE=db        # дефолт в коде и в .env.example
```

`service.PaymentScheduler` генерирует периоды из шаблонов (горизонт `today + max(31, REMINDER_LEAD_DAYS)`)
и переводит просроченные в overdue; движок напоминаний читает их через SQL-источник. Для демонстрации
без базы есть `REMINDER_SOURCE=stub`: демо-периоды из памяти, сообщения помечаются `[DEMO]`, командный
чат задаётся только вручную через `/bind` (`TELEGRAM_TEAM_CHAT_ID` не применяется). В production при
включённом боте `stub` запрещён, backend не стартует.

> Переключение `stub`→`db` на живом чате: reconcile не находит в `payments` ранее отправленные из
> `stub` `[DEMO]`-сообщения и редактирует их в «Платёж отменён администратором». Перед сменой источника
> очистите старые строки outbox от `stub` (или заведите отдельный тестовый чат), чтобы в командном чате
> не появились ложные «отмены».

Для ассистента после мёржа таблиц B дев-схема-зеркало больше не нужна: таблицы `recurrent_payments`,
`user_recurrent_payments`, `payments` лежат в `public`. Поэтому `NLQ_DEV_SCHEMA` в `.env.example` и в
`docker-compose.local.yaml` пустая — ассистент читает реальные данные. Непустой `NLQ_DEV_SCHEMA`
(например `nlq_test`) нужен только для pytest. Если локальный том был создан со старой переменной
`nlq_dev`, сбросьте зеркала: `python -m nlq.dbinit --drop-dev` (или `docker compose ... down -v`),
иначе `search_path` укажет на пустые зеркала и ассистент всегда ответит «Подходящих платежей не найдено».
`NLQ_TODAY=YYYY-MM-DD` фиксирует «сегодня» для демо. Пока в базе нет платежей, ассистент честно отвечает
«Подходящих платежей не найдено».

> Миграции B после мёржа перенумерованы в `000006`–`000010` (telegram остаётся `000004`/`000005`).
> Локальную базу разработчика B со старой нумерацией (версия 7) нужно сбросить через `down -v`, иначе
> `migrate up` упадёт на `000008` (`CREATE TABLE payments`).

## Деплой в прод

Образы собирает workflow `.github/workflows/docker-publish.yml` на push в `main` и публикует в GHCR:
`ghcr.io/<owner>/invoice-keeper-{backend,migrations,frontend,nginx,nlq}` с тегами `latest` и `<sha>`.
Сервер ничего не собирает.

1. Склонируйте репозиторий на сервер (нужны `docker-compose.yaml`, `scripts/cert.sh`, `.env`).
2. Подготовьте `.env` из `.env.example`:
   - `DOMAIN=<домен или публичный IP>`, `LE_EMAIL=<почта>`;
   - сильные `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` (обязательны, без дефолтов);
   - свои `JWT_ACCESS_SECRET`, `JWT_REFRESH_SECRET`, `ADMIN_LOGIN`, `ADMIN_PASSWORD`;
   - `ALLOWED_ORIGINS=https://<DOMAIN>`;
   - **`DEBUG=false`**: в продовом compose дефолт `DEBUG=true`, что открывает Swagger UI наружу;
   - `APP_ENV` задавать не нужно: compose фиксирует `production`;
   - Telegram: `TELEGRAM_MODE=webhook`, `TELEGRAM_BOT_TOKEN`, `TELEGRAM_BOT_USERNAME`,
     `TELEGRAM_WEBHOOK_URL=https://<DOMAIN>/api/v1/telegram/webhook`, `TELEGRAM_WEBHOOK_SECRET`
     (например `openssl rand -hex 32`), `MINIAPP_URL`, `REMINDER_SOURCE=db`;
   - ассистент: ключи (`docker run --rm ghcr.io/avgsync/invoice-keeper-nlq:latest nlq.keygen`),
     `NLQ_ENABLED=true`, `NLQ_DB_PASSWORD`; `OPENROUTER_API_KEY` в файле `.env.nlq`.
3. Выпустите сертификат (порт 80 должен быть свободен):

   ```bash
   ./scripts/cert.sh issue         # Let's Encrypt, кладёт certs/fullchain.pem и certs/key.pem
   ./scripts/cert.sh self-signed   # самоподписанный, для проверки без интернета (Telegram такой не примет)
   ```

   Продление без простоя: `./scripts/cert.sh renew`. Для домена и для IP добавьте в cron:
   `0 4 * * * cd /opt/invoice-keeper && ./scripts/cert.sh renew >> cert.log 2>&1`. Сертификат на IP
   живёт 6 дней, поэтому `renew` нужен ежедневно.
4. Запустите:

   ```bash
   docker compose pull
   docker compose up -d
   ```

   Другой владелец или тег: `IMAGE_PREFIX=ghcr.io/<owner>/invoice-keeper IMAGE_TAG=<sha>`.
5. Проверьте `https://<DOMAIN>/api/v1/ping` и `docker compose ps`.

В проде наружу открыты только порты 80 и 443 (nginx). nginx отдаёт `/api/` в backend, остальное во
frontend, SSE-стрим ассистента `/api/v1/assistant/ask/stream` идёт без буферизации.

## Миграции

Файлы лежат в `backend/migrations`. В обоих compose-файлах сервис `migrate` выполняет `up` перед стартом
backend. Ручное управление (через локальный compose, нужен `.env`):

```bash
make migrate-up
make migrate-down steps=1
make migrate-version
make migrate-create seq=add_something      # создать пару up/down
```

## Тесты

```bash
make test                       # go test ./... (backend)
make vet                        # go vet ./...
cd backend && go test -tags integration ./...   # интеграционные, нужен TEST_DATABASE_URL

cd nlq
uv sync
uv run ruff check .
uv run pytest -q                # БД-тесты пропускаются без TEST_DATABASE_URL
```

Интеграционные тесты и БД-тесты nlq пишут в базу, поэтому имя БД обязано оканчиваться на `_test`. Как
подготовить базу (миграции, `nlq.dbinit`) см. `.github/workflows/backend-tests.yml` и
`.github/workflows/nlq-tests.yml`. Те же проверки идут в GitHub Actions. Оффлайн-проверка маршрутизации
вопросов ассистента: `cd nlq && uv run python eval/run_eval.py --mode rules`.

## Проблемы и решения

| Симптом | Причина и решение |
|---|---|
| `port is already allocated` / `address already in use` на 5432 | Порт занят локальным Postgres. Задайте `LOCAL_POSTGRES_PORT=15432` в `.env` и перезапустите. Так же для 8080 (`LOCAL_API_PORT`) и 5050 (`LOCAL_PGADMIN_PORT`). |
| `Cannot connect to the Docker daemon` | Запустите Docker (`systemctl start docker`), проверьте права пользователя на группу `docker`. |
| `container name "/pgadmin" is already in use` | В local compose у pgAdmin фиксированное `container_name: pgadmin`. Удалите старый контейнер: `docker rm -f pgadmin`. |
| backend падает: `JWT_ACCESS_SECRET` / `ADMIN_LOGIN must be...` | Не заполнен `.env` или значения не проходят проверку (`config: validate` в логе): `make logs`. |
| backend падает: `NLQ_ACTOR_PRIVATE_KEY is required when NLQ_ENABLED=true` | Сгенерируйте ключи (шаг 2) или поставьте `NLQ_ENABLED=false`. |
| Бот молчит | Проверьте `TELEGRAM_MODE` и токен: при пустом токене бот выключен (в логе предупреждение). |
| Лог: `telegram poller: conflict 409` (getUpdates) | У токена установлен webhook или другой процесс уже делает getUpdates. Нельзя запускать polling с продовым токеном: заведите отдельного dev-бота. Если webhook остался от прежних экспериментов, снимите его: `curl "https://api.telegram.org/bot<TOKEN>/deleteWebhook"`. |
| Mini App отвечает 401 | `invalid_init_data`: открыт не из Telegram или токен бота в `.env` не тот, что у бота Mini App. `init_data_expired`: `initData` старше `TELEGRAM_INITDATA_MAX_AGE`, откройте Mini App заново. Также нужен HTTPS и верный `MINIAPP_URL`. |
| `/tg/` показывает сайт (Nuxt), а не Mini App | Не поднят сервис/образ `miniapp` или не перезапущен nginx после обновления конфига. Проверьте `docker compose ps miniapp` и перезапустите nginx. |
| Webhook не регистрируется | Лог `setWebhook failed`: проверьте HTTPS-сертификат (не самоподписанный), доступность домена снаружи, `TELEGRAM_WEBHOOK_URL`. |
| `/assistant/ask` отвечает 503 `assistant_disabled` | `NLQ_ENABLED=false`. Включите и задайте ключи. |
| `nlq healthz` не OK (контейнер `unhealthy`) | `docker compose -f docker-compose.local.yaml logs nlq-mcp nlq-agent`. Частые причины: пустой или битый `NLQ_ACTOR_PUBLIC_KEY`, не отработал `nlq-db-init` (проверьте `NLQ_DB_PASSWORD` и `ps -a`). У агента статус `degraded` без ключа OpenRouter допустим, он работает как `rules`. |
| Swagger `404` | Swagger доступен только при `DEBUG=true`, путь `/api/v1/docs/index.html`. |
| Telegram не принимает сертификат | Самоподписанный сертификат подходит только для проверки nginx. Используйте `./scripts/cert.sh issue`. |
