import type { Metadata } from 'next'
import { Instrument_Serif, Instrument_Sans, JetBrains_Mono, Caveat } from 'next/font/google'
import './globals.css'
import Providers from './Provider';

// Fuentes de la landing; se exponen como variables CSS (ver tailwind.config.ts)
const display = Instrument_Serif({ subsets: ['latin'], weight: '400', style: ['normal', 'italic'], variable: '--font-display' });
const body = Instrument_Sans({ subsets: ['latin'], variable: '--font-body' });
const mono = JetBrains_Mono({ subsets: ['latin'], variable: '--font-mono' });
const hand = Caveat({ subsets: ['latin'], variable: '--font-hand' });

export const metadata: Metadata = {
  title: 'Caresia · Software para clínicas dentales y consultorios médicos',
  description: 'Agenda, expedientes e historial clínico en un solo lugar. Para clínicas dentales, médicos generales y clínicas con varias especialidades.',
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="es" className={`${display.variable} ${body.variable} ${mono.variable} ${hand.variable}`}>
      <body>
        <Providers>
          {children}
        </Providers>
      </body>
    </html>
  )
}
