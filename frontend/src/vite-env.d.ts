/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_URL?: string;
  readonly DEV?: boolean;
  readonly VITE_PROXY_TARGET?: string;
  readonly VITE_USE_MOCK?: string;
  readonly VITE_ADMIN_USERNAME?: string;
  readonly VITE_TELEGRAM_BOT_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
