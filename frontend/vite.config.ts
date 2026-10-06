import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [sveltekit()],
  server: {
    // In development the Go API runs on :8080; production serves both from one origin.
    proxy: { '/api': { target: process.env.API_URL ?? 'http://localhost:8080', changeOrigin: false } }
  }
});
