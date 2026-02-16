import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// Build output lands in ../web, which the Go server serves at "/" with SPA fallback.
// The dev server proxies API + WebSocket traffic to the Go server on :8080.
export default defineConfig({
  plugins: [react()],
  base: '/',
  build: {
    outDir: '../web',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/ws': { target: 'ws://localhost:8080', ws: true },
    },
  },
});
