# =============================================================================
#  Invoice Keeper — BOT (aiogram 3 + APScheduler)
#  Исходники: bot/ (распакован из bot.zip). Код бота не изменяется —
#  меняется только сборка и окружение.
# =============================================================================
# syntax=docker/dockerfile:1

FROM python:3.11-slim

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    TZ=Europe/Moscow

# curl — для healthcheck; tzdata — для TIMEZONE
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
        curl \
        tzdata \
 && rm -rf /var/lib/apt/lists/* \
 && ln -snf "/usr/share/zoneinfo/${TZ}" /etc/localtime \
 && echo "${TZ}" > /etc/timezone

WORKDIR /app

# Зависимости (aiogram, APScheduler, aiohttp, python-dotenv, pydantic)
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

# Исходники бота (venv/, .env, кэши исключены через bot/.dockerignore)
COPY . ./

# Каталог для локальной БД дедупликации уведомлений (notifications.db).
# В compose монтируется как volume botdata — переживает пересборку.
RUN mkdir -p /app/data

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=5 \
    CMD test -r /proc/1/cmdline || exit 1

# Telegram API может быть временно недоступен. Не завершаем контейнер: повторяем
# запуск приложения с backoff, не изменяя Python-логику участника.
CMD ["sh", "-c", "while true; do python main.py; code=$?; echo \"bot exited with code $code, retrying in 30s\"; sleep 30; done"]
