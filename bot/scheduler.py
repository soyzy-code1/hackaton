"""
APScheduler-based cron job that checks for upcoming unpaid payments
and sends Telegram reminders.

Runs once a day at the configured hour/minute (in the configured
timezone). For every unpaid occurrence due within `reminder_days_ahead`
days that hasn't already been notified about (tracked via
NotificationStore / SQLite), it sends a reminder message with an
inline "✅ Оплачено" button.
"""

import logging

from aiogram import Bot
from aiogram.types import InlineKeyboardButton, InlineKeyboardMarkup
from apscheduler.schedulers.asyncio import AsyncIOScheduler
from apscheduler.triggers.cron import CronTrigger

from api_client import BackendAPIClient, BackendAPIError
from config import Settings
from handlers import build_pay_callback_data
from storage import NotificationStore

logger = logging.getLogger(__name__)


def _build_reminder_keyboard(occurrence_id: int) -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(
        inline_keyboard=[
            [
                InlineKeyboardButton(
                    text="✅ Оплачено",
                    callback_data=build_pay_callback_data(occurrence_id),
                )
            ]
        ]
    )


async def check_and_send_reminders(
    bot: Bot,
    api_client: BackendAPIClient,
    settings: Settings,
    notification_store: NotificationStore,
) -> None:
    """The core job: fetch upcoming unpaid occurrences and notify about
    any that haven't been notified yet. Never raises -- all errors are
    caught and logged (and optionally reported to the admin)."""
    logger.info("Running scheduled reminder check...")

    try:
        occurrences = await api_client.get_upcoming_unpaid_occurrences(
            days_ahead=settings.reminder_days_ahead
        )
    except BackendAPIError as exc:
        logger.error("Backend unavailable during scheduled check: %s", exc)
        await _notify_admin(bot, settings, str(exc))
        return
    except Exception as exc:  # noqa: BLE001 - scheduler job must never crash
        logger.exception("Unexpected error during scheduled check: %s", exc)
        await _notify_admin(bot, settings, f"Unexpected error: {exc}")
        return

    if not occurrences:
        logger.info("No upcoming unpaid occurrences found.")
        return

    for occurrence in occurrences:
        try:
            already_sent = await notification_store.already_notified(
                occurrence.id, occurrence.due_date
            )
            if already_sent:
                logger.debug(
                    "Skipping occurrence %s (due %s): already notified.",
                    occurrence.id,
                    occurrence.due_date,
                )
                continue

            await bot.send_message(
                chat_id=settings.telegram_chat_id,
                text=occurrence.format_reminder(),
                reply_markup=_build_reminder_keyboard(occurrence.id),
            )
            await notification_store.mark_notified(occurrence.id, occurrence.due_date)
            logger.info(
                "Sent reminder for occurrence %s (due %s).",
                occurrence.id,
                occurrence.due_date,
            )
        except Exception as exc:  # noqa: BLE001 - keep processing other occurrences
            logger.exception(
                "Failed to send/record reminder for occurrence %s: %s",
                occurrence.id,
                exc,
            )
            await _notify_admin(bot, settings, str(exc))


async def _notify_admin(bot: Bot, settings: Settings, error_text: str) -> None:
    """Best-effort notification to the admin's personal chat. Never raises."""
    if not settings.admin_telegram_id:
        return
    try:
        await bot.send_message(
            settings.admin_telegram_id,
            f"⚠️ Invoice Keeper Bot: ошибка при обращении к backend.\n\n{error_text}",
        )
    except Exception as exc:  # noqa: BLE001
        logger.error("Failed to notify admin about backend error: %s", exc)


def create_scheduler(
    bot: Bot,
    api_client: BackendAPIClient,
    settings: Settings,
    notification_store: NotificationStore,
) -> AsyncIOScheduler:
    """Build (but do not start) the APScheduler instance with the daily
    cron job registered."""
    scheduler = AsyncIOScheduler(timezone=settings.timezone)

    scheduler.add_job(
        check_and_send_reminders,
        trigger=CronTrigger(
            hour=settings.scheduler_hour,
            minute=settings.scheduler_minute,
            timezone=settings.timezone,
        ),
        kwargs={
            "bot": bot,
            "api_client": api_client,
            "settings": settings,
            "notification_store": notification_store,
        },
        id="daily_payment_reminder_check",
        replace_existing=True,
        misfire_grace_time=3600,  # tolerate up to 1h delay (e.g. container restart)
    )

    return scheduler
