import { faroList } from '$lib/api/faro';
import type { PageLoad } from './$types';

// El encabezado y la lista de artículos salen en el HTML (con enlaces reales); los artículos propios se suman en el navegador.
export const ssr = true;
export const prerender = true;
export const load: PageLoad = async () => ({ faro: await faroList() });
