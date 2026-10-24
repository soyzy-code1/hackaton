import { Bar } from 'react-chartjs-2';
import type { ChartOptions } from 'chart.js';
import type { Currency } from '@/api/types';
import { CURRENCIES } from '@/api/types';
import type { CurrencyTotals } from '@/lib/planning';
import { useSettingsStore } from '@/store/settings';
import { CHART_COLORS, chartThemeColors, formatChartNumber } from './registerCharts';

interface PaidVsRemainingChartProps {
  totals: Record<Currency, CurrencyTotals>;
}

/**
 * Stacked horizontal bars, one per currency: paid / remaining / overdue.
 * Each currency is its own bar, so RUB never dwarfs USD and EUR — amounts in
 * different currencies are not comparable and are never summed.
 */
export function PaidVsRemainingChart({ totals }: PaidVsRemainingChartProps) {
  const theme = useSettingsStore((state) => state.theme);
  const colors = chartThemeColors(theme);
  const labels = CURRENCIES.map((currency) => currency);
  const paid = CURRENCIES.map((currency) => totals[currency].paid);
  const overdue = CURRENCIES.map((currency) => totals[currency].overdue);
  const pending = CURRENCIES.map((currency) =>
    Math.max(0, totals[currency].remaining - totals[currency].overdue),
  );

  // Normalise per currency to a percentage, so every bar is readable.
  const toPercent = (values: number[]) =>
    values.map((value, index) => {
      const planned = totals[CURRENCIES[index] as Currency].planned;
      return planned > 0 ? (value / planned) * 100 : 0;
    });

  const raw = { paid, pending, overdue };

  const options: ChartOptions<'bar'> = {
    indexAxis: 'y',
    responsive: true,
    maintainAspectRatio: false,
    animation: { duration: 300 },
    scales: {
      x: {
        stacked: true,
        max: 100,
        grid: { color: colors.grid },
        ticks: { color: colors.text, callback: (value) => `${value}%` },
      },
      y: { stacked: true, grid: { display: false }, ticks: { color: colors.text } },
    },
    plugins: {
      legend: { position: 'bottom', labels: { boxWidth: 12, boxHeight: 12, color: colors.text } },
      tooltip: {
        callbacks: {
          label: (context) => {
            const key = (['paid', 'pending', 'overdue'] as const)[context.datasetIndex];
            const currency = CURRENCIES[context.dataIndex] as Currency;
            const amount = key ? raw[key][context.dataIndex] ?? 0 : 0;
            return `${context.dataset.label}: ${formatChartNumber(amount)} ${currency} (${Math.round(
              Number(context.raw),
            )}%)`;
          },
        },
      },
    },
  };

  return (
    <div className="relative h-52 w-full min-w-0">
      <Bar
        options={options}
        data={{
          labels,
          datasets: [
            { label: 'Оплачено', data: toPercent(paid), backgroundColor: CHART_COLORS.paid, borderRadius: 2 },
            { label: 'Осталось', data: toPercent(pending), backgroundColor: CHART_COLORS.remaining, borderRadius: 2 },
            { label: 'Просрочено', data: toPercent(overdue), backgroundColor: CHART_COLORS.overdue, borderRadius: 2 },
          ],
        }}
      />
    </div>
  );
}
