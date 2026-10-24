import { create } from 'zustand';
import { api, ApiError } from '@/api/client';
import type { LoginPayload, User } from '@/api/types';

const TOKEN_COOKIE_KEY = 'invoice_keeper_token';
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

function persistToken(token: string): void {
  writeCookie(TOKEN_COOKIE_KEY, token);
}

function persistUser(user: User): void {
  writeCookie(USER_COOKIE_KEY, JSON.stringify(user));
}

export function clearSession(): void {
  removeCookie(TOKEN_COOKIE_KEY);
  removeCookie(USER_COOKIE_KEY);
}

export interface AuthState {
  token: string | null;
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
  token: readCookie(TOKEN_COOKIE_KEY),
  user: readStoredUser(),
  isInitializing: true,
  isSubmitting: false,
  error: null,

  login: async (payload) => {
    set({ isSubmitting: true, error: null });
    try {
      const response = await api.auth.login(payload);
      persistToken(response.token);
      persistUser(response.user);
      set({ token: response.token, user: response.user, isSubmitting: false, error: null });
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
      set({ token: null, user: null, error: null });
    }
  },

  restoreSession: async () => {
    // StrictMode mounts effects twice: make the restore idempotent.
    if (restorePromise) return restorePromise;

    restorePromise = (async () => {
      const token = readCookie(TOKEN_COOKIE_KEY);
      if (!token) {
        set({ token: null, user: null, isInitializing: false });
        return;
      }
      try {
        const user = await api.auth.me();
        persistUser(user);
        set({ token, user, isInitializing: false, error: null });
      } catch {
        // An expired/invalid token must not block the app: drop the session.
        clearSession();
        set({ token: null, user: null, isInitializing: false, error: null });
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
