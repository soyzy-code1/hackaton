# Invoice Keeper — Frontend

SPA для учёта регулярных платежей. Реализовано на React + TypeScript + Vite + Tailwind CSS.

## Запуск

```bash
cd frontend
npm install
npm run dev
```

Production-проверка:

```bash
npm run build
```

## Финансовое планирование

На Dashboard под итогами два виджета (графики на Chart.js):

- **Бюджет и итоги на дату.** Переключатель «Месяц / Год» и поле «Итоги на» (например, 15 ноября). Итоги считаются на выбранный день: если платёж подтвердили позже этой даты, на этот день он считается неоплаченным. Для каждой валюты показаны план, оплачено, осталось, просрочено и заполнение бюджета; при превышении лимита — предупреждение. Строка-сводка вроде «В этом месяце 5 в RUB, 3 в USD, 2 в EUR счетов. Просрочено: 1 в RUB». График «Оплачено vs Осталось» строится в процентах отдельно по каждой валюте, потому что суммы в разных валютах не складываются и не сравниваются.
- **Прогноз кассового разрыва.** По оси X — дни месяца, столбцы — сумма к оплате за день, линия — остаток на счёте после платежей, пунктир — ноль. Остаток вводится вручную для каждой валюты. Если остаток уходит в минус, показывается предупреждение: «Внимание! 25.11 нужно оплатить 3 счёта на 120 000 RUB…» и сколько не хватает. Просрочки прошлых месяцев переносятся на 1-е число: их всё равно нужно оплатить.

Лимиты на месяц и на год по каждой валюте редактируются на странице «Аналитика».

## Импорт платежей из CSV

На странице «Платежи» (только администратор) есть кнопка «Импорт из CSV»:

1. «Скачать шаблон» — CSV с колонками `name, amount, currency, next_payment_date, periodicity, category, assigned_usernames, notes` и примерами строк.
2. Заполнить в Excel или Google Таблицах и сохранить как «CSV UTF-8».
3. Загрузить файл. До отправки на сервер видны строки, готовые к импорту, и ошибки с номерами строк.

Парсер понимает разделители `,` и `;`, кавычки, BOM из Excel, суммы вида `45 000` и `19,99`, даты `ГГГГ-ММ-ДД` и `ДД.ММ.ГГГГ`, категории ключом (`housing`) или по-русски («Жильё»). Отклоняются несуществующие даты (30.02), неизвестные валюты и пользователи, а также дубликаты внутри файла. Валидные строки уходят одним запросом. Если хоть одна строка не прошла серверную проверку, не создаётся ничего (транзакция).

## Дополнительные возможности

Помимо базовых ролей, персональной изоляции ledger-записей, календаря и аналитики, добавлены:

- inbox уведомлений с пагинацией, фильтром непрочитанных, mark-read и mark-all-read;
- серверный контракт для bulk-операций над периодами;
- серверный контракт для редактирования регулярного платежа;
- категории, теги, заметки и ссылка на чек у регулярного платежа;
- архивирование регулярных платежей без потери истории;
- CSV-экспорт платежей и аналитики;
- месячный бюджет отдельно для RUB/USD/EUR;
- часовой пояс, дата напоминаний и настройки уведомлений;
- журнал действий пользователя;
- календарь сроков с детализацией дня;
- аналитика трендов и крупнейших расходов;
- поиск и фильтр «требуют внимания» на Dashboard.

## DB-ready API contract

Все новые функции спроектированы через типизированный [`src/api/types.ts`](src/api/types.ts) и API-клиент [`src/api/client.ts`](src/api/client.ts). Mock используется только как локальный fallback в dev-режиме; UI вызывает те же сервисные методы, что и production API.

Ожидаемые новые backend endpoints:

```text
PATCH  /api/v1/recurring-payments/{id}
GET    /api/v1/notifications?page=1&page_size=50&unread_only=true
PATCH  /api/v1/notifications/{id}
POST   /api/v1/notifications/read-all
PATCH  /api/v1/payment-occurrences/bulk
GET    /api/v1/payments/summary?as_of_date=2026-11-15&from_date=2026-11-01&to_date=2026-11-30
GET    /api/v1/budgets?as_of_date=2026-11-15
PUT    /api/v1/budgets/{currency}
POST   /api/v1/recurring-payments/import
```

`POST /recurring-payments/import` принимает `{ "rows": CsvImportRow[] }`. Пользователи в `assigned_usernames` указываются логинами; пустой список означает «все обычные пользователи». Endpoint должен работать в одной транзакции и повторно валидировать строки: проверка в браузере нужна только для удобства, доверять ей нельзя. Ответ — массив созданных `RecurringPayment`.

Рекомендуемая схема хранения:

- `recurring_payments`: id, owner/created_by, name, amount, currency, category, notes, receipt_url, tags JSON/таблица, archived, periodicity, next_payment_date;
- `payment_occurrences`: id, recurring_payment_id, user_id, due_date, status, actual_payment_date, unique(recurring_payment_id, user_id, due_date);
- `notifications`: id, user_id, occurrence_id nullable, type, title, message, channels JSON, is_read, created_at;
- `notification_preferences`: user_id, reminder_lead_days, timezone, telegram_enabled, email_enabled, weekly_digest_enabled;
- `budgets`: user_id, currency, period (`month`/`year`), limit_amount, starts_at;
- `cash_balances`: user_id, currency, amount, updated_at — остаток для прогноза кассового разрыва (во frontend пока хранится в настройках браузера);
- `activity_log`: id, actor_id, action, entity_type, entity_id, metadata JSON, created_at.

Backend должен возвращать те же DTO, что объявлены во frontend types. Для notification list используется:

```json
{
  "items": [],
  "total": 0,
  "page": 1,
  "page_size": 50
}
```

## Environment

Скопируйте `.env.example` в `.env`:

- `VITE_API_URL=/api/v1` — базовый URL API;
- `VITE_PROXY_TARGET=http://localhost:8000` — адрес backend в dev;
- `VITE_USE_MOCK=false` — принудительный mock-режим;
- `VITE_ADMIN_USERNAME` — admin username только для demo fallback.

Если backend недоступен в dev, клиент использует mock-адаптер, но production-код остаётся server-first: API-ответы и API-ошибки не подменяются mock-данными.

## Тестовые пользователи mock

- `admin` / `admin` — администратор;
- `ivan` / `ivan` — обычный пользователь;
- `anna` / `anna` — обычный пользователь.

Платежи персонально назначаются пользователям. Статус и фактическая дата периода сохраняются в отдельной записи ledger для каждого `user_id`.
