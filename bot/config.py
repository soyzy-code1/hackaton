"""Environment-backed configuration shared by all bot channels."""

import os
from dataclasses import dataclass
from pathlib import Path

from dotenv import load_dotenv

BASE_DIR = Path(__file__).resolve().parent
load_dotenv(BASE_DIR / ".env")


def _env(name: str, default: str | None = None) -> str | None:
    value = os.getenv(name, default)
    return value.strip() if value else None


def _int(name: str, default: int | None = None) -> int | None:
    value = _env(name)
    if not value:
        return default
    try:
        return int(value)
    except ValueError as exc:
        raise RuntimeError(f"Environment variable {name} must be an integer") from exc


@dataclass(frozen=True)
class Settings:
    backend_url: str
    admin_api_token: str
    timezone: str
    reminder_days_ahead: int
    scheduler_hour: int
    scheduler_minute: int
    sqlite_path: str
    http_timeout: int
    reminder_poll_seconds: int
    telegram_proxy: str | None
    telegram_api_url: str | None
    bot_token: str | None
    telegram_chat_id: int | None
    admin_telegram_id: int | None
    vk_group_token: str | None
    vk_group_id: int | None
    vk_peer_id: int | None
    telegram_polling_enabled: bool

    @property
    def telegram_enabled(self) -> bool:
        return bool(self.bot_token and self.telegram_chat_id is not None)

    @property
    def vk_enabled(self) -> bool:
        return bool(self.vk_group_token and self.vk_group_id and self.vk_peer_id)


def load_settings() -> Settings:
    backend_url = _env("BACKEND_URL")
    admin_token = _env("ADMIN_API_TOKEN")
    if not backend_url:
        raise RuntimeError("Missing required environment variable: BACKEND_URL")
    if not admin_token:
        raise RuntimeError("Missing required environment variable: ADMIN_API_TOKEN")
    return Settings(
        backend_url=backend_url.rstrip("/"),
        admin_api_token=admin_token,
        timezone=_env("TIMEZONE", "Europe/Moscow") or "Europe/Moscow",
        reminder_days_ahead=_int("REMINDER_DAYS_AHEAD", 3) or 3,
        scheduler_hour=_int("SCHEDULER_HOUR", 10) or 10,
        scheduler_minute=_int("SCHEDULER_MINUTE", 0) or 0,
        sqlite_path=_env("SQLITE_PATH", str(BASE_DIR / "sent_log.db")) or str(BASE_DIR / "sent_log.db"),
        http_timeout=_int("HTTP_TIMEOUT", 15) or 15,
        reminder_poll_seconds=max(_int("REMINDER_POLL_SECONDS", 60) or 60, 10),
        telegram_proxy=_env("TELEGRAM_PROXY"),
        telegram_api_url=_env("TELEGRAM_API_URL"),
        bot_token=_env("BOT_TOKEN") or _env("TELEGRAM_BOT_TOKEN"),
        telegram_chat_id=_int("TELEGRAM_CHAT_ID"),
        telegram_polling_enabled=(_env("TELEGRAM_POLLING_ENABLED", "true") or "true").lower() in {"1", "true", "yes", "on"},
        admin_telegram_id=_int("ADMIN_TELEGRAM_ID"),
        vk_group_token=_env("VK_GROUP_TOKEN"),
        vk_group_id=_int("VK_GROUP_ID"),
        vk_peer_id=_int("VK_PEER_ID"),
    )


settings = load_settings()
