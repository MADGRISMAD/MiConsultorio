// Early allergy warning while the prescriber picks a medicine. The server repeats the check with the
// same families when the receta is created, so this only has to be good enough to warn in time.

const norm = (s: string) =>
  s
    .toLowerCase()
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .replace(/\s+/g, ' ')
    .trim();

interface Family {
  name: string;
  triggers: string[];
  members: string[];
}

// "^x" = word prefix, "=x" = whole word, otherwise substring
const FAMILIES: Family[] = [
  { name: 'Penicilinas', triggers: ['penicilin', 'betalactam', 'beta lactam', 'amoxicilin', 'ampicilin', 'dicloxacilin'], members: ['penicilin', 'amoxicilin', 'ampicilin', 'dicloxacilin', 'cloxacilin', 'oxacilin', 'piperacilin'] },
  { name: 'Cefalosporinas', triggers: ['cefalosporin', 'betalactam', 'beta lactam', '^cef'], members: ['^cef', 'cefalosporin'] },
  { name: 'Macrólidos', triggers: ['macrolid', 'azitromicin', 'claritromicin', 'eritromicin'], members: ['macrolid', 'azitromicin', 'claritromicin', 'eritromicin'] },
  { name: 'Quinolonas', triggers: ['quinolon', 'floxacin'], members: ['floxacin', 'quinolon'] },
  { name: 'Tetraciclinas', triggers: ['tetraciclin', 'doxiciclin', 'minociclin'], members: ['tetraciclin', 'doxiciclin', 'minociclin'] },
  { name: 'Aminoglucósidos', triggers: ['aminoglucosid', 'gentamicin', 'amikacin', 'tobramicin', 'neomicin'], members: ['aminoglucosid', 'gentamicin', 'amikacin', 'tobramicin', 'neomicin'] },
  { name: 'Lincosamidas', triggers: ['lincosamid', 'clindamicin', 'lincomicin'], members: ['clindamicin', 'lincomicin'] },
  { name: 'Sulfonamidas', triggers: ['=sulfa', '=sulfas', '^sulfonamid', '^sulfametox', '^sulfadiazin'], members: ['^sulfonamid', '^sulfametox', '^sulfadiazin'] },
  { name: 'Nitroimidazoles', triggers: ['nitroimidazol', 'metronidazol', 'tinidazol', 'secnidazol'], members: ['metronidazol', 'tinidazol', 'secnidazol'] },
  {
    name: 'AINEs',
    triggers: ['=aine', '=aines', '^antiinflamat', 'aspirin', 'acido acetilsalicilico', 'salicilat', 'ibuprofen', 'naproxen', 'diclofenac', 'ketorolac', 'meloxicam', 'piroxicam', 'indometacin', 'celecoxib', 'etoricoxib', 'nimesulid', 'ketoprofen'],
    members: ['ibuprofen', 'naproxen', 'diclofenac', 'ketorolac', 'meloxicam', 'piroxicam', 'indometacin', 'celecoxib', 'etoricoxib', 'nimesulid', 'ketoprofen', 'acido acetilsalicilico', 'aspirin', 'firocoxib', 'carprofen', 'robenacoxib']
  },
  { name: 'Pirazolonas (metamizol)', triggers: ['metamizol', 'dipirona', 'pirazolon'], members: ['metamizol', 'dipirona'] },
  { name: 'Paracetamol', triggers: ['paracetamol', 'acetaminofen'], members: ['paracetamol', 'acetaminofen'] },
  { name: 'Opioides', triggers: ['opioide', 'tramadol', 'codein', 'morfin', 'fentanil'], members: ['tramadol', 'codein', 'morfin', 'fentanil', 'opioide'] },
  { name: 'Anestésicos locales', triggers: ['anestesic', 'anestesia local', 'lidocain', 'bupivacain', 'articain', 'mepivacain'], members: ['lidocain', 'bupivacain', 'articain', 'mepivacain'] },
  { name: 'Yodo', triggers: ['yodo', 'iodo', 'povidona'], members: ['povidona', 'yodo'] },
  { name: 'Benzodiacepinas', triggers: ['benzodiacepin', 'diazepam', 'clonazepam', 'alprazolam', 'lorazepam'], members: ['diazepam', 'clonazepam', 'alprazolam', 'lorazepam', 'bromazepam'] }
];

function stem(text: string, s: string): boolean {
  if (s[0] === '^' || s[0] === '=') {
    return text.split(/[^a-z]+/).some((w) => (s[0] === '^' ? w.startsWith(s.slice(1)) : w === s.slice(1)));
  }
  return text.includes(s);
}

const NONE = /^(ninguna|ningun|niega|negada|negadas|no|sin alergias|n\/a|na|desconoce)\b/;

/** Allergy phrases written in the patient's profile ("Penicilina, sulfas y látex" gives three). */
export function patientAllergies(profile: Record<string, unknown> | undefined): string[] {
  const out: string[] = [];
  const add = (s: string) => {
    for (const part of s.split(/[,;\n/]+|\s+y\s+|\s+e\s+|\.\s+/)) {
      const t = part.trim();
      if (t && !NONE.test(norm(t))) out.push(t);
    }
  };
  const p = profile ?? {};
  for (const k of ['allergies_text', 'allergies', 'drug_allergies']) {
    const v = p[k];
    if (typeof v === 'string') add(v);
    else if (Array.isArray(v))
      for (const x of v) {
        if (typeof x === 'string') add(x);
        else if (x && typeof x === 'object') {
          const o = x as Record<string, unknown>;
          const f = ['substance', 'name', 'agent', 'allergen'].find((k2) => typeof o[k2] === 'string');
          if (f) add(o[f] as string);
        }
      }
  }
  if (p.dental_anesthesia_allergy === 'Sí') out.push('Anestésicos locales');
  return out;
}

/** Allergies that match a medicine, with the family when the match is by family. */
export function allergyMatches(allergies: string[], medicine: string, brand = ''): { allergy: string; family: string }[] {
  const name = norm(medicine);
  const drug = norm(`${medicine} ${brand}`);
  const out: { allergy: string; family: string }[] = [];
  if (name.length < 3) return out;
  for (const al of allergies) {
    const a = norm(al);
    if (a.length < 4) continue;
    if (drug.includes(a) || (name.length >= 5 && a.includes(name))) {
      out.push({ allergy: al, family: '' });
      continue;
    }
    const fam = FAMILIES.find((f) => f.triggers.some((t) => stem(a, t)) && f.members.some((m) => stem(drug, m)));
    if (fam) out.push({ allergy: al, family: fam.name });
  }
  return out;
}
