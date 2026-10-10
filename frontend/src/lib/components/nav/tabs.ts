import { PERMISSIONS } from '$lib/types';
import type { Tab } from './SectionTabs.svelte';

/** One sidebar entry, "Agenda", with its views as tabs. */
export const AGENDA_TABS: Tab[] = [
  { label: 'Citas', href: '/admin/navegar-citas', also: ['/admin/admin-citas'], perms: [PERMISSIONS.navAppointments, PERMISSIONS.adminAppointments] },
  { label: 'En proceso', href: '/en-proceso', perms: [PERMISSIONS.navAppointments] },
  { label: 'Lista de espera', href: '/agenda/espera', perms: [PERMISSIONS.navAppointments] }
];

/** One sidebar entry, "Cobros", with the till, the catalog and the back office as tabs. Commissions live in Ajustes. */
export const COBROS_TABS: Tab[] = [
  { label: 'Punto de venta', href: '/pos/cobros', perms: [PERMISSIONS.pos] },
  { label: 'Caja', href: '/pos/caja', perms: [PERMISSIONS.pos] },
  { label: 'Servicios y precios', href: '/pos/servicios', perms: [PERMISSIONS.pos] },
  { label: 'Inventario', href: '/pos/inventario', perms: [PERMISSIONS.pos] },
  { label: 'Facturación', href: '/pos/facturacion', perms: [PERMISSIONS.pos] },
  { label: 'Cuentas por cobrar', href: '/pos/cuentas', perms: [PERMISSIONS.pos] },
  { label: 'Reportes de ventas', href: '/pos/reportes', perms: [PERMISSIONS.posReports] }
];

/** One sidebar entry, "Reportes". */
export const REPORT_TABS: Tab[] = [
  { label: 'Indicadores', href: '/indicadores', perms: [PERMISSIONS.adminUsers] },
  { label: 'Operación y pacientes', href: '/reportes', perms: [PERMISSIONS.adminUsers, PERMISSIONS.navHistorials] }
];
