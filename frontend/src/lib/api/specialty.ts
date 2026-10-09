import { request, seg } from '$lib/api';
import type {
  ChartKind,
  ChartList,
  Consent,
  ConsentInput,
  DueVaccination,
  NutritionDay,
  NutritionMeal,
  NutritionPlanData,
  NutritionTargets,
  PatientChart,
  PlanInput,
  PlanItemInput,
  SignatureInput,
  TreatmentPlan,
  Vaccination,
  VaccinationInput,
  VaccinationList,
  WeightPoint
} from '$lib/types/specialty';

const p = (id: string) => `/patients/${seg(id)}`;

export const specialtyApi = {
  vaccinations: (patientId: string) => request<VaccinationList>('GET', `${p(patientId)}/vaccinations`),
  addVaccination: (patientId: string, v: VaccinationInput) =>
    request<{ vaccination: Vaccination }>('POST', `${p(patientId)}/vaccinations`, v).then((r) => r.vaccination),
  voidVaccination: (id: string, reason: string) => request<{ vaccination: Vaccination }>('POST', `/vaccinations/${seg(id)}/void`, { reason }),
  dueVaccinations: (days = 30) => request<{ due: DueVaccination[]; days: number }>('GET', `/vaccinations/due?days=${days}`).then((r) => r.due),
  weights: (patientId: string) => request<{ weights: WeightPoint[] }>('GET', `${p(patientId)}/weights`).then((r) => r.weights),

  charts: (patientId: string, kind: ChartKind) => request<ChartList>('GET', `${p(patientId)}/charts?kind=${kind}`),
  saveChart: (patientId: string, kind: ChartKind, data: unknown, note: string) =>
    request<{ chart: PatientChart }>('POST', `${p(patientId)}/charts`, { kind, data, note }).then((r) => r.chart),

  /** a recommended next visit: lands in the agenda pending confirmation */
  followUp: (patientId: string, body: { date: string; start_hour?: string; reason?: string }) =>
    request<{ appointment: { id: string; date: string; startHour: string } }>('POST', `${p(patientId)}/follow-up`, body).then((r) => r.appointment),
  /** Changes one meal (or a whole day) of the menu with the AI, looking at the rest of the week. Nothing is saved. */
  nutritionFragment: (patientId: string, body: { days: NutritionDay[]; day: number; meal?: number; dislike: string; request: string; dislikes: string }) =>
    request<{ meals: NutritionMeal[]; warnings?: string[] }>('POST', `${p(patientId)}/nutrition-plan/ai/fragment`, body),
  nutritionAI: (patientId: string, body: { goal: string; weight_kg: number; height_cm: number; activity_factor: number; activity: string; kcal: number; meals: number; snacks: boolean; preferences: string; dislikes: string; body_fat_pct?: number }) =>
    request<{ plan: NutritionPlanData; warnings?: string[] }>('POST', `${p(patientId)}/nutrition-plan/ai`, body),

  nutritionCalc: (patientId: string, body: { goal: string; weight_kg: number; height_cm: number; activity_factor: number; body_fat_pct?: number }) =>
    request<NutritionTargets>('POST', `${p(patientId)}/nutrition-plan/calc`, body),

  plans: (patientId: string) => request<{ plans: TreatmentPlan[] }>('GET', `${p(patientId)}/plans`).then((r) => r.plans),
  plan: (id: string) => request<{ plan: TreatmentPlan }>('GET', `/plans/${seg(id)}`).then((r) => r.plan),
  createPlan: (patientId: string, plan: PlanInput) => request<{ plan: TreatmentPlan }>('POST', `${p(patientId)}/plans`, plan).then((r) => r.plan),
  updatePlan: (id: string, plan: PlanInput) => request<{ plan: TreatmentPlan }>('PUT', `/plans/${seg(id)}`, plan).then((r) => r.plan),
  proposePlan: (id: string) => request<{ plan: TreatmentPlan }>('POST', `/plans/${seg(id)}/propose`).then((r) => r.plan),
  acceptPlan: (id: string, sig: SignatureInput) => request<{ plan: TreatmentPlan }>('POST', `/plans/${seg(id)}/accept`, sig).then((r) => r.plan),
  cancelPlan: (id: string, reason: string) => request<{ plan: TreatmentPlan }>('POST', `/plans/${seg(id)}/cancel`, { reason }).then((r) => r.plan),
  addPlanItems: (id: string, items: PlanItemInput[], reason: string) =>
    request<{ plan: TreatmentPlan }>('POST', `/plans/${seg(id)}/items`, { items, reason }).then((r) => r.plan),
  doneItem: (id: string, itemId: string, encounterId?: string) =>
    request<{ plan: TreatmentPlan }>('POST', `/plans/${seg(id)}/items/${seg(itemId)}/done`, encounterId ? { encounter_id: encounterId } : {}).then((r) => r.plan),
  cancelItem: (id: string, itemId: string, reason: string) =>
    request<{ plan: TreatmentPlan }>('POST', `/plans/${seg(id)}/items/${seg(itemId)}/cancel`, { reason }).then((r) => r.plan),

  consents: (patientId: string) => request<{ consents: Consent[] }>('GET', `${p(patientId)}/consents`).then((r) => r.consents),
  consent: (id: string) => request<{ consent: Consent; patient_name: string }>('GET', `/consents/${seg(id)}`),
  signConsent: (patientId: string, c: ConsentInput) => request<{ consent: Consent }>('POST', `${p(patientId)}/consents`, c).then((r) => r.consent)
};
