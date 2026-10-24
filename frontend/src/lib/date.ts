/**
 * Date helpers.
 *
 * The API speaks ISO 8601 (UTC). The UI uses native <input type="date"> which
 * speaks `YYYY-MM-DD`, and renders dates as `DD.MM.YYYY`.
 *
 * A display timezone can be configured (see `store/settings.ts`). All
 * "today" calculations and calendar placement use that timezone, so the UI
 * stays consistent with the configured clock.
 */

export const DEFAULT_TIMEZONE = 'Europe/Moscow';

const MONTH_NAMES = [
  'январь',
  'февраль',
  'март',
  'апрель',
  'май',
  'июнь',
  'июль',
  'август',
  'сентябрь',
  'октябрь',
  'ноябрь',
  'декабрь',
];

export const MONTH_NAMES_GENITIVE = [
  'января',
  'февраля',
  'марта',
  'апреля',
  'мая',
  'июня',
  'июля',
  'августа',
  'сентября',
  'октября',
  'ноября',
  'декабря',
];

export const WEEKDAY_NAMES_SHORT = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'];

export function monthName(monthIndex: number): string {
  return MONTH_NAMES[monthIndex] ?? '';
}

export function monthNameGenitive(monthIndex: number): string {
  return MONTH_NAMES_GENITIVE[monthIndex] ?? '';
}

/* -------------------------------------------------------------------------- */
/* Timezone helpers                                                            */
/* -------------------------------------------------------------------------- */

function isValidTimeZone(timeZone: string): boolean {
  try {
    new Intl.DateTimeFormat('en-US', { timeZone });
    return true;
  } catch {
    return false;
  }
}

export function normalizeTimeZone(timeZone: string): string {
  return isValidTimeZone(timeZone) ? timeZone : DEFAULT_TIMEZONE;
}

