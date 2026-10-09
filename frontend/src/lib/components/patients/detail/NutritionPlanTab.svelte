<script lang="ts">
  import { onMount } from 'svelte';
  import { specialtyApi } from '$lib/api/specialty';
  import { Op } from '$lib/op.svelte';
  import { printNutritionPlan } from '$lib/print';
  import { toast } from '$lib/toast.svelte';
  import type { Encounter, Patient } from '$lib/types';
  import type { NutritionDay, NutritionMeal, NutritionPlanData, PatientChart } from '$lib/types/specialty';
  import EmptyState from '../../ui/EmptyState.svelte';
  import Icon from '../../ui/Icon.svelte';

  let { patient, canWrite, encounters = [] }: { patient: Patient; canWrite: boolean; encounters?: Encounter[] } = $props();

  const WEEK = ['Lunes', 'Martes', 'Miércoles', 'Jueves', 'Viernes', 'Sábado', 'Domingo'];
  const empty = (): NutritionPlanData => ({ goal: '', basis: '', kcal: 0, protein_pct: 0, carb_pct: 0, fat_pct: 0, water_liters: 0, meals: [], days: [], dislikes: '', recommendations: '', avoid: '', supplements: '', follow_up_days: 0 });
  const copy = <T,>(v: T): T => JSON.parse(JSON.stringify(v));
  /** plans saved before the weekly format become a single typical day */
  function normalize(d: NutritionPlanData): NutritionPlanData {
    const x = { ...empty(), ...copy(d) };
    if (!x.days?.length && x.meals?.length) x.days = [{ name: 'Día tipo', meals: x.meals }];
    x.meals = [];
    return x;
  }
  const asData = (c: PatientChart) => normalize(c.data as NutritionPlanData);
  const dt = (iso: string) => new Date(iso).toLocaleString('es-MX', { day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false });

  let history = $state<PatientChart[]>([]);
  let loading = $state(true);
  let error = $state('');
  let work = $state<NutritionPlanData>(empty());
  let base = $state('');
  let editing = $state(false);
  let viewing = $state<string | null>(null);
  let note = $state('');
  const saveOp = new Op();
  let scheduleFollow = $state(true);

  // ---- generator: energy needs from the patient's own data ----
  const lastMeasure = (key: string) => {
    const e = [...encounters].filter((x) => !x.hidden && Number(x.measures?.[key]) > 0).sort((a, b) => b.occurred_at.localeCompare(a.occurred_at))[0];
    return e ? String(e.measures[key]) : '';
  };
  const GOALS = ['Bajar de peso', 'Mantener', 'Subir de peso', 'Ganar masa muscular', 'Control de enfermedad', 'Alimentación saludable'];
  const GOAL_CHOICES = [...GOALS, 'Otro (escribir)'];
  const ACTIVITY: { v: number; label: string }[] = [
    { v: 1.2, label: 'Sedentario (casi no se mueve)' },
    { v: 1.375, label: 'Ligera (1 a 3 días a la semana)' },
    { v: 1.55, label: 'Moderada (3 a 5 días)' },
    { v: 1.725, label: 'Intensa (6 a 7 días)' }
  ];
  // carbohydrates, proteins, fats
  const MACROS: Record<string, [number, number, number]> = {
    'Bajar de peso': [45, 25, 30],
    'Ganar masa muscular': [45, 30, 25],
    'Control de enfermedad': [45, 20, 35]
  };
  const ADJUST: Record<string, number> = { 'Bajar de peso': -500, 'Subir de peso': 400, 'Ganar masa muscular': 300 };
  /** the day's structure by number of meals (the server uses the same one) */
  const SLOTS: Record<number, [string, string, number][]> = {
    3: [['Desayuno', '08:00', 30], ['Comida', '14:30', 40], ['Cena', '20:30', 30]],
    5: [['Desayuno', '08:00', 25], ['Snack', '11:00', 10], ['Comida', '14:30', 30], ['Snack', '17:30', 10], ['Cena', '20:30', 25]]
  };
  const OTHER = 'Otro (escribir)';

  let gGoal = $state('Alimentación saludable');
  let gCustom = $state('');
  let gWeight = $state('');
  let gHeight = $state('');
  let gActivity = $state(1.375);
  let gSnacks = $state(true);
  let gPrefs = $state('');
  let gDislikes = $state('');
  let gOpen = $state(false);
  let gError = $state('');
  const aiOp = new Op();
  const goalText = $derived(gGoal === OTHER ? gCustom.trim() : gGoal);

  const bmi = $derived.by(() => {
    const w = Number(gWeight.replace(',', '.'));
    const h = Number(gHeight.replace(',', '.')) / 100;
    if (!(w > 0) || !(h > 0.3)) return null;
    const v = w / (h * h);
    const label = v < 18.5 ? 'bajo peso' : v < 25 ? 'peso normal' : v < 30 ? 'sobrepeso' : 'obesidad';
    return { value: v.toFixed(1), label };
  });

  function openGenerator() {
    gWeight = lastMeasure('weight_kg');
    gHeight = lastMeasure('height_cm');
    const pa = String(patient.profile?.physical_activity ?? '');
    gActivity = pa === 'Ninguna' ? 1.2 : pa === 'Moderada' ? 1.55 : pa === 'Intensa' ? 1.725 : 1.375;
    const pg = String(patient.profile?.nutrition_goal ?? '');
    gGoal = GOALS.includes(pg) ? pg : GOALS.includes(gGoal) ? gGoal : 'Alimentación saludable';
    gDislikes = work.dislikes || gDislikes;
    gError = '';
    aiOp.reset();
    gOpen = true;
  }

  /** Mifflin-St Jeor resting energy, times activity, adjusted for the goal (the server does the same for the AI). */
  function target(): { kcal: number; bmr: number; w: number; h: number } | null {
    const w = Number(gWeight.replace(',', '.'));
    const h = Number(gHeight.replace(',', '.'));
    const age = patient.age;
    if (!(w > 0) || !(h > 0) || age == null) {
      gError = 'Escribe el peso y la talla (y registra la fecha de nacimiento del paciente) para calcular.';
      return null;
    }
    const sexTerm = patient.sex === 'Hombre' ? 5 : patient.sex === 'Mujer' ? -161 : -78;
    const bmr = 10 * w + 6.25 * h - 5 * age + sexTerm;
    const floor = patient.sex === 'Hombre' ? 1500 : 1200;
    return { kcal: Math.max(floor, Math.round((bmr * gActivity + (ADJUST[goalText] ?? 0)) / 10) * 10), bmr: Math.round(bmr), w, h };
  }

  /** the 7 days with the day's structure and the calories of each meal; the foods are written by the nutritionist */
  function skeleton(kcal: number): NutritionDay[] {
    const slots = SLOTS[gSnacks ? 5 : 3];
    return WEEK.map((name) => {
      let left = kcal;
      const meals: NutritionMeal[] = slots.map(([n, time, pct], i) => {
        const k = i === slots.length - 1 ? left : Math.round((kcal * pct) / 1000) * 10;
        left -= k;
        return { name: n, time, items: '', kcal: k };
      });
      return { name, meals };
    });
  }

  function generate() {
    const t = target();
    if (!t) return;
    const [carb, prot, fat] = MACROS[goalText] ?? [50, 20, 30];
    work = {
      ...work,
      goal: goalText || gGoal,
      kcal: t.kcal,
      carb_pct: carb,
      protein_pct: prot,
      fat_pct: fat,
      water_liters: Math.min(4, Math.max(1.5, Math.round(t.w * 0.035 * 10) / 10)),
      days: work.days.length === 7 ? work.days : skeleton(t.kcal),
      dislikes: gDislikes.trim(),
      follow_up_days: work.follow_up_days || 30,
      basis: `Calculado con la fórmula de Mifflin-St Jeor: metabolismo basal ${t.bmr} kcal × actividad ${gActivity}, ajustado al objetivo (${ADJUST[goalText] ?? 0} kcal). Es una estimación: ajústala con tu criterio clínico.`
    };
    
    editing = true;
    gOpen = false;
  }

  /** The server computes the calories and drafts the 7 days with the AI, avoiding the foods the patient dislikes. */
  async function generateAI() {
    if (!goalText) {
      gError = 'Escribe el objetivo del plan.';
      return;
    }
    gError = '';
    if (!((Number(gWeight.replace(',', '.')) > 0) && (Number(gHeight.replace(',', '.')) > 0) && patient.age != null)) {
      gError = 'Escribe el peso y la talla (y registra la fecha de nacimiento del paciente): con ellos se calculan las calorías.';
      return;
    }
    let drafted: NutritionPlanData | undefined;
    let warn: string[] = [];
    const ok = await aiOp.run(async () => {
      const r = await specialtyApi.nutritionAI(patient.id, {
        goal: goalText,
        weight_kg: Number(gWeight.replace(',', '.')) || 0,
        height_cm: Number(gHeight.replace(',', '.')) || 0,
        activity_factor: gActivity,
        activity: ACTIVITY.find((a) => a.v === gActivity)?.label ?? '',
        kcal: 0,
        meals: gSnacks ? 5 : 3,
        snacks: gSnacks,
        preferences: gPrefs.trim(),
        dislikes: gDislikes.trim()
      });
      drafted = r.plan;
      warn = r.warnings ?? [];
    });
    if (!ok || !drafted) return;
    work = normalize(drafted);
    
    editing = true;
    gOpen = false;
    toast.show(warn.length ? `Borrador listo, pero revisa estos alimentos: ${warn.join('; ')}` : 'Borrador de la semana listo: revísalo y ajústalo antes de guardar', warn.length ? 'error' : undefined);
  }

  async function load() {
    try {
      const r = await specialtyApi.charts(patient.id, 'nutrition_plan');
      history = r.charts;
      work = r.latest ? asData(r.latest) : empty();
      base = JSON.stringify(work);
      error = '';
    } catch (e) {
      error = e instanceof Error ? e.message : 'No se pudo cargar el plan nutricional.';
    } finally {
      loading = false;
    }
  }
  onMount(load);

  const viewed = $derived(viewing ? history.find((h) => h.id === viewing) : undefined);
  const shown = $derived<NutritionPlanData>(viewed ? asData(viewed) : work);
  const dirty = $derived(JSON.stringify(work) !== base);
  const readonly = $derived(!canWrite || !editing || !!viewed);
  const grams = (pct: number, per: number) => (shown.kcal && pct ? Math.round((shown.kcal * pct) / 100 / per) : 0);
  const macroSum = $derived(shown.protein_pct + shown.carb_pct + shown.fat_pct);

  const num = (v: string) => {
    const n = Number(v.replace(',', '.'));
    return Number.isFinite(n) && n > 0 ? n : 0;
  };
  /** the columns of the weekly grid are the meals of the day, by position */
  const cols = $derived((shown.days[0]?.meals ?? []).map((m) => m.name));
  let wide = $state(true);
  onMount(() => {
    const mq = window.matchMedia('(min-width: 900px)');
    const upd = () => (wide = mq.matches);
    upd();
    mq.addEventListener('change', upd);
    return () => mq.removeEventListener('change', upd);
  });
  const dayTotal = (d: NutritionDay) => d.meals.reduce((s, m) => s + (Number(m.kcal) || 0), 0);

  function setMeal(di: number, mi: number, patch: Partial<NutritionMeal>) {
    work = { ...work, days: work.days.map((d, j) => (j === di ? { ...d, meals: d.meals.map((m, k) => (k === mi ? { ...m, ...patch } : m)) } : d)) };
  }
  /** renames a column in every day */
  function renameColumn(mi: number, name: string) {
    work = { ...work, days: work.days.map((d) => ({ ...d, meals: d.meals.map((m, k) => (k === mi ? { ...m, name } : m)) })) };
  }
  function addColumn() {
    if (!work.days.length) work = { ...work, days: [{ name: 'Día tipo', meals: [] }] };
    if ((work.days[0]?.meals.length ?? 0) >= 8) return;
    work = { ...work, days: work.days.map((d) => ({ ...d, meals: [...d.meals, { name: 'Snack', time: '', items: '', kcal: 0 }] })) };
  }
  function removeColumn(mi: number) {
    work = { ...work, days: work.days.map((d) => ({ ...d, meals: d.meals.filter((_, k) => k !== mi) })) };
  }
  /** the first day's meals become the plan of every day of the week */
  function copyToAll() {
    const src = work.days[0];
    if (!src) return;
    work = { ...work, days: WEEK.map((name) => ({ name, meals: copy(src.meals) })) };
    toast.show('El primer día se copió a toda la semana');
  }
  function useAsBase(c: PatientChart) {
    work = asData(c);
    viewing = null;
    editing = true;
    
    toast.show('Cargado como borrador: guarda para crear una nueva versión');
  }
  function startNew() {
    work = empty();
    viewing = null;
    editing = true;
    
    openGenerator();
  }

  /** the follow-up the plan recommends lands in the agenda, pending confirmation */
  async function scheduleFollowUp(days: number) {
    const d = new Date(Date.now() + days * 86400000);
    const date = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
    try {
      const a = await specialtyApi.followUp(patient.id, { date, reason: 'Seguimiento del plan nutricional' });
      toast.show(`Cita de seguimiento agendada el ${a.date} a las ${a.startHour} (por confirmar)`);
    } catch (e) {
      toast.show(`El plan quedó guardado, pero la cita de seguimiento no: ${e instanceof Error ? e.message : 'inténtalo de nuevo'}`, 'error');
    }
  }

  async function save() {
    const data: NutritionPlanData = {
      ...work,
      kcal: Math.round(work.kcal || 0),
      meals: [],
      dislikes: work.dislikes.trim(),
      days: work.days.map((d) => ({ ...d, meals: d.meals.filter((m) => m.name.trim()).map((m) => ({ ...m, name: m.name.trim(), kcal: Math.round(Number(m.kcal) || 0) })) }))
    };
    if (!data.goal.trim() && !data.kcal && !data.days.some((d) => d.meals.length) && !data.recommendations.trim()) {
      saveOp.fail('Agrega al menos el objetivo, las calorías o una comida.');
      return;
    }
    if (macroSum > 100) {
      saveOp.fail('Los porcentajes de proteínas, carbohidratos y grasas suman más de 100 %.');
      return;
    }
    if (await saveOp.run(() => specialtyApi.saveChart(patient.id, 'nutrition_plan', data, note.trim()))) {
      toast.show('Plan nutricional guardado');
      if (scheduleFollow && data.follow_up_days > 0) await scheduleFollowUp(data.follow_up_days);
      note = '';
      editing = false;
      viewing = null;
      await load();
    }
  }
  async function print() {
    const v = viewed;
    try {
      await printNutritionPlan(patient, { data: shown, note: v?.note ?? note, at: v?.created_at ?? new Date().toISOString(), by: v?.created_by_name ?? '' });
    } catch (e) {
      toast.show(e instanceof Error ? e.message : 'No se pudo imprimir.', 'error');
    }
  }
