// Regenerates the PWA icons in frontend/static/icons from the brand mark in favicon.svg.
// Uses the Playwright already installed for the e2e suite (no new dependency):
//   cd e2e && npm install && cd .. && E2E_CHROMIUM_PATH=/path/to/chrome node scripts/gen-pwa-icons.mjs
import { createRequire } from 'node:module';
import { mkdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const { chromium } = createRequire(join(root, 'e2e', 'package.json'))('@playwright/test');
const out = join(root, 'frontend', 'static', 'icons');
mkdirSync(out, { recursive: true });

// Same mark as favicon.svg: a plus sign and a blue dot on the navy tile.
const mark = (tile) => `
  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="100%" height="100%">
    ${tile}
    <path d="M16 8v16M8 16h16" stroke="#F4F8FB" stroke-width="3.2" stroke-linecap="round" fill="none"/>
    <circle cx="23.5" cy="8.5" r="2.5" fill="#1673D1"/>
  </svg>`;
const rounded = mark('<rect width="32" height="32" rx="9" fill="#0B2540"/>');
// Maskable icons are cropped by the OS: full-bleed background and the mark inside the central 80%.
const maskable = `
  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="100%" height="100%">
    <rect width="32" height="32" fill="#0B2540"/>
    <g transform="translate(16 16) scale(0.62) translate(-16 -16)">
      <path d="M16 8v16M8 16h16" stroke="#F4F8FB" stroke-width="3.2" stroke-linecap="round" fill="none"/>
      <circle cx="23.5" cy="8.5" r="2.5" fill="#1673D1"/>
    </g>
  </svg>`;

const jobs = [
  ['icon-192.png', 192, rounded],
  ['icon-512.png', 512, rounded],
  ['icon-maskable-512.png', 512, maskable]
];

const browser = await chromium.launch({ executablePath: process.env.E2E_CHROMIUM_PATH || undefined });
for (const [name, size, svg] of jobs) {
  const page = await browser.newPage({ viewport: { width: size, height: size } });
  await page.setContent(`<body style="margin:0;background:transparent">${svg}</body>`);
  await page.screenshot({ path: join(out, name), omitBackground: true });
  await page.close();
  console.log('icons/' + name);
}
await browser.close();
