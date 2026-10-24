import { create } from 'zustand';
import { api, ApiError } from '@/api/client';
import type { LoginPayload, User } from '@/api/types';

const USER_COOKIE_KEY = 'invoice_keeper_user';
const COOKIE_MAX_AGE_SECONDS = 60 * 60 * 24;

function cookiePath(): string {
  return 'Path=/';
}

function readCookie(name: string): string | null {
  const prefix = `${encodeURIComponent(name)}=`;
  const item = document.cookie.split('; ').find((value) => value.startsWith(prefix));
  return item ? decodeURIComponent(item.slice(prefix.length)) : null;
}

function writeCookie(name: string, value: string): void {
  document.cookie = `${encodeURIComponent(name)}=${encodeURIComponent(value)}; Max-Age=${COOKIE_MAX_AGE_SECONDS}; ${cookiePath()}; SameSite=Lax`;
}

function removeCookie(name: string): void {
  document.cookie = `${encodeURIComponent(name)}=; Max-Age=0; ${cookiePath()}; SameSite=Lax`;
}

interface StoredUser {
  id: string;
  username: string;
  role: User['role'];
}

function readStoredUser(): User | null {
  const raw = readCookie(USER_COOKIE_KEY);
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw) as StoredUser;
    if (
      parsed &&
      typeof parsed.id === 'string' &&
      typeof parsed.username === 'string' &&
      (parsed.role === 'admin' || parsed.role === 'user')
    ) {
      return { id: parsed.id, username: parsed.username, role: parsed.role };
    }
    return null;
  } catch {
    return null;
  }
}

// This is only a non-sensitive UI cache. The server-side `/auth/me` response
// remains authoritative, but the cache prevents a transient API/proxy failure
// during a reload from flashing the login page for an already signed-in user.
const cachedUser = readStoredUser();

function persistUser(user: User): void {
  writeCookie(USER_COOKIE_KEY, JSON.stringify(user));
}

export function clearSession(): void {
  removeCookie(USER_COOKIE_KEY);
}

export interface AuthState {
  user: User | null;
  /** True until the initial session restore / `me` request finishes. */
  isInitializing: boolean;
  isSubmitting: boolean;
  error: string | null;

  login: (payload: LoginPayload) => Promise<boolean>;
  logout: () => Promise<void>;
  restoreSession: () => Promise<void>;
  clearError: () => void;
}

let restorePromise: Promise<void> | null = null;

export const useAuthStore = create<AuthState>((set) => ({
  user: cachedUser,
  isInitializing: true,
  isSubmitting: false,
  error: null,

  login: async (payload) => {
    set({ isSubmitting: true, error: null });
    try {
      const response = await api.auth.login(payload);
      persistUser(response.user);
      set({ user: response.user, isSubmitting: false, error: null });
      return true;
    } catch (error) {
      const message =
        error instanceof ApiError
          ? error.message
          : error instanceof Error
            ? error.message
            : 'Не удалось выполнить вход';
      set({ isSubmitting: false, error: message });
      return false;
    }
  },

  logout: async () => {
    try {
      await api.auth.logout();
    } finally {
      clearSession();
      set({ user: null, error: null });
    }
  },

  restoreSession: async () => {
    // StrictMode mounts effects twice: make the restore idempotent.
    if (restorePromise) return restorePromise;

    restorePromise = (async () => {
      try {
        const user = await api.auth.me();
        persistUser(user);
        set({ user, isInitializing: false, error: null });
      } catch (error) {
        // A 401/403 means the HttpOnly session is genuinely invalid. For
        // network errors and temporary 5xx responses, keep the cached user so
        // a reload does not incorrectly redirect an authenticated user to
        // /login while the API is restarting or the proxy is reconnecting.
        const isAuthFailure =
          error instanceof ApiError && (error.status === 401 || error.status === 403);
        if (isAuthFailure) {
          clearSession();
          set({ user: null, isInitializing: false, error: null });
        } else {
          set({ user: cachedUser, isInitializing: false, error: null });
        }
      }
    })();

    try {
      await restorePromise;
    } finally {
      restorePromise = null;
    }
  },

  clearError: () => set({ error: null }),
}));
