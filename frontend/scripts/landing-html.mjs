// Deja dos páginas de entrada: index.html (la que recibe "/") con carga anticipada (modulepreload) de todo lo que
// necesita la página de inicio, y spa.html (la original, sin precarga) para el resto de las rutas.
// Sin esto, el navegador solo se entera de esos ~80 KB cuando arranca la app, y los descarga en una segunda tanda
// en serie. En Vercel, "/" se sirve desde el archivo que existe (index.html) y las demás rutas se reescriben a
// spa.html (ver vercel.json). El servidor de Go sigue usando index.html como respaldo: solo precarga de más.
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

// Solo en la portada (index.html): dirección canónica con https, vista previa al compartir y datos de la
// organización con su logo. Van en el HTML para que Google y las redes los lean sin ejecutar JavaScript.
const SITE = 'https://caresia.mx';
const description = /<meta name="description" content="([^"]*)"/.exec(html)?.[1] ?? '';
const title = 'Caresia · Software para clínicas dentales y consultorios médicos';
const org = {
  '@context': 'https://schema.org',
  '@graph': [
    { '@type': 'Organization', '@id': `${SITE}/#org`, name: 'Caresia', url: `${SITE}/`, logo: `${SITE}/icons/icon-512.png`, email: 'caresia-info@caresia.mx' },
    { '@type': 'WebSite', '@id': `${SITE}/#web`, name: 'Caresia', url: `${SITE}/`, inLanguage: 'es-MX', publisher: { '@id': `${SITE}/#org` } }
  ]
};
const seo = [
  `<link rel="canonical" href="${SITE}/">`,
  `<meta property="og:type" content="website">`,
  `<meta property="og:site_name" content="Caresia">`,
  `<meta property="og:locale" content="es_MX">`,
  `<meta property="og:url" content="${SITE}/">`,
  `<meta property="og:title" content="${title}">`,
  `<meta property="og:description" content="${description}">`,
  `<meta property="og:image" content="${SITE}/icons/icon-512.png">`,
  `<meta name="twitter:card" content="summary">`,
  `<script type="application/ld+json">${JSON.stringify(org).replace(/</g, '\\u003c')}</script>`
].join('\n\t\t');
fs.writeFileSync('build/spa.html', html);
fs.writeFileSync('build/index.html', html.replace('</head>', `\t\t${seo}\n\t\t${tags}\n  </head>`));
console.log(`index.html (landing) y spa.html: ${nuevos.length} módulos, ${nuevosCss.length} estilos y ${fuentes.length} fuentes precargados (nodo ${node})`);
