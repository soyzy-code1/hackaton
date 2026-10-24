import { useMemo } from 'react';
import { CalendarSearch } from 'lucide-react';
import type { Currency, PaymentOccurrence } from '@/api/types';
import { CURRENCIES, CURRENCY_SYMBOLS } from '@/api/types';
import { useSettingsStore } from '@/store/settings';
import type { BudgetPeriod } from '@/store/settings';
import { formatAmount } from '@/lib/date';
import { countByCurrency, monthRange, totalsAsOf, yearRange } from '@/lib/planning';
import type { CurrencyTotals } from '@/lib/planning';
import { PaidVsRemainingChart } from '@/components/charts/PaidVsRemainingChart';
import '@/components/charts/registerCharts';

interface PlanningOverviewProps {
  occurrences: PaymentOccurrence[];
  asOf: string;
  onAsOfChange: (value: string) => void;
  period: BudgetPeriod;
  onPeriodChange: (value: BudgetPeriod) => void;
}

/**
 * «Бюджет + Итоги на дату»: totals for the month or year known on the
 * selected day, budget usage per currency and a paid-vs-remaining chart.
 */
export function PlanningOverview({
  occurrences,
  asOf,
  onAsOfChange,
  period,
  onPeriodChange,
}: PlanningOverviewProps) {
  const budget = useSettingsStore((state) => state.budget);
  const yearlyBudget = useSettingsStore((state) => state.yearlyBudget);

  const range = useMemo(
    () => (period === 'month' ? monthRange(asOf) : yearRange(asOf)),
    [asOf, period],
  );
  const totals = useMemo(() => totalsAsOf(occurrences, range, asOf), [occurrences, range, asOf]);
  const counts = useMemo(() => countByCurrency(occurrences, range), [occurrences, range]);
  const limits = period === 'month' ? budget : yearlyBudget;

  const overdueParts = CURRENCIES.filter((c) => totals[c].overdueCount > 0).map(
    (c) => `${totals[c].overdueCount} в ${c}`,
  );
  const countParts = CURRENCIES.filter((c) => counts[c] > 0).map((c) => `${counts[c]} в ${c}`);
  const periodLabel = period === 'month' ? 'В этом месяце' : 'В этом году';

  return (
    <section className="card min-w-0 p-4 sm:p-5" aria-label="Бюджет и итоги на дату">
      <div className="mb-4 flex flex-col gap-3 2xl:flex-row 2xl:items-start 2xl:justify-between">
        <div className="min-w-0">
          <h2 className="text-sm font-semibold text-gray-900">Бюджет и итоги на дату</h2>
          <p className="mt-1 text-sm text-gray-600">
            {countParts.length > 0
              ? `${periodLabel} ${countParts.join(', ')} ${pluralizeBills(
                  CURRENCIES.reduce((sum, c) => sum + counts[c], 0),
                )}.`
              : `${periodLabel} платежей нет.`}{' '}
            {overdueParts.length > 0 ? (
              <span className="font-medium text-red-600">Просрочено: {overdueParts.join(', ')}.</span>
            ) : (
              <span className="text-green-700">Просрочек нет.</span>
            )}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <div className="flex rounded-md border border-gray-200 p-0.5" role="tablist" aria-label="Период бюджета">
            {(['month', 'year'] as const).map((item) => (
              <button
                key={item}
                type="button"
                role="tab"
                aria-selected={period === item}
                onClick={() => onPeriodChange(item)}
                className={[
                  'rounded px-2.5 py-1 text-xs font-medium transition-colors',
                  period === item ? 'bg-blue-600 text-white' : 'text-gray-600 hover:bg-gray-100',
                ].join(' ')}
              >
                {item === 'month' ? 'Месяц' : 'Год'}
              </button>
            ))}
          </div>
          <label className="flex min-w-0 flex-wrap items-center gap-2 text-xs text-gray-600">
            <CalendarSearch className="h-4 w-4 text-gray-400" aria-hidden="true" />
            Итоги на
            <input
              type="date"
              value={asOf}
              onChange={(event) => event.target.value && onAsOfChange(event.target.value)}
              className="h-8 min-w-0 rounded-md border border-gray-300 bg-white px-2 text-sm text-gray-900"
              aria-label="Дата, на которую считаются итоги"
            />
          </label>
        </div>
      </div>

      {/* auto-fit: cards wrap to new rows instead of squeezing big amounts */}
      <div className="grid gap-3 [grid-template-columns:repeat(auto-fit,minmax(min(100%,11rem),1fr))]">
        {CURRENCIES.map((currency) => (
          <BudgetCard key={currency} totals={totals[currency]} limit={limits[currency]} currency={currency} />
        ))}
      </div>

      <div className="mt-5">
        <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-500">
          Оплачено vs Осталось
        </h3>
        <PaidVsRemainingChart totals={totals} />
      </div>
    </section>
  );
}

