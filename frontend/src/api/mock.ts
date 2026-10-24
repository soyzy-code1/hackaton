/**
 * In-memory mock backend.
 *
 * Used for local development when the real API is unreachable, so that the
 * SPA stays fully functional (login, dashboard, CRUD) without a backend.
 *
 * Data model:
 *
 * - A recurring payment is a *schedule* owned by the administrator. It is
 *   assigned to a set of users (`assigned_user_ids`).
 * - Every assigned user has their **own ledger**: their own periods
 *   (payment occurrences) with their own due dates and their own
 *   "paid / unpaid" marks. One user's actions never affect another user's
 *   payment state.
 * - Periods are generated on a monthly calendar: an overdue period never
 *   stops the following periods, and a new period never erases past payments.
 *
 * Data lives in module scope, so it resets on a full page reload — which is
 * the desired behaviour for a mock.
 */
import type {
  CreateRecurringPaymentPayload,
  CreateUserPayload,
  CsvImportRow,
  Currency,
  PaymentOccurrence,
  PaymentStatus,
  Periodicity,
  RecurringPayment,
  Summary,
  UpdateOccurrencePayload,
  UpdateUserPayload,
  User,
} from './types';

/** Prefix used to build mock auth tokens; the user id follows it. */
export const MOCK_TOKEN_PREFIX = 'mock-token:';
let mockSessionUserId: string | null = null;

export function getMockSessionUserId(): string | null {
  return mockSessionUserId;
}

const ADMIN_USERNAME = import.meta.env.VITE_ADMIN_USERNAME || 'admin';
const ADMIN_PASSWORD = import.meta.env.VITE_ADMIN_PASSWORD || 'admin';

interface MockUser extends User {
  password: string;
}

/** A single period (payment occurrence) of a recurring payment, for one user. */
interface MockPeriod {
  id: string;
  recurring_payment_id: string;
  /** Owner of the period — the ledger it belongs to. */
  user_id: string;
  due_date: string;
  is_paid: boolean;
  actual_payment_date: string | null;
}

interface MockRecurringPayment {
  id: string;
  name: string;
  amount: number;
  currency: Currency;
  periodicity: Periodicity;
  category?: import('./types').PaymentCategory;
  notes?: string;
  receipt_url?: string | null;
  /** Due date of the very first period. */
  anchor_date: string;
  /** Due date of the nearest period that still has to be paid. */
  next_payment_date: string;
  /** Users the payment is assigned to. */
  assigned_user_ids: string[];
  /** False once the payment is stopped; its history is kept. */
  active: boolean;
}

/**
 * Counter for generated ids. It starts above the range used by the seeded
 * records so that generated ids can never collide with them.
 */
let idCounter = 1000;
function nextId(prefix: string): string {
  idCounter += 1;
  return `${prefix}-${idCounter}`;
}

/* -------------------------------------------------------------------------- */
/* Date helpers (UTC based, so the behaviour does not depend on the timezone)  */
/* -------------------------------------------------------------------------- */

function startOfToday(): Date {
  const now = new Date();
  return new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()));
}

/** ISO string of a day that is `daysFromToday` days away from today. */
function isoDate(daysFromToday: number): string {
  const date = startOfToday();
  date.setUTCDate(date.getUTCDate() + daysFromToday);
  return date.toISOString();
}

/**
 * Adds `months` to an ISO date, keeping the original day of the month.
 * When the target month has no such day, the last day of that month is used
 * (31 January + 1 month -> 28/29 February).
 */
function addMonthsClamped(iso: string, months: number): string {
  const source = new Date(iso);
  const day = source.getUTCDate();
  const target = new Date(
    Date.UTC(source.getUTCFullYear(), source.getUTCMonth() + months, 1),
  );
  const lastDayOfMonth = new Date(
    Date.UTC(target.getUTCFullYear(), target.getUTCMonth() + 1, 0),
  ).getUTCDate();
  target.setUTCDate(Math.min(day, lastDayOfMonth));
  return target.toISOString();
}

