"""Shared async HTTP client for the backend API."""

import json
import logging
from datetime import date, timedelta
from typing import Any

import aiohttp
from aiohttp import ClientTimeout

from config import Settings
from models import PaymentOccurrence, PaymentOccurrenceList

logger = logging.getLogger(__name__)


class BackendAPIError(Exception):
    pass


class BackendAPIClient:
    def __init__(self, settings: Settings):
        self._base_url = settings.backend_url
        self._headers = {
            "Accept": "application/json",
            "Authorization": f"Bearer {settings.admin_api_token}",
            "X-Internal-Token": settings.admin_api_token,
        }
        self._timeout = ClientTimeout(total=settings.http_timeout)
        self._session: aiohttp.ClientSession | None = None

    async def start(self) -> None:
        if self._session is None or self._session.closed:
            self._session = aiohttp.ClientSession(timeout=self._timeout, headers=self._headers)

    async def close(self) -> None:
        if self._session and not self._session.closed:
            await self._session.close()

    async def _request(self, method: str, path: str, **kwargs: Any) -> Any:
        await self.start()
        assert self._session is not None
        url = f"{self._base_url}{path}"
        try:
            async with self._session.request(method, url, **kwargs) as response:
                if response.status >= 400:
                    raise BackendAPIError(f"{method} {url} failed ({response.status}): {await response.text()}")
                if response.status in (204, 205):
                    return None
                body = await response.text()
                if not body.strip():
                    return None
                try:
                    return json.loads(body)
                except json.JSONDecodeError as exc:
                    raise BackendAPIError(
                        f"{method} {url} returned invalid JSON ({response.status})"
                    ) from exc
        except aiohttp.ClientError as exc:
            raise BackendAPIError(f"Network error calling {url}: {exc}") from exc

    async def record_notification(self, payment: PaymentOccurrence, channel: str) -> None:
        await self._request("POST", "/api/v1/internal/notifications", json={
            "type": "reminder",
            "title": f"Напоминание: {payment.title}",
            "message": payment.format_reminder(),
            "occurrence_id": payment.id,
            "channel": channel,
        })

    async def get_upcoming_unpaid_occurrences(self, days_ahead: int) -> list[PaymentOccurrence]:
        raw = await self._request("GET", "/api/v1/payment-occurrences")
        occurrences = PaymentOccurrenceList.from_raw(raw).items
        limit = date.today() + timedelta(days=days_ahead)
        return [item for item in occurrences if not item.is_paid and item.due_date <= limit]

    async def update_payment_status(
        self,
        occurrence_id: int,
        is_paid: bool,
        actual_payment_date: date | None = None,
    ) -> PaymentOccurrence:
        payload: dict[str, Any] = {"is_paid": is_paid}
        if is_paid:
            payload["actual_payment_date"] = (
                actual_payment_date or date.today()
            ).isoformat()
        raw = await self._request(
            "PATCH",
            f"/api/v1/payment-occurrences/{occurrence_id}",
            json=payload,
        )
        return PaymentOccurrence.model_validate(raw)

    async def toggle_payment_status(self, occurrence_id: int) -> PaymentOccurrence:
        raw = await self._request(
            "GET",
            f"/api/v1/payment-occurrences/{occurrence_id}",
        )
        current = PaymentOccurrence.model_validate(raw)
        return await self.update_payment_status(
            occurrence_id,
            not current.is_paid,
            date.today(),
        )
