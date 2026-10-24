import { useEffect, useMemo, useState } from 'react';
import { Download, TrendingUp } from 'lucide-react';
import { api } from '@/api/client';
import type { Currency, PaymentOccurrence } from '@/api/types';
import { CURRENCIES, CURRENCY_SYMBOLS } from '@/api/types';
import { useAsyncResource } from '@/hooks/useAsyncResource';
import { useSettingsStore } from '@/store/settings';
import { buildMonthTrend, downloadCsv, formatAmount } from '@/lib/finance';
import { formatDate, todayDateInputValue } from '@/lib/date';
import { totalsAsOf, yearRange } from '@/lib/planning';
import { PaidVsRemainingChart } from '@/components/charts/PaidVsRemainingChart';
import '@/components/charts/registerCharts';
import { Button } from '@/components/ui/Button';
import { ErrorMessage } from '@/components/ui/ErrorMessage';
import { Input } from '@/components/ui/Input';

export function AnalyticsPage() {
  const occurrences = useAsyncResource<PaymentOccurrence[]>(() => api.occurrences.list(), []);
  const [budget, setBudgetState] = useState<Record<Currency, number>>({ RUB: 0, USD: 0, EUR: 0 });
  const [yearlyBudget, setYearlyBudgetState] = useState<Record<Currency, number>>({ RUB: 0, USD: 0, EUR: 0 });
  const [budgetSaving, setBudgetSaving] = useState(false);
  const [budgetLoading, setBudgetLoading] = useState(true);
  const [budgetError, setBudgetError] = useState<string | null>(null);
  const [budgetNotice, setBudgetNotice] = useState<string | null>(null);
  const timeZone = useSettingsStore((state) => state.timeZone);
  const [currency, setCurrency] = useState<Currency>('RUB');

  useEffect(() => {
    let cancelled = false;
    setBudgetLoading(true);
    void api.budgets.list()
      .then((plans) => {
        if (cancelled) return;
        const monthly: Record<Currency, number> = { RUB: 0, USD: 0, EUR: 0 };
        const yearly: Record<Currency, number> = { RUB: 0, USD: 0, EUR: 0 };
        for (const plan of plans) {
          monthly[plan.currency] = plan.monthly_limit;
          yearly[plan.currency] = plan.yearly_limit;
        }
        setBudgetState(monthly);
        setYearlyBudgetState(yearly);
      })
      .catch((error: unknown) => {
        if (!cancelled) setBudgetError(error instanceof Error ? error.message : 'Не удалось загрузить лимиты');
      })
      .finally(() => { if (!cancelled) setBudgetLoading(false); });
    return () => { cancelled = true; };
  }, []);

  const updateBudget = async (nextMonthly: number, nextYearly: number) => {
    const monthly = Math.max(0, nextMonthly);
    const yearly = Math.max(0, nextYearly);
    setBudgetState((value) => ({ ...value, [currency]: monthly }));
    setYearlyBudgetState((value) => ({ ...value, [currency]: yearly }));
    setBudgetSaving(true);
    setBudgetError(null);
    setBudgetNotice(null);
    try {
      await api.budgets.update(currency, { monthly_limit: monthly, yearly_limit: yearly });
      setBudgetNotice('Лимит сохранён');
    } catch (error) {
      setBudgetError(error instanceof Error ? error.message : 'Не удалось сохранить лимит');
    } finally {
      setBudgetSaving(false);
    }
  };

  const rows = useMemo(() => occurrences.data ?? [], [occurrences.data]);
  const today = todayDateInputValue(timeZone);
  const yearTotals = useMemo(() => totalsAsOf(rows, yearRange(today), today), [rows, today]);
  const trends = useMemo(() => buildMonthTrend(rows, currency), [rows, currency]);
  const maxTrend = Math.max(...trends.map((item) => item.amount), 1);
  const current = yearTotals[currency];
  const yearUsage = yearlyBudget[currency] > 0 ? (current.planned / yearlyBudget[currency]) * 100 : 0;

  const exportCsv = () =>
    downloadCsv('invoice-keeper-analytics.csv', [
      ['Название', 'Сумма', 'Валюта', 'Срок', 'Статус'],
      ...rows.map((row) => [row.name, String(row.amount), row.currency, formatDate(row.due_date), row.status]),
    ]);

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-xl font-semibold text-gray-900">Бюджет и аналитика</h1>
          <p className="text-sm text-gray-500">Лимиты на месяц и год, динамика и крупнейшие расходы</p>
        </div>
        <Button variant="secondary" leftIcon={<Download className="h-4 w-4" />} onClick={exportCsv} disabled={rows.length === 0}>
          Экспорт CSV
        </Button>
      </div>

      {occurrences.error ? <ErrorMessage message={occurrences.error} onRetry={occurrences.refresh} /> : null}

      <section className="card p-5" aria-label="Итоги года">
        <h2 className="mb-1 text-sm font-semibold text-gray-900">Итоги года по валютам</h2>
        <p className="mb-3 text-xs text-gray-500">На {formatDate(`${today}T00:00:00.000Z`)} — каждая валюта считается отдельно</p>
        <PaidVsRemainingChart totals={yearTotals} />
      </section>

      <div className="flex flex-wrap gap-2" role="tablist" aria-label="Валюта">
        {CURRENCIES.map((item) => (
          <button
            type="button"
            role="tab"
            aria-selected={currency === item}
            key={item}
            onClick={() => setCurrency(item)}
            className={[
              'rounded-md border px-3 py-1.5 text-sm',
              currency === item ? 'border-blue-600 bg-blue-50 text-blue-700' : 'border-gray-200 text-gray-600 hover:bg-gray-100',
            ].join(' ')}
          >
            {CURRENCY_SYMBOLS[item]} {item}
          </button>
        ))}
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <div className="card min-w-0 p-5">
          <p className="text-xs uppercase tracking-wide text-gray-500">План на год</p>
          <p className="mt-2 break-all text-xl font-semibold text-gray-900 sm:text-2xl">{formatAmount(current.planned, currency)}</p>
          <p className="mt-1 text-xs text-gray-500">{Math.round(yearUsage)}% годового бюджета</p>
        </div>
        <div className="card min-w-0 p-5">
          <p className="text-xs uppercase tracking-wide text-gray-500">Оплачено за год</p>
          <p className="mt-2 break-all text-xl font-semibold text-green-700 sm:text-2xl">{formatAmount(current.paid, currency)}</p>
          <p className="mt-1 break-all text-xs text-gray-500">Осталось {formatAmount(current.remaining, currency)}</p>
        </div>
        <div className="card min-w-0 space-y-3 p-5 sm:col-span-2 lg:col-span-1">
          <Input label="Бюджет / месяц" type="number" min="0" value={budget[currency]} disabled={budgetLoading || budgetSaving} onChange={(e) => setBudgetState((value) => ({ ...value, [currency]: Number(e.target.value) }))} />
          <Input label="Бюджет / год" type="number" min="0" value={yearlyBudget[currency]} disabled={budgetLoading || budgetSaving} onChange={(e) => setYearlyBudgetState((value) => ({ ...value, [currency]: Number(e.target.value) }))} />
          <Button type="button" disabled={budgetLoading || budgetSaving} onClick={() => void updateBudget(budget[currency], yearlyBudget[currency])}>
            {budgetSaving ? 'Сохранение…' : 'Сохранить лимиты'}
          </Button>
          {budgetError ? <p className="text-xs text-red-600">{budgetError}</p> : null}
          {budgetNotice ? <p className="text-xs text-green-600">{budgetNotice}</p> : null}
        </div>
      </div>

      <section className="card p-5">
        <div className="mb-5 flex items-center gap-2">
          <TrendingUp className="h-4 w-4 text-blue-600" />
          <h2 className="text-sm font-semibold text-gray-900">Плановые расходы по месяцам</h2>
        </div>
        {trends.length === 0 ? (
          <p className="py-10 text-center text-sm text-gray-500">Нет данных в {currency}</p>
        ) : (
          <div className="flex h-48 items-end gap-1.5 sm:gap-3">
            {trends.map((item) => {
              const overMonthly = budget[currency] > 0 && item.amount > budget[currency];
              return (
                <div key={item.label} className="flex min-w-0 flex-1 flex-col items-center gap-2">
                  <span className="max-w-full truncate text-[10px] text-gray-500 sm:text-[11px]">{Math.round(item.amount).toLocaleString('ru-RU')}</span>
                  <div className={['w-full rounded-t', overMonthly ? 'bg-red-600' : 'bg-blue-600'].join(' ')} style={{ height: `${Math.max(8, (item.amount / maxTrend) * 130)}px` }} />
                  <span className="text-xs text-gray-500">{item.label}</span>
                </div>
              );
            })}
          </div>
        )}
        <p className="mt-3 text-xs text-gray-500">Красным — месяцы, где план превышает месячный бюджет.</p>
      </section>

      <section className="card overflow-hidden">
        <div className="border-b border-gray-200 px-5 py-4">
          <h2 className="text-sm font-semibold text-gray-900">Крупнейшие расходы</h2>
        </div>
        {rows.filter((r) => r.currency === currency).length === 0 ? (
          <p className="px-5 py-8 text-sm text-gray-500">Нет данных</p>
        ) : (
          rows
            .filter((r) => r.currency === currency)
            .sort((a, b) => b.amount - a.amount)
            .slice(0, 5)
            .map((r) => (
              <div key={r.id} className="flex flex-col gap-1 border-b border-gray-100 px-4 py-3 text-sm last:border-0 sm:flex-row sm:items-center sm:justify-between sm:gap-4 sm:px-5">
                <span className="min-w-0 break-words text-gray-700 [overflow-wrap:anywhere]">{r.name} <span className="text-xs text-gray-400">· {formatDate(r.due_date)}</span></span>
                <span className="flex-shrink-0 font-medium tabular-nums text-gray-900">{formatAmount(r.amount, r.currency)}</span>
              </div>
            ))
        )}
      </section>
    </div>
  );
}
