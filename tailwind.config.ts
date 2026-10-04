import type { Config } from 'tailwindcss'

const config: Config = {
  content: [
    './src/pages/**/*.{js,ts,jsx,tsx,mdx}',
    './src/components/**/*.{js,ts,jsx,tsx,mdx}',
    './src/app/**/*.{js,ts,jsx,tsx,mdx}',
  ],
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
      },
      fontFamily: {
        display: ['var(--font-display)', 'Georgia', 'serif'],
        body: ['var(--font-body)', 'system-ui', 'sans-serif'],
        mono: ['var(--font-mono)', 'ui-monospace', 'monospace'],
        hand: ['var(--font-hand)', 'cursive'],
      },
      transitionTimingFunction: {
        'out-strong': 'cubic-bezier(0.23, 1, 0.32, 1)',
      },
      backgroundImage: {
        'gradient-radial': 'radial-gradient(var(--tw-gradient-stops))',
        'gradient-conic':
          'conic-gradient(from 180deg at 50% 50%, var(--tw-gradient-stops))',
      },
    },
  },
  plugins: [],
}
export default config
