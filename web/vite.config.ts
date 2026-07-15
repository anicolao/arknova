import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

const apiOrigin = process.env.ARKNOVA_DEV_API_ORIGIN ?? 'http://127.0.0.1:8081';

export default defineConfig({
  plugins: [sveltekit()],
  server: {
    proxy: {
      '/api': apiOrigin,
      '/healthz': apiOrigin,
      '/ws': { target: apiOrigin, ws: true }
    }
  }
});
