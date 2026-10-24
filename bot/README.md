# Бот напоминаний Invoice Keeper

## Запуск

1. Скопируйте `.env.example` в `.env` внутри директории `bot`.
2. Заполните `BACKEND_URL` и `ADMIN_API_TOKEN`.
3. Для Telegram укажите `BOT_TOKEN` и `TELEGRAM_CHAT_ID`.
4. Установите зависимости: `pip install -r requirements.txt`.
5. Запустите: `python main.py`.

Можно включить один или оба канала. Общий планировщик выполняет проверку каждый день в `SCHEDULER_HOUR:SCHEDULER_MINUTE` в зоне `TIMEZONE`. Перед отправкой используется локальный SQLite-журнал `sent_log.db`.

## VK Bots Long Poll

1. Откройте [VK для разработчиков](https://vk.com/dev) и создайте сообщество.
2. В сообществе откройте **Управление → Работа с API → Ключи доступа**, создайте токен сообщества с правом **Сообщения сообщества** и сохраните его в `VK_GROUP_TOKEN`.
3. Скопируйте числовой ID сообщества в `VK_GROUP_ID`.
4. В **Управление → Настройки → Работа с API → Long Poll API** включите Long Poll API сообщества.
5. В событиях Long Poll включите событие **message_event**. Для отправки сообщений также включите обработку входящих сообщений, если это требуется настройками сообщества.
6. Узнайте `VK_PEER_ID`: для личного диалога это ID пользователя; для беседы — `2000000000 + номер беседы` (например, беседа №1 имеет `2000000001`). Также peer_id можно определить по объекту входящего события Long Poll или через API VK.
7. Заполните `VK_GROUP_TOKEN`, `VK_GROUP_ID`, `VK_PEER_ID` и перезапустите бота.

Публичный URL для VK не нужен: бот сам получает события через исходящий Long Poll. При нажатии inline-кнопки `✅ Оплачено` бот обрабатывает `message_event`, отвечает через `messages.sendMessageEventAnswer` и вызывает backend PATCH.

## Backend API

Бот использует:

- `GET /api/v1/payment-occurrences` для списка платежей;
- `PATCH /api/v1/payment-occurrences/{id}` с телом `{"is_paid": true, "actual_payment_date": "YYYY-MM-DD"}`.

Авторизация выполняется заголовком `Authorization: Bearer <ADMIN_API_TOKEN>`.
