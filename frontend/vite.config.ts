import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';
import { fileURLToPath, URL } from 'node:url';

/**
 * Header used to tell the client that the upstream API is unreachable.
 * The client falls back to the built-in mock backend when it sees it.
 */
const BACKEND_UNREACHABLE_HEADER = 'X-Backend-Unreachable';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');

  return {
    plugins: [react()],
    build: {
      rollupOptions: {
        output: {
          // Chart.js is loaded only by the dashboard/analytics pages.
          manualChunks: (id) =>
            id.includes('node_modules/chart.js') || id.includes('node_modules/react-chartjs-2')
              ? 'charts'
              : undefined,
        },
      },
    },
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
    },
    server: {
      port: 5173,
      proxy: {
        '/api': {
          target: env.VITE_PROXY_TARGET ?? 'http://localhost:8000',
          changeOrigin: true,
          configure: (proxy) => {
            proxy.on('error', (error, _req, res) => {
              // The upstream API is down: answer with a marked response so the
              // client can switch to the mock backend instead of failing.
              if (
                res &&
                typeof res.writeHead === 'function' &&
                !('headersSent' in res && res.headersSent)
              ) {
                res.writeHead(503, {
                  'Content-Type': 'application/json; charset=utf-8',
                  [BACKEND_UNREACHABLE_HEADER]: '1',
                });
                res.end(
                  JSON.stringify({ detail: `Backend unreachable: ${error.message}` }),
                );
              }
            });
          },
        },
      },
    },
  };
});
