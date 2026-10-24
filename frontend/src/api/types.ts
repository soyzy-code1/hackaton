export type Currency = 'RUB' | 'USD' | 'EUR';
export type Role = 'admin' | 'user';

export interface User {
  id: string;
  username: string;
  role: Role;
}

export type Periodicity = 'monthly';

export interface RecurringPayment {
  id: string;
  name: string;
  amount: number;
  currency: Currency;
  /** ISO string */
  next_payment_date: string;
  periodicity: Periodicity;
  category?: PaymentCategory;
  notes?: string;
  receipt_url?: string | null;
  tags?: string[];
  archived?: boolean;
  /**
   * Ids of the users this payment is assigned to. Every user has a personal
   * ledger: the periods and their marks are tracked per user, so one user's
   * actions never affect another user's payment state.
   */
  assigned_user_ids: string[];
}

export type PaymentStatus = 'paid' | 'unpaid' | 'overdue';
export type PaymentCategory = 'housing' | 'subscriptions' | 'services' | 'infrastructure' | 'transport' | 'other';

export const PAYMENT_CATEGORY_LABELS: Record<PaymentCategory, string> = {
  housing: 'Жильё',
  subscriptions: 'Подписки',
  services: 'Сервисы',
  infrastructure: 'Инфраструктура',
  transport: 'Транспорт',
  other: 'Другое',
};

export interface PaymentOccurrence {
  id: string;
  recurring_payment_id: string;
  name: string;
  amount: number;
  currency: Currency;
  /** ISO string */
  due_date: string;
  status: PaymentStatus;
  actual_payment_date: string | null;
  /**
   * Id of the owner of this period. Occurrences are per user: a user only
   * ever sees and edits the periods of their own ledger.
   */
  user_id: string;
}

export type NotificationType = 'reminder' | 'overdue' | 'system';
export type NotificationChannel = 'in_app' | 'telegram' | 'email';

export interface Notification {
  id: string;
  type: NotificationType;
  title: string;
  message: string;
  occurrence_id: string | null;
  is_read: boolean;
  created_at: string;
  channels: NotificationChannel[];
}

export interface PaginatedResponse<T> {
  items: T[];
  total: number;
  page: number;
  page_size: number;
}

export interface Summary {
  RUB: number;
  USD: number;
  EUR: number;
}

export interface SummaryQuery {
  as_of_date?: string;
  from_date?: string;
  to_date?: string;
}

export interface BudgetPlan {
  currency: Currency;
  monthly_limit: number;
  yearly_limit: number;
  spent_month: number;
  spent_year: number;
  remaining_month: number;
  remaining_year: number;
}

export interface CsvImportRow {
  name: string;
  amount: number;
  currency: Currency;
  next_payment_date: string;
  periodicity: Periodicity;
  assigned_usernames?: string[];
  category?: PaymentCategory;
  notes?: string;
}

export interface CsvImportError {
  row: number;
  message: string;
}

export interface CsvImportPreview {
  valid_rows: CsvImportRow[];
  errors: CsvImportError[];
}

export interface LoginResponse {
  token: string;
  user: User;
}

export interface LoginPayload {
  username: string;
  password: string;
}

export interface CreateRecurringPaymentPayload {
  name: string;
  amount: number;
  currency: Currency;
  next_payment_date: string;
  periodicity: Periodicity;
  /** Users the payment is assigned to. Defaults to every non-admin user. */
  assigned_user_ids?: string[];
  category?: PaymentCategory;
  notes?: string;
  receipt_url?: string | null;
  tags?: string[];
}

export interface UpdateRecurringPaymentPayload {
  name?: string;
  amount?: number;
  currency?: Currency;
  next_payment_date?: string;
  periodicity?: Periodicity;
  category?: PaymentCategory;
  notes?: string;
  receipt_url?: string | null;
  assigned_user_ids?: string[];
  tags?: string[];
  archived?: boolean;
}

export interface NotificationQuery {
  page?: number;
  page_size?: number;
  unread_only?: boolean;
}

export interface BulkOccurrenceUpdatePayload {
  occurrence_ids: string[];
  status: Extract<PaymentStatus, 'paid' | 'unpaid'>;
  actual_payment_date?: string | null;
}

export interface UpdateOccurrencePayload {
  status?: Extract<PaymentStatus, 'paid' | 'unpaid'>;
  actual_payment_date?: string | null;
}

export interface CreateUserPayload {
  username: string;
  password: string;
  role?: Role;
}

export interface UpdateUserPayload {
  username?: string;
  password?: string;
  role?: Role;
}

export const CURRENCIES: readonly Currency[] = ['RUB', 'USD', 'EUR'] as const;

export const CURRENCY_SYMBOLS: Record<Currency, string> = {
  RUB: '₽',
  USD: '$',
  EUR: '€',
};

export const PERIODICITIES: readonly Periodicity[] = ['monthly'] as const;

export const PERIODICITY_LABELS: Record<Periodicity, string> = {
  monthly: 'Ежемесячно',
};

export const STATUS_LABELS: Record<PaymentStatus, string> = {
  paid: 'Оплачено',
  unpaid: 'Не оплачено',
  overdue: 'Просрочка',
};

export const ROLE_LABELS: Record<Role, string> = {
  admin: 'Администратор',
  user: 'Пользователь',
};
