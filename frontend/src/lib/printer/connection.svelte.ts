// Connection to a thermal printer from the browser: Web Serial, WebUSB or Web Bluetooth.
// These APIs work in Chrome/Edge (desktop and Android) over HTTPS or localhost. The chosen device is
// remembered per browser: the browser keeps the permission and we find the device again on the next visit.
/* eslint-disable @typescript-eslint/no-explicit-any */

import type { PosSettings, Sale } from '$lib/types';
import { cartaHtml, ticketBytes, ticketHtml, printHtml } from './ticket';
import { EscPos } from './escpos';

export type Transport = 'serial' | 'usb' | 'bluetooth';

const KEY = 'caresia_printer';
const BT_SERVICES = [
  '000018f0-0000-1000-8000-00805f9b34fb',
  'e7810a71-73ae-499d-8c15-faa9aef0c3f2',
  '49535343-fe7d-4ae5-8fa9-9fafd205e455',
  '0000ff00-0000-1000-8000-00805f9b34fb'
];

export function supports(t: Transport): boolean {
  if (typeof navigator === 'undefined') return false;
  return t === 'serial' ? 'serial' in navigator : t === 'usb' ? 'usb' in navigator : 'bluetooth' in navigator;
}

interface Saved {
  transport: Transport;
  name: string;
  baud: number;
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

class PrinterConnection {
  transport = $state<Transport | null>(null);
  name = $state('');
  connected = $state(false);
  busy = $state(false);
  baud = $state(9600);
  error = $state('');

  #port: any = null;
  #usb: { dev: any; ep: number } | null = null;
  #bt: { dev: any; ch: any } | null = null;

  constructor() {
    if (typeof localStorage === 'undefined') return;
    try {
      const saved: Saved | null = JSON.parse(localStorage.getItem(KEY) ?? 'null');
      if (saved) {
        this.transport = saved.transport;
        this.name = saved.name;
        this.baud = saved.baud;
      }
    } catch {
      /* private mode */
    }
  }

  #remember() {
    try {
      localStorage.setItem(KEY, JSON.stringify({ transport: this.transport as Transport, name: this.name, baud: this.baud } satisfies Saved));
    } catch {
      /* ignore */
    }
  }

  /** Asks the person to pick a printer (must be called from a click). */
  async pair(t: Transport, baud = 9600): Promise<void> {
    this.error = '';
    this.busy = true;
    try {
      await this.disconnect(false);
      if (t === 'serial') {
        const port = await (navigator as any).serial.requestPort();
        await port.open({ baudRate: baud });
        this.#port = port;
        const info = port.getInfo?.() ?? {};
        this.name = info.usbVendorId ? `Puerto serie ${info.usbVendorId.toString(16)}:${(info.usbProductId ?? 0).toString(16)}` : 'Puerto serie';
        this.baud = baud;
      } else if (t === 'usb') {
        const dev = await (navigator as any).usb.requestDevice({ filters: [] });
        await this.#openUsb(dev);
        this.name = dev.productName || 'Impresora USB';
      } else {
        const dev = await (navigator as any).bluetooth.requestDevice({ acceptAllDevices: true, optionalServices: BT_SERVICES });
        await this.#openBt(dev);
        this.name = dev.name || 'Impresora Bluetooth';
      }
      this.transport = t;
      this.connected = true;
      this.#remember();
    } catch (e) {
      this.connected = false;
      this.error = explain(e);
      throw new Error(this.error);
    } finally {
      this.busy = false;
    }
  }

  /** Reconnects to the printer the browser already has permission for, without asking. */
  async reconnect(): Promise<boolean> {
    if (this.connected) return true;
    if (!this.transport) return false;
    this.error = '';
    this.busy = true;
    try {
      if (this.transport === 'serial' && supports('serial')) {
        const ports = await (navigator as any).serial.getPorts();
        if (!ports.length) return false;
        await ports[0].open({ baudRate: this.baud }).catch((e: any) => {
          if (e?.name !== 'InvalidStateError') throw e; // already open
        });
        this.#port = ports[0];
      } else if (this.transport === 'usb' && supports('usb')) {
        const devs = await (navigator as any).usb.getDevices();
        if (!devs.length) return false;
        await this.#openUsb(devs[0]);
      } else if (this.transport === 'bluetooth' && supports('bluetooth') && (navigator as any).bluetooth.getDevices) {
        const devs = await (navigator as any).bluetooth.getDevices();
        const dev = devs.find((d: any) => d.name === this.name) ?? devs[0];
        if (!dev) return false;
        await this.#openBt(dev);
      } else return false;
      this.connected = true;
      return true;
    } catch (e) {
      this.error = explain(e);
      this.connected = false;
      return false;
    } finally {
      this.busy = false;
    }
  }

