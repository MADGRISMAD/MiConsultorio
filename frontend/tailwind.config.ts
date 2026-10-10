import type { Config } from 'tailwindcss';

export default {
  // `dark:` utilities apply under any element carrying data-theme="dark"
  darkMode: ['selector', '[data-theme="dark"]'],
  content: ['./src/**/*.{html,js,svelte,ts}'],
  // hover: styles only on devices that can hover, so taps don't leave them stuck
  future: { hoverOnlyWhenSupported: true },
  theme: {
    extend: {
      colors: {
        // Landing palette: values live in CSS variables (--l-*) so the page can switch theme
        paper: 'rgb(var(--l-paper) / <alpha-value>)',
        desk: 'rgb(var(--l-desk) / <alpha-value>)',
        panel: 'rgb(var(--l-panel) / <alpha-value>)',
        ink: { DEFAULT: 'rgb(var(--l-ink) / <alpha-value>)', soft: 'rgb(var(--l-ink-soft) / <alpha-value>)', faint: 'rgb(var(--l-ink-faint) / <alpha-value>)' },
        signal: { DEFAULT: 'rgb(var(--l-signal) / <alpha-value>)', soft: 'rgb(var(--l-signal-soft) / <alpha-value>)' },
        mint: { DEFAULT: 'rgb(var(--l-mint) / <alpha-value>)', soft: 'rgb(var(--l-mint-soft) / <alpha-value>)' },
        // Panel/app palette: driven by CSS variables so it follows the light/dark theme
        app: Object.fromEntries(
          ['primary', 'accent', 'ink', 'muted', 'surface', 'panel', 'elevated', 'danger', 'success', 'warning', 'on-primary'].map((k) => [
            k,
            `rgb(var(--app-${k}) / <alpha-value>)`
          ])
        )
      },
      boxShadow: { app: 'var(--app-shadow)' },
      opacity: { 6: '0.06', 8: '0.08', 12: '0.12', 14: '0.14' },
      fontFamily: {
        display: ['"Instrument Serif"', 'Georgia', 'serif'],
        body: ['"Instrument Sans Variable"', 'system-ui', 'sans-serif'],
        mono: ['"JetBrains Mono"', 'ui-monospace', 'monospace'],
        hand: ['Caveat', 'cursive']
      },
      transitionTimingFunction: { 'out-strong': 'cubic-bezier(0.23, 1, 0.32, 1)' }
    }
  },
  plugins: []
} satisfies Config;
