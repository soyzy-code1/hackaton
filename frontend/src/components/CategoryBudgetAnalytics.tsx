import { useEffect, useMemo, useState } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import { api } from '@/api/client';
import type { CategoryAnalytics, Currency } from '@/api/types';
import { CURRENCIES, CURRENCY_SYMBOLS, PAYMENT_CATEGORY_LABELS } from '@/api/types';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';

export function CategoryBudgetAnalytics() {
  const [currency, setCurrency] = useState<Currency>('RUB');
  const [analytics, setAnalytics] = useState<CategoryAnalytics | null>(null);
  const [categories, setCategories] = useState<string[]>([]);
  const [newCategory, setNewCategory] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = async () => {
    setLoading(true); setError(null);
    try {
      const [result, names] = await Promise.all([api.analytics.categories(currency), api.categories.list()]);
      setAnalytics(result); setCategories(names);
    } catch (reason) { setError(reason instanceof Error ? reason.message : 'Не удалось загрузить категории'); }
    finally { setLoading(false); }
  };
  useEffect(() => { void load(); }, [currency]);

  const total = useMemo(() => analytics?.items.reduce((sum, item) => sum + item.amount, 0) ?? 0, [analytics]);
  const add = async () => { const name = newCategory.trim(); if (!name) return; await api.categories.create(name); setNewCategory(''); void load(); };
  const remove = async (name: string) => { await api.categories.remove(name); void load(); };

  return <div className="space-y-4">
    <section className="card p-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div><h2 className="text-sm font-semibold text-gray-900">Расходы по категориям</h2><p className="text-xs text-gray-500">Плановые активные платежи</p></div>
        <div className="flex gap-2">{CURRENCIES.map((item) => <button key={item} type="button" onClick={() => setCurrency(item)} className={['rounded-md border px-3 py-1.5 text-sm', currency === item ? 'border-blue-600 bg-blue-50 text-blue-700' : 'border-gray-200 text-gray-600'].join(' ')}>{CURRENCY_SYMBOLS[item]}</button>)}</div>
      </div>
      {error ? <p className="mt-3 text-sm text-red-600">{error}</p> : null}
      <div className="mt-4 space-y-3">{loading ? <p className="text-sm text-gray-500">Загрузка…</p> : analytics?.items.length ? analytics.items.map((item) => <div key={item.category}><div className="flex justify-between text-sm"><span>{PAYMENT_CATEGORY_LABELS[item.category as keyof typeof PAYMENT_CATEGORY_LABELS] ?? item.category}</span><span className="font-medium">{item.amount.toLocaleString('ru-RU')} {CURRENCY_SYMBOLS[currency]}</span></div><div className="mt-1 h-2 rounded bg-gray-100"><div className="h-2 rounded bg-blue-600" style={{ width: `${total ? Math.max(2, item.amount / total * 100) : 0}%` }} /></div><p className="mt-1 text-xs text-gray-500">{item.count} платеж(ей)</p></div>) : <p className="text-sm text-gray-500">Нет данных</p>}</div>
    </section>
    <section className="card p-5"><h2 className="text-sm font-semibold text-gray-900">Категории платежей</h2><div className="mt-3 flex gap-2"><Input label="Новая категория" value={newCategory} onChange={(event) => setNewCategory(event.target.value)} /><Button type="button" className="mt-6" onClick={() => void add()} leftIcon={<Plus className="h-4 w-4" />}>Добавить</Button></div><div className="mt-3 flex flex-wrap gap-2">{categories.map((name) => <span key={name} className="inline-flex items-center gap-2 rounded-md border border-gray-200 px-2 py-1 text-xs">{PAYMENT_CATEGORY_LABELS[name as keyof typeof PAYMENT_CATEGORY_LABELS] ?? name}{name !== 'other' ? <button type="button" onClick={() => void remove(name)} aria-label={`Удалить ${name}`}><Trash2 className="h-3 w-3 text-red-500" /></button> : null}</span>)}</div></section>
  </div>;
}
