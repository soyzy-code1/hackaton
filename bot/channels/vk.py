"""Manual aiohttp implementation of VK Bots Long Poll transport."""

import asyncio
import json
import logging
import random
from datetime import date
from typing import Any

import aiohttp

from api_client import BackendAPIClient, BackendAPIError
from config import Settings
from models import PaymentOccurrence
from storage import NotificationStore

logger = logging.getLogger("channel.vk")
VK_API = "https://api.vk.com/method"
VK_VERSION = "5.199"


class VKChannel:
    name = "vk"

    def __init__(self, settings: Settings, api_client: BackendAPIClient, store: NotificationStore):
        self.settings = settings
        self.api_client = api_client
        self.store = store
        self._session: aiohttp.ClientSession | None = None
        self._stop = asyncio.Event()

    async def _call(self, method: str, **params: Any) -> dict[str, Any]:
        if self._session is None or self._session.closed:
            self._session = aiohttp.ClientSession(timeout=aiohttp.ClientTimeout(total=40))
        params.update(access_token=self.settings.vk_group_token, v=VK_VERSION)
        async with self._session.post(f"{VK_API}/{method}", data=params) as response:
            data = await response.json()
        if "error" in data:
            error = data["error"]
            if method == "groups.getLongPollServer" and error.get("error_code") == 15:
                raise RuntimeError(
                    "VK group token has no access to groups.getLongPollServer; "
                    "enable Bots Long Poll API and issue a group token with messages permissions"
                )
            raise RuntimeError(f"VK {method}: {error}")
        return data["response"]

    async def send_reminder(self, payment: PaymentOccurrence) -> bool:
        keyboard = self._keyboard(payment)
        await self._call(
            "messages.send",
            peer_id=self.settings.vk_peer_id,
            message=self._render_text(payment),
            keyboard=keyboard,
            random_id=random.randint(-(2**31), 2**31 - 1),
        )
        logger.info("[vk] reminder sent for occurrence %s", payment.id)
        return True

    async def handle_payment_callback(self, occurrence_id: int, channel_user_id: str, target_paid: bool) -> PaymentOccurrence | None:
        try:
            occurrence = await self.api_client.update_payment_status(occurrence_id, target_paid, date.today())
            if occurrence.is_paid:
                await self.store.clear_for_occurrence(occurrence_id)
            return occurrence
        except BackendAPIError:
            logger.exception("[vk] failed to set occurrence %s status", occurrence_id)
            return None

    @staticmethod
    def _keyboard(payment: PaymentOccurrence) -> str:
        label = "↩️ Вернуть в неоплаченное" if payment.is_paid else "✅ Отметить оплаченным"
        return json.dumps({"inline": True, "buttons": [[{
            "action": {
                "type": "callback",
                "label": label,
                "payload": json.dumps({"occurrence_id": payment.id, "target_paid": int(not payment.is_paid)}),
            }
        }]]}, ensure_ascii=False)

    @staticmethod
    def _render_text(payment: PaymentOccurrence) -> str:
        status = "✅ Оплачено" if payment.is_paid else "❌ Не оплачено"
        return f"{payment.format_reminder()}\n\n{status}"

    async def _answer_event(self, event_id: str, user_id: int, text: str) -> None:
        try:
            await self._call("messages.sendMessageEventAnswer", event_id=event_id, user_id=user_id, event_data=json.dumps({"type": "show_snackbar", "text": text}, ensure_ascii=False))
        except Exception:
            logger.warning("[vk] failed to answer message_event", exc_info=True)

    async def _get_long_poll_server(self) -> tuple[str, str, str]:
        result = await self._call("groups.getLongPollServer", group_id=self.settings.vk_group_id)
        return result["server"], result["key"], result["ts"]

    async def run(self) -> None:
        backoff = 1
        try:
            while not self._stop.is_set():
                try:
                    if self._session is None or self._session.closed:
                        self._session = aiohttp.ClientSession(timeout=aiohttp.ClientTimeout(total=40))
                    server, key, ts = await self._get_long_poll_server()
                    backoff = 1
                    while not self._stop.is_set():
                        async with self._session.get(server, params={"act": "a_check", "key": key, "ts": ts, "wait": 25}) as response:
                            result = await response.json()
                        backoff = 1
                        if "ts" in result:
                            ts = result["ts"]
                        if result.get("failed"):
                            break
                        for update in result.get("updates", []):
                            await self._handle_update(update)
                except asyncio.CancelledError:
                    raise
                except (aiohttp.ClientError, asyncio.TimeoutError, RuntimeError, KeyError, ValueError):
                    logger.warning("[vk] Long Poll error; retrying", exc_info=True)
                    await asyncio.sleep(backoff)
                    backoff = min(backoff * 2, 30)
        finally:
            await self.close()

    async def _handle_update(self, update: dict[str, Any]) -> None:
        if update.get("type") != "message_event":
            return
        obj = update.get("object", {})
        payload = obj.get("payload", {})
        if isinstance(payload, str):
            try:
                payload = json.loads(payload)
            except json.JSONDecodeError:
                payload = {}
        try:
            occurrence_id = int(payload["occurrence_id"])
            user_id = str(obj.get("user_id", ""))
            target_paid = bool(int(payload.get("target_paid", 1)))
            occurrence = await self.handle_payment_callback(occurrence_id, user_id, target_paid)
            if occurrence is not None:
                await self._answer_event(
                    obj.get("event_id", ""),
                    int(obj.get("user_id", 0)),
                    "Оплачено ✅" if occurrence.is_paid else "Не оплачено ❌",
                )
                conversation_message_id = obj.get("conversation_message_id")
                if conversation_message_id:
                    await self._call(
                        "messages.edit",
                        peer_id=obj.get("peer_id", self.settings.vk_peer_id),
                        conversation_message_id=conversation_message_id,
                        message=self._render_text(occurrence),
                        keyboard=self._keyboard(occurrence),
                    )
            else:
                await self._answer_event(obj.get("event_id", ""), int(obj.get("user_id", 0)), "Ошибка, попробуйте позже")
        except (KeyError, TypeError, ValueError):
            logger.warning("[vk] malformed message_event: %r", update)

    async def close(self) -> None:
        self._stop.set()
        if self._session and not self._session.closed:
            await self._session.close()
