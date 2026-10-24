import { useEffect, useMemo, useState } from 'react';
import type { FormEvent } from 'react';
import { Download, Search, SlidersHorizontal } from 'lucide-react';
import { api } from '@/api/client';
import type { Currency, PaymentOccurrence, PaymentCategory, Summary } from '@/api/types';
import { CURRENCIES, STATUS_LABELS } from '@/api/types';
import { useAsyncResource } from '@/hooks/useAsyncResource';
import { attentionCount, downloadCsv } from '@/lib/finance';
import { formatAmount, formatDate, fromDateInputValue, todayDateInputValue } from '@/lib/date';
import { useSettingsStore } from '@/store/settings';
import type { BudgetPeriod } from '@/store/settings';
import { CashFlowWidget } from '@/components/CashFlowWidget';
import { PlanningOverview } from '@/components/PlanningOverview';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Input } from '@/components/ui/Input';
import { Modal } from '@/components/ui/Modal';
import { Table } from '@/components/ui/Table';
import type { Column } from '@/components/ui/Table';

type FilterKey = 'all' | 'unpaid' | 'overdue' | 'paid';

const FILTERS: { key: FilterKey; label: string }[] = [
  { key: 'all', label: 'Все' },
  { key: 'unpaid', label: 'Неоплаченные' },
  { key: 'overdue', label: 'Просроченные' },
  { key: 'paid', label: 'Оплаченные' },
];

const CURRENCY_LABELS: Record<Currency, string> = {
  RUB: '₽ RUB',
  USD: '$ USD',
  EUR: '€ EUR',
};

function SummaryCard({
  currency,
  value,
  isLoading,
}: {
  currency: Currency;
  value: number;
  isLoading: boolean;
}) {
  return (
    <div className="card min-w-0 p-4 sm:p-5">
      <p className="text-xs font-medium uppercase tracking-wide text-gray-500">
        {CURRENCY_LABELS[currency]}
      </p>
      {isLoading ? (
        <div className="mt-3 h-7 w-28 animate-pulse rounded bg-gray-100" />
      ) : (
        <p className="mt-2 break-all text-xl font-semibold tabular-nums tracking-tight text-gray-900 sm:text-2xl">
          {new Intl.NumberFormat('ru-RU', {
            minimumFractionDigits: 0,
            maximumFractionDigits: 2,
          }).format(value)}
        </p>
      )}
    </div>
  );
}

