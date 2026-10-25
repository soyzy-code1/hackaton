"""Verify the Telegram delivery path in production.

The script answers a single question: can the bot reach api.telegram.org and
deliver a message to the configured chat? It prints a machine-readable JSON
report and never prints the bot token, the chat id or any credential.

Exit codes:
    0 - the channel is configured and the delivery succeeded
    1 - the channel is configured but the delivery failed
    3 - the channel is not configured, nothing was checked
"""

import asyncio
import json
import sys
from datetime import datetime, timezone

EXIT_OK = 0
EXIT_FAILURE = 1
EXIT_SKIPPED = 3

TEST_MESSAGE = (
    "✅ Invoice Keeper: проверка доставки Telegram.\n"
    "Если вы видите это сообщение — канал настроен и напоминания доходят."
)


def emit(payload: dict) -> None:
    print(json.dumps(payload, ensure_ascii=False))


async def run() -> int:
    # The configuration module validates the backend variables at import time.
    # A broken environment must be reported as a structured result instead of a
    # traceback, otherwise the deploy log hides the real cause.
    try:
        from channels.telegram import create_bot
        from config import settings
    except Exception as exc:  # noqa: BLE001 - the reason is reported as JSON
        emit({
            "channel": "telegram",
            "status": "failed",
            "ok": False,
            "stage": "load_config",
            "error": f"{type(exc).__name__}: {exc}",
        })
        return EXIT_FAILURE

    if not settings.telegram_enabled:
        emit({
            "channel": "telegram",
            "status": "skipped",
            "ok": False,
            "reason": "telegram channel is not configured (BOT_TOKEN / TELEGRAM_CHAT_ID)",
        })
        return EXIT_SKIPPED

    bot = await create_bot(settings)
    try:
        try:
            me = await bot.get_me()
        except Exception as exc:  # noqa: BLE001 - the reason is reported as JSON
            emit({
                "channel": "telegram",
                "status": "failed",
                "ok": False,
                "stage": "get_me",
                "error": f"{type(exc).__name__}: {exc}",
            })
            return EXIT_FAILURE

        try:
            sent = await bot.send_message(chat_id=settings.telegram_chat_id, text=TEST_MESSAGE)
        except Exception as exc:  # noqa: BLE001 - the reason is reported as JSON
            emit({
                "channel": "telegram",
                "status": "failed",
                "ok": False,
                "stage": "send_message",
                "bot": f"@{me.username}",
                "error": f"{type(exc).__name__}: {exc}",
            })
            return EXIT_FAILURE

        emit({
            "channel": "telegram",
            "status": "ok",
            "ok": True,
            "stage": "send_message",
            "bot": f"@{me.username}",
            "message_id": sent.message_id,
            "sent_at": datetime.now(timezone.utc).isoformat(),
        })
        return EXIT_OK
    finally:
        await bot.session.close()


def main() -> int:
    try:
        return asyncio.run(run())
    except Exception as exc:  # noqa: BLE001 - the reason is reported as JSON
        emit({
            "channel": "telegram",
            "status": "failed",
            "ok": False,
            "error": f"{type(exc).__name__}: {exc}",
        })
        return EXIT_FAILURE


if __name__ == "__main__":
    sys.exit(main())
