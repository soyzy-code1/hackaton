"""Business-level channel dispatcher with per-channel deduplication."""

import asyncio
import logging

from models import PaymentOccurrence
from storage import NotificationStore

logger = logging.getLogger(__name__)


class Notifier:
    def __init__(self, channels: list, store: NotificationStore):
        self.channels = channels
        self.store = store

    async def send(self, payment: PaymentOccurrence) -> None:
        async def send_one(channel) -> None:
            if await self.store.already_notified(payment.id, channel.name):
                logger.debug("[%s] occurrence %s already sent", channel.name, payment.id)
                return
            try:
                if await channel.send_reminder(payment):
                    await self.store.mark_notified(payment.id, channel.name)
                    if hasattr(channel, "api_client"):
                        await channel.api_client.record_notification(payment, channel.name)
            except Exception:
                logger.exception("[%s] failed to send occurrence %s", channel.name, payment.id)

        await asyncio.gather(*(send_one(channel) for channel in self.channels))
