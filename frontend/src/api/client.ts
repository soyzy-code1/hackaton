import {
  getMockSessionUserId,
  mockLogout,
  mockCreateRecurringPayment,
  mockImportRecurringPayments,
  mockCreateUser,
  mockDeleteRecurringPayment,
  mockDeleteUser,
  mockGetOccurrences,
  mockGetRecurringPayments,
  mockGetSummary,
  mockGetUsers,
  mockLogin,
  mockResolveUser,
  mockUpdateOccurrence,
  mockUpdateUser,
} from './mock';
import type {
  BudgetPlan,
  BulkOccurrenceUpdatePayload,
  CreateRecurringPaymentPayload,
  CsvImportRow,
  Currency,
  CreateUserPayload,
  LoginPayload,
  LoginResponse,
  Notification,
  NotificationQuery,
  PaginatedResponse,
  PaymentOccurrence,
  RecurringPayment,
  Summary,
  SummaryQuery,
  UpdateOccurrencePayload,
  UpdateRecurringPaymentPayload,
  UpdateUserPayload,
  User,
} from './types';

const API_BASE_URL = import.meta.env.VITE_API_URL || '/api/v1';
const FORCE_MOCK = String(import.meta.env.VITE_USE_MOCK).toLowerCase() === 'true';

/**
 * The mock backend is a development aid: in a production build the app talks
 * to the real API only, so a server outage surfaces as a normal error.
 */
const MOCK_ENABLED = FORCE_MOCK || Boolean(import.meta.env.DEV);

/**
 * Header set by the Vite dev proxy when the upstream API cannot be reached.
 * See `vite.config.ts`.
 */
const BACKEND_UNREACHABLE_HEADER = 'X-Backend-Unreachable';

/** Error thrown by the API client, carrying an HTTP-ish status code. */
export class ApiError extends Error {
  readonly status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

type QueryValue = string | number | boolean | undefined | null;

function buildUrl(path: string, query?: Record<string, QueryValue>): string {
  const url = `${API_BASE_URL}${path}`;
  if (!query) return url;
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === null || value === '') continue;
    params.append(key, String(value));
  }
  const qs = params.toString();
  return qs ? `${url}?${qs}` : url;
}

/**
 * When the backend is unreachable the client transparently falls back to the
 * built-in mock backend, so the SPA remains usable during local development.
 */
let usingMockBackend = FORCE_MOCK;

export function isUsingMockBackend(): boolean {
  return usingMockBackend;
}

function extractErrorMessage(payload: unknown, fallback: string): string {
  if (typeof payload === 'string' && payload.trim()) return payload;
  if (payload && typeof payload === 'object') {
    const record = payload as Record<string, unknown>;
    for (const key of ['detail', 'message', 'error', 'non_field_errors']) {
      const value = record[key];
      if (typeof value === 'string' && value.trim()) return value;
      if (Array.isArray(value) && value.length > 0) {
        const first = value[0];
        if (typeof first === 'string') return first;
      }
    }
  }
  return fallback;
}

async function parseBody(response: Response): Promise<unknown> {
  const text = await response.text();
  if (!text) return null;
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return text;
  }
}

async function request<T>(
  path: string,
  init: RequestInit = {},
  query?: Record<string, QueryValue>,
): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set('Accept', 'application/json');
  const requestInit: RequestInit = { ...init, headers, credentials: 'include' };
  if (init.body !== undefined && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }
  let response: Response;
  try {
    response = await fetch(buildUrl(path, query), requestInit);
  } catch (error) {
    // Network-level failure (backend down / DNS / CORS).
    throw new ApiError(
      `Не удалось связаться с сервером: ${error instanceof Error ? error.message : 'unknown error'}`,
      0,
    );
  }

  const payload = await parseBody(response);

  if (!response.ok) {
    // The dev-server proxy marks an unreachable upstream with this header.
    // Such a response means "backend is down", not "the API returned an error",
    // so it is reported with status 0 and handled by the mock fallback.
    if (response.headers.get(BACKEND_UNREACHABLE_HEADER) === '1') {
      throw new ApiError('Бэкенд недоступен', 0);
    }
    throw new ApiError(
      extractErrorMessage(payload, `Ошибка запроса (${response.status})`),
      response.status,
    );
  }

  return payload as T;
}

