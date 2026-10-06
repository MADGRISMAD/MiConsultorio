import type { IconName } from './components/ui/Icon.svelte';

export interface PosWindow {
  slug: string;
  title: string;
  icon: IconName;
  summary: string;
  features: { title: string; text: string }[];
}

// Placeholder windows for the billing/POS area. Content comes from the product spec;
// each one is a visible, linkable screen to build on later.
export const POS_WINDOWS: PosWindow[] = [
  {
    slug: 'cobros',
    title: 'Punto de venta',
    icon: 'cash',
    summary: 'Cobra lo que se atendió en la consulta, sin fugas de ingresos.',
    features: [
      { title: 'Pre-cuenta desde la consulta', text: 'El profesional elige los servicios e insumos usados y pasan automáticamente a la caja.' },
      { title: 'Cotizaciones y presupuestos', text: 'Genera presupuestos por fases para tratamientos de varias sesiones.' },
      { title: 'Múltiples métodos de pago', text: 'Efectivo, tarjeta (Stripe, Mercado Pago, terminales físicas) y transferencia SPEI.' },
      { title: 'Cobro ligado al paciente', text: 'Cada venta queda asociada a la atención y al expediente.' }
    ]
  },
  {
    slug: 'caja',
    title: 'Caja',
    icon: 'wallet',
    summary: 'Control del dinero del día, de la apertura al cierre.',
    features: [
      { title: 'Apertura y cierre', text: 'Registra el fondo inicial y cierra la caja al terminar la jornada.' },
      { title: 'Corte de caja y arqueos', text: 'Compara lo esperado contra lo contado, por método de pago.' },
      { title: 'Caja chica', text: 'Entradas y salidas de efectivo con su motivo.' },
      { title: 'Historial de cierres', text: 'Consulta y reimprime los cortes anteriores.' }
    ]
  },
  {
    slug: 'servicios',
    title: 'Servicios y precios',
    icon: 'tag',
    summary: 'Catálogo de servicios según el giro de tu consultorio.',
    features: [
      { title: 'Servicios por giro', text: 'Medicina general: curaciones y certificados. Dental: limpieza y resinas. Veterinaria: vacunas y grooming.' },
      { title: 'Precios sugeridos', text: 'Define el precio base de cada servicio y modifícalo al cobrar.' },
      { title: 'Impuestos y descuentos', text: 'Configura las tasas aplicables y los descuentos permitidos.' },
      { title: 'Categorías', text: 'Organiza el catálogo para encontrar cada servicio rápido.' }
    ]
  },
  {
    slug: 'inventario',
    title: 'Inventario',
    icon: 'box',
    summary: 'Medicamentos e insumos siempre bajo control.',
    features: [
      { title: 'Descuento automático', text: 'Al registrar un procedimiento se descuenta el insumo usado (una vacuna, un kit estéril).' },
      { title: 'Alertas de caducidad', text: 'Aviso antes de que los medicamentos caduquen.' },
      { title: 'Stock mínimo', text: 'Recibe una alerta cuando un insumo se esté agotando.' },
      { title: 'Entradas y ajustes', text: 'Registra compras, mermas y conteos físicos.' }
    ]
  },
  {
    slug: 'facturacion',
    title: 'Facturación',
    icon: 'receipt',
    summary: 'Comprobantes fiscales de tus cobros.',
    features: [
      { title: 'Facturación electrónica', text: 'Integración con el sistema fiscal local (CFDI en México o su equivalente).' },
      { title: 'Datos fiscales del paciente', text: 'Guarda los datos de facturación de quien lo solicite.' },
      { title: 'Comprobantes y cancelaciones', text: 'Emite, consulta y cancela comprobantes desde la venta.' },
      { title: 'Recibos simples', text: 'Comprobante de pago para quien no requiera factura.' }
    ]
  },
  {
    slug: 'reportes',
    title: 'Reportes',
    icon: 'chart',
    summary: 'Cómo va tu consultorio, en números.',
    features: [
      { title: 'Financieros', text: 'Ingresos por periodo, servicios más rentables, corte por método de pago y comisiones.' },
      { title: 'Operativos', text: 'Consultas por especialista, porcentaje de ausentismo y tiempo promedio de atención.' },
      { title: 'Clínicos', text: 'Pacientes nuevos vs. recurrentes y seguimientos pendientes.' },
      { title: 'Exportación', text: 'Descarga los reportes para tu contador.' }
    ]
  }
];
