"""
Entry point for the Invoice Keeper reminder bot.

Wires together:
  - aiogram Bot/Dispatcher (long polling)
  - BackendAPIClient (HTTP client to the backend)
  - NotificationStore (local SQLite dedup store)
  - APScheduler (daily cron job for reminders)

The bot is a "thin client": it holds no payment state of its own
(other than the local notification-dedup log) and relies entirely on
the backend API for payment data.
"""

import asyncio
import logging
import os
import sys

from aiogram import Bot, Dispatcher
from aiogram.client.default import DefaultBotProperties
from aiogram.client.session.aiohttp import AiohttpSession
from aiogram.enums import ParseMode

from api_client import BackendAPIClient
from config import settings
from handlers import register_handlers
from scheduler import create_scheduler
from storage import NotificationStore


def setup_logging() -> None:
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s | %(levelname)-8s | %(name)s | %(message)s",
        handlers=[logging.StreamHandler(sys.stdout)],
    )
    # Quiet down noisy third-party loggers a bit.
    logging.getLogger("apscheduler").setLevel(logging.WARNING)

async def main() -> None:
    setup_logging()
    logger = logging.getLogger(__name__)

    logger.info("Starting Invoice Keeper bot...")

    telegram_proxy = os.getenv("TELEGRAM_PROXY", "").strip()
    session = AiohttpSession(proxy=telegram_proxy) if telegram_proxy else None
    bot = Bot(
        token=settings.bot_token,
        default=DefaultBotProperties(parse_mode=ParseMode.HTML),
        session=session,
    )
    dispatcher = Dispatcher()

    api_client = BackendAPIClient(settings)
    notification_store = NotificationStore(settings.sqlite_path)

    # Register handlers and inject shared dependencies so they are
    # available as handler kwargs (aiogram 3 workflow data injection).
    register_handlers(dispatcher)
    dispatcher["api_client"] = api_client
    dispatcher["settings"] = settings
    dispatcher["notification_store"] = notification_store

    scheduler = create_scheduler(bot, api_client, settings, notification_store)
    scheduler.start()
    logger.info(
        "Scheduler started: daily check at %02d:%02d (%s).",
        settings.scheduler_hour,
        settings.scheduler_minute,
        settings.timezone,
    )

    try:
        await bot.delete_webhook(drop_pending_updates=True)
        await dispatcher.start_polling(bot)
    finally:
        scheduler.shutdown(wait=False)
        await bot.session.close()


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except (KeyboardInterrupt, SystemExit):
        logging.getLogger(__name__).info("Bot stopped.")
