// Genera build/landing.html: la misma página de entrada que index.html, pero con carga anticipada (modulepreload)
// de todo lo que necesita la página de inicio. Sin esto, el navegador solo se entera de esos ~80 KB cuando arranca
// la app, y los descarga en una segunda tanda en serie. Vercel sirve este archivo únicamente en "/" (ver vercel.json);
// el resto de las rutas sigue usando index.html, y el servidor de Go no cambia.
import fs from 'node:fs';

const manifest = JSON.parse(fs.readFileSync('.svelte-kit/output/client/.vite/manifest.json', 'utf8'));
const app = fs.readFileSync('.svelte-kit/generated/client-optimized/app.js', 'utf8');
const html = fs.readFileSync('build/index.html', 'utf8');

const node = /"\/":\s*\[(\d+)\]/.exec(app)?.[1];
if (node === undefined) throw new Error('No encontré el nodo de la ruta "/" en app.js');
const entry = Object.keys(manifest).find((k) => k.endsWith(`/nodes/${node}.js`));
if (!entry) throw new Error(`No encontré nodes/${node}.js en el manifiesto`);

// dependencias estáticas (las dinámicas se cargan a demanda y no hacen falta para pintar)
const files = new Set();
const css = new Set();
(function walk(key, seen = new Set()) {
  if (seen.has(key)) return;
  seen.add(key);
  const e = manifest[key];
  files.add(e.file);
  (e.css ?? []).forEach((c) => css.add(c));
  (e.imports ?? []).forEach((i) => walk(i, seen));
})(entry);

const nuevos = [...files].filter((f) => !html.includes(`href="/${f}"`));
const nuevosCss = [...css].filter((f) => !html.includes(`href="/${f}"`));
const assets = fs.readdirSync('build/_app/immutable/assets');
const FUENTES = ['instrument-serif-latin-400-normal', 'instrument-serif-latin-400-italic', 'instrument-sans-latin-wght-normal'];
const fuentes = FUENTES.map((n) => assets.find((f) => f.startsWith(n) && f.endsWith('.woff2'))).filter(Boolean);
const tags = [
  ...fuentes.map((f) => `<link rel="preload" href="/_app/immutable/assets/${f}" as="font" type="font/woff2" crossorigin>`),
  ...nuevos.map((f) => `<link href="/${f}" rel="modulepreload">`),
  ...nuevosCss.map((f) => `<link href="/${f}" rel="stylesheet">`)
].join('\n\t\t');
if (!html.includes('</head>')) throw new Error('index.html sin </head>');
fs.writeFileSync('build/landing.html', html.replace('</head>', `\t\t${tags}\n  </head>`));
console.log(`landing.html: ${nuevos.length} módulos, ${nuevosCss.length} estilos y ${fuentes.length} fuentes precargados (nodo ${node})`);
