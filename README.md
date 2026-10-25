<div align="center">

# 💰 Invoice Keeper

**Веб-панель учёта регулярных платежей с автоматическими напоминаниями в Telegram, VK и по email**

[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev/)
[![React](https://img.shields.io/badge/React-18-61DAFB?style=flat-square&logo=react&logoColor=black)](https://react.dev/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.6-3178C6?style=flat-square&logo=typescript&logoColor=white)](https://www.typescriptlang.org/)
[![Vite](https://img.shields.io/badge/Vite-5.4-646CFF?style=flat-square&logo=vite&logoColor=white)](https://vitejs.dev/)
[![Python](https://img.shields.io/badge/Python-3.11-3776AB?style=flat-square&logo=python&logoColor=white)](https://www.python.org/)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?style=flat-square&logo=docker&logoColor=white)](https://docs.docker.com/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?style=flat-square&logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/Redis-7-DC382D?style=flat-square&logo=redis&logoColor=white)](https://redis.io/)
[![nginx](https://img.shields.io/badge/nginx-1.27-009639?style=flat-square&logo=nginx&logoColor=white)](https://nginx.org/)
[![License](https://img.shields.io/badge/license-MIT-blue?style=flat-square)](https://opensource.org/licenses/MIT)

[Демо-стенд](http://193.233.246.197/) · [Swagger UI](http://193.233.246.197/api/v1/docs) · [OpenAPI](http://193.233.246.197/api/v1/openapi.json)

</div>

---

## 📖 О проекте

**Invoice Keeper** — система учёта регулярных платежей (аренда, подписки, серверы, коммунальные услуги), которая сама напоминает о сроках оплаты и ведёт историю каждого периода.

Ключевая идея: пользователь один раз описывает платёж — сумму, валюту, периодичность и ответственных, — а система сама материализует **периоды (occurrences)**, считает просрочку, строит бюджет по месяцам и годам и рассылает напоминания в привычные каналы связи.

<div align="center">

```
┌─────────────────────────────────────────────────────────────────────┐
│                          Браузер пользователя                        │
└───────────────────────────────┬─────────────────────────────────────┘
                                │ HTTP
┌───────────────────────────────▼─────────────────────────────────────┐
│                        nginx :80 (reverse proxy)                    │
│         /api/* → backend:8000    /  → статика SPA (nginx)          │
└───────────┬───────────────────────────────────┬─────────────────────┘
            │                                   │
┌───────────▼────────────┐          ┌───────────▼─────────────┐
│  frontend (React SPA)  │          │  backend (Go + Gin)     │
│  nginx:1.27-alpine     │          │  :8000                  │
│  порт 3000             │          │  JWT + RBAC + audit log │
└────────────────────────┘          │  SQLite / volume        │
                                    │  notify worker (SMTP)   │
                                    └───────────┬─────────────┘
                                                │ X-Internal-Token
                                    ┌───────────▼─────────────┐
                                    │  bot (Python + aiogram) │
                                    │  Telegram · VK · QR     │
                                    │  APScheduler           │
                                    └─────────────────────────┘
```

</div>

---

## ✨ Основной функционал

### 💳 Учёт регулярных платежей

- **Регулярные платежи** — карточка платежа: название, сумма, валюта, периодичность (`monthly`, `quarterly`, `yearly`, …), дата следующего платежа, список ответственных.
- **Автоматическая материализация периодов** — каждый платёж порождает цепочку начислений; при отметке «оплачено» автоматически создаётся следующий период.
- **Мультивалютность** — RUB, USD, EUR и другие; сводки и бюджеты считаются раздельно по валютам, без скрытого конвертирования.
- **Импорт CSV** — массовое заполнение платежей через модальное окно импорта.
- **Soft-delete** — удалённый платёж не пропадает из истории, а просто перестаёт быть активным.

### 🔔 Напоминания и уведомления

| Канал | Реализация | Особенности |
|-------|-----------|-------------|
| **Telegram** | `aiogram` long polling | Inline-кнопка **«✅ Оплачено»**, QR-код платежа, дедупликация по `occurrence_id`, устойчивый polling-цикл без падения при `TelegramConflictError` |
| **VK** | `aiohttp` + Bots Long Poll | Inline-кнопка отметки оплаты, `message_event`-обработка |
| **Email (SMTP)** | `gomail` в Go-воркере | Fallback портов `465 → 587 → 2525`, `From = SMTP_USER`, защита от повторной отправки |
| **Webhook** | `net/http` в Go | Подписка на события для внешних систем |

- Общий планировщик: ежедневная проверка в `SCHEDULER_HOUR:SCHEDULER_MINUTE` + оперативный опрос каждые `REMINDER_POLL_SECONDS` секунд.
- Локальный SQLite-журнал отправок не даёт прислать одно и то же напоминание дважды.
- **GigaChat** — встроенный AI-ассистент на странице `/assistant` отвечает на вопросы по платежам.

### 🔎 Анализ подписок (AI + поиск в интернете)

- Страница `/subscription-analysis` сравнивает подписки из базы с актуальными ценами из интернета.
- **Как это работает:** бэкенд определяет тематическую категорию сервиса (стриминг музыки, облачное хранилище, …), ищет цены через GigaChat (функция `web_search`) и собирает подтверждённые цены со страниц-источников.
- **Что показывается сверху:** название более дешёвого сервиса, на сколько дешевле в месяц и в год, процент экономии, ссылка на источник и цитата, подтверждающая цену.
- **Гарантии корректности:** экономия пересчитывается бэкендом по ценам из базы, а не берётся из ответа модели; отбрасываются варианты с другим функционалом, с той же ценой или без подтверждённой цены.
- Требуются `GIGACHAT_ENABLED=true`, `GIGACHAT_CREDENTIALS` и `SEARCH_ENABLED=true`.

### 📊 Аналитика и планирование

- **Дашборд** — сводка по доходам/расходам, ближайшие платежи, просроченные начисления, фильтры по статусу и валюте.
- **Календарь** — сетка платежей по дням месяца с быстрой отметкой оплаты.
- **Аналитика** — тренды по месяцам, динамика «оплачено / осталось», экспорт в CSV.
- **Бюджет** — лимиты по валютам на месяц и год, сравнение план/факт/перерасход.
- **Планирование** — сводка «на дату»: сколько к оплате, сколько просрочено, укладывается ли в бюджет.

### 🔐 Безопасность и администрирование

- JWT-аутентификация с отзывом токенов (`revoked_tokens`) и TTL сессии.
- **RBAC**: администратор управляет пользователями, платежами, бюджетами и аудит-логом; обычный пользователь — только свои данные.
- **Аудит-лог** — каждое изменение фиксируется с автором, действием и таймстампом; доступен с пагинацией и фильтром по типу действия.
- **Управление пользователями** — создание, редактирование, блокировка, смена ролей.
- **Внутренний API** для бота защищён заголовком `X-Internal-Token` и не выставлен в публичный интерфейс.

### 🌱 Прочее

- Тёмная/светлая тема, адаптивная вёрстка, i18n-формат дат и валют под выбранный регион.
- Встроенный **mock-режим** фронтенда (`VITE_USE_MOCK=true`) — демонстрация интерфейса без бэкенда.
- Полная OpenAPI 3.1 спецификация со встроенным Swagger UI.
- Автоматический деплой на VPS через GitHub Actions с preflight-проверкой секретов, health-check и smoke-тестами каналов.

---

## 🛠 Стек

### Frontend — [`frontend/`](frontend)

| Технология | Версия | Назначение |
|------------|--------|------------|
| React | 18.3 | UI-библиотека |
| TypeScript | 5.6 | Типизация, `tsc -b` в сборке |
| Vite | 5.4 | Бандлер и dev-сервер |
| Tailwind CSS | 3.4 | Стили и дизайн-токены |
| React Router | 6.28 | Клиентская маршрутизация |
| Zustand | 5.0 | Хранилище состояния (auth, settings) |
| Chart.js + react-chartjs-2 | 4.4 | Графики аналитики |
| lucide-react | 0.454 | Иконки |

### Backend — [`backend/`](backend)

| Технология | Версия | Назначение |
|------------|--------|------------|
| Go | 1.25 | Язык сервиса |
| Gin | 1.10 | HTTP-роутинг, middleware |
| `golang-jwt/jwt/v5` | 5.2 | Аутентификация |
| `modernc.org/sqlite` | 1.34 | SQLite **без cgo** (чистый Go) |
| `jackc/pgx/v5` | 5.7 | Драйвер PostgreSQL (подготовлен) |
| `shopspring/decimal` | 1.4 | Точные денежные расчёты |
| `gopkg.in/gomail.v2` | — | SMTP-отправка писем |
| `skip2/go-qrcode` | — | QR-коды платежей |
| `gopkg.in/yaml.v3` | 3.0 | Разбор OpenAPI-спецификации |

### Bot — [`bot/`](bot)

| Технология | Версия | Назначение |
|------------|--------|------------|
| Python | 3.11 | Язык сервиса |
| aiogram | 3.31 | Telegram Bot API |
| aiohttp / aiohttp-socks | 3.14 | HTTP-клиент и VK Long Poll, SOCKS-прокси |
| APScheduler | 3.10 | Планировщик задач |
| Pydantic | 2.13 | Валидация моделей данных |
| qrcode + Pillow | 7.4 | Генерация QR-кодов |

### Инфраструктура

| Компонент | Версия | Роль |
|-----------|--------|------|
| Docker Compose | v2 | Оркестрация сервисов |
| nginx | 1.27-alpine | Reverse proxy, раздача SPA, health-check |
| PostgreSQL | 16-alpine | Подготовленный сервис БД (бэкенд использует SQLite) |
| Redis | 7-alpine | Кэш/очереди, health-check |
| node:20-alpine | — | Сборка фронтенда в multi-stage образе |
| GitHub Actions | — | CI: тесты + деплой на VPS |

---

## 📂 Структура репозитория

```
.
├── backend/                  # Go + Gin API
│   ├── cmd/server/           # точка входа HTTP-сервера
│   ├── cmd/smoketest/        # smoke-тест доставки уведомлений
│   ├── internal/
│   │   ├── apperr/           # типизированные ошибки
│   │   ├── auth/             # JWT, хеширование паролей
│   │   ├── config/           # конфигурация из окружения
│   │   ├── dates/            # расчёт дат и периодов
│   │   ├── db/               # подключение и миграции
│   │   ├── handlers/         # HTTP-хендлеры и роутинг
│   │   ├── models/           # доменные модели
│   │   ├── notify/           # SMTP, webhook, события
│   │   ├── service/          # бизнес-логика, notify worker
│   │   └── store/            # доступ к данным
│   ├── openapi.yaml          # OpenAPI 3.1 спецификация
│   └── api_test.go           # интеграционные тесты
├── frontend/                 # React SPA
│   ├── public/               # статика (favicon, иконки)
│   └── src/
│       ├── api/              # клиент API и мок-данные
│       ├── components/       # UI-кит и layout
│       ├── hooks/            # переиспользуемые хуки
│       ├── pages/            # страницы приложения
│       └── store/            # Zustand-сторы
├── bot/                      # Python-бот напоминаний
│   ├── channels/             # транспорты: telegram, vk
│   ├── api_client.py         # клиент backend API
│   ├── main.py               # event loop и polling
│   ├── notifier.py           # диспетчер каналов
│   ├── scheduler.py          # планировщик напоминаний
│   ├── smoketest.py          # smoke-тест Telegram-доставки
│   └── storage.py            # SQLite-журнал отправок
├── infra/nginx/              # конфигурация reverse proxy
├── scripts/                  # seed, health-check, валидация
├── .github/workflows/        # CI/CD
├── docker-compose.yml        # описание стенда
├── Dockerfile.frontend       # multi-stage сборка SPA
└── .env.example              # шаблон переменных окружения
```

---

## 🚀 Быстрый старт

### Требования

- Docker Engine / Docker Desktop
- Docker Compose v2
- Свободные порты `80`, `3000`, `8000`, `5432`, `6379`

### Запуск

```bash
# 1. Подготовьте окружение
cp .env.example .env
#    Windows CMD:      copy .env.example .env
#    PowerShell:       Copy-Item .env.example .env

# 2. Заполните в .env реальные значения
#    BOT_TOKEN, TELEGRAM_CHAT_ID, ADMIN_LOGIN, ADMIN_PASSWORD,
#    INTERNAL_API_TOKEN, SECRET_KEY, SMTP_*

# 3. Поднимите стенд
docker compose up --build -d
docker compose ps

# 4. Проверьте готовность
./scripts/check-stand.sh          # Windows: scripts\check-stand.bat
```

Приложение будет доступно на <http://localhost/> (или на адрес вашего стенда), Swagger UI — на `/api/v1/docs`.

> 🔐 Секреты из `.env` **никогда** не коммитятся: файл уже в `.gitignore`. Переменные с префиксом `VITE_*` попадают в публичный JS-бандл — секреты туда класть нельзя.

---

## ⚙️ Переменные окружения

Полный шаблон с комментариями — в [`.env.example`](.env.example). Основные группы:

| Группа | Переменные | Назначение |
|--------|-----------|------------|
| **Auth** | `ADMIN_LOGIN`, `ADMIN_PASSWORD`, `SECRET_KEY`, `SESSION_TTL_HOURS` | Учётная запись администратора и подпись JWT |
| **Internal API** | `INTERNAL_API_TOKEN`, `ADMIN_API_TOKEN` | Доступ бота к служебным эндпоинтам |
| **Telegram** | `BOT_TOKEN`, `TELEGRAM_CHAT_ID`, `SCHEDULER_HOUR`, `REMINDER_POLL_SECONDS` | Бот и расписание напоминаний |
| **Telegram proxy** | `TELEGRAM_PROXY`, `TELEGRAM_API_URL`, `TELEGRAM_PROXY_WORKER_HOST` | Обход блокировки `api.telegram.org` |
| **VK** | `VK_GROUP_TOKEN`, `VK_GROUP_ID`, `VK_PEER_ID` | Канал VK Long Poll |
| **SMTP** | `SMTP_ENABLED`, `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_FROM`, `SMTP_TO` | Email-напоминания |
| **Webhook** | `WEBHOOK_ENABLED`, `WEBHOOK_URL`, `WEBHOOK_SECRET` | Внешние подписки на события |
| **GigaChat** | `GIGACHAT_ENABLED`, `GIGACHAT_CREDENTIALS`, `GIGACHAT_SCOPE`, `GIGACHAT_AUTH_URL`, `GIGACHAT_API_URL`, `GIGACHAT_MODEL`, `GIGACHAT_VERIFY_TLS` | AI-ассистент на странице `/assistant` (шаблон — в [`backend/.env.example`](backend/.env.example)) |
| **Поиск** | `SEARCH_ENABLED`, `SEARCH_ENGINE`, `SEARCH_FETCH_PAGES`, `SEARCH_TIMEOUT_SECONDS`, `SEARCH_MAX_PAYMENTS` | Поиск цен в интернете для анализа подписок |
| **БД** | `DATABASE_URL`, `POSTGRES_*`, `REDIS_URL` | Подключения к хранилищам |
| **Порты** | `HTTP_PORT`, `FRONTEND_PORT`, `BACKEND_PORT`, `POSTGRES_PORT`, `REDIS_PORT` | Публикация портов на хост |

### Про блокировку Telegram

Если с сервера недоступен `api.telegram.org` — поднимите Cloudflare Worker-зеркало Bot API:

```env
TELEGRAM_PROXY=http://127.0.0.1:8080
TELEGRAM_PROXY_WORKER_HOST=<your-worker>.workers.dev
```

```bash
docker compose up -d --build --force-recreate bot
```

Внутри контейнера запускается `tgwsapiproxy`, и все Telegram-запросы идут через Worker. Системный VPN при этом не нужен — SMTP работает напрямую.

---

## 🔌 REST API

Базовый префикс — `/api/v1`. Аутентификация — заголовок `Authorization: Bearer <access_token>`.

| Группа | Методы и пути | Доступ |
|--------|----------------|--------|
| **auth** | `POST /auth/login`, `GET /auth/me`, `POST /auth/logout` | все |
| **users** | `GET/POST /users`, `PATCH/DELETE /users/{id}` | admin |
| **recurring-payments** | `GET/POST /recurring-payments`, `PATCH/DELETE /recurring-payments/{id}` | чтение — все, запись — admin |
| **payment-occurrences** | `GET /payment-occurrences`, `GET/PATCH /payment-occurrences/{id}` | все |
| **budgets** | `GET /budgets`, `PUT /budgets/{currency}` | все |
| **notifications** | `GET /notifications`, `PATCH /notifications/{id}`, `POST /notifications/read-all` | все |
| **summary** | `GET /payments/summary` | все |
| **audit-log** | `GET /audit-log` | admin |
| **assistant** | `POST /chat/gigachat` | user |
| **assistant** | `POST /assistant/subscription-analysis` | user |
| **internal** | `GET /internal/due-occurrences`, `POST /internal/occurrences/{id}/mark-reminded`, `POST /internal/notifications` | `X-Internal-Token` |

Полное описание — в [`backend/openapi.yaml`](backend/openapi.yaml) и в Swagger UI на `/api/v1/docs`.

---

## 🧪 Тесты и проверки

```bash
# Go-тесты бэкенда
cd backend && go test ./...

# Сборка и типы фронтенда
cd frontend && npm ci && npm run build

# Синтаксическая проверка бота
python -m compileall -q bot

# Полная проверка стенда (HTTP, контейнеры, логи, БД)
./scripts/check-stand.sh
```

### Тестовые данные

```bash
./scripts/seed-test-data.sh      # Windows: scripts\seed-test-data.bat
```

Скрипт идемпотентен: создаёт ровно три активных платежа с маркером `[DEMO]` — аренда (RUB), сервер (USD) и подписка (EUR) — со сроком через 2 дня. Повторный запуск не плодит дубликаты.

### Smoke-тесты уведомлений

```bash
docker compose exec backend /app/smoketest -channel email
docker compose exec bot python smoketest.py
```

Коды выхода: `0` — доставка подтверждена, `1` — ошибка доставки, `3` — канал не настроен (пропуск, не ошибка деплоя). Секреты при этом не печатаются.

---

## 🚢 Деплой

### GitHub Actions

Ветка `mainn` → пайплайн [`.github/workflows/deploy.yml`](.github/workflows/deploy.yml):

1. **test** — Go-тесты, сборка фронтенда, проверка синтаксиса Python.
2. **deploy** — preflight секретов → настройка SSH → выгрузка `git archive` → `docker compose up -d --build --force-recreate` → health-check → smoke-тест email и Telegram.

Секреты не попадают в вывод: preflight сообщает только **имена** отсутствующих переменных, но не их значения.

Обязательные secrets в окружении `production`:

| Secret | Пример |
|--------|--------|
| `VPS_HOST` | `193.233.246.197` |
| `VPS_USER` | `root` |
| `VPS_PATH` | `/opt/invoice-keeper` |
| `VPS_SSH_PRIVATE_KEY` | приватный ключ в формате OpenSSH |

### Ручной деплой

```bash
git archive --format=tar.gz --output=invoice-keeper.tar.gz HEAD
scp invoice-keeper.tar.gz user@host:/tmp/
ssh user@host "mkdir -p /opt/invoice-keeper && tar -xzf /tmp/invoice-keeper.tar.gz -C /opt/invoice-keeper && cd /opt/invoice-keeper && docker compose up -d --build --force-recreate"
```

---

## 🛠 Скрипты

| Скрипт | Назначение |
|--------|-----------|
| [`scripts/check-stand.sh`](scripts/check-stand.sh) | Проверка стенда: HTTP-коды, статусы контейнеров, логи, наличие БД |
| [`scripts/seed-test-data.sh`](scripts/seed-test-data.sh) | Идемпотентный сид демо-данных через API |
| [`scripts/seed-all-data.sh`](scripts/seed-all-data.sh) | Полный набор данных по всем валютам |
| [`scripts/healthcheck.sh`](scripts/healthcheck.sh) | Быстрая проверка доступности сервисов |
| [`scripts/check-bot.sh`](scripts/check-bot.sh) | Диагностика Telegram-бота |
| [`scripts/init-db.sh`](scripts/init-db.sh) | Инициализация схемы БД |
| [`scripts/validate-env.py`](scripts/validate-env.py) | Валидация `.env` |
| [`scripts/validate-nginx.py`](scripts/validate-nginx.py) | Валидация nginx-конфигурации |

Для Windows без WSL доступны `.bat`-версии: `scripts\check-stand.bat`, `scripts\seed-test-data.bat`, `scripts\reset-demo.bat`.

---

## ⚠️ Известные ограничения

- **PostgreSQL в compose не используется бэкендом.** Текущий Go-код написан под SQLite: DDL-миграции используют `AUTOINCREMENT`, запросы — плейсхолдеры `?`, а код опирается на `Result.LastInsertId()`, что `pgx` не поддерживает. Переключение требует правок Go-кода (`SERIAL`, `$1`, `RETURNING id`). Рабочие данные лежат в volume `invoice_backenddata`.
- **DELETE — это soft-delete.** История периодов сохраняется, платёж просто перестаёт быть активным и не попадает в напоминания.
- **Публикация PostgreSQL и Redis на хост.** В `docker-compose.yml` порты `5432` и `6379` пробрасываются наружу. Для продакшена их следует привязать к `127.0.0.1` или убрать из `ports:`.
- **Кракозябры в кириллице** — признак отправки данных в `cp1251`/`cp866`. Скрипты хранятся в UTF-8 без BOM и используют `curl --data-binary` с заголовком `Content-Type: application/json; charset=utf-8`.
- **Путь `api.telegram.org` может быть заблокирован** провайдером — используйте вариант с Cloudflare Worker.

---

## 📄 Лицензия

MIT. Исходный код фронтенда взят из [`Vatalone/aeza`](https://github.com/Vatalone/aeza), бэкенд — из [`reznicenkodaniil5/hack`](https://github.com/reznicenkodaniil5/hack), бот написан для этого проекта.

---

<p align="center">
  <sub>Invoice Keeper — учёт регулярных платежей с напоминаниями. Сделано с ❤️ для осеннего хакатона.</sub>
</p>
