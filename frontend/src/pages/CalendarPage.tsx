import { useMemo, useState } from 'react';
import { ChevronLeft, ChevronRight, CheckCircle2, CalendarDays } from 'lucide-react';
import { api } from '@/api/client';
import type { PaymentOccurrence } from '@/api/types';
import { useAsyncResource } from '@/hooks/useAsyncResource';
import { useSettingsStore } from '@/store/settings';
import { buildMonthGrid, formatAmount, formatDateInTz, monthName } from '@/lib/date';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Modal } from '@/components/ui/Modal';

export function CalendarPage() {
  const timeZone = useSettingsStore((state) => state.timeZone);
  const [cursor, setCursor] = useState(() => new Date());
  const [selectedDay, setSelectedDay] = useState<string | null>(null);
  const occurrences = useAsyncResource<PaymentOccurrence[]>(() => api.occurrences.list(), []);
  const year = cursor.getFullYear();
  const month = cursor.getMonth();
  const days = useMemo(() => buildMonthGrid(year, month, timeZone), [year, month, timeZone]);
  const byDay = useMemo(() => {
    const map = new Map<string, PaymentOccurrence[]>();
    for (const item of occurrences.data ?? []) {
      const key = formatDateInTz(item.due_date, timeZone).split('.').reverse().join('-');
      map.set(key, [...(map.get(key) ?? []), item]);
    }
    return map;
  }, [occurrences.data, timeZone]);
  const selectedItems = selectedDay ? byDay.get(selectedDay) ?? [] : [];

  const shiftMonth = (amount: number) => setCursor(new Date(year, month + amount, 1));
  const markPaid = async (item: PaymentOccurrence) => {
    await api.occurrences.update(item.id, { status: 'paid', actual_payment_date: new Date().toISOString() });
    occurrences.refresh();
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div><h1 className="text-xl font-semibold text-gray-900">Календарь оплат</h1><p className="text-sm text-gray-500">Сроки платежей и план на месяц</p></div>
        <div className="flex flex-wrap items-center gap-2"><Button size="sm" variant="secondary" onClick={() => setCursor(new Date())}>Сегодня</Button><Button size="sm" variant="secondary" aria-label="Предыдущий месяц" onClick={() => shiftMonth(-1)} leftIcon={<ChevronLeft className="h-4 w-4" />}/><span className="min-w-32 flex-1 text-center text-sm font-semibold capitalize text-gray-900 sm:flex-none">{monthName(month)} {year}</span><Button size="sm" variant="secondary" aria-label="Следующий месяц" onClick={() => shiftMonth(1)} rightIcon={<ChevronRight className="h-4 w-4" />}/></div>
      </div>
      <div className="card overflow-hidden">
        <div className="grid grid-cols-7 border-b border-gray-200 bg-gray-50">{['Пн','Вт','Ср','Чт','Пт','Сб','Вс'].map((day) => <div key={day} className="p-2 text-center text-xs font-semibold text-gray-500">{day}</div>)}</div>
        <div className="grid grid-cols-7">
          {days.map((day) => { const items = byDay.get(day.key) ?? []; return <button type="button" key={day.key} onClick={() => setSelectedDay(day.key)} className={['min-h-16 min-w-0 border-b border-r border-gray-100 p-1 text-left transition-colors hover:bg-blue-50 sm:min-h-28 sm:p-2', !day.inCurrentMonth ? 'bg-gray-50 text-gray-400' : '', day.isToday ? 'bg-blue-50/60' : ''].join(' ')}><span className={['inline-flex h-6 w-6 items-center justify-center rounded-full text-xs', day.isToday ? 'bg-blue-600 font-semibold text-white' : 'text-gray-700'].join(' ')}>{day.dayNumber}</span>{items.length > 0 ? <span className="ml-1 inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-blue-600 px-1 text-[10px] font-semibold text-white sm:hidden">{items.length}</span> : null}<div className="mt-2 hidden space-y-1 sm:block">{items.slice(0,3).map((item) => <div key={item.id} className={['truncate rounded border px-1.5 py-1 text-[11px]', item.status === 'paid' ? 'border-green-200 bg-green-50 text-green-700' : item.status === 'overdue' ? 'border-red-200 bg-red-50 text-red-700' : 'border-yellow-200 bg-yellow-50 text-yellow-700'].join(' ')}>{item.name}</div>)}{items.length > 3 ? <p className="text-[11px] text-gray-500">+ ещё {items.length - 3}</p> : null}</div></button>; })}
        </div>
      </div>
      <Modal isOpen={selectedDay !== null} title={selectedDay ? `Платежи ${selectedDay.split('-').reverse().join('.')}` : 'Платежи'} onClose={() => setSelectedDay(null)}>
        {selectedItems.length === 0 ? <div className="py-8 text-center text-sm text-gray-500">На эту дату платежей нет</div> : <div className="space-y-3">{selectedItems.map((item) => <div key={item.id} className="flex flex-col gap-3 rounded-md border border-gray-200 p-3 sm:flex-row sm:items-center sm:justify-between"><div className="min-w-0"><p className="break-words text-sm font-medium text-gray-900 [overflow-wrap:anywhere]">{item.name}</p><p className="mt-1 text-xs text-gray-500">{formatAmount(item.amount,item.currency)} · {formatDateInTz(item.due_date,timeZone)}</p></div><div className="flex flex-shrink-0 items-center gap-2"><Badge variant={item.status}>{item.status === 'paid' ? 'Оплачено' : item.status === 'overdue' ? 'Просрочка' : 'Не оплачено'}</Badge>{item.status !== 'paid' ? <Button size="sm" onClick={() => void markPaid(item)} leftIcon={<CheckCircle2 className="h-3.5 w-3.5"/>}>Оплатить</Button> : null}</div></div>)}</div>}
      </Modal>
      <div className="flex items-center gap-2 text-xs text-gray-500"><CalendarDays className="h-4 w-4"/>Часовой пояс календаря: {timeZone}. Напоминание по умолчанию — за 3 дня.</div>
    </div>
  );
}