function isBefore(iso: string, reference: Date): boolean {
  return new Date(iso).getTime() < reference.getTime();
}

/* -------------------------------------------------------------------------- */
/* Seed data                                                                   */
/* -------------------------------------------------------------------------- */

const ADMIN_ID = 'user-1';

const users: MockUser[] = [
  { id: ADMIN_ID, username: ADMIN_USERNAME, password: ADMIN_PASSWORD, role: 'admin' },
  { id: 'user-2', username: 'ivan', password: 'ivan', role: 'user' },
  { id: 'user-3', username: 'anna', password: 'anna', role: 'user' },
];

interface SeedPayment {
  id: string;
  name: string;
  amount: number;
  currency: Currency;
  /** Due date of the first period, relative to today. */
  anchorOffset: number;
  /** Users the payment is assigned to. */
  assignedTo: string[];
  /** Periods (by index) that are already confirmed as paid, per user. */
  paidPeriods?: Record<string, number[]>;
}

const SEED_PAYMENTS: SeedPayment[] = [
  // Аренда в рублях — просрочена, за ней идут следующие периоды.
  {
    id: 'rp-rent',
    name: 'Аренда офиса',
    amount: 45000,
    currency: 'RUB',
    anchorOffset: -6,
    assignedTo: ['user-2'],
  },
  // Сервер в долларах — тестовые данные из ТЗ.
  {
    id: 'rp-server',
    name: 'Сервер VPS',
    amount: 9.99,
    currency: 'USD',
    anchorOffset: -1,
    assignedTo: ['user-2'],
  },
  // Подписка в евро — тестовые данные из ТЗ.
  {
    id: 'rp-subscription',
    name: 'Подписка Figma',
    amount: 15,
    currency: 'EUR',
    anchorOffset: 3,
    assignedTo: ['user-2', 'user-3'],
  },
  // Домен: первый период оплачен — история оплат сохраняется.
  {
    id: 'rp-domain',
    name: 'Домен и SSL',
    amount: 950,
    currency: 'RUB',
    anchorOffset: -35,
    assignedTo: ['user-3'],
    paidPeriods: { 'user-3': [0] },
  },
  // Связь: anna оплатила первый период, ivan — второй. У каждого своя история.
  {
    id: 'rp-internet',
    name: 'Интернет и связь',
    amount: 1200,
    currency: 'RUB',
    anchorOffset: -40,
    assignedTo: ['user-2', 'user-3'],
    paidPeriods: { 'user-2': [1], 'user-3': [0] },
  },
];

const recurringPayments: MockRecurringPayment[] = [];

/** All periods of all payments, each belonging to exactly one user's ledger. */
let periods: MockPeriod[] = [];

function createPeriod(
  recurringPaymentId: string,
  userId: string,
  dueDate: string,
): MockPeriod {
  return {
    id: nextId('po'),
    recurring_payment_id: recurringPaymentId,
    user_id: userId,
    due_date: dueDate,
    is_paid: false,
    actual_payment_date: null,
  };
}

/**
 * Generates the periods of a recurring payment for every assigned user on a
 * monthly calendar: every period from the anchor up to the current one, plus
 * the next one.
 *
 * Existing periods are matched by (user, due date), so their status and
 * actual payment date survive regeneration.
 */
