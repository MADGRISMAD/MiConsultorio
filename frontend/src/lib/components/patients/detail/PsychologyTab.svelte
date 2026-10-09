<script lang="ts">
  import { onMount } from 'svelte';
  import { specialtyApi } from '$lib/api/specialty';
  import { Op } from '$lib/op.svelte';
  import { SCALES, scaleById, type ScaleDef } from '$lib/specialty/scales';
  import { toast } from '$lib/toast.svelte';
  import type { Patient } from '$lib/types';
  import type { PatientChart } from '$lib/types/specialty';
  import MetricChart, { type Series } from '../../specialty/MetricChart.svelte';
  import Icon from '../../ui/Icon.svelte';

  let { patient, canWrite }: { patient: Patient; canWrite: boolean } = $props();

  interface ScaleData {
    scale: string;
    score: number;
    severity: string;
    flag?: string;
  }
  interface Goal {
    text: string;
    status: 'active' | 'done' | 'paused';
  }
  interface Task {
    text: string;
    due: string;
    done: boolean;
  }
  interface TherapyPlan {
    goals: Goal[];
    tasks: Task[];
    notes: string;
  }

  const dt = (iso: string) => new Date(iso).toLocaleDateString('es-MX', { day: 'numeric', month: 'short', year: 'numeric' });
  let loading = $state(true);
  let error = $state('');
  let results = $state<PatientChart[]>([]);
  let planHistory = $state<PatientChart[]>([]);

  // ---- scales ----
  let scaleId = $state<ScaleDef['id']>('phq9');
  const scale = $derived(scaleById(scaleId)!);
  let answers = $state<(number | null)[]>([]);
  let lastResult = $state<ScaleData | null>(null);
  const scaleOp = new Op();
  $effect(() => {
    answers = scale.items.map(() => null);
    lastResult = null;
  });
  const answered = $derived(answers.filter((a) => a != null).length);

  async function saveScale() {
    if (answers.some((a) => a == null)) return scaleOp.fail(`Responde las ${scale.items.length} preguntas.`);
    let saved: PatientChart | undefined;
    if (await scaleOp.run(async () => (saved = await specialtyApi.saveChart(patient.id, 'scale', { scale: scaleId, answers }, '')))) {
      lastResult = saved!.data as unknown as ScaleData;
      toast.show(`${scale.name}: ${lastResult.score} puntos (${lastResult.severity})`);
      answers = scale.items.map(() => null);
      await load();
    }
  }

  const byScale = (id: string) => results.filter((c) => (c.data as unknown as ScaleData).scale === id);
  const seriesFor = (id: string): Series[] => [
    { label: 'Puntos', points: [...byScale(id)].reverse().map((c) => ({ date: c.created_at.slice(0, 10), value: (c.data as unknown as ScaleData).score })) }
  ];

  // ---- therapy plan: goals and tasks between sessions ----
  let plan = $state<TherapyPlan>({ goals: [], tasks: [], notes: '' });
  let planBase = $state(JSON.stringify({ goals: [], tasks: [], notes: '' }));
  const planOp = new Op();
  const dirty = $derived(JSON.stringify(plan) !== planBase);

  async function load() {
    try {
      const [s, p] = await Promise.all([specialtyApi.charts(patient.id, 'scale'), specialtyApi.charts(patient.id, 'therapy_plan')]);
      results = s.charts;
      planHistory = p.charts;
      if (!dirty) {
        const d = (p.latest?.data ?? {}) as Partial<TherapyPlan>;
        plan = { goals: d.goals ?? [], tasks: d.tasks ?? [], notes: d.notes ?? '' };
        planBase = JSON.stringify(plan);
      }
      error = '';
    } catch (e) {
      error = e instanceof Error ? e.message : 'No se pudo cargar.';
    } finally {
      loading = false;
    }
  }
  onMount(load);

  async function savePlan() {
    const clean: TherapyPlan = {
      goals: plan.goals.filter((g) => g.text.trim()).map((g) => ({ ...g, text: g.text.trim() })),
      tasks: plan.tasks.filter((t) => t.text.trim()).map((t) => ({ ...t, text: t.text.trim() })),
      notes: plan.notes.trim()
    };
    if (await planOp.run(() => specialtyApi.saveChart(patient.id, 'therapy_plan', clean, ''))) {
      toast.show('Objetivos y tareas guardados');
      plan = clean;
      planBase = JSON.stringify(plan);
      await load();
    }
  }
  const STATUS: Record<Goal['status'], string> = { active: 'En curso', done: 'Logrado', paused: 'En pausa' };
