import { useEffect, useState } from 'react';
import type { ReactNode } from 'react';
import { NavLink, useNavigate } from 'react-router-dom';
import {
  BarChart3,
  Bell,
  CalendarClock,
  ClipboardList,
  LayoutDashboard,
  LogOut,
  Menu,
  ReceiptText,
  Settings,
  Users,
  X,
  MessageCircleQuestion,
} from 'lucide-react';
import { isUsingMockBackend } from '@/api/client';
import { ROLE_LABELS } from '@/api/types';
import { useAuthStore } from '@/store/auth';
import { Button } from './ui/Button';

interface NavItem {
  to: string;
  label: string;
  icon: ReactNode;
  adminOnly?: boolean;
  userOnly?: boolean;
  end?: boolean;
}

const NAV_ITEMS: NavItem[] = [
  { to: '/', label: 'Дашборд', icon: <LayoutDashboard className="h-4 w-4" aria-hidden="true" />, end: true },
  { to: '/calendar', label: 'Календарь', icon: <CalendarClock className="h-4 w-4" aria-hidden="true" /> },
  { to: '/analytics', label: 'Аналитика', icon: <BarChart3 className="h-4 w-4" aria-hidden="true" /> },
  { to: '/notifications', label: 'Уведомления', icon: <Bell className="h-4 w-4" aria-hidden="true" /> },
  { to: '/assistant', label: 'Помощь ИИ', icon: <MessageCircleQuestion className="h-4 w-4" aria-hidden="true" />, userOnly: true },
  { to: '/recurring', label: 'Платежи', icon: <ReceiptText className="h-4 w-4" aria-hidden="true" />, adminOnly: true },
  { to: '/users', label: 'Пользователи', icon: <Users className="h-4 w-4" aria-hidden="true" />, adminOnly: true },
  { to: '/audit-log', label: 'Журнал действий', icon: <ClipboardList className="h-4 w-4" aria-hidden="true" />, adminOnly: true },
  { to: '/settings', label: 'Настройки', icon: <Settings className="h-4 w-4" aria-hidden="true" /> },
];

export function Layout({ children }: { children: ReactNode }) {
  const user = useAuthStore((state) => state.user);
  const logout = useAuthStore((state) => state.logout);
  const navigate = useNavigate();
  const [isMenuOpen, setIsMenuOpen] = useState(false);
  const [isLoggingOut, setIsLoggingOut] = useState(false);

  const isAdmin = user?.role === 'admin';
  const visibleItems = NAV_ITEMS.filter((item) => (!item.adminOnly || isAdmin) && (!item.userOnly || !isAdmin));

  // The mock backend is only detected after the first request, so the badge
  // is refreshed on every navigation.
  const [isMock, setIsMock] = useState(isUsingMockBackend());
  useEffect(() => {
    setIsMock(isUsingMockBackend());
  });

  const handleLogout = async () => {
    setIsLoggingOut(true);
    await logout();
    setIsLoggingOut(false);
    navigate('/login', { replace: true });
  };

  const handleNavigate = () => setIsMenuOpen(false);

  const sidebarContent = (
    <>
      <div className="flex h-16 items-center gap-2 border-b border-gray-200 px-5">
        <ReceiptText className="h-5 w-5 text-blue-600" aria-hidden="true" />
        <span className="text-base font-semibold tracking-tight text-gray-900">
          Invoice Keeper
        </span>
      </div>

      {isMock ? (
        <div className="border-b border-yellow-200 bg-yellow-50 px-5 py-2 text-xs text-yellow-700">
          Демо-режим: бэкенд недоступен, используются мок-данные
        </div>
      ) : null}

      <nav className="flex-1 space-y-1 p-3" aria-label="Основная навигация">
        {visibleItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.end}
            onClick={handleNavigate}
            className={({ isActive }) =>
              [
                'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors',
                isActive
                  ? 'bg-blue-50 text-blue-700'
                  : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900',
              ].join(' ')
            }
          >
            {item.icon}
            {item.label}
          </NavLink>
        ))}
      </nav>

      <div className="border-t border-gray-200 p-3">
        <Button
          variant="secondary"
          fullWidth
          isLoading={isLoggingOut}
          onClick={() => void handleLogout()}
          leftIcon={<LogOut className="h-4 w-4" aria-hidden="true" />}
        >
          Выйти
        </Button>
      </div>
    </>
  );

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Mobile top bar */}
      <header className="sticky top-0 z-30 flex h-16 items-center justify-between border-b border-gray-200 bg-white px-4 lg:hidden">
        <div className="flex items-center gap-2">
          <ReceiptText className="h-5 w-5 text-blue-600" aria-hidden="true" />
          <span className="text-base font-semibold text-gray-900">Invoice Keeper</span>
        </div>
        <button
          type="button"
          onClick={() => setIsMenuOpen((open) => !open)}
          className="rounded-md p-2 text-gray-600 hover:bg-gray-100"
          aria-label={isMenuOpen ? 'Закрыть меню' : 'Открыть меню'}
          aria-expanded={isMenuOpen}
        >
          {isMenuOpen ? (
            <X className="h-5 w-5" aria-hidden="true" />
          ) : (
            <Menu className="h-5 w-5" aria-hidden="true" />
          )}
        </button>
      </header>

      {/* Mobile drawer */}
      {isMenuOpen ? (
        <div className="fixed inset-0 z-40 lg:hidden">
          <div
            className="absolute inset-0 bg-gray-900/40"
            onClick={() => setIsMenuOpen(false)}
            role="presentation"
          />
          <aside className="relative z-10 flex h-full w-72 max-w-[85vw] flex-col overflow-y-auto border-r border-gray-200 bg-white">
            {sidebarContent}
          </aside>
        </div>
      ) : null}

      {/* Desktop sidebar */}
      <aside className="fixed inset-y-0 left-0 z-30 hidden w-64 flex-col border-r border-gray-200 bg-white lg:flex">
        {sidebarContent}
      </aside>

      <div className="min-w-0 lg:pl-64">
        <header className="sticky top-0 z-20 hidden h-16 items-center justify-end border-b border-gray-200 bg-white px-6 lg:flex">
          <div className="min-w-0 text-right">
            <p className="max-w-xs truncate text-sm font-medium text-gray-900">{user?.username ?? '—'}</p>
            <p className="text-xs text-gray-500">
              {user ? ROLE_LABELS[user.role] : ''}
            </p>
          </div>
        </header>

        <main className="px-3 py-5 sm:px-6 sm:py-6 lg:px-8">
          <div className="mx-auto w-full min-w-0 max-w-7xl">{children}</div>
        </main>
      </div>
    </div>
  );
}