function materializePeriods(recurringPayment: MockRecurringPayment): void {
  if (!recurringPayment.active) return;

  const today = startOfToday();
  const dueDates: string[] = [];

  for (let index = 0; ; index += 1) {
    const dueDate = addMonthsClamped(recurringPayment.anchor_date, index);
    dueDates.push(dueDate);
    if (!isBefore(dueDate, today)) break;
  }
  // One more period, so the "next period" is always visible.
  dueDates.push(addMonthsClamped(recurringPayment.anchor_date, dueDates.length));

  const existingByKey = new Map(
    periods
      .filter((period) => period.recurring_payment_id === recurringPayment.id)
      .map((period) => [`${period.user_id}|${period.due_date}`, period]),
  );

  const regenerated: MockPeriod[] = [];
  for (const userId of recurringPayment.assigned_user_ids) {
    for (const dueDate of dueDates) {
      const key = `${userId}|${dueDate}`;
      regenerated.push(
        existingByKey.get(key) ??
          createPeriod(recurringPayment.id, userId, dueDate),
      );
    }
  }

  // Keep the periods of users that are no longer assigned, so their payment
  // history is preserved.
  for (const period of periods) {
    if (period.recurring_payment_id !== recurringPayment.id) continue;
    if (recurringPayment.assigned_user_ids.includes(period.user_id)) continue;
    regenerated.push(period);
  }

  periods = [
    ...periods.filter((p) => p.recurring_payment_id !== recurringPayment.id),
    ...regenerated,
  ];

  const nextUnpaid = regenerated.find(
    (period) =>
      recurringPayment.assigned_user_ids.includes(period.user_id) && !period.is_paid,
  );
  const lastPeriod = regenerated[regenerated.length - 1];
  const reference = nextUnpaid ?? lastPeriod;
  if (reference) {
    recurringPayment.next_payment_date = reference.due_date;
  }
}

function seedPayments(): void {
  recurringPayments.splice(0, recurringPayments.length);
  periods = [];

  for (const seed of SEED_PAYMENTS) {
    const anchor = isoDate(seed.anchorOffset);
    const payment: MockRecurringPayment = {
      id: seed.id,
      name: seed.name,
      amount: seed.amount,
      currency: seed.currency,
      periodicity: 'monthly',
      anchor_date: anchor,
      next_payment_date: anchor,
      assigned_user_ids: [...seed.assignedTo],
      active: true,
    };
    materializePeriods(payment);
    recurringPayments.push(payment);
  }

  // Apply the seeded payments: each user has their own history.
  for (const seed of SEED_PAYMENTS) {
    const payment = recurringPayments.find((rp) => rp.id === seed.id);
    if (!payment) continue;
    for (const [userId, indexes] of Object.entries(seed.paidPeriods ?? {})) {
      for (const index of indexes) {
        const userPeriods = periods
          .filter((p) => p.recurring_payment_id === seed.id && p.user_id === userId)
          .sort((a, b) => a.due_date.localeCompare(b.due_date));
        const period = userPeriods[index];
        if (!period) continue;
        period.is_paid = true;
        period.actual_payment_date = period.due_date;
      }
    }
  }
}

seedPayments();

/* -------------------------------------------------------------------------- */
/* Helpers                                                                     */
/* -------------------------------------------------------------------------- */

function toPublicUser(user: MockUser): User {
  return { id: user.id, username: user.username, role: user.role };
}

/**
 * Derives the status of a period:
 * - `paid` — the payment is confirmed;
 * - `overdue` — the due date has passed (overdue starts the day *after* the
 *   due date, so paying on the due date is still on time);
 * - `unpaid` — otherwise.
 */
function deriveStatus(period: MockPeriod): PaymentStatus {
  if (period.is_paid) return 'paid';
  return isBefore(period.due_date, startOfToday()) ? 'overdue' : 'unpaid';
}

function toPublicOccurrence(
  recurringPayment: MockRecurringPayment,
  period: MockPeriod,
): PaymentOccurrence {
  return {
    id: period.id,
    recurring_payment_id: recurringPayment.id,
    name: recurringPayment.name,
    amount: recurringPayment.amount,
    currency: recurringPayment.currency,
    due_date: period.due_date,
    status: deriveStatus(period),
    actual_payment_date: period.actual_payment_date,
    user_id: period.user_id,
  };
}

function toPublicRecurringPayment(payment: MockRecurringPayment): RecurringPayment {
  return {
    id: payment.id,
    name: payment.name,
    amount: payment.amount,
    currency: payment.currency,
    next_payment_date: payment.next_payment_date,
    periodicity: payment.periodicity,
    assigned_user_ids: [...payment.assigned_user_ids],
    category: payment.category,
    notes: payment.notes,
    receipt_url: payment.receipt_url,
  };
}

/**
 * Resolves the acting user from an auth token issued by the mock.
 * The token format is `mock-token:<user id>|<random>`.
 */
