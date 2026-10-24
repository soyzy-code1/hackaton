import { useMemo, useState } from 'react';
import { AlertTriangle, CheckCircle2, Wallet } from 'lucide-react';
import type { Currency, PaymentOccurrence } from '@/api/types';
import { CURRENCIES, CURRENCY_SYMBOLS } from '@/api/types';
import { useSettingsStore } from '@/store/settings';
import { formatAmount } from '@/lib/date';
import { monthRange, projectCashFlow } from '@/lib/planning';
import { CashFlowChart } from '@/components/charts/CashFlowChart';
import '@/components/charts/registerCharts';

interface CashFlowWidgetProps {
  occurrences: PaymentOccurrence[];
  /** `YYYY-MM-DD` — the month containing this day is projected. */
  asOf: string;
}

/**
 * «Прогноз кассового разрыва»: projects the balance through the month and
 * highlights the days when the scheduled payments exceed the available money.
 */
export function CashFlowWidget({ occurrences, asOf }: CashFlowWidgetProps) {
  const cashBalance = useSettingsStore((state) => state.cashBalance);
  const setCashBalance = useSettingsStore((state) => state.setCashBalance);
  const [currency, setCurrency] = useState<Currency>('RUB');

  const range = useMemo(() => monthRange(asOf), [asOf]);
  const projection = useMemo(
    () => projectCashFlow(occurrences, currency, range, cashBalance[currency]),
    [occurrences, currency, range, cashBalance],
  );

  const gap = projection.firstGapDay;
  const gapDate = gap ? gap.key.split('-').reverse().join('.') : '';
  const shortfall = Math.max(0, -projection.minBalance);

  return (
    <section className="card min-w-0 p-4 sm:p-5" aria-label="Прогноз кассового разрыва">
      <div className="mb-4 flex flex-col gap-3 2xl:flex-row 2xl:items-start 2xl:justify-between">
        <div className="min-w-0">
          <h2 className="text-sm font-semibold text-gray-900">Прогноз кассового разрыва</h2>
          <p className="mt-1 text-xs text-gray-500">
            Неоплаченные периоды месяца против остатка на счёте
          </p>
        </div>
        <div className="flex flex-wrap items-end gap-2">
          <div className="flex rounded-md border border-gray-200 p-0.5" role="tablist" aria-label="Валюта прогноза">
            {CURRENCIES.map((item) => (
              <button
                key={item}
                type="button"
                role="tab"
                aria-selected={currency === item}
                onClick={() => setCurrency(item)}
                className={[
                  'rounded px-2.5 py-1 text-xs font-medium transition-colors',
                  currency === item ? 'bg-blue-600 text-white' : 'text-gray-600 hover:bg-gray-100',
                ].join(' ')}
              >
                {CURRENCY_SYMBOLS[item]} {item}
              </button>
            ))}
          </div>
          <label className="flex min-w-0 flex-wrap items-center gap-2 text-xs text-gray-600">
            <Wallet className="h-4 w-4 text-gray-400" aria-hidden="true" />
            Остаток на счёте
            <input
              type="number"
              min="0"
              step="100"
              value={cashBalance[currency]}
              onChange={(event) => setCashBalance(currency, Number(event.target.value))}
              className="h-8 w-36 min-w-0 rounded-md border border-gray-300 bg-white px-2 text-sm text-gray-900"
              aria-label={`Остаток на счёте, ${currency}`}
            />
          </label>
        </div>
      </div>

      {gap ? (
        <div
          className="mb-4 flex gap-3 rounded-md border border-red-200 bg-red-50 px-3 py-2.5 text-sm text-red-800"
          role="alert"
        >
          <AlertTriangle className="mt-0.5 h-4 w-4 flex-shrink-0 text-red-600" aria-hidden="true" />
          <div className="min-w-0 break-words [overflow-wrap:anywhere]">
            <p className="font-medium">
              Внимание! {gapDate} нужно оплатить {gap.items.length}{' '}
              {pluralizeBills(gap.items.length)} на {formatAmount(gap.outflow, currency)}, и остаток уйдёт в минус.
            </p>
            <p className="mt-0.5 text-red-700">
              За месяц к оплате {formatAmount(projection.totalOutflow, currency)} при остатке{' '}
              {formatAmount(projection.startBalance, currency)} — не хватает{' '}
              {formatAmount(shortfall, currency)}.
            </p>
          </div>
        </div>
      ) : (
        <div className="mb-4 flex gap-2 rounded-md border border-green-200 bg-green-50 px-3 py-2.5 text-sm text-green-800">
          <CheckCircle2 className="mt-0.5 h-4 w-4 flex-shrink-0 text-green-600" aria-hidden="true" />
          <p className="min-w-0 break-words [overflow-wrap:anywhere]">
            Кассового разрыва нет: к оплате {formatAmount(projection.totalOutflow, currency)}, в конце месяца
            останется {formatAmount(projection.startBalance - projection.totalOutflow, currency)}.
          </p>
        </div>
      )}

      {projection.totalOutflow === 0 ? (
        <p className="py-10 text-center text-sm text-gray-500">
          В этом месяце нет неоплаченных платежей в {currency}
        </p>
      ) : (
        <CashFlowChart projection={projection} currency={currency} />
      )}
    </section>
  );
}

function pluralizeBills(count: number): string {
  const mod10 = count % 10;
  const mod100 = count % 100;
  if (mod10 === 1 && mod100 !== 11) return 'счёт';
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return 'счёта';
  return 'счетов';
}
