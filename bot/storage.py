"""SQLite log preventing duplicate reminders per occurrence and channel."""

import asyncio
import sqlite3
from contextlib import closing
from datetime import datetime, timezone


class NotificationStore:
    def __init__(self, db_path: str):
        self._db_path = db_path
        self._lock = asyncio.Lock()
        self._init_db()

    def _init_db(self) -> None:
        with closing(sqlite3.connect(self._db_path)) as conn:
            conn.execute("""
                CREATE TABLE IF NOT EXISTS sent_log (
                    occurrence_id INTEGER NOT NULL,
                    channel TEXT NOT NULL,
                    sent_at TEXT NOT NULL,
                    PRIMARY KEY (occurrence_id, channel)
                )
            """)
            conn.commit()

    async def already_notified(self, occurrence_id: int, channel: str) -> bool:
        async with self._lock:
            return await asyncio.to_thread(self._already_sync, occurrence_id, channel)

    def _already_sync(self, occurrence_id: int, channel: str) -> bool:
        with closing(sqlite3.connect(self._db_path)) as conn:
            row = conn.execute(
                "SELECT 1 FROM sent_log WHERE occurrence_id = ? AND channel = ?",
                (occurrence_id, channel),
            ).fetchone()
            return row is not None

    async def mark_notified(self, occurrence_id: int, channel: str) -> None:
        async with self._lock:
            await asyncio.to_thread(self._mark_sync, occurrence_id, channel)

    def _mark_sync(self, occurrence_id: int, channel: str) -> None:
        with closing(sqlite3.connect(self._db_path)) as conn:
            conn.execute(
                "INSERT OR IGNORE INTO sent_log (occurrence_id, channel, sent_at) VALUES (?, ?, ?)",
                (occurrence_id, channel, datetime.now(timezone.utc).isoformat()),
            )
            conn.commit()

    async def clear_for_occurrence(self, occurrence_id: int) -> None:
        async with self._lock:
            await asyncio.to_thread(self._clear_sync, occurrence_id)

    def _clear_sync(self, occurrence_id: int) -> None:
        with closing(sqlite3.connect(self._db_path)) as conn:
            conn.execute("DELETE FROM sent_log WHERE occurrence_id = ?", (occurrence_id,))
            conn.commit()
