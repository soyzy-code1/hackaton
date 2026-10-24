/**
 * Financial planning helpers: totals as of a date, paid vs remaining,
 * cash-flow projection and CSV import parsing.
 *
 * Every function here is pure. The UI and the mock backend share the same
 * logic, and a real backend can implement the same contract server-side
 * (see `GET /payments/summary?as_of_date=` and `GET /budgets`).
 */
import type {
  CsvImportError,
  CsvImportPreview,
  CsvImportRow,
  Currency,
  PaymentCategory,
  PaymentOccurrence,
} from '@/api/types';
import { CURRENCIES, PAYMENT_CATEGORY_LABELS } from '@/api/types';

/* -------------------------------------------------------------------------- */
/* Period helpers                                                              */
/* -------------------------------------------------------------------------- */

/** `YYYY-MM-DD` of an ISO instant (UTC calendar day, matching due dates). */
export function dayKey(iso: string): string {
  return iso.slice(0, 10);
}

export interface PeriodRange {
  /** Inclusive `YYYY-MM-DD`. */
  from: string;
  /** Inclusive `YYYY-MM-DD`. */
  to: string;
}

/** Calendar month that contains `asOf` (a `YYYY-MM-DD` key). */
export function monthRange(asOf: string): PeriodRange {
  const [year, month] = asOf.split('-').map(Number) as [number, number];
  const last = new Date(Date.UTC(year, month, 0)).getUTCDate();
  const mm = String(month).padStart(2, '0');
  return { from: `${year}-${mm}-01`, to: `${year}-${mm}-${String(last).padStart(2, '0')}` };
}

/** Calendar year that contains `asOf`. */
export function yearRange(asOf: string): PeriodRange {
  const year = asOf.slice(0, 4);
  return { from: `${year}-01-01`, to: `${year}-12-31` };
}

function inRange(key: string, range: PeriodRange): boolean {
  return key >= range.from && key <= range.to;
}

/* -------------------------------------------------------------------------- */
/* Totals as of a date                                                         */
/* -------------------------------------------------------------------------- */

export interface CurrencyTotals {
  currency: Currency;
  /** Everything due in the period. */
  planned: number;
  /** Paid in the period, counted only if paid on or before `asOf`. */
  paid: number;
  /** planned - paid. */
  remaining: number;
  /** Unpaid and already past due as of `asOf`. */
  overdue: number;
  plannedCount: number;
  paidCount: number;
  overdueCount: number;
}

function emptyTotals(currency: Currency): CurrencyTotals {
  return {
    currency,
    planned: 0,
    paid: 0,
    remaining: 0,
    overdue: 0,
    plannedCount: 0,
    paidCount: 0,
    overdueCount: 0,
  };
}

/**
 * Totals per currency for the periods due in `range`, evaluated as of the
 * `asOf` day. A payment confirmed *after* `asOf` is treated as unpaid on that
 * day, so «итоги на 15 ноября» show exactly what was known on 15 November.
 */
export function totalsAsOf(
  occurrences: PaymentOccurrence[],
  range: PeriodRange,
  asOf: string,
): Record<Currency, CurrencyTotals> {
  const result = {} as Record<Currency, CurrencyTotals>;
  for (const currency of CURRENCIES) result[currency] = emptyTotals(currency);

  for (const item of occurrences) {
    const due = dayKey(item.due_date);
    if (!inRange(due, range)) continue;
    const bucket = result[item.currency];
    bucket.planned += item.amount;
    bucket.plannedCount += 1;

    const paidKey = item.actual_payment_date ? dayKey(item.actual_payment_date) : null;
    const paidByAsOf = item.status === 'paid' && paidKey !== null && paidKey <= asOf;
    if (paidByAsOf) {
      bucket.paid += item.amount;
      bucket.paidCount += 1;
    } else if (due < asOf) {
      // Overdue starts the day after the due date.
      bucket.overdue += item.amount;
      bucket.overdueCount += 1;
    }
  }

  for (const currency of CURRENCIES) {
    const bucket = result[currency];
    bucket.remaining = Math.max(0, bucket.planned - bucket.paid);
  }
  return result;
}

/** Number of payments due in the range, per currency (for the «3 счёта в USD» line). */
export function countByCurrency(
  occurrences: PaymentOccurrence[],
  range: PeriodRange,
): Record<Currency, number> {
  const result: Record<Currency, number> = { RUB: 0, USD: 0, EUR: 0 };
  for (const item of occurrences) {
    if (inRange(dayKey(item.due_date), range)) result[item.currency] += 1;
  }
  return result;
}

/* -------------------------------------------------------------------------- */
/* Cash-flow projection                                                        */
/* -------------------------------------------------------------------------- */