  async #openUsb(dev: any) {
    await dev.open();
    if (dev.configuration === null) await dev.selectConfiguration(1);
    for (const iface of dev.configuration.interfaces) {
      for (const alt of iface.alternates) {
        const out = alt.endpoints.find((e: any) => e.direction === 'out' && e.type === 'bulk');
        if (out && (alt.interfaceClass === 7 || alt.interfaceClass === 255)) {
          await dev.claimInterface(iface.interfaceNumber);
          if (alt.alternateSetting) await dev.selectAlternateInterface(iface.interfaceNumber, alt.alternateSetting);
          this.#usb = { dev, ep: out.endpointNumber };
          return;
        }
      }
    }
    throw new Error('Este dispositivo USB no parece una impresora de tickets.');
  }

  async #openBt(dev: any) {
    const server = await dev.gatt.connect();
    for (const uuid of BT_SERVICES) {
      let service;
      try {
        service = await server.getPrimaryService(uuid);
      } catch {
        continue;
      }
      for (const ch of await service.getCharacteristics()) {
        if (ch.properties.write || ch.properties.writeWithoutResponse) {
          this.#bt = { dev, ch };
          return;
        }
      }
    }
    throw new Error('No encontramos el canal de impresión de este dispositivo Bluetooth.');
  }

  async disconnect(forget = true) {
    try {
      await this.#port?.close?.();
    } catch {
      /* ignore */
    }
    try {
      await this.#usb?.dev.close?.();
    } catch {
      /* ignore */
    }
    try {
      this.#bt?.dev.gatt?.disconnect?.();
    } catch {
      /* ignore */
    }
    this.#port = this.#usb = this.#bt = null;
    this.connected = false;
    if (forget) {
      this.transport = null;
      this.name = '';
      try {
        localStorage.removeItem(KEY);
      } catch {
        /* ignore */
      }
    }
  }

  /** Sends raw bytes. */
  async write(bytes: Uint8Array): Promise<void> {
    if (!this.connected && !(await this.reconnect())) throw new Error('La impresora no está conectada.');
    try {
      if (this.#port) {
        const w = this.#port.writable.getWriter();
        try {
          await w.write(bytes);
        } finally {
          w.releaseLock();
        }
      } else if (this.#usb) {
        for (let i = 0; i < bytes.length; i += 4096) await this.#usb.dev.transferOut(this.#usb.ep, bytes.slice(i, i + 4096));
      } else if (this.#bt) {
        const ch = this.#bt.ch;
        if (!this.#bt.dev.gatt.connected) await this.#bt.dev.gatt.connect();
        for (let i = 0; i < bytes.length; i += 100) {
          const chunk = bytes.slice(i, i + 100);
          if (ch.properties.writeWithoutResponse) await ch.writeValueWithoutResponse(chunk);
          else await ch.writeValue(chunk);
          await sleep(20);
        }
      } else throw new Error('La impresora no está conectada.');
    } catch (e) {
      this.connected = false;
      throw new Error(explain(e));
    }
  }
}

function explain(e: unknown): string {
  const err = e as { name?: string; message?: string };
  if (err?.name === 'NotFoundError' || /cancel/i.test(err?.message ?? '')) return 'No se eligió ninguna impresora.';
  if (err?.name === 'SecurityError') return 'El navegador bloqueó el acceso. Usa Chrome o Edge en una página segura (https).';
  if (err?.name === 'NetworkError' || err?.name === 'InvalidStateError') return 'No se pudo hablar con la impresora. Revisa que esté encendida y no la use otro programa.';
  return err?.message || 'No se pudo conectar con la impresora.';
}

export const printer = new PrinterConnection();

// ---------------------------------------------------------------------------
// High level: print a sale / a test page according to the business settings
// ---------------------------------------------------------------------------

const PLACEHOLDER: Sale = {
  id: 'test', folio: 0, customer_name: '', customer_curp: '', note: '', subtotal_cents: 0, discount_cents: 0, tax_cents: 0, total_cents: 0,
  status: 'paid', void_reason: '', voided_by: '', created_by: '', created_at: new Date().toISOString(), lines: [], payments: []
};

export async function printSale(sale: Sale, s: PosSettings, opts: { reprint?: boolean } = {}): Promise<void> {
  for (let i = 0; i < Math.max(1, s.printer.copies); i++) {
    if (s.printer.kind === 'browser' || !printer.transport) await printHtml(ticketHtml(sale, s, opts));
    else await printer.write(ticketBytes(sale, s, opts));
  }
}

export async function printLetter(sale: Sale, s: PosSettings): Promise<void> {
  await printHtml(cartaHtml(sale, s));
}

export async function printTest(s: PosSettings): Promise<void> {
  const sale = { ...PLACEHOLDER, created_at: new Date().toISOString() };
  if (s.printer.kind === 'browser' || !printer.transport) await printHtml(ticketHtml(sale, s, { test: true }));
  else await printer.write(ticketBytes(sale, s, { test: true }));
}

/** Opens the cash drawer through the printer (needs a connected thermal printer). */
export async function openDrawer(): Promise<void> {
  await printer.write(new EscPos(80).drawer().build());
}