/** Offset of the timezone relative to UTC, in milliseconds, at the given moment. */
export function getTimeZoneOffsetMs(date: Date, timeZone: string): number {
  const formatter = new Intl.DateTimeFormat('en-US', {
    timeZone,
    hour12: false,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
  const parts = formatter.formatToParts(date);
  const values: Record<string, number> = {};
  for (const part of parts) {
    if (part.type === 'literal') continue;
    values[part.type] = Number(part.value);
  }
  const hour = values.hour === 24 ? 0 : values.hour;
  const asUtc = Date.UTC(
    values.year ?? 1970,
    (values.month ?? 1) - 1,
    values.day ?? 1,
    hour,
    values.minute ?? 0,
    values.second ?? 0,
  );
  return asUtc - date.getTime();
}

/** The UTC instant of midnight in the given timezone. */
export function startOfTodayInTz(timeZone: string): Date {
  const zone = normalizeTimeZone(timeZone);
  const now = new Date();
  const offset = getTimeZoneOffsetMs(now, zone);
  const local = new Date(now.getTime() + offset);
  return new Date(
    Date.UTC(local.getUTCFullYear(), local.getUTCMonth(), local.getUTCDate()) - offset,
  );
}

/** Calendar parts of an instant in the given timezone. */
function partsInTz(date: Date, timeZone: string): {
  year: number;
  month: number;
  day: number;
} {
  const formatter = new Intl.DateTimeFormat('en-CA', {
    timeZone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  });
  const parts = formatter.formatToParts(date);
  const values: Record<string, number> = {};
  for (const part of parts) {
    if (part.type === 'literal') continue;
    values[part.type] = Number(part.value);
  }
  return {
    year: values.year ?? 1970,
    month: values.month ?? 1,
    day: values.day ?? 1,
  };
}

/** `YYYY-MM-DD` of an instant in the given timezone. */
export function dayKeyInTz(iso: string, timeZone: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return '';
  const { year, month, day } = partsInTz(date, normalizeTimeZone(timeZone));
  return `${year}-${String(month).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
}

/** `YYYY-MM-DD` of today in the given timezone. */
export function todayKeyInTz(timeZone: string): string {
  const { year, month, day } = partsInTz(
    startOfTodayInTz(timeZone),
    normalizeTimeZone(timeZone),
  );
  return `${year}-${String(month).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
}

/** `YYYY-MM-DD` of an ISO date, used by native date inputs (UTC based). */
export function toDateInputValue(iso: string | null | undefined): string {
  if (!iso) return '';
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return '';
  const year = date.getUTCFullYear();
  const month = String(date.getUTCMonth() + 1).padStart(2, '0');
  const day = String(date.getUTCDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

/** Converts a `YYYY-MM-DD` input value into an ISO string for the API. */
export function fromDateInputValue(value: string): string {
  if (!value) return '';
  const parsed = new Date(`${value}T00:00:00.000Z`);
  if (Number.isNaN(parsed.getTime())) return '';
  return parsed.toISOString();
}

/** Formats an ISO date as `DD.MM.YYYY`. */
export function formatDate(iso: string | null | undefined): string {
  if (!iso) return '—';
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return '—';
  const day = String(date.getUTCDate()).padStart(2, '0');
  const month = String(date.getUTCMonth() + 1).padStart(2, '0');
  return `${day}.${month}.${date.getUTCFullYear()}`;
}

/** Formats an ISO date as `DD.MM.YYYY` in the given timezone. */
export function formatDateInTz(
  iso: string | null | undefined,
  timeZone: string,
): string {
  if (!iso) return '—';
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return '—';
  const { year, month, day } = partsInTz(date, normalizeTimeZone(timeZone));
  return `${String(day).padStart(2, '0')}.${String(month).padStart(2, '0')}.${year}`;
}

/** Formats a money amount with thin-space thousands separators. */
export function formatAmount(amount: number, currency: string): string {
  const formatted = new Intl.NumberFormat('ru-RU', {
    minimumFractionDigits: Number.isInteger(amount) ? 0 : 2,
    maximumFractionDigits: 2,
  }).format(amount);
  return `${formatted} ${currency}`;
}

/** Returns today's date as a `YYYY-MM-DD` string (local time). */
export function todayDateInputValue(timeZone: string = DEFAULT_TIMEZONE): string {
  return todayKeyInTz(timeZone);
}

/** Difference between two ISO dates in whole days (b - a). */
export function daysBetween(aIso: string, bIso: string): number {
  const a = new Date(aIso).getTime();
  const b = new Date(bIso).getTime();
  if (Number.isNaN(a) || Number.isNaN(b)) return 0;
  return Math.round((b - a) / 86_400_000);
}

/** Number of days in a month (1-based month index). */
export function daysInMonth(year: number, month: number): number {
  return new Date(Date.UTC(year, month + 1, 0)).getUTCDate();
}

/**
 * The due date of a monthly period, clamped to the last day of the month
 * (31 January + 1 month -> 28/29 February).
 */
export function monthlyDueDate(
  anchorIso: string,
  year: number,
  month: number,
): string {
  const anchor = new Date(anchorIso);
  const day = anchor.getUTCDate();
  const clamped = Math.min(day, daysInMonth(year, month));
  return new Date(Date.UTC(year, month, clamped)).toISOString();
}

/** Monday-based weekday index (0 = Monday). */
export function weekdayIndexInTz(iso: string, timeZone: string): number {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return 0;
  const { year, month, day } = partsInTz(date, normalizeTimeZone(timeZone));
  const utc = new Date(Date.UTC(year, month - 1, day)).getUTCDay();
  return (utc + 6) % 7;
}

export interface CalendarDay {
  /** `YYYY-MM-DD` */
  key: string;
  /** ISO instant of the day start. */
  iso: string;
  dayNumber: number;
  inCurrentMonth: boolean;
  isToday: boolean;
  isWeekend: boolean;
}

/**
 * Builds a 6x7 grid (Monday first) for the given month.
 */
export function buildMonthGrid(
  year: number,
  month: number,
  timeZone: string,
): CalendarDay[] {
  const zone = normalizeTimeZone(timeZone);
  const todayKey = todayKeyInTz(zone);
  const firstOfMonth = new Date(Date.UTC(year, month, 1));
  const offset = firstOfMonth.getUTCDay();
  const leadingBlanks = (offset + 6) % 7;

  const days: CalendarDay[] = [];
  for (let index = 0; index < 42; index += 1) {
    const date = new Date(Date.UTC(year, month, 1 - leadingBlanks + index));
    const iso = date.toISOString();
    const key = `${date.getUTCFullYear()}-${String(date.getUTCMonth() + 1).padStart(2, '0')}-${String(
      date.getUTCDate(),
    ).padStart(2, '0')}`;
    const weekday = (date.getUTCDay() + 6) % 7;
    days.push({
      key,
      iso,
      dayNumber: date.getUTCDate(),
      inCurrentMonth: date.getUTCMonth() === month && date.getUTCFullYear() === year,
      isToday: key === todayKey,
      isWeekend: weekday >= 5,
    });
  }
  return days;
}
