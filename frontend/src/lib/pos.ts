import type { IconName } from './components/ui/Icon.svelte';
import { PERMISSIONS } from './types';

export interface PosLink {
  slug: string;
  title: string;
  icon: IconName;
  summary: string;
  perm: string;
}

/** The windows of the cobros area, in menu order. */
export const POS_LINKS: PosLink[] = [
  { slug: 'cobros', title: 'Punto de venta', icon: 'cash', summary: 'Cobra consultas y productos', perm: PERMISSIONS.pos },
  { slug: 'caja', title: 'Caja', icon: 'wallet', summary: 'Apertura, cortes y movimientos', perm: PERMISSIONS.pos },
  { slug: 'servicios', title: 'Servicios y precios', icon: 'tag', summary: 'Tu catálogo de servicios', perm: PERMISSIONS.pos },
  { slug: 'inventario', title: 'Inventario', icon: 'box', summary: 'Existencias y movimientos', perm: PERMISSIONS.pos },
  { slug: 'facturacion', title: 'Facturación', icon: 'receipt', summary: 'Solicitudes de factura', perm: PERMISSIONS.pos },
  { slug: 'reportes', title: 'Reportes', icon: 'chart', summary: 'Ventas, márgenes y existencias', perm: PERMISSIONS.posReports },
  { slug: 'ajustes', title: 'Ajustes de cobros', icon: 'settings', summary: 'Datos fiscales, ticket e impresora', perm: PERMISSIONS.posManage }
];