function BudgetCard({
  totals,
  limit,
  currency,
}: {
  totals: CurrencyTotals;
  limit: number;
  currency: Currency;
}) {
  const usage = limit > 0 ? (totals.planned / limit) * 100 : 0;
  const overBudget = limit > 0 && totals.planned > limit;
  const barColor = overBudget ? 'bg-red-600' : usage > 85 ? 'bg-yellow-500' : 'bg-blue-600';

  return (
    <div className="min-w-0 rounded-md border border-gray-200 p-4">
      <div className="flex items-baseline justify-between gap-2">
        <p className="text-xs font-medium uppercase tracking-wide text-gray-500">
          {CURRENCY_SYMBOLS[currency]} {currency}
        </p>
        <p className="text-xs text-gray-400">{totals.plannedCount} пл.</p>
      </div>
      <p className="mt-1 break-all text-lg font-semibold tabular-nums text-gray-900 sm:text-xl">
        {formatAmount(totals.planned, currency)}
      </p>
      <dl className="mt-2 space-y-0.5 text-xs">
        <div className="flex flex-wrap justify-between gap-x-2">
          <dt className="text-gray-500">Оплачено</dt>
          <dd className="break-all tabular-nums text-green-700">{formatAmount(totals.paid, currency)}</dd>
        </div>
        <div className="flex flex-wrap justify-between gap-x-2">
          <dt className="text-gray-500">Осталось</dt>
          <dd className="break-all tabular-nums text-yellow-700">{formatAmount(totals.remaining, currency)}</dd>
        </div>
        {totals.overdue > 0 ? (
          <div className="flex flex-wrap justify-between gap-x-2">
            <dt className="text-gray-500">Просрочено</dt>
            <dd className="break-all tabular-nums text-red-600">{formatAmount(totals.overdue, currency)}</dd>
          </div>
        ) : null}
      </dl>
      <div className="mt-3">
        <div className="flex flex-wrap justify-between gap-x-2 text-[11px] text-gray-500">
          <span className="break-all">Бюджет {formatAmount(limit, currency)}</span>
          <span className={overBudget ? 'font-medium text-red-600' : ''}>{Math.round(usage)}%</span>
        </div>
        <div className="mt-1 h-1.5 overflow-hidden rounded-full bg-gray-100" role="progressbar" aria-valuenow={Math.round(usage)} aria-valuemin={0} aria-valuemax={100}>
          <div className={`h-full ${barColor}`} style={{ width: `${Math.min(100, usage)}%` }} />
        </div>
        {overBudget ? (
          <p className="mt-1 break-all text-[11px] font-medium text-red-600">
            Превышение на {formatAmount(totals.planned - limit, currency)}
          </p>
        ) : null}
      </div>
    </div>
  );
}

function pluralizeBills(count: number): string {
  const mod10 = count % 10;
  const mod100 = count % 100;
  if (mod10 === 1 && mod100 !== 11) return 'счёт';
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return 'счёта';
  return 'счетов';
}
