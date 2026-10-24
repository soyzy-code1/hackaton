"""
Local SQLite-backed storage used only for de-duplicating reminder
notifications.

The bot itself is a "thin client" and does NOT store payment data --
all payment state lives in the backend. This module only tracks which
(occurrence_id, due_date) pairs have already been notified about, so
that a bot restart does not cause duplicate reminder messages to be
sent for the same day.
"""

import asyncio
import logging
import sqlite3
from contextlib import closing
from datetime import date

logger = logging.getLogger(__name__)


class NotificationStore:
    """Tracks which payment occurrences have already been reminded about
    for a given due date, backed by a local SQLite file."""

    def __init__(self, db_path: str):
        self._db_path = db_path
        self._lock = asyncio.Lock()
        self._init_db()

    def _init_db(self) -> None:
        with closing(sqlite3.connect(self._db_path)) as conn:
            conn.execute(
                """
                CREATE TABLE IF NOT EXISTS sent_notifications (
                    occurrence_id INTEGER NOT NULL,
                    due_date TEXT NOT NULL,
                    sent_at TEXT NOT NULL,
                    PRIMARY KEY (occurrence_id, due_date)
                )
                """
            )
            conn.commit()

    async def already_notified(self, occurrence_id: int, due_date: date) -> bool:
        """Return True if a reminder for this occurrence/due_date was
        already sent."""
        async with self._lock:
            return await asyncio.to_thread(
                self._already_notified_sync, occurrence_id, due_date
            )

    def _already_notified_sync(self, occurrence_id: int, due_date: date) -> bool:
        with closing(sqlite3.connect(self._db_path)) as conn:
            cur = conn.execute(
                "SELECT 1 FROM sent_notifications WHERE occurrence_id = ? AND due_date = ?",
                (occurrence_id, due_date.isoformat()),
            )
            return cur.fetchone() is not None

    async def mark_notified(self, occurrence_id: int, due_date: date) -> None:
        """Record that a reminder for this occurrence/due_date was sent."""
        async with self._lock:
            await asyncio.to_thread(self._mark_notified_sync, occurrence_id, due_date)

    def _mark_notified_sync(self, occurrence_id: int, due_date: date) -> None:
        with closing(sqlite3.connect(self._db_path)) as conn:
            conn.execute(
                """
                INSERT OR IGNORE INTO sent_notifications
                    (occurrence_id, due_date, sent_at)
                VALUES (?, ?, ?)
                """,
                (occurrence_id, due_date.isoformat(), date.today().isoformat()),
            )
            conn.commit()

    async def clear_for_occurrence(self, occurrence_id: int) -> None:
        """Remove all notification records for an occurrence, e.g. after
        it has been marked as paid (so re-adding a similar one later
        would not be silently suppressed by stale rows)."""
        async with self._lock:
            await asyncio.to_thread(self._clear_for_occurrence_sync, occurrence_id)

    def _clear_for_occurrence_sync(self, occurrence_id: int) -> None:
        with closing(sqlite3.connect(self._db_path)) as conn:
            conn.execute(
                "DELETE FROM sent_notifications WHERE occurrence_id = ?",
                (occurrence_id,),
            )
            conn.commit()
