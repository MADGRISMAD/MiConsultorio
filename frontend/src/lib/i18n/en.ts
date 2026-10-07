// English: only the public, patient-facing pages are translated. Staff screens and emails stay in Spanish.
import { en as common } from './dict/common';
import { en as booking } from './dict/booking';
import { en as portal } from './dict/portal';
import { en as arco } from './dict/arco';

export const en: Record<string, string> = { ...common, ...booking, ...portal, ...arco };
