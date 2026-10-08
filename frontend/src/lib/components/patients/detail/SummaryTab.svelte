<script lang="ts">
  import type { Encounter, Patient, PatientSchema } from '$lib/types';
  import Icon from '../../ui/Icon.svelte';
  import OwnerCard from './OwnerCard.svelte';
  import { measureRows, show } from './util';

  interface Props {
    patient: Patient;
    schema: PatientSchema;
    encounters: Encounter[];
  }
  let { patient: p, schema, encounters }: Props = $props();
  const animal = $derived(p.subject === 'animal');

  const dateOf = (iso: string | null) => (iso ? new Date(`${iso.slice(0, 10)}T12:00:00`).toLocaleDateString('es-MX', { day: 'numeric', month: 'long', year: 'numeric' }) : '');

  const general = $derived(
    [
      ['Sexo', p.sex],
      ['Fecha de nacimiento', dateOf(p.birth_date)],
      ['Edad', p.age != null ? `${p.age} ${p.age === 1 ? 'año' : 'años'}` : ''],
      ...(animal ? [] : [['CURP', p.curp]]),
      ['Domicilio', p.address]
    ].filter(([, v]) => v)
  );
  const contact = $derived([['Teléfono', p.phone], ['Correo', p.email]].filter(([, v]) => v));
  const guardian = $derived(
    [
      [animal ? 'Propietario' : 'Responsable / tutor', p.guardian_name],
      ['Parentesco', p.guardian_relation],
      ['Teléfono', p.guardian_phone],
      ['Correo', p.guardian_email]
    ].filter(([, v]) => v)
  );

  const groups = $derived.by(() => {
    const out: { name: string; rows: { label: string; value: string; multi: boolean }[] }[] = [];
    for (const f of schema.profile[p.subject] ?? []) {
      const v = show(f, p.profile);
      if (!v) continue;
      let g = out.find((x) => x.name === f.group);
      if (!g) out.push((g = { name: f.group, rows: [] }));
      g.rows.push({ label: f.label, value: v, multi: f.type === 'multiselect' });
      (g.rows[g.rows.length - 1] as { raw?: string[] }).raw = Array.isArray(p.profile?.[f.key]) ? (p.profile[f.key] as string[]) : undefined;
    }
    return out;
  });

  const last = $derived(
    [...encounters].filter((e) => !e.hidden && Object.keys(e.measures ?? {}).length).sort((a, b) => b.occurred_at.localeCompare(a.occurred_at))[0]
  );
  const signs = $derived(last ? measureRows((schema.measures_all ?? schema.measures)[p.subject] ?? [], last.measures) : []);
</script>

{#snippet list(rows: string[][])}
  <dl class="grid gap-x-6 gap-y-3 sm:grid-cols-2">
    {#each rows as [k, v]}<div class="min-w-0"><dt class="text-xs text-app-muted">{k}</dt><dd class="break-words text-[15px]">{v}</dd></div>{/each}
  </dl>
{/snippet}

<div class="space-y-4">
  {#if animal}<OwnerCard patientId={p.id} />{/if}
  {#if signs.length}
    <section class="card p-5" aria-label="Últimos signos">
      <p class="section-title flex items-center gap-2"><Icon name="activity" size={14} />Últimos signos · {new Date(last!.occurred_at).toLocaleDateString('es-MX', { day: 'numeric', month: 'short', year: 'numeric' })}</p>
      <dl class="mt-3 flex flex-wrap gap-2">
        {#each signs as r}<div class="rounded-xl bg-app-primary/8 px-3 py-2 text-sm"><dt class="text-xs text-app-muted">{r.label}</dt><dd class="font-medium">{r.value}</dd></div>{/each}
      </dl>
    </section>
  {/if}

  <div class="grid gap-4 lg:grid-cols-2">
    <section class="card p-5 sm:p-6">
      <h2 class="display mb-4 text-2xl">Datos generales</h2>
      {#if general.length}{@render list(general)}{:else}<p class="text-sm text-app-muted">Sin datos capturados.</p>{/if}
    </section>
    <section class="card p-5 sm:p-6">
      <h2 class="display mb-4 text-2xl">Contacto{guardian.length ? ' y responsable' : ''}</h2>
      {#if contact.length}{@render list(contact)}{:else}<p class="text-sm text-app-muted">Sin contacto registrado.</p>{/if}
      {#if guardian.length}
        <p class="section-title mb-3 mt-5">{animal ? 'Propietario' : 'Responsable'}</p>
        {@render list(guardian)}
      {/if}
    </section>
  </div>

  {#each groups as g (g.name)}
    <section class="card p-5 sm:p-6">
      <h2 class="display mb-4 text-2xl">{g.name}</h2>
      <dl class="grid gap-x-6 gap-y-4 sm:grid-cols-2">
        {#each g.rows as r}
          {@const raw = (r as { raw?: string[] }).raw}
          <div class="min-w-0 {r.multi ? 'sm:col-span-2' : ''}">
            <dt class="text-xs text-app-muted">{r.label}</dt>
            <dd class="mt-0.5 break-words text-[15px] whitespace-pre-line">
              {#if raw}<span class="flex flex-wrap gap-1.5">{#each raw as o}<span class="badge">{o}</span>{/each}</span>{:else}{r.value}{/if}
            </dd>
          </div>
        {/each}
      </dl>
    </section>
  {/each}
</div>
