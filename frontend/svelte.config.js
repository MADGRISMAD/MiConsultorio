import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
export default {
  preprocess: vitePreprocess(),
  kit: {
    // App de una sola página; las rutas públicas de arriba se prerenderizan para buscadores.
    // spa.html es la página de entrada del resto de las rutas; / , /privacidad y /terminos se generan con su HTML completo.
    adapter: adapter({ fallback: 'spa.html' })
  }
};
