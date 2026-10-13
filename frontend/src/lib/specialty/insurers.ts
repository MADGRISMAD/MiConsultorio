/** Aseguradoras de uso común en México, para elegirlas en vez de escribirlas. El nombre guardado es el de `name`. */
export interface Insurer {
  id: string;
  name: string;
  /** letras del distintivo cuando no hay logo en /seguros/<id>.svg */
  mark: string;
  color: string;
}

export const INSURERS: Insurer[] = [
  { id: 'gnp', name: 'GNP Seguros', mark: 'GNP', color: '#F26B21' },
  { id: 'axa', name: 'AXA', mark: 'AXA', color: '#00008F' },
  { id: 'metlife', name: 'MetLife', mark: 'ML', color: '#0090DA' },
  { id: 'monterrey', name: 'Seguros Monterrey New York Life', mark: 'SM', color: '#0B5FA5' },
  { id: 'allianz', name: 'Allianz', mark: 'AZ', color: '#003781' },
  { id: 'zurich', name: 'Zurich', mark: 'Z', color: '#2167AE' },
  { id: 'mapfre', name: 'Mapfre', mark: 'MP', color: '#D81E05' },
  { id: 'atlas', name: 'Seguros Atlas', mark: 'AT', color: '#004B8D' },
  { id: 'inbursa', name: 'Seguros Inbursa', mark: 'IN', color: '#00529B' },
  { id: 'bbva', name: 'BBVA Seguros', mark: 'BB', color: '#072146' },
  { id: 'banorte', name: 'Seguros Banorte', mark: 'BN', color: '#EB0029' },
  { id: 'sura', name: 'SURA', mark: 'SU', color: '#0033A0' },
  { id: 'hdi', name: 'HDI Seguros', mark: 'HDI', color: '#006A4D' },
  { id: 'plan-seguro', name: 'Plan Seguro', mark: 'PS', color: '#0077B6' },
  { id: 'chubb', name: 'Chubb', mark: 'CH', color: '#3B0F6B' },
  { id: 'bupa', name: 'Bupa México', mark: 'BP', color: '#0079C8' },
  { id: 'general-de-seguros', name: 'General de Seguros', mark: 'GS', color: '#1B4F9C' },
  { id: 've-por-mas', name: 'Ve por Más Seguros', mark: 'VM', color: '#E2231A' }
];

const fold = (s: string) => s.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase().trim();

/** The catalog entry of a saved name (accents and capitals do not matter), if it is one of the known ones. */
export function insurerByName(name: string): Insurer | undefined {
  const n = fold(name);
  return INSURERS.find((i) => fold(i.name) === n || i.id === n || fold(i.mark) === n);
}
