"""
HTTP client for communicating with the Invoice Keeper backend API.

This is the only module that talks to the backend. It wraps aiohttp,
handles authentication (bearer token or HTTP Basic), timeouts, and
error handling so that the rest of the bot never deals with raw
network exceptions.
"""

import logging
from datetime import date, timedelta
from typing import Any, Optional

import aiohttp
from aiohttp import BasicAuth, ClientTimeout

from config import Settings
from models import PaymentOccurrence, PaymentOccurrenceList

logger = logging.getLogger(__name__)


class BackendAPIError(Exception):
    """Raised when the backend returns an error or is unreachable."""


class BackendAPIClient:
    """Thin async client for the backend's payment-occurrences API."""

    def __init__(self, settings: Settings):
        self._settings = settings
        self._base_url = settings.backend_url
        self._timeout = ClientTimeout(total=settings.http_timeout)

    def _auth(self) -> Optional[BasicAuth]:
        """Return BasicAuth if login/password are configured, else None
        (in which case the bearer token header is used instead, if set)."""
        if self._settings.admin_basic_auth_login and self._settings.admin_basic_auth_password:
            return BasicAuth(
                self._settings.admin_basic_auth_login,
                self._settings.admin_basic_auth_password,
            )
        return None

    def _headers(self) -> dict:
        headers = {"Accept": "application/json"}
        if self._settings.admin_api_token:
            headers["Authorization"] = f"Bearer {self._settings.admin_api_token}"
        return headers

    async def get_upcoming_unpaid_occurrences(
        self, days_ahead: int
    ) -> list[PaymentOccurrence]:
        """Fetch unpaid payment occurrences due within `days_ahead` days.

        Calls: GET /api/v1/payment-occurrences?is_paid=false&due_before=...
        """
        url = f"{self._base_url}/api/v1/payment-occurrences"
        due_before_date = date.today() + timedelta(days=days_ahead)

        params = {
            "is_paid": "false",
            "due_date_to": due_before_date.isoformat(),
        }

        try:
            async with aiohttp.ClientSession(
                timeout=self._timeout, headers=self._headers(), auth=self._auth()
            ) as session:
                async with session.get(url, params=params) as resp:
                    if resp.status >= 400:
                        body = await resp.text()
                        raise BackendAPIError(
                            f"GET {url} failed with status {resp.status}: {body}"
                        )
                    raw = await resp.json()
        except aiohttp.ClientError as exc:
            raise BackendAPIError(f"Network error calling {url}: {exc}") from exc
        except Exception as exc:  # noqa: BLE001 - we want to wrap *any* failure
            raise BackendAPIError(f"Unexpected error calling {url}: {exc}") from exc

        try:
            occurrence_list = PaymentOccurrenceList.from_raw(raw)
        except Exception as exc:  # noqa: BLE001
            raise BackendAPIError(f"Failed to parse response from {url}: {exc}") from exc

        # Defensive client-side filtering in case the backend ignores
        # query params or has slightly different filtering semantics.
        result = [
            occ
            for occ in occurrence_list.items
            if not occ.is_paid and occ.due_date <= due_before_date
        ]
        return result

    async def mark_as_paid(
        self, occurrence_id: int, actual_payment_date: Optional[date] = None
    ) -> PaymentOccurrence:
        """Mark a payment occurrence as paid.

        Calls: PATCH /api/v1/payment-occurrences/{id}
        Body: {"is_paid": true, "actual_payment_date": "YYYY-MM-DD"}
        """
        url = f"{self._base_url}/api/v1/payment-occurrences/{occurrence_id}"
        payment_date = actual_payment_date or date.today()
        payload: dict[str, Any] = {
            "is_paid": True,
            "actual_payment_date": payment_date.isoformat(),
        }

        try:
            async with aiohttp.ClientSession(
                timeout=self._timeout, headers=self._headers(), auth=self._auth()
            ) as session:
                async with session.patch(url, json=payload) as resp:
                    if resp.status >= 400:
                        body = await resp.text()
                        raise BackendAPIError(
                            f"PATCH {url} failed with status {resp.status}: {body}"
                        )
                    raw = await resp.json()
        except aiohttp.ClientError as exc:
            raise BackendAPIError(f"Network error calling {url}: {exc}") from exc
        except Exception as exc:  # noqa: BLE001
            raise BackendAPIError(f"Unexpected error calling {url}: {exc}") from exc

        try:
            return PaymentOccurrence.model_validate(raw)
        except Exception as exc:  # noqa: BLE001
            raise BackendAPIError(f"Failed to parse response from {url}: {exc}") from exc
