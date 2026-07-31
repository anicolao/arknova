import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

const apiOrigin = process.env.ARKNOVA_DEV_API_ORIGIN ?? 'http://127.0.0.1:8081';
const publicOrigin = process.env.ARKNOVA_DEV_ORIGIN ?? 'http://localhost:5173';

export default defineConfig({
  plugins: [sveltekit()],
  server: {
    allowedHosts: [new URL(publicOrigin).hostname],
    proxy: {
      '/api': apiOrigin,
      '/healthz': apiOrigin,
      '/ws': { target: apiOrigin, ws: true }
    }
  }
});