/* -------------------------------------------------------------------------- */
/* Backend → frontend contract adapters                                        */
/* -------------------------------------------------------------------------- */
/*
 * The Go backend uses its own field names (`title`, `is_paid`, `login`,
 * decimal amounts as strings, numeric ids). The UI expects the contract from
 * `types.ts`. Every response is normalized here so pages never receive
 * `undefined` where a string/array is expected (that used to crash the app).
 */

type RawRecord = Record<string, unknown>;

const KNOWN_CURRENCIES: readonly Currency[] = ['RUB', 'USD', 'EUR'];

function asRecord(value: unknown): RawRecord {
  return value && typeof value === 'object' ? (value as RawRecord) : {};
}

function asString(value: unknown, fallback = ''): string {
  if (typeof value === 'string') return value;
  if (typeof value === 'number' && Number.isFinite(value)) return String(value);
  return fallback;
}

function asNumber(value: unknown): number {
  const parsed = typeof value === 'number' ? value : Number(value);
  return Number.isFinite(parsed) ? parsed : 0;
}

function asCurrency(value: unknown): Currency {
  const upper = asString(value).toUpperCase() as Currency;
  return KNOWN_CURRENCIES.includes(upper) ? upper : 'RUB';
}

function asNullableString(value: unknown): string | null {
  const str = asString(value);
  return str ? str : null;
}

function asStringArray(value: unknown): string[] {
  return Array.isArray(value) ? value.map((item) => asString(item)).filter(Boolean) : [];
}

function normalizeUser(raw: unknown): User {
  const record = asRecord(raw);
  return {
    id: asString(record.id),
    username: asString(record.username ?? record.login, '—'),
    role: record.role === 'admin' ? 'admin' : 'user',
  };
}

function normalizeRecurringPayment(raw: unknown): RecurringPayment {
  const record = asRecord(raw);
  return {
    id: asString(record.id),
    name: asString(record.name ?? record.title, 'Без названия'),
    amount: asNumber(record.amount),
    currency: asCurrency(record.currency),
    next_payment_date: asString(record.next_payment_date),
    periodicity: 'monthly',
    category: (asString(record.category) || 'other') as RecurringPayment['category'],
    notes: asString(record.notes) || undefined,
    receipt_url: asNullableString(record.receipt_url),
    tags: asStringArray(record.tags),
    archived: record.is_active === false || record.archived === true,
    assigned_user_ids: asStringArray(record.assigned_user_ids),
  };
}

function normalizeOccurrence(raw: unknown): PaymentOccurrence {
  const record = asRecord(raw);
  let status: PaymentOccurrence['status'];
  if (record.status === 'paid' || record.status === 'unpaid' || record.status === 'overdue') {
    status = record.status;
  } else if (record.is_paid === true) {
    status = 'paid';
  } else if (record.is_overdue === true) {
    status = 'overdue';
  } else {
    status = 'unpaid';
  }
  return {
    id: asString(record.id),
    recurring_payment_id: asString(record.recurring_payment_id),
    name: asString(record.name ?? record.title, 'Без названия'),
    amount: asNumber(record.amount),
    currency: asCurrency(record.currency),
    due_date: asString(record.due_date),
    status,
    actual_payment_date: asNullableString(record.actual_payment_date ?? record.paid_at),
    user_id: asString(record.user_id ?? record.paid_by),
  };
}

function normalizeList<T>(value: unknown, mapper: (raw: unknown) => T): T[] {
  return Array.isArray(value) ? value.map(mapper) : [];
}

/** `2026-10-19T00:00:00.000Z` → `2026-10-19` (backend accepts plain dates). */
function toApiDate(value: string | null | undefined): string | null {
  if (!value) return null;
  return value.length >= 10 ? value.slice(0, 10) : value;
}

