"""
Configuration loader for the Invoice Keeper reminder bot.

All settings are loaded from environment variables (optionally via a .env
file in the /bot directory). This module is the single source of truth
for configuration across the project.
"""

import os
from dataclasses import dataclass
from pathlib import Path

from dotenv import load_dotenv

# Load .env file located next to this module (bot/.env)
BASE_DIR = Path(__file__).resolve().parent
load_dotenv(BASE_DIR / ".env")


def _get_env(name: str, default: str | None = None, required: bool = False) -> str:
    value = os.getenv(name, default)
    if required and not value:
        raise RuntimeError(f"Missing required environment variable: {name}")
    return value


def _get_int_env(name: str, default: int) -> int:
    raw = os.getenv(name)
    if raw is None or raw == "":
        return default
    try:
        return int(raw)
    except ValueError:
        raise RuntimeError(f"Environment variable {name} must be an integer, got: {raw}")


@dataclass(frozen=True)
class Settings:
    # Telegram bot token issued by @BotFather.
    bot_token: str

    # Base URL of the backend API, e.g. http://backend:8000
    backend_url: str

    # Chat ID (group/channel/user) where reminders are sent.
    telegram_chat_id: int

    # Service/admin token used to authorize requests to the backend.
    # Sent as "Authorization: Bearer <token>".
    admin_api_token: str | None

    # Optional Basic Auth credentials, used if the backend requires
    # admin login/password instead of a bearer token.
    admin_basic_auth_login: str | None
    admin_basic_auth_password: str | None

    # Timezone used by the APScheduler cron job, e.g. "Europe/Moscow".
    timezone: str

    # Telegram user ID of the administrator who receives error alerts.
    # Optional: if not set, error alerts are only logged.
    admin_telegram_id: int | None

    # How many days ahead to look for upcoming unpaid payments.
    reminder_days_ahead: int

    # Hour/minute (in `timezone`) at which the daily check runs.
    scheduler_hour: int
    scheduler_minute: int

    # Path to the local SQLite DB used for de-duplicating notifications.
    sqlite_path: str

    # HTTP request timeout (seconds) for backend calls.
    http_timeout: int


def load_settings() -> Settings:
    """Build a Settings instance from the current environment."""
    telegram_chat_id_raw = _get_env("TELEGRAM_CHAT_ID", required=True)
    admin_telegram_id_raw = os.getenv("ADMIN_TELEGRAM_ID")

    return Settings(
        bot_token=_get_env("BOT_TOKEN", required=True),
        backend_url=_get_env("BACKEND_URL", required=True).rstrip("/"),
        telegram_chat_id=int(telegram_chat_id_raw),
        admin_api_token=os.getenv("ADMIN_API_TOKEN") or None,
        admin_basic_auth_login=os.getenv("ADMIN_BASIC_AUTH_LOGIN") or None,
        admin_basic_auth_password=os.getenv("ADMIN_BASIC_AUTH_PASSWORD") or None,
        timezone=_get_env("TIMEZONE", default="Europe/Moscow"),
        admin_telegram_id=int(admin_telegram_id_raw) if admin_telegram_id_raw else None,
        reminder_days_ahead=_get_int_env("REMINDER_DAYS_AHEAD", default=3),
        scheduler_hour=_get_int_env("SCHEDULER_HOUR", default=10),
        scheduler_minute=_get_int_env("SCHEDULER_MINUTE", default=0),
        sqlite_path=_get_env(
            "SQLITE_PATH", default=str(BASE_DIR / "notifications.db")
        ),
        http_timeout=_get_int_env("HTTP_TIMEOUT", default=15),
    )


# Singleton settings instance, imported by other modules.
settings = load_settings()
