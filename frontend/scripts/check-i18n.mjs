// Checks the i18n dictionaries: same keys in es-MX and en, same {placeholders}, no orphan keys and no
// t('...') call with a key that does not exist. Run with: npm run test:i18n (Node 22+, no dependencies).
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, dirname, relative } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const src = join(dirname(fileURLToPath(import.meta.url)), '..', 'src');
const dictDir = join(src, 'lib', 'i18n', 'dict');
const problems = [];

const es = {};
const en = {};
for (const f of readdirSync(dictDir).filter((f) => f.endsWith('.ts'))) {
  const mod = await import(pathToFileURL(join(dictDir, f)).href);
  for (const k of Object.keys(mod.es)) {
    if (k in es) problems.push(`duplicate key across dictionaries: ${k}`);
    es[k] = mod.es[k];
  }
  for (const k of Object.keys(mod.en)) en[k] = mod.en[k];
  for (const k of Object.keys(mod.es)) if (!(k in mod.en)) problems.push(`${f}: "${k}" missing in en`);
  for (const k of Object.keys(mod.en)) if (!(k in mod.es)) problems.push(`${f}: "${k}" missing in es-MX`);
}

const params = (s) => [...s.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort().join(',');
for (const k of Object.keys(es)) {
  if (k in en && params(es[k]) !== params(en[k])) problems.push(`placeholders differ in "${k}": es {${params(es[k])}} vs en {${params(en[k])}}`);
  if (!es[k].trim() || (k in en && !en[k].trim())) problems.push(`empty text for "${k}"`);
}

function walk(dir, out = []) {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) walk(p, out);
    else if (/\.(svelte|ts)$/.test(name) && !p.startsWith(dictDir)) out.push(p);
  }
  return out;
}

const files = walk(src);
const text = files.map((f) => readFileSync(f, 'utf8')).join('\n');

// Dynamic keys such as t(`kind.${k}`) mark everything under that prefix as used.
const prefixes = [...text.matchAll(/`([\w.]+)\$\{/g)].map((m) => m[1]);
const used = (key) => text.includes(`'${key}'`) || text.includes(`"${key}"`) || text.includes(`\`${key}\``) || prefixes.some((p) => key.startsWith(p));
for (const k of Object.keys(es)) if (!used(k)) problems.push(`orphan key (not used in src): ${k}`);

for (const f of files) {
  const body = readFileSync(f, 'utf8');
  for (const m of body.matchAll(/\bt\(\s*(['"])([\w.]+)\1/g)) {
    if (!(m[2] in es)) problems.push(`${relative(src, f)}: t('${m[2]}') has no entry`);
  }
}

if (problems.length) {
  console.error(problems.join('\n'));
  process.exit(1);
}
console.log(`i18n ok: ${Object.keys(es).length} keys in es-MX and en`);
