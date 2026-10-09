import { toCanvas } from 'html-to-image';

// The same ticket the print dialog shows, sent straight to a thermal printer as an image (ESC/POS raster).
// Sending text instead prints plain, unstyled lines; this keeps the design (boxes, black total band, stamp).

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

/** Printable dots across: 58 mm paper is 384 dots, 80 mm is 576. */
export const dotsFor = (widthMm: number) => (widthMm === 58 ? 384 : 576);

/** Loads the ticket's HTML in a hidden frame and draws it on a canvas exactly `dots` wide. */
export async function renderTicketCanvas(html: string, widthMm: number): Promise<HTMLCanvasElement> {
  const dots = dotsFor(widthMm);
  const frame = document.createElement('iframe');
  frame.setAttribute('aria-hidden', 'true');
  frame.tabIndex = -1;
  frame.style.cssText = `position:fixed;left:-10000px;top:0;width:${widthMm}mm;height:2400px;border:0;opacity:0;pointer-events:none;`;
  const loaded = new Promise<void>((resolve, reject) => {
    frame.addEventListener('load', () => resolve(), { once: true });
    frame.addEventListener('error', () => reject(new Error('No se pudo preparar el ticket.')), { once: true });
  });
  frame.srcdoc = html;
  document.body.appendChild(frame);
  try {
    await loaded;
    const doc = frame.contentDocument;
    const el = doc?.querySelector<HTMLElement>('.tk');
    if (!doc || !el) throw new Error('No se pudo leer el ticket.');
    await doc.fonts?.ready;
    await sleep(120);
    const width = el.offsetWidth;
    if (!width) throw new Error('El ticket no tiene tamaño.');
    const raw = await toCanvas(el, { pixelRatio: dots / width, backgroundColor: '#ffffff', style: { margin: '0' } });
    const out = document.createElement('canvas');
    out.width = dots;
    out.height = Math.max(1, Math.round((raw.height * dots) / raw.width));
    const ctx = out.getContext('2d')!;
    ctx.fillStyle = '#fff';
    ctx.fillRect(0, 0, out.width, out.height);
    ctx.drawImage(raw, 0, 0, out.width, out.height);
    return out;
  } finally {
    frame.remove();
  }
}

export interface Bitmap {
  width: number;
  height: number;
  bytesPerRow: number;
  data: Uint8Array;
}

/** Canvas to a 1-bit bitmap (black / white), trimming the blank paper left at the bottom. */
export function canvasToBitmap(canvas: HTMLCanvasElement, threshold = 180): Bitmap {
  const { width, height } = canvas;
  const px = canvas.getContext('2d', { willReadFrequently: true })!.getImageData(0, 0, width, height).data;
  const bytesPerRow = Math.ceil(width / 8);
  const data = new Uint8Array(bytesPerRow * height);
  let lastInk = 0;
  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      const i = (y * width + x) * 4;
      const a = px[i + 3] / 255;
      const lum = (0.299 * px[i] + 0.587 * px[i + 1] + 0.114 * px[i + 2]) * a + 255 * (1 - a);
      if (lum < threshold) {
        data[y * bytesPerRow + (x >> 3)] |= 0x80 >> (x & 7);
        lastInk = y;
      }
    }
  }
  return { width, height: Math.min(height, lastInk + 12), bytesPerRow, data };
}

/** ESC/POS bytes: the image in bands (GS v 0), then feed and cut. */
export function rasterBytes(bitmap: Bitmap, opts: { openDrawer?: boolean; cut?: boolean } = {}): Uint8Array {
  const parts: Uint8Array[] = [];
  const put = (...b: number[]) => parts.push(Uint8Array.from(b));
  put(0x1b, 0x40); // init
  if (opts.openDrawer) put(0x1b, 0x70, 0x00, 0x19, 0xfa); // pulse the cash drawer
  put(0x1b, 0x61, 0x01); // centered, in case the printer is wider than the image
  const BAND = 128; // rows per command: printers with little memory need it
  for (let y = 0; y < bitmap.height; y += BAND) {
    const h = Math.min(BAND, bitmap.height - y);
    put(0x1d, 0x76, 0x30, 0x00, bitmap.bytesPerRow & 0xff, bitmap.bytesPerRow >> 8, h & 0xff, h >> 8);
    parts.push(bitmap.data.subarray(y * bitmap.bytesPerRow, (y + h) * bitmap.bytesPerRow));
  }
  put(0x1b, 0x61, 0x00);
  if (opts.cut === false) put(0x1b, 0x64, 0x05);
  else put(0x1b, 0x64, 0x04, 0x1d, 0x56, 0x42, 0x03); // feed and partial cut
  const bytes = new Uint8Array(parts.reduce((n, p) => n + p.length, 0));
  let off = 0;
  for (const p of parts) {
    bytes.set(p, off);
    off += p.length;
  }
  return bytes;
}

/** The ticket HTML as image bytes for the thermal printer. */
export async function htmlToRasterBytes(html: string, widthMm: number, opts: { openDrawer?: boolean; cut?: boolean } = {}): Promise<Uint8Array> {
  return rasterBytes(canvasToBitmap(await renderTicketCanvas(html, widthMm)), opts);
}
