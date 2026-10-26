// Qué rutas son la app (con datos de pacientes y de la clínica) y no deben salir en buscadores.
// Las páginas públicas (portada, blog, directorio, legales y la página de cada consultorio) no están aquí.
const PRIVATE = new Set([
  'admin', 'agenda', 'ajustes', 'arco-solicitudes', 'avisos', 'baja', 'bienvenida', 'cita', 'clinica', 'configuracion', 'cuenta', 'demo',
  'en-proceso', 'encuesta', 'equipo', 'espera', 'forgot', 'indicadores', 'login', 'offline', 'organizacion', 'pacientes', 'plataforma',
  'portal', 'pos', 'recetas', 'register', 'reportes', 'reservar', 'restablecer', 'suscripcion', 'verificar'
]);

export const isPrivateRoute = (pathname: string) => PRIVATE.has(pathname.split('/')[1] ?? '');