export interface CashFlowDay {
  /** `YYYY-MM-DD` */
  key: string;
  dayNumber: number;
  /** Amount due on this day (unpaid periods only). */
  outflow: number;
  /** Cumulative outflow since the start of the month. */
  cumulative: number;
  /** Balance left after the payments of this day. */
  balance: number;
  items: PaymentOccurrence[];
  /** True when the cumulative outflow exceeds the available balance. */
  isGap: boolean;
}

export interface CashFlowProjection {
  days: CashFlowDay[];
  startBalance: number;
  totalOutflow: number;
  minBalance: number;
  /** First day the balance goes below zero, if any. */
  firstGapDay: CashFlowDay | null;
}

/**
 * Projects the balance of one currency through a month: every unpaid period
 * is an outflow on its due date. Overdue periods from earlier days are added
 * on the first day of the month, because they still have to be paid.
 */
export function projectCashFlow(
  occurrences: PaymentOccurrence[],
  currency: Currency,
  range: PeriodRange,
  startBalance: number,
): CashFlowProjection {
  const [year, month] = range.from.split('-').map(Number) as [number, number];
  const lastDay = Number(range.to.slice(8, 10));
  const unpaid = occurrences.filter(
    (item) => item.currency === currency && item.status !== 'paid',
  );

  const carriedOver = unpaid.filter((item) => dayKey(item.due_date) < range.from);

  const days: CashFlowDay[] = [];
  let cumulative = 0;
  let firstGapDay: CashFlowDay | null = null;
  let minBalance = startBalance;

  for (let day = 1; day <= lastDay; day += 1) {
    const key = `${year}-${String(month).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
    const items = unpaid.filter((item) => dayKey(item.due_date) === key);
    if (day === 1) items.unshift(...carriedOver);
    const outflow = items.reduce((sum, item) => sum + item.amount, 0);
    cumulative += outflow;
    const balance = startBalance - cumulative;
    const entry: CashFlowDay = {
      key,
      dayNumber: day,
      outflow,
      cumulative,
      balance,
      items,
      isGap: balance < 0,
    };
    if (entry.isGap && !firstGapDay) firstGapDay = entry;
    minBalance = Math.min(minBalance, balance);
    days.push(entry);
  }

  return { days, startBalance, totalOutflow: cumulative, minBalance, firstGapDay };
}

/* -------------------------------------------------------------------------- */
/* CSV import                                                                  */
/* -------------------------------------------------------------------------- */

export const CSV_TEMPLATE_HEADER = [
  'name',
  'amount',
  'currency',
  'next_payment_date',
  'periodicity',
  'category',
  'assigned_usernames',
  'notes',
];

export const CSV_TEMPLATE_EXAMPLE = [
  ['Аренда офиса', '45000', 'RUB', '2026-11-05', 'monthly', 'housing', 'ivan', 'Договор №12'],
  ['Сервер VPS', '19.99', 'USD', '2026-11-10', 'monthly', 'infrastructure', 'ivan;anna', ''],
  ['Подписка Figma', '15', 'EUR', '2026-11-15', 'monthly', 'subscriptions', '', ''],
];

/** Splits one CSV line, honouring quotes. Accepts `,` and `;` separators. */
function splitCsvLine(line: string, separator: string): string[] {
  const cells: string[] = [];
  let current = '';
  let quoted = false;
  for (let index = 0; index < line.length; index += 1) {
    const char = line[index];
    if (quoted) {
      if (char === '"') {
        if (line[index + 1] === '"') {
          current += '"';
          index += 1;
        } else {
          quoted = false;
        }
      } else {
        current += char;
      }
    } else if (char === '"') {
      quoted = true;
    } else if (char === separator) {
      cells.push(current.trim());
      current = '';
    } else {
      current += char;
    }
  }
  cells.push(current.trim());
  return cells;
}

function detectSeparator(headerLine: string): string {
  const semicolons = headerLine.split(';').length;
  const commas = headerLine.split(',').length;
  return semicolons > commas ? ';' : ',';
}

/** Accepts `YYYY-MM-DD` and `DD.MM.YYYY`; returns `YYYY-MM-DD` or null. */
function normalizeDate(value: string): string | null {
  const iso = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  const ru = /^(\d{2})\.(\d{2})\.(\d{4})$/.exec(value);
  let year: number;
  let month: number;
  let day: number;
  if (iso) {
    year = Number(iso[1]);
    month = Number(iso[2]);
    day = Number(iso[3]);
  } else if (ru) {
    day = Number(ru[1]);
    month = Number(ru[2]);
    year = Number(ru[3]);
  } else {
    return null;
  }
  const date = new Date(Date.UTC(year, month - 1, day));
  if (
    date.getUTCFullYear() !== year ||
    date.getUTCMonth() !== month - 1 ||
    date.getUTCDate() !== day
  ) {
    return null;
  }
  return `${year}-${String(month).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
}

const CATEGORY_ALIASES: Record<string, PaymentCategory> = Object.fromEntries(
  (Object.keys(PAYMENT_CATEGORY_LABELS) as PaymentCategory[]).flatMap((key) => [
    [key, key],
    [PAYMENT_CATEGORY_LABELS[key].toLocaleLowerCase('ru'), key],
  ]),
);

/**
 * Parses and validates a CSV file. Valid rows are returned ready to be sent
 * to `POST /recurring-payments/import`; invalid rows are reported with the
 * line number so the administrator can fix the file.
 */
export function parsePaymentsCsv(
  text: string,
  knownUsernames: string[],
): CsvImportPreview {
  const lines = text
    .replace(/^\uFEFF/, '')
    .split(/\r?\n/)
    .map((line) => line.trimEnd());
  const errors: CsvImportError[] = [];
  const validRows: CsvImportRow[] = [];

  const headerIndex = lines.findIndex((line) => line.trim() !== '');
  if (headerIndex === -1) {
    return { valid_rows: [], errors: [{ row: 0, message: 'Файл пустой' }] };
  }

  const headerLine = lines[headerIndex] ?? '';
  const separator = detectSeparator(headerLine);
  const header = splitCsvLine(headerLine, separator).map((cell) =>
    cell.toLowerCase(),
  );
  const column = (name: string) => header.indexOf(name);

  for (const required of ['name', 'amount', 'currency', 'next_payment_date']) {
    if (column(required) === -1) {
      errors.push({ row: headerIndex + 1, message: `Нет обязательной колонки «${required}»` });
    }
  }
  if (errors.length > 0) return { valid_rows: [], errors };

  const known = new Set(knownUsernames.map((name) => name.toLowerCase()));
  const seen = new Set<string>();

  for (let index = headerIndex + 1; index < lines.length; index += 1) {
    const line = lines[index] ?? '';
    if (line.trim() === '') continue;
    const rowNumber = index + 1;
    const cells = splitCsvLine(line, separator);
    const get = (name: string) => {
      const position = column(name);
      return position === -1 ? '' : (cells[position] ?? '').trim();
    };

    const rowErrors: string[] = [];
    const name = get('name');
    if (!name) rowErrors.push('не указано название');

    const amount = Number(get('amount').replace(/\s/g, '').replace(',', '.'));
    if (!Number.isFinite(amount) || amount <= 0) rowErrors.push('некорректная сумма');

    const currency = get('currency').toUpperCase() as Currency;
    if (!CURRENCIES.includes(currency)) rowErrors.push('валюта должна быть RUB, USD или EUR');

    const date = normalizeDate(get('next_payment_date'));
    if (!date) rowErrors.push('дата должна быть в формате ГГГГ-ММ-ДД или ДД.ММ.ГГГГ');

    const periodicity = (get('periodicity') || 'monthly').toLowerCase();
    if (periodicity !== 'monthly') rowErrors.push('поддерживается только периодичность monthly');

    const rawCategory = get('category').toLocaleLowerCase('ru');
    const category = rawCategory ? CATEGORY_ALIASES[rawCategory] : 'other';
    if (!category) rowErrors.push(`неизвестная категория «${get('category')}»`);

    const usernames = get('assigned_usernames')
      .split(/[;|]/)
      .map((value) => value.trim())
      .filter(Boolean);
    const unknown = usernames.filter((value) => !known.has(value.toLowerCase()));
    if (unknown.length > 0) rowErrors.push(`неизвестные пользователи: ${unknown.join(', ')}`);

    const duplicateKey = `${name.toLowerCase()}|${currency}|${date ?? ''}`;
    if (name && date && seen.has(duplicateKey)) rowErrors.push('дубликат строки в файле');

    if (rowErrors.length > 0) {
      errors.push({ row: rowNumber, message: rowErrors.join('; ') });
      continue;
    }

    seen.add(duplicateKey);
    validRows.push({
      name,
      amount,
      currency,
      next_payment_date: `${date}T00:00:00.000Z`,
      periodicity: 'monthly',
      category: category ?? 'other',
      assigned_usernames: usernames,
      notes: get('notes') || undefined,
    });
  }

  if (validRows.length === 0 && errors.length === 0) {
    errors.push({ row: headerIndex + 1, message: 'В файле нет строк с данными' });
  }

  return { valid_rows: validRows, errors };
}
