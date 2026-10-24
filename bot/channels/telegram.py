"""Aiogram transport for Telegram."""

import logging
from datetime import date

from aiogram import Bot, Dispatcher, F, Router
from aiogram.client.default import DefaultBotProperties
from aiogram.client.session.aiohttp import AiohttpSession
from aiogram.enums import ParseMode
from aiogram.types import BufferedInputFile, CallbackQuery, InlineKeyboardButton, InlineKeyboardMarkup

from api_client import BackendAPIClient, BackendAPIError
from config import Settings
from models import PaymentOccurrence
from storage import NotificationStore

logger = logging.getLogger("channel.telegram")


def build_qr_png(payment: PaymentOccurrence) -> bytes:
    import io
    import qrcode
    payload = f"invoice-keeper:{payment.id}:{payment.title}:{payment.amount}:{payment.currency}:{payment.due_date.isoformat()}"
    output = io.BytesIO()
    qrcode.make(payload).save(output, format="PNG")
    return output.getvalue()
PAY_PREFIX = "pay:"


def build_pay_callback_data(occurrence_id: int, target_paid: bool) -> str:
    return f"{PAY_PREFIX}{occurrence_id}:{int(target_paid)}"


class TelegramChannel:
    name = "telegram"

    def __init__(self, bot: Bot, api_client: BackendAPIClient, store: NotificationStore):
        self.bot = bot
        self.api_client = api_client
        self.store = store
        self.router = Router(name="telegram_channel")
        self.router.callback_query(F.data.startswith(PAY_PREFIX))(self._callback)

    async def send_reminder(self, payment: PaymentOccurrence) -> bool:
        await self.bot.send_message(
            chat_id=self._chat_id,
            text=self._render_text(payment),
            reply_markup=self._keyboard(payment),
        )
        qr_png = build_qr_png(payment)
        await self.bot.send_photo(
            chat_id=self._chat_id,
            photo=BufferedInputFile(qr_png, filename=f"receipt-{payment.id}.png"),
            caption=f"Чек по платежу «{payment.title}»\nСумма: {payment.amount} {payment.currency}",
        )
        logger.info("[telegram] reminder sent for occurrence %s", payment.id)
        return True

    async def handle_payment_callback(self, occurrence_id: int, channel_user_id: str, target_paid: bool) -> PaymentOccurrence | None:
        try:
            occurrence = await self.api_client.update_payment_status(occurrence_id, target_paid, date.today())
        except BackendAPIError as exc:
            logger.error(
                "[telegram] failed to set occurrence %s target_paid=%s: %s",
                occurrence_id,
                target_paid,
                exc,
                exc_info=True,
            )
            return None
        if occurrence.is_paid:
            await self.store.clear_for_occurrence(occurrence_id)
        return occurrence

    @staticmethod
    def _keyboard(payment: PaymentOccurrence) -> InlineKeyboardMarkup:
        label = "↩️ Вернуть в неоплаченное" if payment.is_paid else "✅ Отметить оплаченным"
        return InlineKeyboardMarkup(inline_keyboard=[[
            InlineKeyboardButton(text=label, callback_data=build_pay_callback_data(payment.id, not payment.is_paid))
        ]])

    @staticmethod
    def _render_text(payment: PaymentOccurrence) -> str:
        status = "✅ Оплачено" if payment.is_paid else "❌ Не оплачено"
        return f"{payment.format_reminder()}\n\n{status}"

    def configure(self, chat_id: int) -> None:
        self._chat_id = chat_id

    async def _callback(self, callback: CallbackQuery) -> None:
        try:
            data = callback.data or ""
            parts = data[len(PAY_PREFIX):].split(":")
            if len(parts) not in (1, 2) or (len(parts) == 2 and parts[1] not in {"0", "1"}):
                raise ValueError("invalid callback state")
            occurrence_id = int(parts[0])
            # Legacy pay:<id> buttons always meant "mark as paid".
            target_paid = len(parts) == 1 or parts[1] == "1"
        except (TypeError, ValueError):
            await callback.answer("Некорректные данные кнопки.", show_alert=True)
            return
        logger.info(
            "[telegram] callback received occurrence=%s target_paid=%s user=%s",
            occurrence_id,
            target_paid,
            callback.from_user.id,
        )
        occurrence = await self.handle_payment_callback(occurrence_id, str(callback.from_user.id), target_paid)
        if occurrence is not None:
            if callback.message:
                try:
                    await callback.message.edit_text(
                        self._render_text(occurrence),
                        reply_markup=self._keyboard(occurrence),
                    )
                except Exception:
                    logger.warning("[telegram] failed to edit payment message", exc_info=True)
            await callback.answer(
                "Платёж отмечен как оплаченный ✅"
                if occurrence.is_paid
                else "Платёж отмечен как неоплаченный ❌"
            )
        else:
            await callback.answer("Не удалось изменить статус платежа. Попробуйте позже.", show_alert=True)

    def register(self, dispatcher: Dispatcher) -> None:
        dispatcher.include_router(self.router)


async def create_bot(settings: Settings) -> Bot:
    """Create the aiogram Bot.

    TELEGRAM_PROXY  — http://, https://, socks5:// proxy for api.telegram.org
                      (e.g. http://host.docker.internal:10801 for a proxy on the host).
    TELEGRAM_API_URL — alternative Bot API base URL (reverse proxy / Cloudflare Worker),
                      e.g. https://my-worker.workers.dev
    """
    session_kwargs: dict = {}
    if settings.telegram_proxy:
        session_kwargs["proxy"] = settings.telegram_proxy
        logger.info("[telegram] using proxy %s", settings.telegram_proxy.split("@")[-1])
    if settings.telegram_api_url:
        from aiogram.client.telegram import TelegramAPIServer

        session_kwargs["api"] = TelegramAPIServer.from_base(settings.telegram_api_url.rstrip("/"))
        logger.info("[telegram] using API server %s", settings.telegram_api_url)
    session = AiohttpSession(**session_kwargs) if session_kwargs else None
    return Bot(
        token=settings.bot_token or "",
        session=session,
        default=DefaultBotProperties(parse_mode=ParseMode.HTML),
    )
