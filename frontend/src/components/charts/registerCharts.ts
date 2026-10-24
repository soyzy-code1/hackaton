/**
 * Registers only the Chart.js pieces the app uses, so the bundle stays small
 * (Chart.js is tree-shakeable). Import this module once before any chart.
 */
import {
  BarController,
  BarElement,
  CategoryScale,
  Chart,
  Filler,
  Legend,
  LinearScale,
  LineController,
  LineElement,
  PointElement,
  Tooltip,
} from 'chart.js';

Chart.register(
  BarController,
  BarElement,
  CategoryScale,
  Filler,
  Legend,
  LinearScale,
  LineController,
  LineElement,
  PointElement,
  Tooltip,
);

Chart.defaults.font.family =
  "Inter, ui-sans-serif, system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif";
Chart.defaults.font.size = 12;
Chart.defaults.color = '#6b7280';
Chart.defaults.borderColor = '#e5e7eb';

/** Flat palette that matches the Tailwind theme (no gradients). */
export const CHART_COLORS = {
  paid: '#16a34a',
  remaining: '#ca8a04',
  overdue: '#dc2626',
  accent: '#2563eb',
  grid: '#f3f4f6',
  muted: '#9ca3af',
} as const;

/**
 * Axis/legend colours for the current UI theme. Chart.js draws on a canvas,
 * so CSS dark-mode overrides do not reach it — charts must ask explicitly.
 */
export function chartThemeColors(theme: 'light' | 'dark'): { text: string; grid: string } {
  return theme === 'dark'
    ? { text: '#cbd5e1', grid: '#334155' } // slate-300 / slate-700
    : { text: '#6b7280', grid: '#f3f4f6' }; // gray-500 / gray-100
}

export function formatChartNumber(value: number): string {
  return new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 0 }).format(value);
}
