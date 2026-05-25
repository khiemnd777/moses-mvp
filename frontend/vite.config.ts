import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { fileURLToPath, URL } from 'node:url';

const apiProxyTarget = process.env.DEV_API_PROXY_TARGET;
const apiProxyPaths = [
  '/auth/',
  '/admin/',
  '/documents',
  '/document-uploads',
  '/document-versions/',
  '/doc-types',
  '/ingest-jobs',
  '/conversations',
  '/messages',
  '/search',
  '/answer',
  '/chat',
  '/citations/',
  '^/assets/[^/]+/download$',
  '/health',
  '/metrics'
];

const apiProxy = apiProxyTarget
  ? Object.fromEntries(
      apiProxyPaths.map((path) => [
        path,
        {
          target: apiProxyTarget,
          changeOrigin: true,
          secure: false
        }
      ])
    )
  : undefined;

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  server: {
    port: 5173,
    proxy: apiProxy
  }
});
