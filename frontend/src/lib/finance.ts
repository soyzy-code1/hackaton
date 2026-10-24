import type { PaymentOccurrence, Currency } from '@/api/types';
import { CURRENCY_SYMBOLS } from '@/api/types';
import { formatAmount, formatDate } from '@/lib/date';

export { formatAmount };

export function downloadCsv(filename: string, rows: string[][]): void {
  const csv = rows
    .map((row) => row.map((cell) => `"${cell.split('"').join('""')}"`).join(';'))
    .join('\r\n');
  const blob = new Blob([`\uFEFF${csv}`], { type: 'text/csv;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(url);
}

export function exportOccurrencesCsv(occurrences: PaymentOccurrence[]): void {
  downloadCsv('invoice-keeper-payments.csv', [
    ['Название', 'Сумма', 'Валюта', 'Срок', 'Статус', 'Фактическая дата', 'Пользователь'],
    ...occurrences.map((item) => [
      item.name,
      String(item.amount),
      item.currency,
      formatDate(item.due_date),
      item.status,
      formatDate(item.actual_payment_date),
      item.user_id,
    ]),
  ]);
}

export function currencyTotal(occurrences: PaymentOccurrence[], currency: Currency, paidOnly = false): number {
  return occurrences
    .filter((item) => item.currency === currency && (!paidOnly || item.status === 'paid'))
    .reduce((sum, item) => sum + item.amount, 0);
}

export function currencyLabel(currency: Currency): string {
  return `${CURRENCY_SYMBOLS[currency]} ${currency}`;
}

export function buildMonthTrend(occurrences: PaymentOccurrence[], currency: Currency): { label: string; amount: number }[] {
  const totals = new Map<string, number>();
  for (const occurrence of occurrences) {
    if (occurrence.currency !== currency) continue;
    const key = occurrence.due_date.slice(0, 7);
    totals.set(key, (totals.get(key) ?? 0) + occurrence.amount);
  }
  return [...totals.entries()]
    .sort(([a], [b]) => a.localeCompare(b))
    .slice(-6)
    .map(([key, amount]) => ({ label: key.slice(5), amount }));
}

export function attentionCount(occurrences: PaymentOccurrence[]): number {
  return occurrences.filter((item) => item.status === 'overdue' || item.status === 'unpaid').length;
}

export function occurrenceDescription(item: PaymentOccurrence): string {
  return `${item.name} · ${formatAmount(item.amount, item.currency)} · ${formatDate(item.due_date)}`;
}
