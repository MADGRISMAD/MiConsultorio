import type { PatientRow, Subject } from '$lib/types';

/** Whole years between an ISO date (yyyy-mm-dd) and today; null when the date is empty or invalid. */
export function ageFrom(birth: string): number | null {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(birth)) return null;
  const b = new Date(birth + 'T12:00:00');
  if (isNaN(b.getTime())) return null;
  const now = new Date();
  if (b > now) return null;
  let age = now.getFullYear() - b.getFullYear();
  if (now.getMonth() < b.getMonth() || (now.getMonth() === b.getMonth() && now.getDate() < b.getDate())) age--;
  return age;
}

export function ageText(age: number | null | undefined): string {
  if (age == null) return '';
  if (age < 1) return 'menos de 1 año';
  return age === 1 ? '1 año' : `${age} años`;
}

export const fullName = (p: { names: string; last_names: string }) => `${p.names} ${p.last_names}`.trim();

export const SUBJECT_LABEL: Record<Subject, string> = { person: 'Persona', animal: 'Animal' };

/** Second line under a patient's name: age and species for animals (with owner), age for people. */
export function subtitle(p: PatientRow, withSpecies = true): string {
  const parts: string[] = [];
  if (p.subject === 'animal') {
    if (withSpecies && p.species) parts.push(p.species);
    const a = ageText(p.age);
    if (a) parts.push(a);
    if (p.guardian_name) parts.push(`dueño: ${p.guardian_name}`);
  } else {
    const a = ageText(p.age);
    if (a) parts.push(a);
  }
  return parts.join(' · ');
}
