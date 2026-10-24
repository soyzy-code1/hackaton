import { useEffect, useState } from 'react';
import type { FormEvent } from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { ReceiptText } from 'lucide-react';
import { useAuthStore } from '@/store/auth';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';

interface LocationState {
  from?: string;
}

export function LoginPage() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [validationError, setValidationError] = useState<string | null>(null);

  const token = useAuthStore((state) => state.token);
  const isInitializing = useAuthStore((state) => state.isInitializing);
  const isSubmitting = useAuthStore((state) => state.isSubmitting);
  const error = useAuthStore((state) => state.error);
  const login = useAuthStore((state) => state.login);
  const clearError = useAuthStore((state) => state.clearError);

  const location = useLocation();
  const state = location.state as LocationState | null;

  useEffect(() => {
    return () => {
      clearError();
    };
  }, [clearError]);

  if (!isInitializing && token) {
    return <Navigate to={state?.from ?? '/'} replace />;
  }

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setValidationError(null);
    clearError();

    if (!username.trim() || !password) {
      setValidationError('Заполните логин и пароль');
      return;
    }

    await login({ username: username.trim(), password });
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-50 px-4 py-12">
      <div className="w-full max-w-sm">
        <div className="mb-6 flex flex-col items-center gap-3">
          <div className="flex h-11 w-11 items-center justify-center rounded-lg border border-gray-200 bg-white">
            <ReceiptText className="h-5 w-5 text-blue-600" aria-hidden="true" />
          </div>
          <div className="text-center">
            <h1 className="text-lg font-semibold text-gray-900">Invoice Keeper</h1>
            <p className="mt-1 text-sm text-gray-500">Войдите в аккаунт</p>
          </div>
        </div>

        <div className="card p-6">
          <form onSubmit={handleSubmit} className="space-y-4" noValidate>
            <Input
              label="Логин"
              type="text"
              name="username"
              autoComplete="username"
              placeholder="Введите логин"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              disabled={isSubmitting}
            />

            <Input
              label="Пароль"
              type="password"
              name="password"
              autoComplete="current-password"
              placeholder="Введите пароль"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              disabled={isSubmitting}
            />

            {validationError ? (
              <p className="text-sm text-red-600" role="alert">
                {validationError}
              </p>
            ) : null}

            {error ? (
              <div
                className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700"
                role="alert"
              >
                {error}
              </div>
            ) : null}

            <Button type="submit" fullWidth isLoading={isSubmitting}>
              Войти
            </Button>
          </form>
        </div>
      </div>
    </div>
  );
}