export function mockResolveUser(token: string | null): MockUser | null {
  if (!token || !token.startsWith(MOCK_TOKEN_PREFIX)) return null;
  const userId = token.slice(MOCK_TOKEN_PREFIX.length).split('|')[0];
  return users.find((user) => user.id === userId) ?? null;
}

export function mockLogout(): void {
  mockSessionUserId = null;
}

/** Ids of all non-admin users — the default set of assignees. */
function defaultAssigneeIds(): string[] {
  return users.filter((user) => user.role === 'user').map((user) => user.id);
}

/* -------------------------------------------------------------------------- */
/* Auth                                                                        */
/* -------------------------------------------------------------------------- */

export function mockLogin(
  username: string,
  password: string,
): { user: User } {
  const user = users.find((u) => u.username === username && u.password === password);
  if (!user) {
    throw new Error('Неверный логин или пароль');
  }
  mockSessionUserId = user.id;
  return { user: toPublicUser(user) };
}

/* -------------------------------------------------------------------------- */
/* Users                                                                       */
/* -------------------------------------------------------------------------- */

export function mockGetUsers(): User[] {
  return users.map(toPublicUser);
}

export function mockCreateUser(payload: CreateUserPayload): User {
  if (users.some((u) => u.username === payload.username)) {
    throw new Error('Пользователь с таким логином уже существует');
  }
  const user: MockUser = {
    id: nextId('user'),
    username: payload.username,
    password: payload.password,
    role: payload.role ?? 'user',
  };
  users.push(user);
  return toPublicUser(user);
}

export function mockUpdateUser(id: string, payload: UpdateUserPayload): User {
  const user = users.find((u) => u.id === id);
  if (!user) throw new Error('Пользователь не найден');
  if (payload.username !== undefined) user.username = payload.username;
  if (payload.password !== undefined && payload.password !== '') user.password = payload.password;
  if (payload.role !== undefined) user.role = payload.role;
  return toPublicUser(user);
}

export function mockDeleteUser(id: string): void {
  const index = users.findIndex((u) => u.id === id);
  if (index === -1) throw new Error('Пользователь не найден');
  users.splice(index, 1);
}

/* -------------------------------------------------------------------------- */
/* Recurring payments                                                          */
/* -------------------------------------------------------------------------- */

/**
 * Returns the recurring payments visible to the acting user.
 * The administrator sees every payment, a user sees only the payments
 * assigned to them.
 */
export function mockGetRecurringPayments(actor: MockUser | null): RecurringPayment[] {
  return recurringPayments
    .filter((payment) => payment.active)
    .filter((payment) => {
      if (!actor) return false;
      if (actor.role === 'admin') return true;
      return payment.assigned_user_ids.includes(actor.id);
    })
    .map(toPublicRecurringPayment);
}

export function mockCreateRecurringPayment(
  payload: CreateRecurringPaymentPayload,
): RecurringPayment {
  const assignedUserIds =
    payload.assigned_user_ids && payload.assigned_user_ids.length > 0
      ? payload.assigned_user_ids.filter((id) => users.some((u) => u.id === id))
      : defaultAssigneeIds();

  const payment: MockRecurringPayment = {
    id: nextId('rp'),
    name: payload.name,
    amount: payload.amount,
    currency: payload.currency,
    periodicity: payload.periodicity,
    category: payload.category ?? 'other',
    notes: payload.notes,
    receipt_url: payload.receipt_url ?? null,
    anchor_date: payload.next_payment_date,
    next_payment_date: payload.next_payment_date,
    assigned_user_ids: assignedUserIds,
    active: true,
  };
  materializePeriods(payment);
  recurringPayments.push(payment);
  return toPublicRecurringPayment(payment);
}

/**
 * Stops a recurring payment. Its periods are kept, so the payment history
 * stays visible — only new periods are no longer generated.
 */
/**
 * Creates every row as a recurring payment. The whole batch is validated
 * first, so either all rows are created or none (transaction semantics).
 */
