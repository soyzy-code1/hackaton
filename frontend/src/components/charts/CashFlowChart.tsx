import { Chart } from 'react-chartjs-2';
import type { ChartData, ChartOptions } from 'chart.js';
import type { Currency } from '@/api/types';
import type { CashFlowProjection } from '@/lib/planning';
import { useSettingsStore } from '@/store/settings';
import { CHART_COLORS, chartThemeColors, formatChartNumber } from './registerCharts';

interface CashFlowChartProps {
  projection: CashFlowProjection;
  currency: Currency;
}

/**
 * Mixed chart for one month:
 * - bars: amount due on each day (red on cash-gap days);
 * - line: projected balance after that day's payments;
 * - dashed line: zero — crossing it means a cash gap.
 */
export function CashFlowChart({ projection, currency }: CashFlowChartProps) {
  const theme = useSettingsStore((state) => state.theme);
  const colors = chartThemeColors(theme);
  const labels = projection.days.map((day) => String(day.dayNumber));

  const data: ChartData<'bar' | 'line', number[], string> = {
    labels,
    datasets: [
      {
        type: 'bar',
        label: 'К оплате за день',
        data: projection.days.map((day) => day.outflow),
        backgroundColor: projection.days.map((day) =>
          day.isGap && day.outflow > 0 ? CHART_COLORS.overdue : CHART_COLORS.accent,
        ),
        borderRadius: 2,
        yAxisID: 'y',
        order: 2,
      },
      {
        type: 'line',
        label: 'Остаток на счёте',
        data: projection.days.map((day) => day.balance),
        borderColor: CHART_COLORS.paid,
        borderWidth: 2,
        pointRadius: 0,
        pointHoverRadius: 4,
        tension: 0,
        stepped: 'after',
        segment: {
          borderColor: (context) =>
            context.p1.parsed.y < 0 ? CHART_COLORS.overdue : CHART_COLORS.paid,
        },
        yAxisID: 'y',
        order: 1,
      },
      {
        type: 'line',
        label: 'Ноль (кассовый разрыв)',
        data: projection.days.map(() => 0),
        borderColor: CHART_COLORS.overdue,
        borderWidth: 1,
        borderDash: [4, 4],
        pointRadius: 0,
        yAxisID: 'y',
        order: 0,
      },
    ],
  };

  const options: ChartOptions<'bar' | 'line'> = {
    responsive: true,
    maintainAspectRatio: false,
    animation: { duration: 300 },
    interaction: { mode: 'index', intersect: false },
    scales: {
      x: {
        grid: { display: false },
        ticks: { color: colors.text, autoSkip: true, maxRotation: 0 },
        title: { display: true, text: 'День месяца', color: colors.text },
      },
      y: {
        grid: { color: colors.grid },
        ticks: { color: colors.text, callback: (value) => formatChartNumber(Number(value)) },
        title: { display: true, text: currency, color: colors.text },
      },
    },
    plugins: {
      legend: { position: 'bottom', labels: { boxWidth: 12, boxHeight: 12, color: colors.text } },
      tooltip: {
        callbacks: {
          title: (items) => {
            const index = items[0]?.dataIndex ?? 0;
            const day = projection.days[index];
            return day ? day.key.split('-').reverse().join('.') : '';
          },
          label: (context) =>
            `${context.dataset.label}: ${formatChartNumber(Number(context.raw))} ${currency}`,
          afterBody: (items) => {
            const index = items[0]?.dataIndex ?? 0;
            const day = projection.days[index];
            if (!day || day.items.length === 0) return [];
            return ['', ...day.items.map((item) => `• ${item.name} — ${formatChartNumber(item.amount)}`)];
          },
        },
      },
    },
  };

  return (
    <div className="relative h-64 w-full min-w-0 sm:h-72">
      <Chart type="bar" data={data} options={options} />
    </div>
  );
}
