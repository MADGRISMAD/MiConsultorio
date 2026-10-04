import type { Metadata } from 'next'
import './globals.css'
import Providers from './Provider';

export const metadata: Metadata = {
  title: 'Caresia',
  description: 'Pacientes, citas e historiales clínicos en un solo lugar. Para clínicas dentales y consultorios médicos.',
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="es">
      <body>
        <Providers>
          {children}
        </Providers>
      </body>
    </html>
  )
}
