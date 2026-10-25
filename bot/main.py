"""Run Telegram, VK Long Poll and the shared scheduler in one event loop."""

import asyncio
import logging
import signal
import sys

from aiogram import Bot, Dispatcher
from aiogram.exceptions import TelegramConflictError, TelegramNetworkError, TelegramRetryAfter

from api_client import BackendAPIClient
from channels.telegram import TelegramChannel, create_bot
from channels.vk import VKChannel
from config import settings
from notifier import Notifier
from scheduler import create_scheduler
from storage import NotificationStore

# Long-poll timeout and the request timeout that must outlive it.
POLL_TIMEOUT = 30
POLL_REQUEST_TIMEOUT = POLL_TIMEOUT + 15


def setup_logging() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s | %(levelname)-8s | %(name)s | %(message)s", handlers=[logging.StreamHandler(sys.stdout)])
    logging.getLogger("apscheduler").setLevel(logging.WARNING)


def is_conflict_error(exc: BaseException) -> bool:
    """Detect Telegram's 409 Conflict in a version-independent way.

    aiogram raises TelegramConflictError, but the exact exception class depends
    on the installed version and on the transport layer, so the message is
    checked as well. A conflict means another consumer holds the same token:
    retrying only hides the problem, so the caller must stop polling.
    """
    if isinstance(exc, TelegramConflictError):
        return True
    message = str(exc).lower()
    return "terminated by other getupdates request" in message or "conflict" in message


async def run_telegram_polling(dispatcher: Dispatcher, bot: Bot) -> None:
    """Run exactly one polling consumer and stop on Telegram's conflict response.

    The loop is implemented explicitly instead of relying on
    ``Dispatcher.start_polling`` so that a conflict is detected by type, logged
    with an actionable message and terminates the poller instead of spinning in
    a retry loop that would spam the Bot API.
    """
    logger = logging.getLogger("channel.telegram")
    delay = 5
    offset: int | None = None
    while True:
        try:
            if offset is None:
                await bot.delete_webhook(drop_pending_updates=False)
                me = await bot.get_me()
                logger.info("[telegram] connected as @%s, chat_id=%s", me.username, settings.telegram_chat_id)
            updates = await bot.get_updates(
                offset=offset,
                timeout=POLL_TIMEOUT,
                request_timeout=POLL_REQUEST_TIMEOUT,
                allowed_updates=["message", "callback_query"],
            )
            delay = 5
            for update in updates:
                offset = update.update_id + 1
                try:
                    await dispatcher.feed_update(bot, update)
                except Exception:
                    logger.exception("[telegram] failed to handle update %s", update.update_id)
        except asyncio.CancelledError:
            raise
        except TelegramRetryAfter as exc:
            retry_after = int(getattr(exc, "retry_after", 5) or 5)
            logger.warning("[telegram] flood control: retry after %ss", retry_after)
            await asyncio.sleep(retry_after)
        except Exception as exc:
            if is_conflict_error(exc):
                logger.critical(
                    "[telegram] polling stopped: another process is using this bot token (%s). "
                    "Stop every other bot instance/local polling process before restarting this container.",
                    exc,
                )
                return
            if isinstance(exc, TelegramNetworkError):
                logger.error(
                    "[telegram] api.telegram.org unavailable (%s). Retry in %ss. "
                    "If Telegram is blocked in your network, set TELEGRAM_PROXY or TELEGRAM_API_URL in .env.",
                    exc, delay,
                )
            else:
                logger.exception("[telegram] polling failed (%s). Retry in %ss", exc, delay)
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
