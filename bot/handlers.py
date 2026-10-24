"""
Aiogram handlers: processing the "✅ Оплачено" inline button callback.

When a user taps the button attached to a reminder message, the bot
sends a PATCH request to the backend marking that payment occurrence
as paid (with today's date as the actual payment date), then edits
the original message to remove the button and show a confirmation.
"""

import logging
from datetime import date

from aiogram import Bot, Dispatcher, F, Router
from aiogram.types import CallbackQuery

from api_client import BackendAPIClient, BackendAPIError
from config import Settings
from storage import NotificationStore

logger = logging.getLogger(__name__)

router = Router(name="invoice_keeper_handlers")

# Prefix used in callback_data so we can route the callback and easily
# extract the occurrence id. Format: "pay:<occurrence_id>"
PAY_CALLBACK_PREFIX = "pay:"


def build_pay_callback_data(occurrence_id: int) -> str:
    return f"{PAY_CALLBACK_PREFIX}{occurrence_id}"


def _extract_occurrence_id(callback_data: str) -> int:
    return int(callback_data[len(PAY_CALLBACK_PREFIX):])


@router.callback_query(F.data.startswith(PAY_CALLBACK_PREFIX))
async def handle_mark_as_paid(
    callback: CallbackQuery,
    api_client: BackendAPIClient,
    settings: Settings,
    notification_store: NotificationStore,
) -> None:
    """Handle a tap on the "✅ Оплачено" inline button."""
    try:
        occurrence_id = _extract_occurrence_id(callback.data or "")
    except (ValueError, TypeError):
        logger.error("Malformed callback_data received: %r", callback.data)
        await callback.answer("Некорректные данные кнопки.", show_alert=True)
        return

    try:
        today = date.today()
        occurrence = await api_client.mark_as_paid(
            occurrence_id, actual_payment_date=today
        )
    except BackendAPIError as exc:
        logger.error(
            "Failed to mark occurrence %s as paid: %s", occurrence_id, exc
        )
        await callback.answer(
            "Не удалось отметить платёж как оплаченный. Попробуйте ещё раз позже.",
            show_alert=True,
        )
        await _notify_admin(callback.bot, settings, str(exc))
        return

    # Drop any pending reminder records for this occurrence so that if a
    # similar one is created later it is not silently suppressed.
    await notification_store.clear_for_occurrence(occurrence_id)

    paid_date = occurrence.actual_payment_date or today
    paid_date_str = paid_date.strftime("%d.%m.%Y")

    original_text = callback.message.text or occurrence.format_reminder()
    new_text = f"{original_text}\n\n✅ Оплачено {paid_date_str}"

    try:
        await callback.message.edit_text(new_text, reply_markup=None)
    except Exception as exc:  # noqa: BLE001 - editing may fail (message too old, etc.)
        logger.warning("Failed to edit message after marking as paid: %s", exc)

    await callback.answer("Платёж отмечен как оплаченный ✅")


async def _notify_admin(bot: Bot, settings: Settings, error_text: str) -> None:
    """Best-effort notification to the admin's personal chat about an
    API failure. Never raises."""
    if not settings.admin_telegram_id:
        return
    try:
        await bot.send_message(
            settings.admin_telegram_id,
            f"⚠️ Invoice Keeper Bot: ошибка при обращении к backend.\n\n{error_text}",
        )
    except Exception as exc:  # noqa: BLE001
        logger.error("Failed to notify admin about backend error: %s", exc)


def register_handlers(dispatcher: Dispatcher) -> None:
    """Attach the router with all handlers to the dispatcher."""
    dispatcher.include_router(router)
