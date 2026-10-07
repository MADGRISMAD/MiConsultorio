// A small ESC/POS encoder for 58 mm and 80 mm thermal printers.
// Text is sent in code page 858 (Western European + €), which covers Spanish.

const ESC = 0x1b;
const GS = 0x1d;

// Unicode → CP858 for the characters Spanish tickets need.
const CP858: Record<string, number> = {
  Ç: 0x80, ü: 0x81, é: 0x82, â: 0x83, ä: 0x84, à: 0x85, å: 0x86, ç: 0x87, ê: 0x88, ë: 0x89, è: 0x8a, ï: 0x8b, î: 0x8c, ì: 0x8d, Ä: 0x8e, Å: 0x8f,
  É: 0x90, æ: 0x91, Æ: 0x92, ô: 0x93, ö: 0x94, ò: 0x95, û: 0x96, ù: 0x97, ÿ: 0x98, Ö: 0x99, Ü: 0x9a, ø: 0x9b, '£': 0x9c, Ø: 0x9d, '×': 0x9e,
  á: 0xa0, í: 0xa1, ó: 0xa2, ú: 0xa3, ñ: 0xa4, Ñ: 0xa5, ª: 0xa6, º: 0xa7, '¿': 0xa8, '®': 0xa9, '¬': 0xaa, '½': 0xab, '¼': 0xac, '¡': 0xad,
  Á: 0xb5, Â: 0xb6, À: 0xb7, '©': 0xb8, Ã: 0xc7, ã: 0xc6, Í: 0xd6, Î: 0xd7, Ï: 0xd8, '€': 0xd5, Ó: 0xe0, Ô: 0xe2, Ò: 0xe3, Ú: 0xe9, Û: 0xea, Ù: 0xeb,
  '°': 0xf8, '·': 0xfa, '“': 0x22, '”': 0x22, '‘': 0x27, '’': 0x27, '–': 0x2d, '—': 0x2d, '…': 0x2e
};

export function encodeText(s: string): number[] {
  const out: number[] = [];
  for (const ch of s) {
    const code = ch.codePointAt(0)!;
    if (code === 0x0a) out.push(0x0a);
    else if (code >= 0x20 && code < 0x7f) out.push(code);
    else if (CP858[ch] !== undefined) out.push(CP858[ch]);
    else out.push(0x3f); // ?
  }
  return out;
}

export type Align = 'left' | 'center' | 'right';

/** Builds a byte stream. All methods return `this` so a ticket reads top to bottom. */
export class EscPos {
  private bytes: number[] = [];
  /** characters per line at the normal font */
  readonly cols: number;

  constructor(widthMm: 58 | 80 = 80) {
    this.cols = widthMm === 58 ? 32 : 48;
    this.bytes.push(ESC, 0x40); // initialize
    this.bytes.push(ESC, 0x74, 19); // code page 858
  }

  align(a: Align) {
    this.bytes.push(ESC, 0x61, a === 'left' ? 0 : a === 'center' ? 1 : 2);
    return this;
  }
  bold(on: boolean) {
    this.bytes.push(ESC, 0x45, on ? 1 : 0);
    return this;
  }
  /** 1 = normal, 2 = double width and height */
  size(n: 1 | 2) {
    this.bytes.push(GS, 0x21, n === 2 ? 0x11 : 0x00);
    return this;
  }
  text(s: string) {
    this.bytes.push(...encodeText(s));
    return this;
  }
  line(s = '') {
    return this.text(s).feed(0);
  }
  /** a line break; n extra blank lines */
  feed(n = 1) {
    this.bytes.push(0x0a);
    for (let i = 0; i < n; i++) this.bytes.push(0x0a);
    return this;
  }
  rule(ch = '-') {
    return this.line(ch.repeat(this.cols));
  }
  /** left text and right text on the same line, padded to the paper width */
  pair(left: string, right: string) {
    const space = Math.max(1, this.cols - [...left].length - [...right].length);
    if ([...left].length + [...right].length + 1 > this.cols) {
      // too long: left text wraps on its own lines, amount goes on the last one
      const lines = wrap(left, this.cols);
      for (const l of lines.slice(0, -1)) this.line(l);
      const last = lines[lines.length - 1] ?? '';
      return this.line(last + ' '.repeat(Math.max(1, this.cols - [...last].length - [...right].length)) + right);
    }
    return this.line(left + ' '.repeat(space) + right);
  }
  qr(data: string, moduleSize = 6) {
    const d = encodeText(data);
    const len = d.length + 3;
    this.bytes.push(GS, 0x28, 0x6b, 4, 0, 0x31, 0x41, 0x32, 0); // model 2
    this.bytes.push(GS, 0x28, 0x6b, 3, 0, 0x31, 0x43, moduleSize);
    this.bytes.push(GS, 0x28, 0x6b, 3, 0, 0x31, 0x45, 0x31); // error correction M
    this.bytes.push(GS, 0x28, 0x6b, len & 0xff, len >> 8, 0x31, 0x50, 0x30, ...d);
    this.bytes.push(GS, 0x28, 0x6b, 3, 0, 0x31, 0x51, 0x30);
    return this;
  }
  /** pulse the cash drawer (pin 2) */
  drawer() {
    this.bytes.push(ESC, 0x70, 0, 25, 250);
    return this;
  }
  cut() {
    this.feed(3);
    this.bytes.push(GS, 0x56, 0x42, 0); // feed and partial cut
    return this;
  }
  build(): Uint8Array {
    return Uint8Array.from(this.bytes);
  }
}

/** Word-wraps to a column width, breaking very long words. */
export function wrap(s: string, width: number): string[] {
  const out: string[] = [];
  for (const para of s.split('\n')) {
    let cur = '';
    for (let word of para.split(/\s+/).filter(Boolean)) {
      while ([...word].length > width) {
        if (cur) {
          out.push(cur);
          cur = '';
        }
        out.push([...word].slice(0, width).join(''));
        word = [...word].slice(width).join('');
      }
      if (!cur) cur = word;
      else if ([...cur].length + 1 + [...word].length <= width) cur += ' ' + word;
      else {
        out.push(cur);
        cur = word;
      }
    }
    out.push(cur);
  }
  return out;
}
