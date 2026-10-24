"""
Pydantic models used to validate data received from the backend API.

The backend is the single source of truth for payments ("payment
occurrences"). The bot never stores payment state itself -- it only
validates and renders what the backend returns.
"""

from datetime import date, datetime
from decimal import Decimal
from typing import Optional

from pydantic import BaseModel, Field, field_validator


class PaymentOccurrence(BaseModel):
    """Represents a single payment occurrence returned by the backend.

    Expected backend fields (per the case spec):
        id, title, amount, currency, due_date, is_paid
    """

    id: int
    title: str
    amount: Decimal
    currency: str
    due_date: date
    is_paid: bool = False
    actual_payment_date: Optional[date] = None

    @field_validator("due_date", "actual_payment_date", mode="before")
    @classmethod
    def _parse_date(cls, value):
        """Allow both 'YYYY-MM-DD' and full ISO datetime strings."""
        if value is None or isinstance(value, date):
            return value
        if isinstance(value, str):
            # Try plain date first, then full ISO datetime.
            try:
                return date.fromisoformat(value[:10])
            except ValueError:
                return datetime.fromisoformat(value).date()
        return value

    def format_reminder(self) -> str:
        """Render the reminder message text for this occurrence."""
        due_str = self.due_date.strftime("%d.%m.%Y")
        return (
            f"🔔 Напоминание об оплате: {self.title} — "
            f"{self.amount} {self.currency}. Срок оплаты: {due_str}."
        )


class PaymentOccurrenceList(BaseModel):
    """Wrapper for a list response, tolerant of either a bare list or a
    paginated object with an 'items'/'results' field."""

    items: list[PaymentOccurrence] = Field(default_factory=list)

    @classmethod
    def from_raw(cls, raw) -> "PaymentOccurrenceList":
        if isinstance(raw, list):
            return cls(items=[PaymentOccurrence.model_validate(item) for item in raw])
        if isinstance(raw, dict):
            for key in ("items", "results", "data"):
                if key in raw and isinstance(raw[key], list):
                    return cls(
                        items=[
                            PaymentOccurrence.model_validate(item)
                            for item in raw[key]
                        ]
                    )
        raise ValueError("Unexpected payment-occurrences response shape")
