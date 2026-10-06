const KEY = 'caresia_ui_theme';
export type Theme = 'light' | 'dark';

function initial(): Theme {
  try {
    const saved = localStorage.getItem(KEY);
    if (saved === 'dark' || saved === 'light') return saved;
  } catch {
    /* storage unavailable */
  }
  return 'light'; // the calm, paper-white look of the landing is the default
}

class ThemeStore {
  mode = $state<Theme>('light');
  #ready = false;

  /** Reads the saved (or system) theme once, in the browser. */
  init() {
    if (this.#ready) return;
    this.#ready = true;
    this.mode = initial();
  }

  toggle() {
    this.mode = this.mode === 'dark' ? 'light' : 'dark';
    try {
      localStorage.setItem(KEY, this.mode);
    } catch {
      /* ignore */
    }
  }
}

export const theme = new ThemeStore();

// ssr is off, so this always runs in the browser: set the theme before anything renders
if (typeof localStorage !== 'undefined') theme.init();
