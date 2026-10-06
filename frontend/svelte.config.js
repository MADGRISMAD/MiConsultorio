import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
export default {
  preprocess: vitePreprocess(),
  kit: {
    // Single-page app: the Go server falls back to index.html for client-side routes.
    adapter: adapter({ fallback: 'index.html' })
  }
};
