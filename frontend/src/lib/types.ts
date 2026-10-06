export const PERMISSIONS = {
  adminUsers: 'adminUsers',
  adminAppointments: 'adminAppointments',
  adminHistorials: 'adminHistorials',
  navHistorials: 'navHistorials',
  navAppointments: 'navAppointments'
} as const;

export type Permission = (typeof PERMISSIONS)[keyof typeof PERMISSIONS];

export interface SessionInfo {
  clinicId: string;
  username: string;
  permissions: string[];
}

export const CLINIC_KINDS = {
  GENERAL_MEDICAL: { label: 'Medicina general', hint: 'Consultorio médico' },
  DENTAL: { label: 'Odontología', hint: 'Clínica dental' },
  VETERINARY: { label: 'Veterinaria', hint: 'Clínica veterinaria' },
  CHIROPRACTIC: { label: 'Quiropráctica', hint: 'Quiropráctica y fisioterapia' }
} as const;

export type ClinicKind = keyof typeof CLINIC_KINDS;

export interface Clinic {
  id: string;
  kind: ClinicKind;
  name: string;
  phone_number: string;
  address: string;
  image_url: string;
}

export interface User {
  username: string;
  permissions: string[];
}

export const CHECKBOX_FIELDS = [
  'diabetes',
  'rheumatic_diseases',
  'fractures',
  'allergies',
  'layed',
  'contractures',
  'cancer',
  'accidents',
  'transfusions',
  'cardiopathies',
  'surgeries',
  'tabaquism',
  'alcoholism',
  'automedication',
  'drug_use',
  'pregnant'
] as const;

export type CheckboxField = (typeof CHECKBOX_FIELDS)[number];

export interface ExpedientInput extends Record<CheckboxField, boolean> {
  CURP: string;
  names: string;
  last_names: string;
  sex: 'Hombre' | 'Mujer';
  date_of_birth: string;
  education: string;
  occupation: string;
  weight: string;
  clothes_size: string;
  height: string;
  ethnicity: string;
  physical_activity: string;
  hobbies: string;
  child: string;
}

export interface Expedient extends ExpedientInput {
  id: string;
  age: number;
}

export interface AppointmentInput {
  names: string;
  last_names: string;
  CURP: string;
  date: string;
  startHour: string;
  endHour: string;
  details: string;
}

export interface Appointment extends AppointmentInput {
  id: string;
}

export function emptyExpedient(): ExpedientInput {
  return {
    CURP: '',
    names: '',
    last_names: '',
    sex: 'Hombre',
    date_of_birth: '',
    education: '',
    occupation: '',
    weight: '',
    clothes_size: '',
    height: '',
    ethnicity: '',
    physical_activity: '',
    hobbies: '',
    child: '',
    ...(Object.fromEntries(CHECKBOX_FIELDS.map((f) => [f, false])) as Record<CheckboxField, boolean>)
  };
}

export function emptyAppointment(): AppointmentInput {
  return { names: '', last_names: '', CURP: '', date: '', startHour: '', endHour: '', details: '' };
}
