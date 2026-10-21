// Traducción de la landing al inglés. Los textos están escritos en español en los componentes; aquí se cambian
// en el navegador por su equivalente de `i18n/dict/landing.ts` (clave = el texto en español, sin espacios sobrantes).
// Un MutationObserver traduce también lo que aparece después (secciones que cargan al hacer scroll, pestañas, preguntas).
import { en } from './en';

const ATTRS = ['title', 'aria-label', 'alt', 'placeholder'];
const texts = new WeakMap<Text, { es: string; en: string }>();
const attrs = new WeakMap<Element, Record<string, { es: string; en: string }>>();

const norm = (s: string) => s.replace(/\s+/g, ' ').trim();

function lookup(raw: string): string | null {
  const m = raw.match(/^(\s*)([\s\S]*?)(\s*)$/)!;
  const out = en[norm(m[2])];
  return out === undefined ? null : m[1] + out + m[3];
}

function doText(n: Text) {
  const st = texts.get(n);
  if (st && n.data === st.en) return; // lo escribimos nosotros
  const out = lookup(n.data);
  if (out === null) return void texts.delete(n);
  texts.set(n, { es: n.data, en: out });
  n.data = out;
}

function doAttrs(el: Element) {
  for (const a of ATTRS) {
    const cur = el.getAttribute(a);
    if (cur === null) continue;
    const rec = attrs.get(el)?.[a];
    if (rec && cur === rec.en) continue;
    const out = lookup(cur);
    if (out === null) continue;
    const map = attrs.get(el) ?? {};
    map[a] = { es: cur, en: out };
    attrs.set(el, map);
    el.setAttribute(a, out);
  }
}

function walk(node: Node, fn: (n: Text) => void, fa: (e: Element) => void) {
  if (node.nodeType === Node.TEXT_NODE) return fn(node as Text);
  if (node.nodeType !== Node.ELEMENT_NODE) return;
  const el = node as Element;
  if (el.tagName === 'SCRIPT' || el.tagName === 'STYLE') return;
  fa(el);
  for (const c of Array.from(el.childNodes)) walk(c, fn, fa);
}

/** Traduce `root` al inglés y sigue traduciendo lo nuevo. Devuelve la función que lo deshace. */
export function translateToEnglish(root: HTMLElement): () => void {
  const esTitle = document.title;
  walk(root, doText, doAttrs);
  const title = lookup(esTitle);
  if (title !== null) document.title = title;
  const obs = new MutationObserver((muts) => {
    for (const m of muts) {
      if (m.type === 'characterData') doText(m.target as Text);
      else if (m.type === 'attributes') doAttrs(m.target as Element);
      else m.addedNodes.forEach((n) => walk(n, doText, doAttrs));
    }
  });
  obs.observe(root, { subtree: true, childList: true, characterData: true, attributes: true, attributeFilter: ATTRS });
  return () => {
    obs.disconnect();
    walk(
      root,
      (n) => {
        const st = texts.get(n);
        if (st && n.data === st.en) n.data = st.es;
        texts.delete(n);
      },
      (el) => {
        const map = attrs.get(el);
        if (!map) return;
        for (const [a, v] of Object.entries(map)) if (el.getAttribute(a) === v.en) el.setAttribute(a, v.es);
        attrs.delete(el);
      }
    );
    document.title = esTitle;
  };
}
