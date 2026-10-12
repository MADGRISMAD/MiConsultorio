/**
 * Equivalentes del Sistema Mexicano de Alimentos Equivalentes (SMAE): cada grupo aporta una energía y unos nutrimentos por
 * «equivalente» y cada alimento lista la porción que equivale a uno. Son valores de referencia para armar el menú; la
 * edición vigente del SMAE (y la etiqueta del producto) mandan cuando hay diferencia.
 */
export interface SmaeFood {
  name: string;
  /** the amount that is one equivalent */
  portion: string;
}
export interface SmaeGroup {
  id: string;
  name: string;
  /** per equivalent */
  kcal: number;
  protein: number;
  carb: number;
  fat: number;
  foods: SmaeFood[];
}

const f = (name: string, portion: string): SmaeFood => ({ name, portion });

export const SMAE: SmaeGroup[] = [
  {
    id: 'verduras', name: 'Verduras', kcal: 25, protein: 2, carb: 4, fat: 0,
    foods: [f('Acelga cocida', '½ taza'), f('Apio crudo', '1 taza'), f('Brócoli cocido', '½ taza'), f('Calabacita cocida', '½ taza'), f('Champiñones', '1 taza'), f('Chayote cocido', '½ taza'), f('Col cocida', '½ taza'), f('Coliflor cocida', '½ taza'), f('Ejotes cocidos', '½ taza'), f('Espinaca cocida', '½ taza'), f('Jitomate', '1 pieza mediana'), f('Lechuga', '2 tazas'), f('Nopal cocido', '½ taza'), f('Pepino', '1 pieza chica'), f('Zanahoria cruda rallada', '½ taza')]
  },
  {
    id: 'frutas', name: 'Frutas', kcal: 60, protein: 0, carb: 15, fat: 0,
    foods: [f('Manzana', '1 pieza chica'), f('Plátano', '½ pieza'), f('Naranja', '2 piezas'), f('Mandarina', '2 piezas'), f('Papaya picada', '1 taza'), f('Melón picado', '1 taza'), f('Sandía picada', '1 taza'), f('Fresa', '1 taza'), f('Piña picada', '¾ taza'), f('Uvas', '12 piezas'), f('Mango picado', '½ taza'), f('Pera', '½ pieza'), f('Durazno', '2 piezas'), f('Guayaba', '3 piezas'), f('Kiwi', '1 pieza grande')]
  },
  {
    id: 'cereales', name: 'Cereales y tubérculos (sin grasa)', kcal: 70, protein: 2, carb: 15, fat: 0,
    foods: [f('Tortilla de maíz', '1 pieza'), f('Pan de caja integral', '1 rebanada'), f('Bolillo sin migajón', '½ pieza'), f('Arroz cocido', '⅓ taza'), f('Pasta cocida', '½ taza'), f('Avena en hojuelas', '⅓ taza'), f('Papa cocida', '½ pieza mediana'), f('Camote cocido', '⅓ taza'), f('Elote desgranado', '½ taza'), f('Cereal de caja sin azúcar', '½ taza'), f('Palomitas naturales', '2½ tazas'), f('Tostada horneada', '1 pieza'), f('Galleta María', '4 piezas')]
  },
  {
    id: 'cereales-grasa', name: 'Cereales con grasa', kcal: 115, protein: 2, carb: 15, fat: 5,
    foods: [f('Pan dulce (concha)', '½ pieza'), f('Tortilla de harina', '1 pieza chica'), f('Papas fritas', '12 piezas'), f('Galletas saladas con grasa', '5 piezas')]
  },
  {
    id: 'leguminosas', name: 'Leguminosas', kcal: 120, protein: 8, carb: 20, fat: 1,
    foods: [f('Frijol cocido', '½ taza'), f('Lenteja cocida', '½ taza'), f('Garbanzo cocido', '½ taza'), f('Haba cocida', '½ taza'), f('Alubia cocida', '½ taza'), f('Soya texturizada hidratada', '½ taza')]
  },
  {
    id: 'poa-muy-bajo', name: 'Alimentos de origen animal · muy bajo aporte de grasa', kcal: 40, protein: 7, carb: 0, fat: 1,
    foods: [f('Pechuga de pollo sin piel', '30 g'), f('Clara de huevo', '2 piezas'), f('Atún en agua', '30 g'), f('Pescado blanco', '40 g'), f('Camarón cocido', '5 piezas'), f('Pechuga de pavo', '30 g'), f('Queso cottage', '¼ taza')]
  },
  {
    id: 'poa-bajo', name: 'Alimentos de origen animal · bajo aporte de grasa', kcal: 55, protein: 7, carb: 0, fat: 3,
    foods: [f('Huevo entero', '1 pieza'), f('Jamón de pavo', '2 rebanadas'), f('Queso panela', '40 g'), f('Salmón', '35 g'), f('Pollo con piel', '30 g'), f('Bistec de res magro', '30 g')]
  },
  {
    id: 'poa-moderado', name: 'Alimentos de origen animal · moderado aporte de grasa', kcal: 75, protein: 7, carb: 0, fat: 5,
    foods: [f('Carne molida de res', '30 g'), f('Queso Oaxaca', '30 g'), f('Queso fresco', '40 g'), f('Costilla de cerdo magra', '30 g'), f('Chuleta de cerdo', '30 g')]
  },
  {
    id: 'poa-alto', name: 'Alimentos de origen animal · alto aporte de grasa', kcal: 100, protein: 7, carb: 0, fat: 8,
    foods: [f('Chorizo', '30 g'), f('Queso manchego', '30 g'), f('Salchicha', '1 pieza'), f('Tocino', '2 rebanadas')]
  },
  {
    id: 'leche-descremada', name: 'Leche descremada', kcal: 95, protein: 9, carb: 12, fat: 2,
    foods: [f('Leche descremada', '1 taza'), f('Yogur natural descremado', '1 taza'), f('Leche de soya sin azúcar', '1 taza')]
  },
  {
    id: 'leche-entera', name: 'Leche entera', kcal: 150, protein: 9, carb: 12, fat: 8,
    foods: [f('Leche entera', '1 taza'), f('Yogur natural entero', '1 taza')]
  },
  {
    id: 'grasas', name: 'Grasas sin proteína', kcal: 45, protein: 0, carb: 0, fat: 5,
    foods: [f('Aceite de oliva o de canola', '1 cucharadita'), f('Aguacate', '⅓ pieza'), f('Mantequilla', '1 cucharadita'), f('Mayonesa', '1 cucharadita'), f('Crema', '1 cucharada'), f('Aceitunas', '5 piezas')]
  },
  {
    id: 'grasas-proteina', name: 'Grasas con proteína', kcal: 70, protein: 3, carb: 3, fat: 5,
    foods: [f('Almendras', '10 piezas'), f('Nuez', '3 piezas'), f('Cacahuate', '14 piezas'), f('Pistaches', '18 piezas'), f('Semillas de girasol', '1 cucharada'), f('Crema de cacahuate', '1 cucharada')]
  },
  {
    id: 'azucares', name: 'Azúcares sin grasa', kcal: 40, protein: 0, carb: 10, fat: 0,
    foods: [f('Azúcar', '2 cucharaditas'), f('Miel', '2 cucharaditas'), f('Mermelada', '2 cucharaditas'), f('Gelatina regular', '¼ taza'), f('Piloncillo', '1 cucharada')]
  },
  {
    id: 'libres', name: 'Alimentos libres (menos de 20 kcal)', kcal: 0, protein: 0, carb: 0, fat: 0,
    foods: [f('Café sin azúcar', '1 taza'), f('Té sin azúcar', '1 taza'), f('Limón', 'al gusto'), f('Especias y hierbas', 'al gusto'), f('Vinagre', 'al gusto'), f('Refresco de dieta', '1 vaso')]
  }
];

const fold = (s: string) => s.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase();

/** Foods whose name or group contains the text (without accents or capitals). */
export function searchSmae(q: string): { group: SmaeGroup; foods: SmaeFood[] }[] {
  const t = fold(q.trim());
  return SMAE.map((group) => ({ group, foods: !t || fold(group.name).includes(t) ? group.foods : group.foods.filter((x) => fold(x.name).includes(t)) })).filter((x) => x.foods.length);
}

/** The text that goes into a meal when a food is picked. */
export const smaeLine = (food: SmaeFood, group: SmaeGroup) => `${food.portion} de ${food.name.toLowerCase()} (1 eq. ${group.name.split(' · ')[0].toLowerCase()})`;
