// index.html (la portada, ya prerenderizada) recibe carga anticipada (modulepreload) de todo lo que necesita la página de
// inicio; spa.html (la entrada del resto de las rutas) lo genera el adaptador sin precarga.
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
const assets = fs.readdirSync('build/_caresia/immutable/assets');
const FUENTES = ['instrument-serif-latin-400-normal', 'instrument-serif-latin-400-italic', 'instrument-sans-latin-wght-normal'];
const fuentes = FUENTES.map((n) => assets.find((f) => f.startsWith(n) && f.endsWith('.woff2'))).filter(Boolean);
const tags = [
  ...fuentes.map((f) => `<link rel="preload" href="/_caresia/immutable/assets/${f}" as="font" type="font/woff2" crossorigin>`),
  ...nuevos.map((f) => `<link href="/${f}" rel="modulepreload">`),
  ...nuevosCss.map((f) => `<link href="/${f}" rel="stylesheet">`)
].join('\n\t\t');
if (!html.includes('</head>')) throw new Error('index.html sin </head>');

fs.writeFileSync('build/index.html', html.replace('</head>', `\t\t${tags}\n  </head>`));
console.log(`index.html (landing) y spa.html: ${nuevos.length} módulos, ${nuevosCss.length} estilos y ${fuentes.length} fuentes precargados (nodo ${node})`);

// Las tres fuentes de la marca se precargan en TODAS las páginas generadas (no solo la portada): si el título grande
// cambia de tipografía después de pintar, mueve todo lo de abajo (salto de diseño, CLS) en artículos, blog y legales.
const fontTags = fuentes.map((f) => `<link rel="preload" href="/_caresia/immutable/assets/${f}" as="font" type="font/woff2" crossorigin>`).join('\n\t\t');
const pages = [];
(function walk(dir) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = `${dir}/${e.name}`;
    if (e.isDirectory()) { if (e.name !== '_caresia') walk(full); }
    else if (e.name.endsWith('.html')) pages.push(full);
  }
})('build');
let conFuentes = 0;
for (const f of pages) {
  const h = fs.readFileSync(f, 'utf8');
  if (!h.includes('</head>') || h.includes(`href="/_caresia/immutable/assets/${fuentes[0]}"`)) continue;
  fs.writeFileSync(f, h.replace('</head>', `\t\t${fontTags}\n  </head>`));
  conFuentes++;
}
console.log(`fuentes precargadas en ${conFuentes} páginas más`);

// ── Texto de las secciones de la portada en el HTML ──────────────────────────────────────────────────────────────
// Landing.svelte carga Funciones, Precios, Preguntas… con LazySection (import dinámico, por velocidad), así que el HTML
// prerenderizado solo traía el hero y los buscadores/asistentes de IA sin JavaScript no veían precios ni preguntas.
// Aquí se renderiza cada sección con el mismo componente (render de servidor de Svelte, ya compilado por el build) y se
// deja dentro del marcador de su LazySection: al montar, el componente real reemplaza el marcador completo.
import { render } from 'svelte/server';
import { pathToFileURL } from 'node:url';
import path from 'node:path';

const SECCIONES = { funciones: 'Features', especialidades: 'Specialties', retencion: 'Retention', incluye: 'Modules', seguridad: 'Security', precios: 'Pricing', preguntas: 'Faq', contacto: 'Cta', pie: 'Footer' };
let landing = fs.readFileSync('build/index.html', 'utf8');
let inyectadas = 0;
for (const [id, name] of Object.entries(SECCIONES)) {
  const marcador = new RegExp(`(<div id="${id}" style="min-height: [^"]+" aria-hidden="true">)(</div>)`);
  if (!marcador.test(landing)) throw new Error(`No encontré el marcador de la sección #${id} en index.html`);
  const mod = await import(pathToFileURL(path.resolve(`.svelte-kit/output/server/chunks/${name}.js`)).href);
  const fn = mod.default ?? Object.values(mod).find((v) => typeof v === 'function');
  const { body } = render(fn, { props: {} });
  landing = landing.replace(marcador, (_, a, b) => a + body + b);
  inyectadas++;
}
fs.writeFileSync('build/index.html', landing);
console.log(`secciones de la portada con su texto en el HTML: ${inyectadas}`);

// llms.txt: se agrega la lista de artículos del blog al compilar (viene de la API pública de Faro, así que siempre está al día).
try {
  const r = await fetch('https://faro.hyperionmkt.com/api/v1/caresia.mx/posts?limit=50');
  const posts = r.ok ? (await r.json()).posts : [];
  if (posts.length) {
    const lista = posts.map((p) => `- [${p.title}](https://caresia.mx/blog/${p.slug}): ${p.summary}`).join('\n');
    fs.appendFileSync('build/llms.txt', `\n## Artículos del blog\n${lista}\n`);
    console.log(`llms.txt: ${posts.length} artículos`);
  }
} catch (e) {
  console.warn('llms.txt sin artículos (Faro no respondió):', e.message);
}
