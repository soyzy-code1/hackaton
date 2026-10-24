import type { Currency } from '@/api/types';
import { DEFAULT_TIMEZONE } from '@/lib/date';
import { create } from 'zustand';

export type Theme = 'light' | 'dark';
export type BudgetPeriod = 'month' | 'year';

export interface BudgetSettings { RUB: number; USD: number; EUR: number; }
export interface ActivityEntry { id: string; action: string; details: string; timestamp: string; }

interface SettingsState {
  reminderLeadDays: number;
  timeZone: string;
  displayCurrency: Currency;
  notificationsEnabled: boolean;
  weeklyDigestEnabled: boolean;
  theme: Theme;
  budget: BudgetSettings;
  yearlyBudget: BudgetSettings;
  budgetPeriod: BudgetPeriod;
  cashBalance: BudgetSettings;
  activity: ActivityEntry[];
  setSettings: (patch: Partial<Omit<SettingsState, 'activity' | 'setSettings' | 'addActivity' | 'resetSettings' | 'setBudget' | 'setYearlyBudget' | 'setCashBalance'>>) => void;
  setBudget: (currency: Currency, value: number) => void;
  setYearlyBudget: (currency: Currency, value: number) => void;
  setCashBalance: (currency: Currency, value: number) => void;
  addActivity: (action: string, details: string) => void;
  resetSettings: () => void;
}

const STORAGE_KEY = 'invoice_keeper_settings';
const DEFAULT_SETTINGS = {
  reminderLeadDays: 3,
  timeZone: DEFAULT_TIMEZONE,
  displayCurrency: 'RUB' as Currency,
  notificationsEnabled: true,
  weeklyDigestEnabled: false,
  theme: 'light' as Theme,
  budgetPeriod: 'month' as BudgetPeriod,
  budget: { RUB: 100000, USD: 500, EUR: 500 },
  yearlyBudget: { RUB: 1_200_000, USD: 6_000, EUR: 6_000 },
  cashBalance: { RUB: 150000, USD: 1000, EUR: 1000 },
  activity: [] as ActivityEntry[],
};

type PersistedSettings = Partial<typeof DEFAULT_SETTINGS>;

function readSettings(): typeof DEFAULT_SETTINGS {
  const raw = localStorage.getItem(STORAGE_KEY);
  if (!raw) return DEFAULT_SETTINGS;
  try {
    const parsed = JSON.parse(raw) as PersistedSettings;
    return { ...DEFAULT_SETTINGS, ...parsed, budget: { ...DEFAULT_SETTINGS.budget, ...parsed.budget }, yearlyBudget: { ...DEFAULT_SETTINGS.yearlyBudget, ...parsed.yearlyBudget }, cashBalance: { ...DEFAULT_SETTINGS.cashBalance, ...parsed.cashBalance }, activity: Array.isArray(parsed.activity) ? parsed.activity.slice(0, 100) : [] };
  } catch { return DEFAULT_SETTINGS; }
}

function persist(state: SettingsState): void {
  const { activity, setSettings, setBudget, setYearlyBudget, setCashBalance, addActivity, resetSettings, ...data } = state;
  void setSettings; void setBudget; void setYearlyBudget; void setCashBalance; void addActivity; void resetSettings;
  localStorage.setItem(STORAGE_KEY, JSON.stringify({ ...data, activity }));
}

export const useSettingsStore = create<SettingsState>((set) => ({
  ...readSettings(),
  setSettings: (patch) => set((state) => { const next = { ...state, ...patch }; persist(next); return next; }),
  setBudget: (currency, value) => set((state) => { const next = { ...state, budget: { ...state.budget, [currency]: Math.max(0, value) } }; persist(next); return next; }),
  setYearlyBudget: (currency, value) => set((state) => { const next = { ...state, yearlyBudget: { ...state.yearlyBudget, [currency]: Math.max(0, value) } }; persist(next); return next; }),
  setCashBalance: (currency, value) => set((state) => { const next = { ...state, cashBalance: { ...state.cashBalance, [currency]: Math.max(0, value) } }; persist(next); return next; }),
  addActivity: (action, details) => set((state) => { const entry = { id: `${Date.now()}-${Math.random().toString(36).slice(2)}`, action, details, timestamp: new Date().toISOString() }; const next = { ...state, activity: [entry, ...state.activity].slice(0, 100) }; persist(next); return next; }),
  resetSettings: () => { localStorage.removeItem(STORAGE_KEY); set(DEFAULT_SETTINGS); },
}));

export function applyTheme(theme: Theme): void {
  document.documentElement.classList.toggle('dark', theme === 'dark');
  document.documentElement.style.colorScheme = theme;
}

export interface TimeZoneOption { value: string; label: string; search: string; }

