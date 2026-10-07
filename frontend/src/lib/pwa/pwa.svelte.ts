// Installable-app state: the install prompt and "new version available". SvelteKit registers the service worker itself.
interface InstallEvent extends Event {
  prompt: () => Promise<void>;
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>;
}

const DISMISS_KEY = 'caresia_pwa_install_dismissed';

class PwaStore {
  /** A new service worker is installed and waiting for the user to accept it. */
  updateReady = $state(false);
  /** The browser offered to install the app (Chromium). */
  canInstall = $state(false);
  /** Safari on iPhone/iPad has no install event: show the manual steps. */
  iosHint = $state(false);
  installed = $state(false);
  dismissed = $state(false);

  #event: InstallEvent | null = null;
  #waiting: ServiceWorker | null = null;
  #started = false;
  #reloading = false;

  /** Idempotent; call once from the root layout. */
  init() {
    if (this.#started || typeof window === 'undefined') return;
    this.#started = true;

    try {
      this.dismissed = localStorage.getItem(DISMISS_KEY) === '1';
    } catch {
      /* storage unavailable */
    }
    const standalone = window.matchMedia?.('(display-mode: standalone)').matches || (navigator as Navigator & { standalone?: boolean }).standalone === true;
    this.installed = !!standalone;
    const ua = navigator.userAgent;
    const ios = /iphone|ipad|ipod/i.test(ua) || (/macintosh/i.test(ua) && navigator.maxTouchPoints > 1);
    this.iosHint = ios && !standalone;

    window.addEventListener('beforeinstallprompt', (e) => {
      e.preventDefault();
      this.#event = e as InstallEvent;
      this.canInstall = true;
    });
    window.addEventListener('appinstalled', () => {
      this.installed = true;
      this.canInstall = false;
      this.#event = null;
    });

    if (!('serviceWorker' in navigator)) return;
    navigator.serviceWorker.addEventListener('controllerchange', () => {
      // Only reload when the user asked for the update; the first claim of a fresh install must not reload the page.
      if (this.#reloading) location.reload();
    });
    navigator.serviceWorker.ready.then((reg) => {
      this.#watch(reg);
      // Look for a new version whenever the app comes back to the foreground.
      document.addEventListener('visibilitychange', () => {
        if (document.visibilityState === 'visible') reg.update().catch(() => {});
      });
    });
  }

  #watch(reg: ServiceWorkerRegistration) {
    const waiting = (w: ServiceWorker | null) => {
      if (w && navigator.serviceWorker.controller) {
        this.#waiting = w;
        this.updateReady = true;
      }
    };
    waiting(reg.waiting);
    reg.addEventListener('updatefound', () => {
      const worker = reg.installing;
      worker?.addEventListener('statechange', () => {
        if (worker.state === 'installed') waiting(worker);
      });
    });
  }

  /** Shows the browser's install dialog. */
  async install() {
    const e = this.#event;
    if (!e) return;
    this.#event = null;
    this.canInstall = false;
    await e.prompt();
    await e.userChoice.catch(() => null);
  }

  /** Activates the waiting worker and reloads once it has taken control. */
  applyUpdate() {
    if (!this.#waiting) return location.reload();
    this.#reloading = true;
    this.#waiting.postMessage({ type: 'SKIP_WAITING' });
  }

  dismissUpdate() {
    this.updateReady = false;
  }

  dismissInstall() {
    this.dismissed = true;
    try {
      localStorage.setItem(DISMISS_KEY, '1');
    } catch {
      /* ignore */
    }
  }
}

export const pwa = new PwaStore();