</script>

{#if loading}
  <div class="card h-64 animate-pulse"></div>
{:else if error}
  <p class="alert" role="alert"><Icon name="alert" size={18} />{error}</p>
{:else}
  <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
    <p class="min-w-0 max-w-xl flex-1 basis-64 text-sm text-app-muted">Plan de 7 días: calcula las calorías con los datos del paciente, arma el menú de la semana (a mano o con IA) y entrégaselo impreso. Cada versión guardada se conserva en el historial.</p>
    <div class="flex flex-wrap gap-2">
      {#if canWrite && !editing && !viewed}
        {#if history.length}
          <button type="button" class="btn-secondary" onclick={() => (editing = true)}><Icon name="edit" size={18} />Editar plan</button>
          <button type="button" class="btn-secondary" onclick={startNew}><Icon name="plus" size={18} />Plan nuevo</button>
        {:else}
          <button type="button" class="btn-primary" onclick={() => { editing = true; openGenerator(); }}><Icon name="sparkles" size={18} />Generar plan nutricional</button>
        {/if}
      {/if}
      {#if history.length || editing}<button type="button" class="btn-secondary" onclick={print}><Icon name="receipt" size={18} />Imprimir</button>{/if}
    </div>
  </div>

  {#if viewed}
    <div class="mb-3 flex flex-wrap items-center justify-between gap-2 rounded-2xl bg-app-warning/12 px-4 py-3 text-sm" role="status">
      <span>Versión del {dt(viewed.created_at)} por {viewed.created_by_name}{viewed.note ? `: ${viewed.note}` : ''}. Solo lectura.</span>
      <button type="button" class="btn-secondary" onclick={() => (viewing = null)}>Volver al plan vigente</button>
    </div>
  {/if}

  {#if !history.length && !editing}
    <div class="card"><EmptyState icon="leaf" title="Sin plan nutricional" text={canWrite ? 'Genera el primer plan: calcula las calorías con el peso y la talla del paciente y arma el menú de los 7 días.' : 'Todavía no hay un plan nutricional para este paciente.'}>
      {#if canWrite}<button type="button" class="btn-primary" onclick={() => { editing = true; openGenerator(); }}><Icon name="sparkles" size={18} />Generar plan nutricional</button>{/if}
    </EmptyState></div>
  {:else}
    {#if gOpen}
      <section class="card mb-4 p-4 sm:p-5" aria-labelledby="gen-h">
        <h3 id="gen-h" class="display text-xl">Calcular necesidades</h3>
        <p class="mt-1 text-sm text-app-muted">Las calorías se calculan con la fórmula de Mifflin-St Jeor. Con IA se arma además el menú de los 7 días, sin los alimentos que no le gustan al paciente.</p>
        <div class="mt-3 grid gap-3 sm:grid-cols-2">
          <div><label class="label" for="g-goal">Objetivo</label>
            <select id="g-goal" class="field" bind:value={gGoal}>{#each GOAL_CHOICES as g}<option>{g}</option>{/each}</select>
            {#if gGoal === OTHER}<input class="field mt-2" maxlength="200" bind:value={gCustom} placeholder="Ej. Control de colesterol, embarazo, deportista de resistencia" aria-label="Objetivo del plan" />{/if}</div>
          <div><label class="label" for="g-act">Actividad física</label>
            <select id="g-act" class="field" bind:value={gActivity}>{#each ACTIVITY as a}<option value={a.v}>{a.label}</option>{/each}</select></div>
          <div><label class="label" for="g-w">Peso (kg)</label><input id="g-w" class="field" inputmode="decimal" bind:value={gWeight} /></div>
          <div><label class="label" for="g-h">Talla (cm)</label><input id="g-h" class="field" inputmode="decimal" bind:value={gHeight} /></div>
        </div>
        <div class="mt-3"><label class="label" for="g-dislikes">Alimentos que no le gustan</label>
          <textarea id="g-dislikes" class="field min-h-20" rows="2" maxlength="1000" bind:value={gDislikes} placeholder="Ej. pescado, hígado, brócoli, leche (separados por comas)"></textarea>
          <p class="hint">La IA no los usa en ningún día de la semana y busca sustitutos parecidos.</p></div>
        <div class="mt-3"><label class="label" for="g-prefs">Otras preferencias o restricciones <span class="font-normal text-app-muted">(opcional)</span></label>
          <textarea id="g-prefs" class="field min-h-20" rows="2" maxlength="1500" bind:value={gPrefs} placeholder="Ej. Vegetariano, presupuesto bajo, come fuera de casa al mediodía"></textarea></div>
        <label class="check-row mt-3">
          <input type="checkbox" class="check" bind:checked={gSnacks} />
          <span class="text-sm leading-snug"><strong class="font-semibold">Incluir snacks</strong> entre comidas <span class="text-app-muted">(con snacks: desayuno, snack, comida, snack y cena; sin ellos: desayuno, comida y cena)</span></span>
        </label>
        {#if bmi}<p class="hint">Índice de masa corporal: <strong>{bmi.value}</strong> ({bmi.label}).</p>{/if}
        <p class="hint">{patient.age != null ? `${patient.age} años · ${patient.sex || 'sexo sin registrar'}` : 'El paciente no tiene fecha de nacimiento registrada.'}. El peso y la talla se toman de la última nota que los tenga.</p>
        {#if gError}<p class="alert mt-3" role="alert"><Icon name="alert" size={18} />{gError}</p>{/if}
        {#if aiOp.phase === 'error'}<p class="alert mt-3" role="alert"><Icon name="alert" size={18} />{aiOp.message}</p>{/if}
        <p class="hint">La IA usa la edad, el sexo, el peso y los antecedentes alimentarios del paciente; no se envía su nombre. Tarda unos segundos y usa uno de los usos de IA de tu plan. Siempre revísalo antes de guardarlo.</p>
        <div class="mt-3 flex flex-wrap gap-2">
          <button type="button" class="btn-primary" disabled={aiOp.phase === 'loading'} onclick={generateAI}>{#if aiOp.phase === 'loading'}<span class="spin"></span>Armando la semana…{:else}<Icon name="sparkles" size={18} />Generar semana con IA{/if}</button>
          <button type="button" class="btn-secondary" disabled={aiOp.phase === 'loading'} onclick={generate}>Calcular y armar a mano</button>
          <button type="button" class="btn-ghost" onclick={() => (gOpen = false)}>Llenar a mano</button>
        </div>
      </section>
    {:else if !readonly}
      <button type="button" class="btn-secondary mb-4" onclick={openGenerator}><Icon name="sparkles" size={18} />Recalcular o generar con IA</button>
    {/if}

    <section class="card p-4 sm:p-5">
      <h3 class="display text-xl">Objetivo y energía</h3>
      <div class="mt-3 grid gap-3 sm:grid-cols-2">
        <!-- while the calculator is open it already asks for the goal and the disliked foods -->
        {#if !gOpen}
          <div class="sm:col-span-2"><label class="label" for="np-goal">Objetivo del plan</label>
            <input id="np-goal" class="field" maxlength="300" readonly={readonly} value={shown.goal} oninput={(e) => (work.goal = e.currentTarget.value)} placeholder="Ej. Bajar 6 kg en 3 meses" /></div>
        {/if}
        <div><label class="label" for="np-kcal">Calorías al día</label>
          <input id="np-kcal" class="field" inputmode="numeric" readonly={readonly} value={shown.kcal || ''} oninput={(e) => (work.kcal = Math.round(num(e.currentTarget.value)))} /></div>
        <div><label class="label" for="np-water">Agua al día (litros)</label>
          <input id="np-water" class="field" inputmode="decimal" readonly={readonly} value={shown.water_liters || ''} oninput={(e) => (work.water_liters = num(e.currentTarget.value))} /></div>
        <div><label class="label" for="np-carb">Carbohidratos (%)</label>
          <input id="np-carb" class="field" inputmode="numeric" readonly={readonly} value={shown.carb_pct || ''} oninput={(e) => (work.carb_pct = Math.round(num(e.currentTarget.value)))} />
          {#if shown.kcal && shown.carb_pct}<p class="hint">{grams(shown.carb_pct, 4)} g al día</p>{/if}</div>
        <div><label class="label" for="np-prot">Proteínas (%)</label>
          <input id="np-prot" class="field" inputmode="numeric" readonly={readonly} value={shown.protein_pct || ''} oninput={(e) => (work.protein_pct = Math.round(num(e.currentTarget.value)))} />
          {#if shown.kcal && shown.protein_pct}<p class="hint">{grams(shown.protein_pct, 4)} g al día</p>{/if}</div>
        <div><label class="label" for="np-fat">Grasas (%)</label>
          <input id="np-fat" class="field" inputmode="numeric" readonly={readonly} value={shown.fat_pct || ''} oninput={(e) => (work.fat_pct = Math.round(num(e.currentTarget.value)))} />
          {#if shown.kcal && shown.fat_pct}<p class="hint">{grams(shown.fat_pct, 9)} g al día</p>{/if}</div>
        <div><label class="label" for="np-fu">Siguiente cita (en días)</label>
          <input id="np-fu" class="field" inputmode="numeric" readonly={readonly} value={shown.follow_up_days || ''} oninput={(e) => (work.follow_up_days = Math.round(num(e.currentTarget.value)))} /></div>
        {#if !gOpen}
          <div class="sm:col-span-2"><label class="label" for="np-dislikes">Alimentos que no le gustan</label>
            <input id="np-dislikes" class="field" maxlength="1000" readonly={readonly} value={shown.dislikes} oninput={(e) => (work.dislikes = e.currentTarget.value)} placeholder="Ej. pescado, hígado, brócoli" /></div>
        {/if}
      </div>
      {#if macroSum > 100}<p class="alert mt-3" role="alert"><Icon name="alert" size={18} />Los porcentajes suman {macroSum} %: no pueden pasar de 100 %.</p>
      {:else if macroSum > 0 && macroSum < 100}<p class="hint">Los macronutrientes suman {macroSum} %.</p>{/if}
      {#if shown.basis}<p class="hint">{shown.basis}</p>{/if}
    </section>

    <section class="card mt-4 p-4 sm:p-5">
      <div class="flex flex-wrap items-baseline justify-between gap-2">
        <h3 class="display text-xl">{shown.days.length > 1 ? 'Alimentación semanal' : 'Menú del día'}</h3>
        {#if shown.kcal}<p class="text-sm text-app-muted">Meta: {shown.kcal} kcal al día</p>{/if}
      </div>
      {#if shown.days.length === 0 || cols.length === 0}
        <p class="mt-3 text-sm text-app-muted">Sin menú todavía. {readonly ? '' : 'Usa «Generar semana con IA» o «Calcular y armar a mano».'}</p>
      {/if}

      {#snippet cell(di: number, mi: number)}
        {@const m = shown.days[di]?.meals[mi]}
        {#if m}
          <textarea class="field min-h-[5.5rem] !px-2.5 !py-2 text-[13px] leading-snug [field-sizing:content]" rows="4" maxlength="1200" readonly={readonly} value={m.items} aria-label="{shown.days[di].name}, {m.name}" oninput={(e) => setMeal(di, mi, { items: e.currentTarget.value })} placeholder={readonly ? '' : 'Alimentos y porciones'}></textarea>
          <label class="mt-1 flex items-center gap-1 text-[11px] text-app-muted"><input class="w-14 rounded-md border border-app-ink/15 bg-app-panel px-1.5 py-0.5 text-right text-[11px] text-app-ink" inputmode="numeric" readonly={readonly} value={m.kcal || ''} aria-label="Calorías de {shown.days[di].name}, {m.name}" oninput={(e) => setMeal(di, mi, { kcal: Math.round(num(e.currentTarget.value)) })} />kcal</label>
        {/if}
      {/snippet}

      {#if cols.length > 0 && wide}
        <div class="mt-3 overflow-x-auto rounded-xl border border-app-ink/10">
          <table class="w-full min-w-[52rem] table-fixed border-collapse text-sm">
            <caption class="sr-only">Plan de alimentación semanal</caption>
            <thead>
              <tr class="bg-app-elevated">
                <th class="w-24 px-2 py-2"></th>
                {#each cols as c, ci}
                  <th class="border-l border-app-ink/10 px-1.5 py-2 text-center">
                    {#if readonly}<span class="text-xs font-semibold uppercase tracking-wide">{c}</span>
                    {:else}
                      <span class="flex items-center gap-1">
                        <input class="w-full min-w-0 rounded-md bg-transparent px-1 text-center text-xs font-semibold uppercase tracking-wide hover:bg-app-ink/5 focus:bg-app-panel" value={c} maxlength="60" aria-label="Nombre de la comida {ci + 1}" oninput={(e) => renameColumn(ci, e.currentTarget.value)} />
                        {#if cols.length > 1}<button type="button" class="icon-btn danger !h-6 !w-6 flex-none" title="Quitar esta comida de todos los días" aria-label="Quitar {c}" onclick={() => removeColumn(ci)}><Icon name="x" size={13} /></button>{/if}
                      </span>
                    {/if}
                  </th>
                {/each}
                <th class="w-20 border-l border-app-ink/10 px-1.5 py-2 text-center text-xs font-semibold uppercase tracking-wide">Total</th>
              </tr>
            </thead>
            <tbody>
              {#each shown.days as d, di}
                <tr class="border-t border-app-ink/10">
                  <th scope="row" class="px-2 py-2 text-left align-top text-sm font-semibold">{d.name}</th>
                  {#each cols as _, ci}<td class="border-l border-app-ink/10 p-1.5 align-top">{@render cell(di, ci)}</td>{/each}
                  <td class="border-l border-app-ink/10 px-1.5 py-2 text-center align-top text-xs {shown.kcal && Math.abs(dayTotal(d) - shown.kcal) > shown.kcal * 0.1 ? 'font-semibold text-app-warning' : 'text-app-muted'}">{dayTotal(d)} kcal</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {:else if cols.length > 0}
        <ul class="mt-3 grid gap-3">
          {#each shown.days as d, di}
            <li class="rounded-xl border border-app-ink/10 p-3">
              <p class="flex items-baseline justify-between font-semibold">{d.name}<span class="text-xs font-normal {shown.kcal && Math.abs(dayTotal(d) - shown.kcal) > shown.kcal * 0.1 ? 'text-app-warning' : 'text-app-muted'}">{dayTotal(d)} kcal</span></p>
              <div class="mt-2 grid gap-2.5">
                {#each cols as c, ci}
                  <div><p class="mb-1 text-[11px] font-semibold uppercase tracking-wide text-app-muted">{c}</p>{@render cell(di, ci)}</div>
                {/each}
              </div>
            </li>
          {/each}
        </ul>
      {/if}

      {#if !readonly}
        <div class="mt-3 flex flex-wrap gap-2">
          {#if cols.length < 8}<button type="button" class="btn-secondary" onclick={addColumn}><Icon name="plus" size={18} />Agregar comida o snack</button>{/if}
          {#if shown.days.length > 1 && cols.length}<button type="button" class="btn-ghost" onclick={copyToAll}>Copiar el primer día a toda la semana</button>{/if}
        </div>
      {/if}
    </section>

    <section class="card mt-4 p-4 sm:p-5">
      <h3 class="display text-xl">Indicaciones</h3>
      <label class="label mt-3" for="np-rec">Recomendaciones generales</label>
      <textarea id="np-rec" class="field min-h-24" rows="4" maxlength="3000" readonly={readonly} value={shown.recommendations} oninput={(e) => (work.recommendations = e.currentTarget.value)} placeholder="Ej. Comer despacio, no saltarse el desayuno, preferir alimentos naturales"></textarea>
      <label class="label mt-3" for="np-avoid">Alimentos o hábitos a evitar</label>
      <textarea id="np-avoid" class="field min-h-20" rows="3" maxlength="1500" readonly={readonly} value={shown.avoid} oninput={(e) => (work.avoid = e.currentTarget.value)}></textarea>
      <label class="label mt-3" for="np-sup">Suplementos</label>
      <textarea id="np-sup" class="field min-h-16" rows="2" maxlength="600" readonly={readonly} value={shown.supplements} oninput={(e) => (work.supplements = e.currentTarget.value)}></textarea>
    </section>

    {#if canWrite && editing && !viewed}
      <section class="card mt-4 p-4 sm:p-5">
        <label class="label" for="np-note">Nota de esta versión (opcional)</label>
        <input id="np-note" class="field" maxlength="500" bind:value={note} placeholder="Ej. Ajuste tras la consulta de seguimiento" />
        {#if work.follow_up_days > 0}
          <label class="mt-3 flex cursor-pointer items-center gap-2 text-sm"><input type="checkbox" class="h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={scheduleFollow} />Agendar la cita de seguimiento en {work.follow_up_days} días (queda por confirmar en la agenda)</label>
        {/if}
        {#if saveOp.phase === 'error'}<p class="alert mt-3" role="alert"><Icon name="alert" size={18} />{saveOp.message}</p>{/if}
        <div class="mt-3 flex flex-wrap gap-2">
          <button type="button" class="btn-primary" disabled={saveOp.phase === 'loading' || (!dirty && history.length > 0)} onclick={save}>
            {#if saveOp.phase === 'loading'}<span class="spin"></span>{/if}<Icon name="check" size={18} />Guardar plan
          </button>
          {#if history.length}<button type="button" class="btn-ghost" onclick={() => { work = asData(history[0]); editing = false; gOpen = false; }}>Descartar cambios</button>{/if}
        </div>
      </section>
    {/if}
  {/if}

  {#if history.length}
    <section class="mt-6" aria-labelledby="np-history">
      <h3 id="np-history" class="display mb-2 text-xl">Historial de planes</h3>
      <ul class="space-y-2">
        {#each history as h, i (h.id)}
          {@const d = asData(h)}
          <li class="card flex flex-wrap items-center justify-between gap-2 p-3 sm:px-5 {viewing === h.id ? 'ring-2 ring-app-primary' : ''}">
            <div class="min-w-0">
              <p class="text-sm font-medium">{dt(h.created_at)}{#if i === 0}<span class="badge ml-2">Vigente</span>{/if}</p>
              <p class="truncate text-xs text-app-muted">{h.created_by_name} · {d.goal || 'Sin objetivo'}{d.kcal ? ` · ${d.kcal} kcal` : ''}{d.days.length > 1 ? ` · ${d.days.length} días` : ''}{h.note ? ` · ${h.note}` : ''}</p>
            </div>
            <div class="flex flex-wrap gap-1">
              <button type="button" class="btn-ghost" onclick={() => { viewing = h.id; editing = false; gOpen = false; }}><Icon name="eye" size={16} />Ver</button>
              {#if canWrite && i > 0}<button type="button" class="btn-ghost" onclick={() => useAsBase(h)}>Usar como base</button>{/if}
            </div>
          </li>
        {/each}
      </ul>
    </section>
  {/if}
{/if}
