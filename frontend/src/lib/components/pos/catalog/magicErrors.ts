import { ApiError } from '$lib/api';

/** Friendly Spanish copy for the Magic (AI) endpoints. */
export function magicMessage(e: unknown): string {
  if (e instanceof ApiError) {
    if (e.code === 'MAGIC_LIMIT') return 'Ya usaste toda la magia incluida en tu plan este mes. Se renueva el mes que entra; mientras tanto puedes capturar o importar tus artículos a mano.';
    if (e.code === 'NOT_CONFIGURED') return 'La magia (inteligencia artificial) todavía no está configurada en el servidor. Avisa a soporte de Caresia; mientras tanto puedes usar “Importar” con una hoja de cálculo.';
    if (e.code === 'PLAN_REQUIRED') return 'Tu plan no incluye esta función. Pide el cambio de plan a tu administrador de Caresia.';
    return e.message;
  }
  return e instanceof Error ? e.message : 'Ocurrió un error inesperado.';
}
