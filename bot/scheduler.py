"""Shared reminder scheduler.

Reminders are checked:
* immediately on bot start;
* every REMINDER_POLL_SECONDS (default 60s) — so a payment added with less than
  REMINDER_DAYS_AHEAD days left is announced almost immediately;
* daily at SCHEDULER_HOUR:SCHEDULER_MINUTE (kept for backwards compatibility).

Per-channel deduplication in NotificationStore guarantees one message per
occurrence, so frequent polling never produces duplicates.
"""

import asyncio
import logging
from datetime import datetime

from apscheduler.schedulers.asyncio import AsyncIOScheduler
from apscheduler.triggers.cron import CronTrigger
from apscheduler.triggers.interval import IntervalTrigger

from api_client import BackendAPIClient
from config import Settings
from notifier import Notifier

logger = logging.getLogger(__name__)

_check_lock = asyncio.Lock()


async def check_and_send_reminders(api_client: BackendAPIClient, settings: Settings, notifier: Notifier) -> None:
    if _check_lock.locked():
        logger.debug("Reminder check already running, skipping")
        return
    async with _check_lock:
        try:
            payments = await api_client.get_upcoming_unpaid_occurrences(settings.reminder_days_ahead)
            logger.info("Reminder check found %d unpaid occurrence(s) due within %d day(s)", len(payments), settings.reminder_days_ahead)
            for payment in payments:
                await notifier.send(payment)
        except Exception:
            logger.exception("Scheduled reminder check failed")


def create_scheduler(api_client: BackendAPIClient, settings: Settings, notifier: Notifier) -> AsyncIOScheduler:
    scheduler = AsyncIOScheduler(timezone=settings.timezone)
    job_kwargs = {"api_client": api_client, "settings": settings, "notifier": notifier}
    scheduler.add_job(
        check_and_send_reminders,
        IntervalTrigger(seconds=settings.reminder_poll_seconds, timezone=settings.timezone),
        kwargs=job_kwargs,
        id="payment_reminder_poll",
        replace_existing=True,
        next_run_time=datetime.now(scheduler.timezone),
        max_instances=1,
        coalesce=True,
        misfire_grace_time=settings.reminder_poll_seconds,
    )
    scheduler.add_job(
        check_and_send_reminders,
        CronTrigger(hour=settings.scheduler_hour, minute=settings.scheduler_minute, timezone=settings.timezone),
        kwargs=job_kwargs,
        id="daily_payment_reminder_check",
        replace_existing=True,
        misfire_grace_time=3600,
    )
    return scheduler
