// Spanish (Mexico): the default language and the source of truth for the key list.
import { es as common } from './dict/common';
import { es as booking } from './dict/booking';
import { es as portal } from './dict/portal';
import { es as arco } from './dict/arco';

export const es: Record<string, string> = { ...common, ...booking, ...portal, ...arco };
