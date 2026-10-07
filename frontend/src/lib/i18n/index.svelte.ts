// Light i18n for the patient-facing public pages. Staff screens, emails and API error messages stay in Spanish.
import { es } from './es-MX';
import { en } from './en';

export type Locale = 'es-MX' | 'en';
export const LOCALES: { code: Locale; label: string; short: string }[] = [
  { code: 'es-MX', label: 'Español', short: 'ES' },
  { code: 'en', label: 'English', short: 'EN' }
];

const KEY = 'caresia_ui_lang';
const DICTS: Record<Locale, Record<string, string>> = { 'es-MX': es, en };
const DEFAULT: Locale = 'es-MX';

const isLocale = (v: unknown): v is Locale => v === 'es-MX' || v === 'en';

class I18n {
  locale = $state<Locale>(DEFAULT);
  #ready = false;

  /** Dictionary used for Intl formatting (en-US for English: the app only serves Mexican clinics). */
  get intl(): string {
    return this.locale === 'en' ? 'en-US' : 'es-MX';
  }

  /**
   * Reads the saved choice and, only when there is none, the browser language. Called by the public pages;
   * the staff app never calls it, so it always stays in Spanish.
   */
  initPublic() {
    if (this.#ready) return;
    this.#ready = true;
    let saved: string | null = null;
    try {
      saved = localStorage.getItem(KEY);
    } catch {
      /* storage unavailable */
    }
    if (isLocale(saved)) this.locale = saved;
    else if (typeof navigator !== 'undefined' && /^en\b/i.test(navigator.language || '')) this.locale = 'en';
    this.#apply();
  }

  set(locale: Locale) {
    this.#ready = true;
    this.locale = locale;
    try {
      localStorage.setItem(KEY, locale);
    } catch {
      /* ignore */
    }
    this.#apply();
  }

  /** Sets <html lang> for the current locale; `restore` puts the app default back when leaving public pages. */
  #apply() {
    if (typeof document !== 'undefined') document.documentElement.lang = this.locale === 'en' ? 'en' : 'es';
  }

  restoreDocumentLang() {
    if (typeof document !== 'undefined') document.documentElement.lang = 'es';
  }

  applyDocumentLang() {
    this.#apply();
  }
}

export const i18n = new I18n();

/** Replaces {name} placeholders. */
function fill(text: string, params?: Record<string, string | number>): string {
  if (!params) return text;
  return text.replace(/\{(\w+)\}/g, (m, k) => (k in params ? String(params[k]) : m));
}

/** Translates a key; falls back to Spanish and then to the key itself so nothing renders blank. */
export function t(key: string, params?: Record<string, string | number>): string {
  return fill(DICTS[i18n.locale][key] ?? es[key] ?? key, params);
}

/** Picks the singular or plural key by count and passes {n}. */
export function tn(key: string, n: number, params: Record<string, string | number> = {}): string {
  return t(`${key}.${n === 1 ? 'one' : 'other'}`, { ...params, n });
}

export const dictionaries = DICTS;

// Dates and money follow the active locale.
export function fmtDate(value: string | Date, opts: Intl.DateTimeFormatOptions = { day: 'numeric', month: 'short', year: 'numeric' }): string {
  const d = typeof value === 'string' ? new Date(value) : value;
  return d.toLocaleDateString(i18n.intl, opts);
}

/** A calendar day "YYYY-MM-DD" in local time (never shifted by the time zone). */
export function fmtDay(ymd: string, opts: Intl.DateTimeFormatOptions = { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' }): string {
  const [y, m, d] = ymd.split('-').map(Number);
  return new Date(y, m - 1, d).toLocaleDateString(i18n.intl, opts);
}

export function fmtDateTime(value: string | Date, opts: Intl.DateTimeFormatOptions = { dateStyle: 'medium', timeStyle: 'short' }): string {
  const d = typeof value === 'string' ? new Date(value) : value;
  return d.toLocaleString(i18n.intl, opts);
}

export function fmtMoneyCents(cents: number, currency = 'MXN'): string {
  return (cents / 100).toLocaleString(i18n.intl, { style: 'currency', currency });
}

export function fmtMoney(amount: number, currency = 'MXN'): string {
  return amount.toLocaleString(i18n.intl, { style: 'currency', currency });
}
