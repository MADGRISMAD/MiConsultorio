// Consentimiento de cookies de Google Analytics (vía Google Tag Manager) en las páginas públicas.
// GTM no se carga hasta que la persona acepta; la decisión se guarda en este navegador.
// La carga real vive en src/app.html (window.caresiaCargarGTM); esta lista de rutas debe coincidir con la de ahí.

export const PUBLIC_PATHS = ['/', '/login', '/register', '/forgot', '/privacidad', '/terminos', '/directorio'];

const KEY = 'caresia-cookies';
type Choice = 'aceptadas' | 'rechazadas';

declare global {
  interface Window {
    caresiaCargarGTM?: () => void;
  }
}

function read(): Choice | null {
  try {
    const v = localStorage.getItem(KEY);
    return v === 'aceptadas' || v === 'rechazadas' ? v : null;
  } catch {
    return null;
  }
}

function save(v: Choice) {
  try {
    localStorage.setItem(KEY, v);
  } catch {
    // modo privado o almacenamiento bloqueado: la decisión dura solo esta visita
  }
}

/** Borra las cookies de Google Analytics (_ga, _ga_XXXX, _gid) del dominio actual y de su dominio padre. */
function clearAnalyticsCookies() {
  const names = document.cookie
    .split(';')
    .map((c) => c.split('=')[0].trim())
    .filter((n) => n === '_gid' || n === '_gat' || n.startsWith('_ga'));
  const host = location.hostname;
  const domains = ['', host, `.${host}`, `.${host.split('.').slice(-2).join('.')}`];
  for (const n of names) {
    for (const d of domains) {
      document.cookie = `${n}=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/${d ? `; domain=${d}` : ''}`;
    }
  }
}

class CookieConsent {
  choice = $state<Choice | null>(null);
  /** Aviso visible: sin decisión guardada, o reabierto desde "Preferencias de cookies". */
  open = $state(false);
  /** Reabierto a mano: se muestra aunque la página no sea pública (p. ej. el aviso dentro de la reserva en línea). */
  manual = $state(false);

  init() {
    this.choice = read();
    this.open = this.choice === null;
  }

  accept() {
    this.choice = 'aceptadas';
    this.open = this.manual = false;
    save('aceptadas');
    window.caresiaCargarGTM?.();
  }

  reject() {
    const was = this.choice;
    this.choice = 'rechazadas';
    this.open = this.manual = false;
    save('rechazadas');
    clearAnalyticsCookies();
    // GTM ya cargado en esta página no se puede descargar: recargar lo quita por completo
    if (was === 'aceptadas') location.reload();
  }

  reopen() {
    this.manual = true;
    this.open = true;
  }
}

export const cookieConsent = new CookieConsent();
