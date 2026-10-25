import { Suspense, lazy, useEffect } from 'react';
import type { ReactNode } from 'react';
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import { RequireAuth } from '@/components/RequireAuth';
import { LoginPage } from '@/pages/LoginPage';
import { NotFoundPage } from '@/pages/NotFoundPage';
import { useAuthStore } from '@/store/auth';

/*
 * Pages are code-split: Chart.js (used by the dashboard and analytics) is
 * only downloaded after login, so the login screen stays light.
 */
const DashboardPage = lazy(() =>
  import('@/pages/DashboardPage').then((m) => ({ default: m.DashboardPage })),
);
const AnalyticsPage = lazy(() =>
  import('@/pages/AnalyticsPage').then((m) => ({ default: m.AnalyticsPage })),
);
const CalendarPage = lazy(() =>
  import('@/pages/CalendarPage').then((m) => ({ default: m.CalendarPage })),
);
const NotificationsPage = lazy(() =>
  import('@/pages/NotificationsPage').then((m) => ({ default: m.NotificationsPage })),
);
const SettingsPage = lazy(() =>
  import('@/pages/SettingsPage').then((m) => ({ default: m.SettingsPage })),
);
const RecurringPaymentsPage = lazy(() =>
  import('@/pages/RecurringPaymentsPage').then((m) => ({ default: m.RecurringPaymentsPage })),
);
const UsersPage = lazy(() =>
  import('@/pages/UsersPage').then((m) => ({ default: m.UsersPage })),
);
const AuditLogPage = lazy(() =>
  import('@/pages/AuditLogPage').then((m) => ({ default: m.AuditLogPage })),
);
const AssistantPage = lazy(() =>
  import('@/pages/AssistantPage').then((m) => ({ default: m.AssistantPage })),
);
const SubscriptionAnalysisPage = lazy(() =>
  import('@/pages/SubscriptionAnalysisPage').then((m) => ({ default: m.SubscriptionAnalysisPage })),
);

function PageLoader() {
  return (
    <div className="flex items-center justify-center gap-3 py-24 text-sm text-gray-500">
      <span className="h-4 w-4 animate-spin rounded-full border-2 border-gray-300 border-t-blue-600" />
      Загрузка…
    </div>
  );
}

function Protected({ children, role }: { children: ReactNode; role?: 'admin' | 'user' }) {
  return (
    <RequireAuth role={role}>
      <Suspense fallback={<PageLoader />}>{children}</Suspense>
    </RequireAuth>
  );
}

export function App() {
  const restoreSession = useAuthStore((state) => state.restoreSession);

  return (
    <BrowserRouter>
      <SessionRestorer onRestore={restoreSession} />
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/" element={<Protected><DashboardPage /></Protected>} />
        <Route path="/calendar" element={<Protected><CalendarPage /></Protected>} />
        <Route path="/analytics" element={<Protected><AnalyticsPage /></Protected>} />
        <Route path="/notifications" element={<Protected><NotificationsPage /></Protected>} />
        <Route path="/settings" element={<Protected><SettingsPage /></Protected>} />
        <Route path="/recurring" element={<Protected role="admin"><RecurringPaymentsPage /></Protected>} />
        <Route path="/users" element={<Protected role="admin"><UsersPage /></Protected>} />
        <Route path="/audit-log" element={<Protected role="admin"><AuditLogPage /></Protected>} />
        <Route path="/assistant" element={<Protected role="user"><AssistantPage /></Protected>} />
        <Route path="/subscription-analysis" element={<Protected role="user"><SubscriptionAnalysisPage /></Protected>} />
        <Route path="/404" element={<NotFoundPage />} />
        <Route path="*" element={<Navigate to="/404" replace />} />
      </Routes>
    </BrowserRouter>
  );
}

/** Restores the persisted session once, before protected routes render. */
function SessionRestorer({ onRestore }: { onRestore: () => Promise<void> }) {
  const isInitializing = useAuthStore((state) => state.isInitializing);

  useEffect(() => {
    if (isInitializing) {
      void onRestore();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return null;
}
