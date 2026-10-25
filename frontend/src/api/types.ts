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
  next_payment_date: string;
  periodicity: Periodicity;
  category?: PaymentCategory;
  notes?: string;
  receipt_url?: string | null;
  tags?: string[];
  archived?: boolean;
  assigned_user_ids: string[];
}

export type PaymentStatus = 'paid' | 'unpaid' | 'overdue';
export type PaymentCategory = 'housing' | 'subscriptions' | 'services' | 'infrastructure' | 'transport' | 'other';

export const PAYMENT_CATEGORY_LABELS: Record<PaymentCategory, string> = {
  housing: 'Жильё', subscriptions: 'Подписки', services: 'Сервисы',
  infrastructure: 'Инфраструктура', transport: 'Транспорт', other: 'Другое',
};

export interface PaymentOccurrence {
  id: string;
  recurring_payment_id: string;
  name: string;
  amount: number;
  currency: Currency;
  due_date: string;
  status: PaymentStatus;
  actual_payment_date: string | null;
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

export interface Summary { RUB: number; USD: number; EUR: number; }
export interface SummaryQuery { as_of_date?: string; from_date?: string; to_date?: string; }

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
  name: string; amount: number; currency: Currency; next_payment_date: string;
  periodicity: Periodicity; assigned_usernames?: string[]; category?: PaymentCategory; notes?: string;
}
export interface CsvImportError { row: number; message: string; }
export interface CsvImportPreview { valid_rows: CsvImportRow[]; errors: CsvImportError[]; }

export interface LoginResponse { user: User; }
export interface LoginPayload { username: string; password: string; }

export interface CreateRecurringPaymentPayload {
  name: string; amount: number; currency: Currency; next_payment_date: string; periodicity: Periodicity;
  assigned_user_ids?: string[]; category?: PaymentCategory; notes?: string; receipt_url?: string | null; tags?: string[];
}
export interface UpdateRecurringPaymentPayload {
  name?: string; amount?: number; currency?: Currency; next_payment_date?: string; periodicity?: Periodicity;
  category?: PaymentCategory; notes?: string; receipt_url?: string | null; assigned_user_ids?: string[]; tags?: string[]; archived?: boolean;
}
export interface NotificationQuery { page?: number; page_size?: number; unread_only?: boolean; }
export interface BulkOccurrenceUpdatePayload { occurrence_ids: string[]; status: Extract<PaymentStatus, 'paid' | 'unpaid'>; actual_payment_date?: string | null; }
export interface UpdateOccurrencePayload { status?: Extract<PaymentStatus, 'paid' | 'unpaid'>; actual_payment_date?: string | null; }
export interface CreateUserPayload { username: string; password: string; role?: Role; }
export interface UpdateUserPayload { username?: string; password?: string; role?: Role; }

export type AlternativeSimilarity = 'high' | 'medium';

export interface SubscriptionAlternative {
  payment_id: number;
  payment_title: string;
  current_price: number;
  current_currency: string;
  service_name: string;
  alternative_price: number;
  alternative_currency: string;
  monthly_saving: number;
  annual_saving: number;
  saving_percent: number;
  similarity: AlternativeSimilarity;
  reason: string;
  source_url: string;
  evidence: string;
}

export interface AnalysedSubscription {
  id: number;
  title: string;
  amount: number;
  currency: string;
  periodicity: string;
  searched: boolean;
  query_count: number;
  price_hits: number;
  best_saving: number | null;
}

export interface SubscriptionAnalysis {
  analyzed_at: string;
  summary: string;
  best_alternative: SubscriptionAlternative | null;
  alternatives: SubscriptionAlternative[];
  subscriptions: AnalysedSubscription[];
  search_queries: number;
  warnings: string[];
}

export interface CategoryAnalyticsItem { category: string; amount: number; count: number; }
export interface CategoryAnalytics { currency: Currency; items: CategoryAnalyticsItem[]; }

export interface AuditLogEntry {
  id: number; user_id: number | null; action: string; entity: string; entity_id: number | null; details: string; created_at: string;
}
export interface AuditLogResponse { items: AuditLogEntry[]; total: number; page: number; page_size: number; }

export interface CategoryAnalyticsItem { category: string; amount: number; count: number; }
export interface CategoryAnalytics { currency: Currency; items: CategoryAnalyticsItem[]; }

export const CURRENCIES: readonly Currency[] = ['RUB', 'USD', 'EUR'] as const;
export const CURRENCY_SYMBOLS: Record<Currency, string> = { RUB: '₽', USD: '$', EUR: '€' };
export const PERIODICITIES: readonly Periodicity[] = ['monthly'] as const;
export const PERIODICITY_LABELS: Record<Periodicity, string> = { monthly: 'Ежемесячно' };
export const STATUS_LABELS: Record<PaymentStatus, string> = { paid: 'Оплачено', unpaid: 'Не оплачено', overdue: 'Просрочка' };
export const ROLE_LABELS: Record<Role, string> = { admin: 'Администратор', user: 'Пользователь' };
