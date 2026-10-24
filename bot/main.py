"""Run Telegram, VK Long Poll and the shared scheduler in one event loop."""

import asyncio
import logging
import signal
import sys

from aiogram import Bot, Dispatcher

from api_client import BackendAPIClient
from channels.telegram import TelegramChannel, create_bot
from channels.vk import VKChannel
from config import settings
from notifier import Notifier
from scheduler import create_scheduler
from storage import NotificationStore


def setup_logging() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s | %(levelname)-8s | %(name)s | %(message)s", handlers=[logging.StreamHandler(sys.stdout)])
    logging.getLogger("apscheduler").setLevel(logging.WARNING)


async def run_telegram_polling(dispatcher: Dispatcher, bot: Bot) -> None:
    """Run exactly one polling consumer and stop on Telegram's conflict response."""
    logger = logging.getLogger("channel.telegram")
    delay = 5
    while True:
        try:
            await bot.delete_webhook(drop_pending_updates=False)
            me = await bot.get_me()
            logger.info("[telegram] connected as @%s, chat_id=%s", me.username, settings.telegram_chat_id)
            delay = 5
            await dispatcher.start_polling(bot, handle_signals=False, close_bot_session=False)
            return
        except asyncio.CancelledError:
            raise
        except Exception as exc:
            if "Conflict: terminated by other getUpdates request" in str(exc):
                logger.critical(
                    "[telegram] polling stopped: another process is using this bot token. "
                    "Stop every other bot instance/local polling process before restarting this container."
                )
                return
            logger.error(
                "[telegram] api.telegram.org unavailable (%s). Retry in %ss. "
                "If Telegram is blocked in your network, set TELEGRAM_PROXY or TELEGRAM_API_URL in .env.",
                exc, delay,
            )
            await asyncio.sleep(delay)
            delay = min(delay * 2, 120)


async def main() -> None:
    setup_logging()
    logger = logging.getLogger(__name__)
    api_client = BackendAPIClient(settings)
    await api_client.start()
    store = NotificationStore(settings.sqlite_path)
    dispatcher = Dispatcher()
    bot = None
    telegram_channel = None
    vk_channel = None
    tasks: list[asyncio.Task] = []
    cron = None

    try:
        if settings.telegram_enabled:
            bot = await create_bot(settings)
            telegram_channel = TelegramChannel(bot, api_client, store)
            telegram_channel.configure(settings.telegram_chat_id)  # type: ignore[arg-type]
            telegram_channel.register(dispatcher)
            if settings.telegram_polling_enabled:
                tasks.append(asyncio.create_task(run_telegram_polling(dispatcher, bot), name="telegram-polling"))
                logger.info("Telegram channel enabled with polling (chat_id=%s)", settings.telegram_chat_id)
            else:
                logger.info("Telegram sending enabled; polling disabled")

        if settings.vk_enabled:
            vk_channel = VKChannel(settings, api_client, store)
            tasks.append(asyncio.create_task(vk_channel.run(), name="vk-long-poll"))
            logger.info("VK channel enabled")

        if not tasks:
            logger.warning(
                "No bot channels configured; Telegram/VK polling is disabled. "
                "Set BOT_TOKEN and TELEGRAM_CHAT_ID or VK_GROUP_TOKEN, VK_GROUP_ID and VK_PEER_ID to enable notifications."
            )

        notifier = Notifier([channel for channel in (telegram_channel, vk_channel) if channel], store)
        cron = create_scheduler(api_client, settings, notifier)
        cron.start()
        logger.info(
            "Reminder scheduler started: every %ss + daily at %02d:%02d, horizon %d day(s)",
            settings.reminder_poll_seconds, settings.scheduler_hour, settings.scheduler_minute, settings.reminder_days_ahead,
        )
        stop_event = asyncio.Event()
        loop = asyncio.get_running_loop()
        for signum in (signal.SIGINT, signal.SIGTERM):
            try:
                loop.add_signal_handler(signum, stop_event.set)
            except NotImplementedError:
                pass
        await stop_event.wait()
    finally:
        if cron:
            cron.shutdown(wait=False)
        if vk_channel:
            await vk_channel.close()
        for task in tasks:
            task.cancel()
        await asyncio.gather(*tasks, return_exceptions=True)
        if bot:
            await bot.session.close()
        await api_client.close()
        logger.info("Bot stopped")


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except (KeyboardInterrupt, SystemExit):
        pass
