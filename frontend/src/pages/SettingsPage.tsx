import { useEffect, useMemo, useState } from 'react';
import { Bell, Moon, RotateCcw, Save, ShieldCheck, Sun } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Select } from '@/components/ui/Select';
import { applyTheme, TIMEZONE_OPTIONS, useSettingsStore } from '@/store/settings';
import { CURRENCIES, CURRENCY_SYMBOLS } from '@/api/types';
import { formatDate } from '@/lib/date';

function timeZoneLabel(value: string): string {
  return TIMEZONE_OPTIONS.find((option) => option.value === value)?.label ?? value;
}

export function SettingsPage() {
  const settings = useSettingsStore();
  const [saved, setSaved] = useState(false);
  const selectedTimeZoneLabel = timeZoneLabel(settings.timeZone);
  const [timeZoneQuery, setTimeZoneQuery] = useState(selectedTimeZoneLabel);
  const [showTimeZoneSuggestions, setShowTimeZoneSuggestions] = useState(false);

  useEffect(() => applyTheme(settings.theme), [settings.theme]);
  useEffect(() => setTimeZoneQuery(selectedTimeZoneLabel), [selectedTimeZoneLabel]);

  const timeZoneSuggestions = useMemo(() => {
    const query = timeZoneQuery.trim().toLocaleLowerCase();
    // Пока в поле стоит выбранное значение (или пусто) — показываем весь справочник.
    if (!query || timeZoneQuery === selectedTimeZoneLabel) return TIMEZONE_OPTIONS;
    return TIMEZONE_OPTIONS.filter((option) => `${option.value} ${option.label} ${option.search}`.toLocaleLowerCase().includes(query));
  }, [timeZoneQuery, selectedTimeZoneLabel]);

  const chooseTimeZone = (value: string) => {
    settings.setSettings({ timeZone: value });
    setTimeZoneQuery(timeZoneLabel(value));
    setShowTimeZoneSuggestions(false);
  };

  const save = () => {
    settings.addActivity('Изменены настройки', 'Настройки интерфейса, часового пояса и уведомлений');
    setSaved(true);
    setTimeout(() => setSaved(false), 1800);
  };

  return (
    <div className="space-y-6">
      <div><h1 className="text-xl font-semibold text-gray-900">Настройки</h1><p className="text-sm text-gray-500">Часовой пояс, напоминания и персональные предпочтения</p></div>
      <div className="grid gap-6 lg:grid-cols-2">
        <section className="card p-5"><div className="mb-5 flex items-center gap-2"><Bell className="h-4 w-4 text-blue-600"/><h2 className="text-sm font-semibold text-gray-900">Уведомления</h2></div><div className="space-y-4"><Input label="Напоминать за (дней)" value={String(settings.reminderLeadDays)} onChange={(event) => settings.setSettings({ reminderLeadDays: Number(event.target.value) })}/><label className="flex items-center gap-2 text-sm text-gray-700"><input type="checkbox" checked={settings.notificationsEnabled} onChange={(event) => settings.setSettings({ notificationsEnabled: event.target.checked })} className="h-4 w-4 text-blue-600"/>Включить уведомления</label><label className="flex items-center gap-2 text-sm text-gray-700"><input type="checkbox" checked={settings.weeklyDigestEnabled} onChange={(event) => settings.setSettings({ weeklyDigestEnabled: event.target.checked })} className="h-4 w-4 text-blue-600"/>Еженедельный дайджест</label></div></section>
        <section className="card p-5"><div className="mb-5 flex items-center gap-2"><ShieldCheck className="h-4 w-4 text-blue-600"/><h2 className="text-sm font-semibold text-gray-900">Региональные настройки</h2></div><div className="space-y-4">
          <div className="relative"><label className="field-label" htmlFor="timezone-input">Часовой пояс</label><input id="timezone-input" value={timeZoneQuery} title={settings.timeZone} onFocus={(event) => { event.target.select(); setShowTimeZoneSuggestions(true); }} onChange={(event) => { setTimeZoneQuery(event.target.value); setShowTimeZoneSuggestions(true); }} onBlur={() => setTimeout(() => { setShowTimeZoneSuggestions(false); setTimeZoneQuery(timeZoneLabel(useSettingsStore.getState().timeZone)); }, 150)} className="w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm font-medium text-gray-900 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100" placeholder="Начните вводить город, регион или UTC+3" autoComplete="off"/>{showTimeZoneSuggestions && timeZoneSuggestions.length > 0 && <div className="absolute z-20 mt-1 max-h-64 w-full overflow-auto rounded-md border border-gray-200 bg-white py-1 shadow-lg">{timeZoneSuggestions.map((option) => <button type="button" key={option.value} onMouseDown={() => chooseTimeZone(option.value)} className={`block w-full px-3 py-2 text-left text-sm text-gray-700 hover:bg-blue-50 ${option.value === settings.timeZone ? 'bg-blue-50' : ''}`}><span className="font-medium">{option.label}</span></button>)}</div>}</div>
          <Select label="Основная валюта отображения" options={CURRENCIES.map((currency) => ({ value: currency, label: `${CURRENCY_SYMBOLS[currency]} ${currency}` }))} value={settings.displayCurrency} onChange={(event) => settings.setSettings({ displayCurrency: event.target.value as typeof settings.displayCurrency })}/>
          <div><p className="field-label">Тема интерфейса</p><div className="flex gap-2"><Button type="button" variant={settings.theme === 'light' ? 'primary' : 'secondary'} onClick={() => settings.setSettings({ theme: 'light' })} leftIcon={<Sun className="h-4 w-4"/>}>Светлая</Button><Button type="button" variant={settings.theme === 'dark' ? 'primary' : 'secondary'} onClick={() => settings.setSettings({ theme: 'dark' })} leftIcon={<Moon className="h-4 w-4"/>}>Тёмная</Button></div></div>
        </div></section>
      </div>
      <div className="flex gap-2"><Button type="button" onClick={save} leftIcon={<Save className="h-4 w-4"/>}>{saved ? 'Сохранено' : 'Сохранить настройки'}</Button><Button type="button" variant="secondary" onClick={settings.resetSettings} leftIcon={<RotateCcw className="h-4 w-4"/>}>Сбросить</Button></div>
      <section className="card overflow-hidden"><div className="border-b border-gray-200 px-5 py-4"><h2 className="text-sm font-semibold text-gray-900">Журнал действий</h2></div>{settings.activity.length === 0 ? <p className="px-5 py-8 text-sm text-gray-500">Действий пока нет</p> : settings.activity.slice(0, 10).map((item) => <div key={item.id} className="flex justify-between gap-4 border-b border-gray-100 px-5 py-3 text-sm last:border-0"><div><p className="font-medium text-gray-800">{item.action}</p><p className="text-xs text-gray-500">{item.details}</p></div><time className="whitespace-nowrap text-xs text-gray-400">{formatDate(item.timestamp)}</time></div>)}</section>
    </div>
  );
}