export const TIMEZONE_OPTIONS: readonly TimeZoneOption[] = [
  { value: 'Europe/Kaliningrad', label: 'Калининградская область — UTC+2', search: 'калининград калининградская область kaliningrad utc+2 +2' },
  { value: 'Europe/Moscow', label: 'Москва, Санкт-Петербург, Центральная Россия — UTC+3', search: 'москва санкт-петербург петербург центральная россия moskva moscow spb utc+3 +3' },
  { value: 'Europe/Kirov', label: 'Кировская область — UTC+3', search: 'киров кировская область kirov utc+3 +3' },
  { value: 'Europe/Volgograd', label: 'Волгоградская область — UTC+3', search: 'волгоград волгоградская область volgograd utc+3 +3' },
  { value: 'Europe/Samara', label: 'Самарская область, Удмуртия — UTC+4', search: 'самара тольятти удмуртия ижевск samara izhevsk utc+4 +4' },
  { value: 'Europe/Astrakhan', label: 'Астраханская область — UTC+4', search: 'астрахань астраханская область astrakhan utc+4 +4' },
  { value: 'Europe/Saratov', label: 'Саратовская область — UTC+4', search: 'саратов саратовская область saratov utc+4 +4' },
  { value: 'Europe/Ulyanovsk', label: 'Ульяновская область — UTC+4', search: 'ульяновск ульяновская область ulyanovsk utc+4 +4' },
  { value: 'Asia/Yekaterinburg', label: 'Урал: Екатеринбург, Челябинск, Тюмень — UTC+5', search: 'екатеринбург челябинск тюмень урал yekaterinburg chelyabinsk tyumen utc+5 +5' },
  { value: 'Asia/Omsk', label: 'Омская область — UTC+6', search: 'омск омская область omsk utc+6 +6' },
  { value: 'Asia/Novosibirsk', label: 'Новосибирская область — UTC+7', search: 'новосибирск новосибирская область сибирь novosibirsk utc+7 +7' },
  { value: 'Asia/Barnaul', label: 'Алтайский край, Республика Алтай — UTC+7', search: 'барнаул алтай алтайский край горно-алтайск barnaul altai utc+7 +7' },
  { value: 'Asia/Tomsk', label: 'Томская область — UTC+7', search: 'томск томская область tomsk utc+7 +7' },
  { value: 'Asia/Novokuznetsk', label: 'Кемеровская область — UTC+7', search: 'кемерово новокузнецк кузбасс kemerovo novokuznetsk utc+7 +7' },
  { value: 'Asia/Krasnoyarsk', label: 'Красноярск, Абакан — UTC+7', search: 'красноярск абакан хакасия красноярский край krasnoyarsk abakan utc+7 +7' },
  { value: 'Asia/Irkutsk', label: 'Иркутск, Улан-Удэ — UTC+8', search: 'иркутск улан-удэ байкал бурятия irkutsk ulan ude utc+8 +8' },
  { value: 'Asia/Chita', label: 'Забайкальский край — UTC+9', search: 'чита забайкалье забайкальский край chita utc+9 +9' },
  { value: 'Asia/Yakutsk', label: 'Якутск, Амурская область — UTC+9', search: 'якутск якутия амурская область благовещенск yakutsk yakutia blagoveshchensk utc+9 +9' },
  { value: 'Asia/Khandyga', label: 'Хандыга (Томпонский район Якутии) — UTC+9', search: 'хандыга томпонский якутия khandyga utc+9 +9' },
  { value: 'Asia/Vladivostok', label: 'Владивосток, Хабаровск, Приморье — UTC+10', search: 'владивосток хабаровск приморье дальний восток vladivostok khabarovsk utc+10 +10' },
  { value: 'Asia/Ust-Nera', label: 'Верхоянье, Усть-Нера — UTC+10', search: 'усть-нера верхоянск магадан ust nera verkhoyansk utc+10 +10' },
  { value: 'Asia/Magadan', label: 'Магаданская область — UTC+11', search: 'магадан магаданская область magadan utc+11 +11' },
  { value: 'Asia/Sakhalin', label: 'Сахалин — UTC+11', search: 'сахалин южно-сахалинск sakhalin yuzhno sakhalinsk utc+11 +11' },
  { value: 'Asia/Srednekolymsk', label: 'Среднеколымск, Колыма — UTC+11', search: 'среднеколымск колыма srednekolymsk kolyma utc+11 +11' },
  { value: 'Asia/Kamchatka', label: 'Камчатка, Петропавловск-Камчатский — UTC+12', search: 'камчатка петропавловск-камчатский kamchatka petropavlovsk utc+12 +12' },
  { value: 'Asia/Anadyr', label: 'Чукотка, Анадырь — UTC+12', search: 'чукотка анадырь anadyr chukotka utc+12 +12' },
];