export function mockImportRecurringPayments(rows: CsvImportRow[]): RecurringPayment[] {
  const resolved = rows.map((row, index) => {
    const ids = (row.assigned_usernames ?? []).map((username) => {
      const user = users.find((u) => u.username.toLowerCase() === username.toLowerCase());
      if (!user) throw new Error(`Строка ${index + 1}: неизвестный пользователь «${username}»`);
      return user.id;
    });
    return { row, ids };
  });

  return resolved.map(({ row, ids }) =>
    mockCreateRecurringPayment({
      name: row.name,
      amount: row.amount,
      currency: row.currency,
      next_payment_date: row.next_payment_date,
      periodicity: row.periodicity,
      category: row.category,
      notes: row.notes,
      assigned_user_ids: ids,
    }),
  );
}

export function mockDeleteRecurringPayment(id: string): void {
  const payment = recurringPayments.find((rp) => rp.id === id);
  if (!payment) throw new Error('Платёж не найден');
  payment.active = false;
}

/* -------------------------------------------------------------------------- */
/* Payment occurrences                                                         */
/* -------------------------------------------------------------------------- */

/**
 * Returns the periods of the acting user's own ledger. A user never sees or
 * edits the periods of another user.
 */
export function mockGetOccurrences(actor: MockUser | null): PaymentOccurrence[] {
  if (!actor) return [];

  const result: PaymentOccurrence[] = [];
  for (const payment of recurringPayments) {
    materializePeriods(payment);
    for (const period of periods) {
      // Only the periods of the current recurring payment.
      if (period.recurring_payment_id !== payment.id) continue;
      const visibleToActor =
        actor.role === 'admin'
          ? true
          : period.user_id === actor.id && payment.assigned_user_ids.includes(actor.id);
      if (!visibleToActor) continue;
      result.push(toPublicOccurrence(payment, period));
    }
  }
  return result.sort((a, b) => a.due_date.localeCompare(b.due_date));
}

export function mockUpdateOccurrence(
  id: string,
  payload: UpdateOccurrencePayload,
  actor: MockUser | null = null,
): PaymentOccurrence {
  if (!actor) throw new Error('Платеж не найден');

  const period = periods.find((po) => po.id === id);
  // A regular user can edit only their own ledger. The administrator can
  // perform the same operation for any assigned user's period.
  if (!period || (actor.role !== 'admin' && period.user_id !== actor.id)) {
    throw new Error('Платеж не найден');
  }

  const payment = recurringPayments.find((rp) => rp.id === period.recurring_payment_id);
  if (!payment) throw new Error('Платеж не найден');

  if (payload.actual_payment_date !== undefined) {
    period.actual_payment_date = payload.actual_payment_date;
    if (payload.actual_payment_date !== null) {
      // A confirmed payment date means the period is paid. Correcting the
      // date recalculates the status, the due date stays untouched.
      period.is_paid = true;
    }
  }

  if (payload.status === 'paid') {
    period.is_paid = true;
    period.actual_payment_date =
      period.actual_payment_date ?? startOfToday().toISOString();
  }

  if (payload.status === 'unpaid') {
    // Cancelling the mark clears the actual payment date.
    period.is_paid = false;
    period.actual_payment_date = null;
  }

  materializePeriods(payment);
  return toPublicOccurrence(payment, period);
}

/* -------------------------------------------------------------------------- */
/* Summary                                                                     */
/* -------------------------------------------------------------------------- */

export function mockGetSummary(actor: MockUser | null): Summary {
  const summary: Record<Currency, number> = { RUB: 0, USD: 0, EUR: 0 };
  for (const occurrence of mockGetOccurrences(actor)) {
    if (occurrence.status === 'paid') {
      summary[occurrence.currency] += occurrence.amount;
    }
  }
  return summary;
}

/** Resets the mock state to its initial seed data. */
export function resetMockData(): void {
  idCounter = 1000;
  users.splice(
    1,
    users.length,
    { id: 'user-2', username: 'ivan', password: 'ivan', role: 'user' },
    { id: 'user-3', username: 'anna', password: 'anna', role: 'user' },
  );
  seedPayments();
}
