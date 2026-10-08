<script lang="ts">
  import { onMount } from 'svelte';
  import { specialtyApi } from '$lib/api/specialty';
  import { Op } from '$lib/op.svelte';
  import { printNutritionPlan } from '$lib/print';
  import { toast } from '$lib/toast.svelte';
  import type { Encounter, Patient } from '$lib/types';
  import type { NutritionMeal, NutritionPlanData, PatientChart } from '$lib/types/specialty';
  import EmptyState from '../../ui/EmptyState.svelte';
  import Icon from '../../ui/Icon.svelte';

  let { patient, canWrite, encounters = [] }: { patient: Patient; canWrite: boolean; encounters?: Encounter[] } = $props();

  const empty = (): NutritionPlanData => ({ goal: '', basis: '', kcal: 0, protein_pct: 0, carb_pct: 0, fat_pct: 0, water_liters: 0, meals: [], recommendations: '', avoid: '', supplements: '', follow_up_days: 0 });
  const copy = <T,>(v: T): T => JSON.parse(JSON.stringify(v));
  const asData = (c: PatientChart) => c.data as NutritionPlanData;
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
  const MACROS: Record<string, [number, number, number]> = {
    'Bajar de peso': [45, 25, 30],
    Mantener: [50, 20, 30],
    'Subir de peso': [50, 20, 30],
    'Ganar masa muscular': [45, 30, 25],
    'Control de enfermedad': [45, 20, 35],
    'Alimentación saludable': [50, 20, 30]
  };
  const ADJUST: Record<string, number> = { 'Bajar de peso': -500, 'Subir de peso': 400, 'Ganar masa muscular': 300 };
  const MEALS: [string, string, number][] = [
    ['Desayuno', '08:00', 25],
    ['Colación 1', '11:00', 10],
    ['Comida', '14:30', 30],
    ['Colación 2', '17:30', 10],
    ['Cena', '20:30', 25]
  ];

  let gGoal = $state('Alimentación saludable');
  let gWeight = $state('');
  let gHeight = $state('');
  let gActivity = $state(1.375);
  let gOpen = $state(false);
  let gError = $state('');
  let gCustom = $state('');
  let gPrefs = $state('');
  let gMeals = $state(5);
  const OTHER = 'Otro (escribir)';
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
    gError = '';
    aiOp.reset();
    gOpen = true;
  }

  /** Mifflin-St Jeor resting energy, times activity, adjusted for the goal. */
  function generate() {
    const w = Number(gWeight.replace(',', '.'));
    const h = Number(gHeight.replace(',', '.'));
    const age = patient.age;
    if (!(w > 0) || !(h > 0) || age == null) {
      gError = 'Escribe el peso y la talla (y registra la fecha de nacimiento del paciente) para calcular.';
      return;
    }
    const sexTerm = patient.sex === 'Hombre' ? 5 : patient.sex === 'Mujer' ? -161 : -78;
    const bmr = 10 * w + 6.25 * h - 5 * age + sexTerm;
    const total = bmr * gActivity;
    const floor = patient.sex === 'Hombre' ? 1500 : 1200;
    const kcal = Math.max(floor, Math.round((total + (ADJUST[gGoal] ?? 0)) / 10) * 10);
    const [carb, prot, fat] = MACROS[gGoal] ?? MACROS.Mantener;
    const meals: NutritionMeal[] = MEALS.map(([name, time, pct]) => ({ name, time, items: '', kcal: Math.round((kcal * pct) / 100 / 10) * 10 }));
    work = {
      ...work,
      goal: goalText || gGoal,
      kcal,
      carb_pct: carb,
      protein_pct: prot,
      fat_pct: fat,
      water_liters: work.water_liters || Math.round(w * 0.035 * 10) / 10,
      meals: work.meals.length ? work.meals : meals,
      follow_up_days: work.follow_up_days || 30,
      basis: `Calculado con la fórmula de Mifflin-St Jeor: ${w} kg, ${h} cm, ${age} años, actividad ×${gActivity}. Es una estimación: ajústala con tu criterio clínico.`
    };
    editing = true;
    gOpen = false;
  }

  /** The AI drafts the whole plan from the goal, the patient's history and the nutritionist's notes. */
  async function generateAI() {
    if (!goalText) {
      gError = 'Escribe el objetivo del plan.';
      return;
    }
    gError = '';
    const w = Number(gWeight.replace(',', '.')) || 0;
    const h = Number(gHeight.replace(',', '.')) || 0;
    const act = ACTIVITY.find((a) => a.v === gActivity)?.label ?? '';
    let drafted: NutritionPlanData | undefined;
    const ok = await aiOp.run(async () => {
      drafted = await specialtyApi.nutritionAI(patient.id, { goal: goalText, weight_kg: w, height_cm: h, activity: act, kcal: 0, meals: gMeals, preferences: gPrefs.trim() });
    });
    if (!ok || !drafted) return;
    work = drafted;
    editing = true;
    gOpen = false;
    toast.show('Borrador listo: revísalo y ajústalo antes de guardar');
  }

  async function load() {
    try {
      const r = await specialtyApi.charts(patient.id, 'nutrition_plan');
      history = r.charts;
      work = r.latest ? copy(asData(r.latest)) : empty();
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
  const mealsKcal = $derived(shown.meals.reduce((s, m) => s + (Number(m.kcal) || 0), 0));
  const macroSum = $derived(shown.protein_pct + shown.carb_pct + shown.fat_pct);

  const num = (v: string) => {
    const n = Number(v.replace(',', '.'));
    return Number.isFinite(n) && n > 0 ? n : 0;
  };
  function addMeal() {
    work = { ...work, meals: [...work.meals, { name: '', time: '', items: '', kcal: 0 }] };
  }
  function removeMeal(i: number) {
    work = { ...work, meals: work.meals.filter((_, j) => j !== i) };
  }
  function setMeal(i: number, patch: Partial<NutritionMeal>) {
    work = { ...work, meals: work.meals.map((m, j) => (j === i ? { ...m, ...patch } : m)) };
  }
  function useAsBase(c: PatientChart) {
    work = copy(asData(c));
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

  async function save() {
    const data = { ...work, meals: work.meals.filter((m) => m.name.trim()).map((m) => ({ ...m, name: m.name.trim(), kcal: Math.round(Number(m.kcal) || 0) })), kcal: Math.round(work.kcal || 0) };
    if (!data.goal.trim() && !data.kcal && !data.meals.length && !data.recommendations.trim()) {
      saveOp.fail('Agrega al menos el objetivo, las calorías o una comida.');
      return;
    }
    if (macroSum > 100) {
      saveOp.fail('Los porcentajes de proteínas, carbohidratos y grasas suman más de 100 %.');
      return;
    }
    if (await saveOp.run(() => specialtyApi.saveChart(patient.id, 'nutrition_plan', data, note.trim()))) {
      toast.show('Plan nutricional guardado');
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
    <p class="min-w-0 max-w-xl flex-1 basis-64 text-sm text-app-muted">Calcula las calorías con los datos del paciente, reparte las comidas y entrégale su plan impreso. Cada versión guardada se conserva en el historial.</p>
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
    <div class="card"><EmptyState icon="leaf" title="Sin plan nutricional" text={canWrite ? 'Genera el primer plan: calcula las calorías con el peso y la talla del paciente y reparte las comidas del día.' : 'Todavía no hay un plan nutricional para este paciente.'}>
      {#if canWrite}<button type="button" class="btn-primary" onclick={() => { editing = true; openGenerator(); }}><Icon name="sparkles" size={18} />Generar plan nutricional</button>{/if}
    </EmptyState></div>
  {:else}
    {#if gOpen}
      <section class="card mb-4 p-4 sm:p-5" aria-labelledby="gen-h">
        <h3 id="gen-h" class="display text-xl">Calcular necesidades</h3>
        <p class="mt-1 text-sm text-app-muted">Genera un borrador con IA para cualquier objetivo, o calcula las calorías con la fórmula de Mifflin-St Jeor y arma las comidas tú.</p>
        <div class="mt-3 grid gap-3 sm:grid-cols-2">
          <div><label class="label" for="g-goal">Objetivo</label>
            <select id="g-goal" class="field" bind:value={gGoal}>{#each GOAL_CHOICES as g}<option>{g}</option>{/each}</select>
            {#if gGoal === OTHER}<input class="field mt-2" maxlength="200" bind:value={gCustom} placeholder="Ej. Control de colesterol, embarazo, deportista de resistencia" aria-label="Objetivo del plan" />{/if}</div>
          <div><label class="label" for="g-act">Actividad física</label>
            <select id="g-act" class="field" bind:value={gActivity}>{#each ACTIVITY as a}<option value={a.v}>{a.label}</option>{/each}</select></div>
          <div><label class="label" for="g-w">Peso (kg)</label><input id="g-w" class="field" inputmode="decimal" bind:value={gWeight} /></div>
          <div><label class="label" for="g-h">Talla (cm)</label><input id="g-h" class="field" inputmode="decimal" bind:value={gHeight} /></div>
        </div>
        <div class="mt-3"><label class="label" for="g-prefs">Preferencias, restricciones o indicaciones para la IA <span class="font-normal text-app-muted">(opcional)</span></label>
          <textarea id="g-prefs" class="field min-h-20" rows="2" maxlength="1500" bind:value={gPrefs} placeholder="Ej. Vegetariano, no le gusta el pescado, presupuesto bajo, come fuera de casa al mediodía"></textarea></div>
        <div class="mt-3"><label class="label" for="g-meals">Comidas al día</label>
          <select id="g-meals" class="field !w-auto" bind:value={gMeals}>{#each [3, 4, 5, 6] as n}<option value={n}>{n}</option>{/each}</select></div>
        {#if bmi}<p class="hint">Índice de masa corporal: <strong>{bmi.value}</strong> ({bmi.label}).</p>{/if}
        <p class="hint">{patient.age != null ? `${patient.age} años · ${patient.sex || 'sexo sin registrar'}` : 'El paciente no tiene fecha de nacimiento registrada.'}. El peso y la talla se toman de la última nota que los tenga.</p>
        {#if gError}<p class="alert mt-3" role="alert"><Icon name="alert" size={18} />{gError}</p>{/if}
        {#if aiOp.phase === 'error'}<p class="alert mt-3" role="alert"><Icon name="alert" size={18} />{aiOp.message}</p>{/if}
        <p class="hint">La IA arma un borrador completo (calorías, macronutrientes, comidas e indicaciones) con la edad, el sexo, el peso y los antecedentes alimentarios del paciente; no se envía su nombre. Siempre revísalo antes de guardarlo. Usa uno de los usos de IA de tu plan.</p>
        <div class="mt-3 flex flex-wrap gap-2">
          <button type="button" class="btn-primary" disabled={aiOp.phase === 'loading'} onclick={generateAI}>{#if aiOp.phase === 'loading'}<span class="spin"></span>{:else}<Icon name="sparkles" size={18} />{/if}Generar con IA</button>
          <button type="button" class="btn-secondary" disabled={aiOp.phase === 'loading'} onclick={generate}>Calcular y armar plan</button>
          <button type="button" class="btn-ghost" onclick={() => (gOpen = false)}>Llenar a mano</button>
        </div>
      </section>
    {:else if !readonly}
      <button type="button" class="btn-secondary mb-4" onclick={openGenerator}><Icon name="sparkles" size={18} />Recalcular con peso y talla</button>
    {/if}

    <section class="card p-4 sm:p-5">
      <h3 class="display text-xl">Objetivo y energía</h3>
      <div class="mt-3 grid gap-3 sm:grid-cols-2">
        <div class="sm:col-span-2"><label class="label" for="np-goal">Objetivo del plan</label>
          <input id="np-goal" class="field" maxlength="300" readonly={readonly} value={shown.goal} oninput={(e) => (work.goal = e.currentTarget.value)} placeholder="Ej. Bajar 6 kg en 3 meses" /></div>
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
      </div>
      {#if macroSum > 100}<p class="alert mt-3" role="alert"><Icon name="alert" size={18} />Los porcentajes suman {macroSum} %: no pueden pasar de 100 %.</p>
      {:else if macroSum > 0 && macroSum < 100}<p class="hint">Los macronutrientes suman {macroSum} %.</p>{/if}
      {#if shown.basis}<p class="hint">{shown.basis}</p>{/if}
    </section>

    <section class="card mt-4 p-4 sm:p-5">
      <div class="flex flex-wrap items-baseline justify-between gap-2">
        <h3 class="display text-xl">Comidas del día</h3>
        {#if shown.kcal && mealsKcal}<p class="text-sm {Math.abs(mealsKcal - shown.kcal) > shown.kcal * 0.1 ? 'text-app-warning' : 'text-app-muted'}">Suman {mealsKcal} de {shown.kcal} kcal</p>{/if}
      </div>
      {#if shown.meals.length === 0}<p class="mt-2 text-sm text-app-muted">Sin comidas. {readonly ? '' : 'Agrega las comidas del día o usa «Calcular y armar plan».'}</p>{/if}
      <ul class="mt-3 grid gap-3">
        {#each shown.meals as m, i}
          <li class="rounded-xl border border-app-ink/10 p-3">
            <div class="grid grid-cols-2 gap-2">
              <div class="col-span-2 min-w-0"><label class="label" for="m-n-{i}">Comida</label><input id="m-n-{i}" class="field" maxlength="60" readonly={readonly} value={m.name} oninput={(e) => setMeal(i, { name: e.currentTarget.value })} /></div>
              <div><label class="label" for="m-t-{i}">Hora</label><input id="m-t-{i}" type="time" class="field" readonly={readonly} value={m.time} oninput={(e) => setMeal(i, { time: e.currentTarget.value })} /></div>
              <div><label class="label" for="m-k-{i}">kcal</label><input id="m-k-{i}" class="field" inputmode="numeric" readonly={readonly} value={m.kcal || ''} oninput={(e) => setMeal(i, { kcal: Math.round(num(e.currentTarget.value)) })} /></div>
            </div>
            <label class="label mt-2" for="m-i-{i}">Alimentos y porciones</label>
            <textarea id="m-i-{i}" class="field min-h-20" rows="3" maxlength="1200" readonly={readonly} value={m.items} oninput={(e) => setMeal(i, { items: e.currentTarget.value })} placeholder="Ej. 1 taza de avena, 1 manzana, 1 huevo cocido"></textarea>
            {#if !readonly}<button type="button" class="btn-ghost mt-1 text-app-danger" onclick={() => removeMeal(i)}><Icon name="trash" size={16} />Quitar comida</button>{/if}
          </li>
        {/each}
      </ul>
      {#if !readonly && shown.meals.length < 12}<button type="button" class="btn-secondary mt-3" onclick={addMeal}><Icon name="plus" size={18} />Agregar comida</button>{/if}
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
        {#if saveOp.phase === 'error'}<p class="alert mt-3" role="alert"><Icon name="alert" size={18} />{saveOp.message}</p>{/if}
        <div class="mt-3 flex flex-wrap gap-2">
          <button type="button" class="btn-primary" disabled={saveOp.phase === 'loading' || (!dirty && history.length > 0)} onclick={save}>
            {#if saveOp.phase === 'loading'}<span class="spin"></span>{/if}<Icon name="check" size={18} />Guardar plan
          </button>
          {#if history.length}<button type="button" class="btn-ghost" onclick={() => { work = copy(asData(history[0])); editing = false; gOpen = false; }}>Descartar cambios</button>{/if}
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
              <p class="truncate text-xs text-app-muted">{h.created_by_name} · {d.goal || 'Sin objetivo'}{d.kcal ? ` · ${d.kcal} kcal` : ''}{h.note ? ` · ${h.note}` : ''}</p>
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
