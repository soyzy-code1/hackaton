import type { ReactNode } from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { useAuthStore } from '@/store/auth';
import { Layout } from '@/components/Layout';
import { ErrorBoundary } from '@/components/ErrorBoundary';

interface RequireAuthProps {
  children: ReactNode;
  /** When set, the route is only accessible to users with this role. */
  role?: 'admin' | 'user';
}

function FullPageLoader() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-50">
      <div className="flex items-center gap-3 text-sm text-gray-500">
        <span className="h-4 w-4 animate-spin rounded-full border-2 border-gray-300 border-t-blue-600" />
        Загрузка…
      </div>
    </div>
  );
}

/**
 * Route guard: requires an authenticated session and (optionally) a role.
 * Users without access are redirected to the dashboard.
 */
export function RequireAuth({ children, role }: RequireAuthProps) {
  const user = useAuthStore((state) => state.user);
  const isInitializing = useAuthStore((state) => state.isInitializing);
  const location = useLocation();

  if (isInitializing) return <FullPageLoader />;

  if (!user) {
    return <Navigate to="/login" state={{ from: location.pathname }} replace />;
  }

  if (role && user.role !== role) {
    return <Navigate to="/" replace />;
  }

  return (
    <Layout>
      <ErrorBoundary resetKey={location.pathname}>{children}</ErrorBoundary>
    </Layout>
  );
}