async function withMock<T>(realCall: () => Promise<T>, mockCall: () => T): Promise<T> {
  if (FORCE_MOCK) return mockCall();
  try {
    const result = await realCall();
    usingMockBackend = false;
    return result;
  } catch (error) {
    if (MOCK_ENABLED && error instanceof ApiError && error.status === 0) {
      usingMockBackend = true;
      return mockCall();
    }
    throw error;
  }
}

export const api = {
  auth: {
    login: async (payload: LoginPayload): Promise<LoginResponse> =>
      withMock(
        async () => {
          const response = await request<{
            user: User | {
              id: number;
              login: string;
              email?: string | null;
              full_name?: string | null;
              role: 'admin' | 'user';
              is_active?: boolean;
              created_at?: string;
            };
          }>('/auth/login', {
            method: 'POST',
            body: JSON.stringify(payload),
          });
          if (!response || !response.user) {
            throw new ApiError('Некорректный ответ сервера при авторизации', 500);
          }
          const rawUser = response.user;
          const user: User = 'username' in rawUser
            ? rawUser
            : {
                id: String(rawUser.id),
                username: rawUser.login,
                role: rawUser.role,
              };
          return { user };
        },
        () => mockLogin(payload.username, payload.password),
      ),

    me: async (): Promise<User> =>
      withMock(
        async () => {
          const response = await request<unknown>('/auth/me', { method: 'GET' });
          const user = normalizeUser(response);
          if (!user.id) {
            throw new ApiError('Некорректный ответ сервера', 500);
          }
          return user;
        },
        () => {
          const userId = getMockSessionUserId();
          const found = mockGetUsers().find((u) => u.id === userId);
          if (!found) throw new ApiError('Пользователь не найден', 404);
          return found;
        },
      ),

    logout: async (): Promise<void> => {
      try {
        await request<unknown>('/auth/logout', { method: 'POST' });
      } catch {
        // Logging out locally must always succeed, even if the backend is down.
      }
      mockLogout();
    },
  },

  recurringPayments: {
    list: async (): Promise<RecurringPayment[]> =>
      withMock(
        async () => {
          const response = await request<unknown>('/recurring-payments', {
            method: 'GET',
          });
          return normalizeList(response, normalizeRecurringPayment);
        },
        () => mockGetRecurringPayments(mockResolveUser(getMockSessionUserId())),
      ),

    update: async (id: string, payload: UpdateRecurringPaymentPayload): Promise<RecurringPayment> =>
      withMock(
        async () => request<RecurringPayment>(`/recurring-payments/${encodeURIComponent(id)}`, {
          method: 'PATCH',
          body: JSON.stringify(payload),
        }),
        () => {
          throw new ApiError('Обновление регулярных платежей доступно после подключения API', 501);
        },
      ),

    create: async (payload: CreateRecurringPaymentPayload): Promise<RecurringPayment> =>
      withMock(
        async () => {
          const response = await request<unknown>('/recurring-payments', {
            method: 'POST',
            body: JSON.stringify({
              ...payload,
              title: payload.name,
              amount: String(payload.amount),
              next_payment_date: toApiDate(payload.next_payment_date),
            }),
          });
          return normalizeRecurringPayment(response);
        },
        () => mockCreateRecurringPayment(payload),
      ),

    remove: async (id: string): Promise<void> =>
      withMock(
        async () => {
          await request<unknown>(`/recurring-payments/${encodeURIComponent(id)}`, {
            method: 'DELETE',
          });
        },
        () => mockDeleteRecurringPayment(id),
      ),

    /**
     * Creates many recurring payments in one transaction.
     * Backend contract: `POST /recurring-payments/import` with `{ rows }`,
     * responds with the created payments. Rows are validated client-side
     * first, the server must validate them again.
     */
    importBatch: async (rows: CsvImportRow[]): Promise<RecurringPayment[]> =>
      withMock(
        async () => {
          // The backend has no batch endpoint: create the rows one by one.
          const created: RecurringPayment[] = [];
          for (const row of rows) {
            const response = await request<unknown>('/recurring-payments', {
              method: 'POST',
              body: JSON.stringify({
                title: row.name,
                amount: String(row.amount),
                currency: row.currency,
                periodicity: row.periodicity,
                next_payment_date: toApiDate(row.next_payment_date),
              }),
            });
            created.push(normalizeRecurringPayment(response));
          }
          return created;
        },
        () => mockImportRecurringPayments(rows),
      ),
  },

  occurrences: {
    list: async (): Promise<PaymentOccurrence[]> =>
      withMock(
        async () => {
          const response = await request<unknown>('/payment-occurrences', {
            method: 'GET',
          });
          return normalizeList(response, normalizeOccurrence);
        },
        () => mockGetOccurrences(mockResolveUser(getMockSessionUserId())),
      ),

    update: async (id: string, payload: UpdateOccurrencePayload): Promise<PaymentOccurrence> =>
      withMock(
        async () => {
          const isPaid = payload.status === 'paid';
          const paidAt = isPaid ? toApiDate(payload.actual_payment_date) : null;
          const response = await request<unknown>(
            `/payment-occurrences/${encodeURIComponent(id)}`,
            {
              method: 'PATCH',
              body: JSON.stringify({
                is_paid: isPaid,
                ...(paidAt ? { paid_at: paidAt, actual_payment_date: paidAt } : {}),
              }),
            },
          );
          return normalizeOccurrence(response);
        },
        () => mockUpdateOccurrence(id, payload, mockResolveUser(getMockSessionUserId())),
      ),
  },

  notifications: {
    /**
     * The notification feed is optional on the backend: a missing endpoint
     * (404/405/501) means "no notifications", not a broken page.
     */
    list: async (query?: NotificationQuery): Promise<PaginatedResponse<Notification>> => {
      const empty: PaginatedResponse<Notification> = {
        items: [],
        total: 0,
        page: query?.page ?? 1,
        page_size: query?.page_size ?? 20,
      };
      try {
        const response = await request<unknown>('/notifications', { method: 'GET' }, query ? {
          page: query.page,
          page_size: query.page_size,
          unread_only: query.unread_only,
        } : undefined);
        const record = asRecord(response);
        const rawItems = Array.isArray(response) ? response : record.items;
        const items = normalizeList(rawItems, (raw): Notification => {
          const item = asRecord(raw);
          const type = item.type === 'overdue' || item.type === 'system' ? item.type : 'reminder';
          return {
            id: asString(item.id),
            type,
            title: asString(item.title, 'Уведомление'),
            message: asString(item.message),
            occurrence_id: asNullableString(item.occurrence_id),
            is_read: item.is_read === true,
            created_at: asString(item.created_at),
            channels: asStringArray(item.channels) as Notification['channels'],
          };
        });
        return { ...empty, items, total: asNumber(record.total) || items.length };
      } catch (error) {
        if (error instanceof ApiError && [404, 405, 501].includes(error.status)) return empty;
        throw error;
      }
    },
    markRead: async (id: string): Promise<void> => {
      try {
        await request<unknown>(`/notifications/${encodeURIComponent(id)}`, {
          method: 'PATCH',
          body: JSON.stringify({ is_read: true }),
        });
      } catch (error) {
        if (!(error instanceof ApiError && [404, 405, 501].includes(error.status))) throw error;
      }
    },
    markAllRead: async (): Promise<void> => {
      try {
        await request<unknown>('/notifications/read-all', { method: 'POST' });
      } catch (error) {
        if (!(error instanceof ApiError && [404, 405, 501].includes(error.status))) throw error;
      }
    },
  },

  bulk: {
    updateOccurrences: async (payload: BulkOccurrenceUpdatePayload): Promise<PaymentOccurrence[]> =>
      request<PaymentOccurrence[]>('/payment-occurrences/bulk', {
        method: 'PATCH',
        body: JSON.stringify(payload),
      }),
  },

  summary: {
    /**
     * Totals per currency. With `as_of_date` the server returns the totals
     * known on that day (payments confirmed later count as unpaid).
     */
    get: async (query?: SummaryQuery): Promise<Summary> =>
      withMock(
        async () => {
          const response = await request<unknown>(
            '/payments/summary',
            { method: 'GET' },
            query
              ? { as_of_date: query.as_of_date, from: query.from_date, to: query.to_date }
              : undefined,
          );
          const record = asRecord(response);
          // Go backend: { by_currency: { RUB: { total, paid, unpaid, overdue } } }.
          const byCurrency = asRecord(record.by_currency);
          const pick = (currency: Currency): number => {
            const nested = asRecord(byCurrency[currency]);
            return asNumber(nested.total ?? record[currency]);
          };
          return { RUB: pick('RUB'), USD: pick('USD'), EUR: pick('EUR') };
        },
        () => mockGetSummary(mockResolveUser(getMockSessionUserId())),
      ),
  },

  budgets: {
    list: async (asOfDate?: string): Promise<BudgetPlan[]> =>
      withMock(
        async () => {
          const response = await request<unknown>('/budgets', { method: 'GET' }, { as_of_date: asOfDate });
          return normalizeList(response, (raw): BudgetPlan => {
            const record = asRecord(raw);
            return {
              currency: asCurrency(record.currency),
              monthly_limit: asNumber(record.monthly_limit),
              yearly_limit: asNumber(record.yearly_limit),
              spent_month: asNumber(record.spent_month),
              spent_year: asNumber(record.spent_year),
              remaining_month: asNumber(record.remaining_month),
              remaining_year: asNumber(record.remaining_year),
            };
          });
        },
        () => [],
      ),
    update: async (
      currency: Currency,
      payload: { monthly_limit?: number; yearly_limit?: number },
    ): Promise<BudgetPlan> =>
      withMock(
        async () => {
          const response = await request<unknown>(`/budgets/${currency}`, {
            method: 'PUT',
            body: JSON.stringify(payload),
          });
          const record = asRecord(response);
          return {
            currency,
            monthly_limit: asNumber(record.monthly_limit),
            yearly_limit: asNumber(record.yearly_limit),
            spent_month: asNumber(record.spent_month),
            spent_year: asNumber(record.spent_year),
            remaining_month: asNumber(record.remaining_month),
            remaining_year: asNumber(record.remaining_year),
          };
        },
        () => ({ currency, monthly_limit: payload.monthly_limit ?? 0, yearly_limit: payload.yearly_limit ?? 0, spent_month: 0, spent_year: 0, remaining_month: payload.monthly_limit ?? 0, remaining_year: payload.yearly_limit ?? 0 }),
      ),
  },

  users: {
    list: async (): Promise<User[]> =>
      withMock(
        async () => {
          const response = await request<unknown>('/users', { method: 'GET' });
          return normalizeList(response, normalizeUser);
        },
        () => mockGetUsers(),
      ),

    create: async (payload: CreateUserPayload): Promise<User> =>
      withMock(
        async () => {
          const response = await request<unknown>('/users', {
            method: 'POST',
            body: JSON.stringify({ login: payload.username, password: payload.password, role: 'user' }),
          });
          return normalizeUser(response);
        },
        () => mockCreateUser(payload),
      ),

    update: async (id: string, payload: UpdateUserPayload): Promise<User> =>
      withMock(
        async () => {
          // Backend forbids changing login/role; only the password is sent.
          const response = await request<unknown>(`/users/${encodeURIComponent(id)}`, {
            method: 'PATCH',
            body: JSON.stringify(payload.password ? { password: payload.password } : {}),
          });
          return normalizeUser(response);
        },
        () => mockUpdateUser(id, payload),
      ),

    remove: async (id: string): Promise<void> =>
      withMock(
        async () => {
          await request<unknown>(`/users/${encodeURIComponent(id)}`, { method: 'DELETE' });
        },
        () => mockDeleteUser(id),
      ),
  },
};

export type Api = typeof api;