export function DashboardPage() {
  const [filter, setFilter] = useState<FilterKey>('all');
  const [payingOccurrence, setPayingOccurrence] = useState<PaymentOccurrence | null>(null);
  const [paymentDate, setPaymentDate] = useState<string>(todayDateInputValue());
  const [actionError, setActionError] = useState<string | null>(null);
  const [pendingId, setPendingId] = useState<string | null>(null);
  const [query, setQuery] = useState('');
  const [category] = useState<PaymentCategory | 'all'>('all');
  const [attentionOnly, setAttentionOnly] = useState(false);
  const timeZone = useSettingsStore((state) => state.timeZone);
  const [asOf, setAsOf] = useState<string>(() => todayDateInputValue(timeZone));
  const [budgetPeriod, setBudgetPeriod] = useState<BudgetPeriod>('month');

  const summary = useAsyncResource<Summary>(() => api.summary.get(), []);
  const occurrences = useAsyncResource<PaymentOccurrence[]>(() => api.occurrences.list(), []);
  const [monthlyBudget, setMonthlyBudget] = useState<Record<Currency, number>>({ RUB: 0, USD: 0, EUR: 0 });
  const [yearlyBudget, setYearlyBudget] = useState<Record<Currency, number>>({ RUB: 0, USD: 0, EUR: 0 });

  useEffect(() => {
    void api.budgets.list().then((plans) => {
      const monthly = { RUB: 0, USD: 0, EUR: 0 };
      const yearly = { RUB: 0, USD: 0, EUR: 0 };
      for (const plan of plans) {
        monthly[plan.currency] = plan.monthly_limit;
        yearly[plan.currency] = plan.yearly_limit;
      }
      setMonthlyBudget(monthly);
      setYearlyBudget(yearly);
    });
  }, []);

  const rows = useMemo(() => occurrences.data ?? [], [occurrences.data]);

  const filteredRows = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase('ru');
    return rows.filter((row) => {
      const statusMatches = filter === 'all' || row.status === filter;
      const textMatches =
        !normalizedQuery || String(row.name ?? '').toLocaleLowerCase('ru').includes(normalizedQuery);
      const categoryMatches = category === 'all' || category === 'other';
      const attentionMatches = !attentionOnly || row.status !== 'paid';
      return statusMatches && textMatches && categoryMatches && attentionMatches;
    });
  }, [rows, filter, query, category, attentionOnly]);

  const openPayDialog = (occurrence: PaymentOccurrence) => {
    setActionError(null);
    setPaymentDate(todayDateInputValue());
    setPayingOccurrence(occurrence);
  };

  const closePayDialog = () => {
    setPayingOccurrence(null);
    setActionError(null);
  };

  const handleConfirmPayment = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!payingOccurrence) return;

    if (!paymentDate) {
      setActionError('Укажите фактическую дату оплаты');
      return;
    }

    setActionError(null);
    setPendingId(payingOccurrence.id);
    try {
      const updated = await api.occurrences.update(payingOccurrence.id, {
        status: 'paid',
        actual_payment_date: fromDateInputValue(paymentDate),
      });
      // The list is reloaded: confirming a period may generate the next one.
      occurrences.setData(
        rows.map((row) => (row.id === updated.id ? updated : row)),
      );
      occurrences.refresh();
      summary.refresh();
      closePayDialog();
    } catch (error) {
      setActionError(error instanceof Error ? error.message : 'Не удалось сохранить оплату');
    } finally {
      setPendingId(null);
    }
  };

  const handleCancelPayment = async (occurrence: PaymentOccurrence) => {
    setPendingId(occurrence.id);
    setActionError(null);
    try {
      const updated = await api.occurrences.update(occurrence.id, {
        status: 'unpaid',
        actual_payment_date: null,
      });
      occurrences.setData(rows.map((row) => (row.id === updated.id ? updated : row)));
      occurrences.refresh();
      summary.refresh();
    } catch (error) {
      setActionError(error instanceof Error ? error.message : 'Не удалось отменить оплату');
    } finally {
      setPendingId(null);
    }
  };

  const columns: Column<PaymentOccurrence>[] = [
    {
      key: 'name',
      header: 'Название',
      render: (row) => (
        <span className={row.status === 'overdue' ? 'font-medium text-red-600 dark:text-red-400' : 'font-medium text-gray-900'}>
          {row.name}
        </span>
      ),
    },
    {
      key: 'amount',
      header: 'Сумма',
      render: (row) => (
        <span className="whitespace-nowrap tabular-nums text-gray-700">
          {formatAmount(row.amount, row.currency)}
        </span>
      ),
    },
    {
      key: 'due_date',
      header: 'Срок оплаты',
      render: (row) => (
        <span
          className={
            row.status === 'overdue' ? 'whitespace-nowrap text-red-600' : 'whitespace-nowrap text-gray-700'
          }
        >
          {formatDate(row.due_date)}
        </span>
      ),
    },
    {
      key: 'status',
      header: 'Статус',
      render: (row) => <Badge variant={row.status}>{STATUS_LABELS[row.status] ?? row.status}</Badge>,
    },
    {
      key: 'actual_payment_date',
      header: 'Факт. дата оплаты',
      render: (row) => (
        <span className="whitespace-nowrap tabular-nums text-gray-700">
          {formatDate(row.actual_payment_date)}
        </span>
      ),
    },
    {
      key: 'actions',
      header: 'Действия',
      headerClassName: 'text-right',
      className: 'text-right',
      render: (row) => (
        <div className="flex items-center justify-end gap-2">
          {row.status === 'paid' ? (
            <Button
              size="sm"
              variant="secondary"
              isLoading={pendingId === row.id}
              onClick={() => void handleCancelPayment(row)}
            >
              Отменить оплату
            </Button>
          ) : (
            <Button
              size="sm"
              variant="primary"
              onClick={() => openPayDialog(row)}
              disabled={pendingId !== null}
            >
              Оплатить
            </Button>
          )}
        </div>
      ),
    },
  ];

  const getRowClassName = (row: PaymentOccurrence) =>
    row.status === 'overdue' ? 'bg-red-50/60 dark:bg-red-950/30' : undefined;

  const summaryValues: Record<Currency, number> = {
    RUB: summary.data?.RUB ?? 0,
    USD: summary.data?.USD ?? 0,
    EUR: summary.data?.EUR ?? 0,
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-1">
        <h1 className="text-xl font-semibold tracking-tight text-gray-900">Дашборд</h1>
        <p className="text-sm text-gray-500">Обзор регулярных платежей и их статусов</p>
      </div>

      <section aria-label="Итоги">
        <h2 className="mb-3 text-sm font-semibold text-gray-900">Итоги</h2>
        {summary.error ? (
          <ErrorMessage message={summary.error} onRetry={summary.refresh} />
        ) : (
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3 sm:gap-4">
            {CURRENCIES.map((currency) => (
              <SummaryCard
                key={currency}
                currency={currency}
                value={summaryValues[currency]}
                isLoading={summary.isLoading}
              />
            ))}
          </div>
        )}
      </section>

      {occurrences.isLoading ? (
        <div className="card h-64 animate-pulse bg-gray-50" aria-hidden="true" />
      ) : occurrences.error ? null : (
        <div className="grid min-w-0 gap-6 xl:grid-cols-2 [&>*]:min-w-0">
          <PlanningOverview
            occurrences={rows}
            asOf={asOf}
            onAsOfChange={setAsOf}
            period={budgetPeriod}
            onPeriodChange={setBudgetPeriod}
            monthlyBudget={monthlyBudget}
            yearlyBudget={yearlyBudget}
          />
          <CashFlowWidget occurrences={rows} asOf={asOf} />
        </div>
      )}

      <section aria-label="Платежи">
        <div className="mb-3 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex flex-wrap items-center gap-3"><h2 className="text-sm font-semibold text-gray-900">Платежи</h2><Badge variant="neutral">Внимание: {attentionCount(rows)}</Badge></div>
          <div
            className="grid grid-cols-2 gap-1 rounded-md border border-gray-200 bg-white p-1 sm:flex sm:flex-wrap sm:items-center"
            role="tablist"
            aria-label="Фильтр платежей"
          >
            {FILTERS.map((item) => (
              <button
                key={item.key}
                type="button"
                role="tab"
                aria-selected={filter === item.key}
                onClick={() => setFilter(item.key)}
                className={[
                  'rounded px-3 py-1.5 text-sm font-medium transition-colors',
                  filter === item.key
                    ? 'bg-blue-600 text-white'
                    : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900',
                ].join(' ')}
              >
                {item.label}
              </button>
            ))}
          </div>
        </div>
        <div className="mb-3 flex flex-col gap-2 sm:flex-row sm:items-center">
          <div className="relative min-w-0 flex-1">
            <Search className="pointer-events-none absolute left-3 top-1/2 z-10 h-4 w-4 -translate-y-1/2 text-gray-400" />
            <Input className="pl-9" placeholder="Поиск по названию…" value={query} onChange={(event) => setQuery(event.target.value)} />
          </div>
          <div className="grid grid-cols-2 gap-2 sm:flex">
            <Button className="h-10" size="sm" variant={attentionOnly ? 'primary' : 'secondary'} onClick={() => setAttentionOnly((value) => !value)} leftIcon={<SlidersHorizontal className="h-4 w-4" />}>Требуют внимания</Button>
            <Button className="h-10" size="sm" variant="secondary" disabled={filteredRows.length === 0} onClick={() => downloadCsv('invoice-keeper-payments.csv', [['Название', 'Сумма', 'Валюта', 'Срок', 'Статус'], ...filteredRows.map((row) => [row.name, String(row.amount), row.currency, formatDate(row.due_date), STATUS_LABELS[row.status] ?? row.status])])} leftIcon={<Download className="h-4 w-4" />}>CSV</Button>
          </div>
        </div>

        {actionError ? (
          <div
            className="mb-3 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700"
            role="alert"
          >
            {actionError}
          </div>
        ) : null}

        <div className="card overflow-hidden">
          <Table
            columns={columns}
            rows={filteredRows}
            rowKey={(row) => row.id}
            isLoading={occurrences.isLoading}
            error={occurrences.error}
            onRetry={occurrences.refresh}
            emptyMessage={
              filter === 'all' && !query && !attentionOnly
                ? 'Нет данных о платежах'
                : 'Нет платежей с выбранным фильтром'
            }
            getRowClassName={getRowClassName}
          />
        </div>
      </section>

      <Modal
        isOpen={payingOccurrence !== null}
        title="Отметить оплату"
        onClose={closePayDialog}
        footer={
          <>
            <Button variant="secondary" onClick={closePayDialog} disabled={pendingId !== null}>
              Отмена
            </Button>
            <Button type="submit" form="pay-occurrence-form" isLoading={pendingId !== null}>
              Подтвердить
            </Button>
          </>
        }
      >
        <form id="pay-occurrence-form" onSubmit={handleConfirmPayment} className="space-y-4">
          <div className="rounded-md border border-gray-200 bg-gray-50 px-3 py-2 text-sm">
            <p className="break-words font-medium text-gray-900 [overflow-wrap:anywhere]">{payingOccurrence?.name}</p>
            <p className="mt-0.5 text-gray-500">
              {payingOccurrence
                ? `${formatAmount(payingOccurrence.amount, payingOccurrence.currency)} · срок ${formatDate(
                    payingOccurrence.due_date,
                  )}`
                : ''}
            </p>
          </div>

          <Input
            label="Фактическая дата оплаты"
            type="date"
            max={todayDateInputValue(timeZone)}
            value={paymentDate}
            onChange={(event) => setPaymentDate(event.target.value)}
            disabled={pendingId !== null}
            error={actionError ?? undefined}
          />
        </form>
      </Modal>
    </div>
  );
}