</script>

{#if loading}
  <div class="card h-48 animate-pulse"></div>
{:else if error}
  <p class="alert" role="alert"><Icon name="alert" size={18} />{error}</p>
{:else}
  <section class="card p-5 sm:p-6" aria-labelledby="ps-scales">
    <h2 id="ps-scales" class="display text-2xl">Escalas de evaluación</h2>
    <p class="mt-1 text-sm text-app-muted">Cuestionarios breves con calificación automática. Orientan al profesional; no sustituyen el juicio clínico ni dan un diagnóstico.</p>

    {#if lastResult?.flag === 'self_harm'}
      <p class="alert mt-4" role="alert"><Icon name="alert" size={18} />La pregunta 9 (pensamientos de estar mejor muerto o de lastimarse) tuvo una respuesta positiva: valora el riesgo con el paciente.</p>
    {/if}

    {#if canWrite}
      <div class="mt-4 flex flex-wrap gap-2" role="radiogroup" aria-label="Escala">
        {#each SCALES as s (s.id)}
          <button type="button" role="radio" aria-checked={scaleId === s.id} class="rounded-xl px-3.5 py-2 text-sm {scaleId === s.id ? 'bg-app-primary/10 ring-2 ring-app-primary' : 'ring-1 ring-inset ring-app-ink/15 hover:bg-app-elevated'}" onclick={() => (scaleId = s.id)}>
            <strong>{s.name}</strong> · {s.topic}
          </button>
        {/each}
      </div>
      <p class="mt-4 text-sm font-medium">{scale.intro}</p>
      <ol class="mt-3 grid gap-3">
        {#each scale.items as item, i}
          <li class="rounded-xl border border-app-ink/10 p-3">
            <p class="text-sm"><span class="mr-1 font-mono text-xs text-app-muted">{i + 1}.</span>{item}</p>
            <div class="mt-2 flex flex-wrap gap-2" role="radiogroup" aria-label="Pregunta {i + 1}">
              {#each scale.options as o, v}
                <label class="cursor-pointer rounded-lg px-2.5 py-1.5 text-xs has-[:checked]:bg-app-ink has-[:checked]:text-app-surface ring-1 ring-inset ring-app-ink/15 has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-app-primary">
                  <input type="radio" class="sr-only" name="{scaleId}-{i}" checked={answers[i] === v} onchange={() => (answers[i] = v)} />{o}
                </label>
              {/each}
            </div>
          </li>
        {/each}
      </ol>
      {#if scaleOp.phase === 'error'}<p class="alert mt-3" role="alert"><Icon name="alert" size={18} />{scaleOp.message}</p>{/if}
      <div class="mt-4 flex flex-wrap items-center gap-3">
        <button type="button" class="btn-primary" disabled={scaleOp.phase === 'loading'} onclick={saveScale}>{#if scaleOp.phase === 'loading'}<span class="spin"></span>{/if}<Icon name="check" size={18} />Calcular y guardar</button>
        <span class="text-sm text-app-muted">{answered} de {scale.items.length} respondidas</span>
        {#if lastResult}<span class="rounded-xl bg-app-primary/10 px-3 py-1.5 text-sm font-medium">Resultado: {lastResult.score} de {scale.max} · {lastResult.severity}</span>{/if}
      </div>
    {/if}

    {#each SCALES as s (s.id)}
      {@const list = byScale(s.id)}
      {#if list.length}
        <div class="mt-6">
          <h3 class="text-sm font-semibold">{s.name} · {s.topic}</h3>
          {#if list.length >= 2}<div class="mt-2 max-w-xl"><MetricChart title="Evolución de {s.name}" unit="puntos" decimals={0} series={seriesFor(s.id)} /></div>{/if}
          <ul class="mt-2 divide-y divide-app-ink/10 rounded-xl ring-1 ring-inset ring-app-ink/10 text-sm">
            {#each list as c (c.id)}
              {@const d = c.data as unknown as ScaleData}
              <li class="flex flex-wrap items-center justify-between gap-2 px-3.5 py-2">
                <span>{dt(c.created_at)} · {c.created_by_name}</span>
                <span class="font-medium">{d.score} de {s.max} · {d.severity}{#if d.flag === 'self_harm'} <span class="badge">Revisar pregunta 9</span>{/if}</span>
              </li>
            {/each}
          </ul>
        </div>
      {/if}
    {/each}
  </section>

  <section class="card mt-4 p-5 sm:p-6" aria-labelledby="ps-plan">
    <h2 id="ps-plan" class="display text-2xl">Objetivos terapéuticos y tareas</h2>
    <p class="mt-1 text-sm text-app-muted">Lo que se trabaja con el paciente y lo que hace entre sesiones. Cada vez que guardas, queda una versión nueva.</p>

    <h3 class="mt-4 text-sm font-semibold">Objetivos</h3>
    <ul class="mt-2 grid gap-2">
      {#each plan.goals as g, i}
        <li class="flex flex-wrap items-center gap-2">
          <input class="field min-w-0 flex-1" maxlength="300" readonly={!canWrite} bind:value={g.text} aria-label="Objetivo {i + 1}" placeholder="Ej. Dormir al menos 7 horas" />
          <select class="field !w-auto" disabled={!canWrite} bind:value={g.status} aria-label="Estado del objetivo {i + 1}">{#each Object.entries(STATUS) as [k, v]}<option value={k}>{v}</option>{/each}</select>
          {#if canWrite}<button type="button" class="icon-btn danger" aria-label="Quitar el objetivo {i + 1}" onclick={() => (plan.goals = plan.goals.filter((_, j) => j !== i))}><Icon name="x" size={16} /></button>{/if}
        </li>
      {/each}
    </ul>
    {#if canWrite}<button type="button" class="btn-ghost mt-2" onclick={() => (plan.goals = [...plan.goals, { text: '', status: 'active' }])}><Icon name="plus" size={16} />Agregar objetivo</button>{/if}

    <h3 class="mt-5 text-sm font-semibold">Tareas entre sesiones</h3>
    <ul class="mt-2 grid gap-2">
      {#each plan.tasks as t, i}
        <li class="flex flex-wrap items-center gap-2">
          <input type="checkbox" class="h-5 w-5 accent-[rgb(var(--app-primary))]" disabled={!canWrite} bind:checked={t.done} aria-label="Tarea {i + 1} realizada" />
          <input class="field min-w-0 flex-1 {t.done ? 'line-through opacity-60' : ''}" maxlength="300" readonly={!canWrite} bind:value={t.text} aria-label="Tarea {i + 1}" placeholder="Ej. Registro de pensamientos 3 veces por semana" />
          <input type="date" class="field !w-auto" readonly={!canWrite} bind:value={t.due} aria-label="Fecha de entrega de la tarea {i + 1}" />
          {#if canWrite}<button type="button" class="icon-btn danger" aria-label="Quitar la tarea {i + 1}" onclick={() => (plan.tasks = plan.tasks.filter((_, j) => j !== i))}><Icon name="x" size={16} /></button>{/if}
        </li>
      {/each}
    </ul>
    {#if canWrite}<button type="button" class="btn-ghost mt-2" onclick={() => (plan.tasks = [...plan.tasks, { text: '', due: '', done: false }])}><Icon name="plus" size={16} />Agregar tarea</button>{/if}

    <label class="label mt-5" for="ps-notes">Notas del plan</label>
    <textarea id="ps-notes" class="field min-h-20" rows="3" maxlength="2000" readonly={!canWrite} bind:value={plan.notes}></textarea>

    {#if canWrite}
      {#if planOp.phase === 'error'}<p class="alert mt-3" role="alert"><Icon name="alert" size={18} />{planOp.message}</p>{/if}
      <button type="button" class="btn-primary mt-4" disabled={planOp.phase === 'loading' || !dirty} onclick={savePlan}>{#if planOp.phase === 'loading'}<span class="spin"></span>{/if}<Icon name="check" size={18} />Guardar objetivos y tareas</button>
    {/if}
    {#if planHistory.length > 1}<p class="hint mt-3">{planHistory.length} versiones guardadas · la última: {dt(planHistory[0].created_at)} por {planHistory[0].created_by_name}.</p>{/if}
  </section>
{/if}
