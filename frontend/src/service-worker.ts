/// <reference types="@sveltejs/kit" />
/// <reference no-default-lib="true"/>
/// <reference lib="esnext" />
/// <reference lib="webworker" />
import { build, files, version } from '$service-worker';

// Static shell only. Clinical data lives behind /api and is never stored, read from or answered by this worker.
const sw = self as unknown as ServiceWorkerGlobalScope;
const CACHE = `caresia-static-${version}`;
const SHELL = '/offline';
const STATIC = new Set<string>([...build, ...files].filter((p) => !p.endsWith('.map')));

sw.addEventListener('install', (event) => {
  event.waitUntil(
    (async () => {
      const cache = await caches.open(CACHE);
      await cache.addAll([...STATIC]);
      // The server answers client-side routes with the SPA shell; /offline is rendered by it.
      const shell = await fetch(SHELL, { cache: 'reload', credentials: 'omit' });
      if (shell.ok && !shell.redirected) await cache.put(SHELL, shell);
      // No skipWaiting here: the page asks for it (UpdateBanner) so open tabs are never swapped under the user.
    })()
  );
});

sw.addEventListener('message', (event) => {
  if (event.data?.type === 'SKIP_WAITING') sw.skipWaiting();
});

sw.addEventListener('activate', (event) => {
  event.waitUntil(
    (async () => {
      for (const key of await caches.keys()) if (key !== CACHE) await caches.delete(key);
      await sw.clients.claim();
    })()
  );
});

sw.addEventListener('fetch', (event) => {
  const { request } = event;
  if (request.method !== 'GET') return;
  const url = new URL(request.url);
  if (url.origin !== sw.location.origin) return;
  // Never touch the API (or anything carrying credentials headers): the browser goes straight to the network.
  if (url.pathname === '/api' || url.pathname.startsWith('/api/') || request.headers.has('authorization')) return;

  if (request.mode === 'navigate') {
    event.respondWith(navigate(request, url));
    return;
  }
  if (STATIC.has(url.pathname)) event.respondWith(cacheFirst(request, url.pathname));
});

async function cacheFirst(request: Request, key: string): Promise<Response> {
  const cache = await caches.open(CACHE);
  const hit = await cache.match(key);
  if (hit) return hit;
  const res = await fetch(request);
  if (res.ok && !res.redirected && res.status === 200) await cache.put(key, res.clone());
  return res;
}

async function navigate(request: Request, url: URL): Promise<Response> {
  try {
    return await fetch(request);
  } catch {
    const cache = await caches.open(CACHE);
    if (url.pathname === SHELL) {
      const shell = await cache.match(SHELL);
      if (shell) return shell;
    } else {
      // Keep the page they were going to, so "Reintentar" can take them back.
      return Response.redirect(new URL(`${SHELL}?from=${encodeURIComponent(url.pathname + url.search)}`, sw.location.origin).href, 302);
    }
    return new Response('Sin conexión', { status: 503, headers: { 'Content-Type': 'text/plain; charset=utf-8' } });
  }
}
