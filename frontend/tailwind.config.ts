import type { Config } from 'tailwindcss';

export default {
  content: ['./src/**/*.{html,js,svelte,ts}'],
  // hover: styles only on devices that can hover, so taps don't leave them stuck
  future: { hoverOnlyWhenSupported: true },
  theme: {
    extend: {
      colors: {
        paper: '#F4F8FB',
        desk: '#DDE7EE',
        ink: { DEFAULT: '#0B2540', soft: '#4B6177', faint: '#8A9BAD' },
        signal: { DEFAULT: '#1673D1', soft: '#DCEAFB' },
        mint: { DEFAULT: '#0F9E8E', soft: '#D3F0EC' },
        // Panel/app palette: driven by CSS variables so it follows the light/dark theme
        app: Object.fromEntries(
          ['primary', 'accent', 'ink', 'muted', 'surface', 'panel', 'elevated', 'danger', 'success', 'warning', 'on-primary'].map((k) => [
            k,
            `rgb(var(--app-${k}) / <alpha-value>)`
          ])
        )
      },
      boxShadow: { app: 'var(--app-shadow)' },
      opacity: { 8: '0.08', 12: '0.12' },
      fontFamily: {
        display: ['"Instrument Serif"', 'Georgia', 'serif'],
        body: ['"Instrument Sans"', 'system-ui', 'sans-serif'],
        mono: ['"JetBrains Mono"', 'ui-monospace', 'monospace'],
        hand: ['Caveat', 'cursive']
      },
      transitionTimingFunction: { 'out-strong': 'cubic-bezier(0.23, 1, 0.32, 1)' }
    }
  },
  plugins: []
} satisfies Config;
