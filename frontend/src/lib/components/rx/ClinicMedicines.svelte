<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import { rxApi } from '$lib/api/rx';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { CatalogMed, ClinicMedInput } from '$lib/types/rx';
  import ConfirmModal from '../ConfirmModal.svelte';
  import Modal from '../Modal.svelte';
  import EmptyState from '../ui/EmptyState.svelte';
  import Icon from '../ui/Icon.svelte';
  import LoadingRows from '../ui/LoadingRows.svelte';
  import PageHeader from '../ui/PageHeader.svelte';

  const ROUTES = ['Oral', 'Sublingual', 'Tópica', 'Oftálmica', 'Ótica', 'Nasal', 'Inhalada', 'Rectal', 'Vaginal', 'Intramuscular', 'Intravenosa', 'Subcutánea', 'Otra'];
  const canWrite = $derived(session.has('adminHistorials'));

  let list = $state<CatalogMed[]>([]);
  const load = new Op();
  const save = new Op();
  const del = new Op();
  let editing = $state<{ id?: string; f: Form } | null>(null);
  let removing = $state<CatalogMed | null>(null);

  interface Form {
    name: string;
    brand: string;
    subject: 'person' | 'animal';
    species: string;
    category: string;
    control: ClinicMedInput['control'];
    route: string;
    presentations: string;
    typical_dose: string;
    mg_per_kg: string;
    max_mg_per_kg_day: string;
    concs: { label: string; mg: string }[];
    notes: string;
  }
  const blank = (): Form => ({ name: '', brand: '', subject: 'person', species: '', category: '', control: 'No', route: 'Oral', presentations: '', typical_dose: '', mg_per_kg: '', max_mg_per_kg_day: '', concs: [], notes: '' });

  const refresh = () => load.run(async () => (list = await rxApi.clinicMeds()));
  $effect(() => {
    refresh();
  });

  function edit(m?: CatalogMed) {
    save.reset();
    editing = m
      ? {
          id: m.id,
          f: {
            name: m.name,
            brand: m.brand ?? '',
            subject: m.subject,
            species: (m.species ?? []).join(', '),
            category: m.category,
            control: m.control === 'Antibiótico' || m.control === 'Fracción III' ? m.control : 'No',
            route: m.route,
            presentations: m.presentations.join('\n'),
            typical_dose: m.typical_dose,
            mg_per_kg: m.mg_per_kg ? String(m.mg_per_kg) : '',
            max_mg_per_kg_day: m.max_mg_per_kg_day ? String(m.max_mg_per_kg_day) : '',
            concs: (m.concentrations ?? []).map((c) => ({ label: c.label, mg: String(c.mg_per_ml) })),
            notes: m.notes ?? ''
          }
        }
      : { f: blank() };
  }

  const n = (s: string) => {
    const v = parseFloat(s.replace(',', '.'));
    return Number.isFinite(v) && v > 0 ? v : null;
  };

  async function submit(ev: SubmitEvent) {
    ev.preventDefault();
    const cur = editing;
    if (!cur) return;
    const f = cur.f;
    if (!f.name.trim()) return save.fail('Escribe la denominación genérica.');
    const body: ClinicMedInput = {
      name: f.name.trim(),
      brand: f.brand.trim(),
      subject: f.subject,
      species: f.species.split(',').map((s) => s.trim()).filter(Boolean),
      category: f.category.trim(),
      control: f.control,
      route: f.route,
      presentations: f.presentations.split('\n').map((s) => s.trim()).filter(Boolean),
      typical_dose: f.typical_dose.trim(),
      mg_per_kg: n(f.mg_per_kg),
      max_mg_per_kg_day: n(f.max_mg_per_kg_day),
      concentrations: f.concs.filter((c) => c.label.trim() && n(c.mg)).map((c) => ({ label: c.label.trim(), mg_per_ml: n(c.mg) as number })),
      notes: f.notes.trim()
    };
    if (await save.run(() => rxApi.saveClinicMed(body, cur.id))) {
      editing = null;
      toast.show(cur.id ? 'Medicamento actualizado' : 'Medicamento agregado');
      refresh();
    }
  }

  async function confirmDelete() {
    const m = removing;
    if (m && (await del.run(() => rxApi.deleteClinicMed(m.id)))) {
      removing = null;
      toast.show('Medicamento eliminado');
      refresh();
    }
  }
</script>

<PageHeader title="Medicamentos de la clínica" subtitle="Fórmulas, marcas o productos propios que aparecen junto al catálogo al hacer una receta.">
  {#snippet actions()}
    {#if canWrite}<button type="button" class="btn-primary" onclick={() => edit()}><Icon name="plus" size={16} />Agregar medicamento</button>{/if}
  {/snippet}
</PageHeader>

{#if load.phase === 'loading' && !list.length}
  <LoadingRows />
{:else if !list.length}
  <EmptyState icon="box" title="Aún no hay medicamentos propios" text="El catálogo general ya incluye los medicamentos de uso común. Agrega aquí lo que sea exclusivo de tu clínica." />
{:else}
  <ul class="card divide-y divide-app-ink/10">
    {#each list as m (m.id)}
      <li class="flex flex-wrap items-center justify-between gap-3 px-5 py-4">
        <div class="min-w-0">
          <p class="font-medium">{m.name}{#if m.brand}<span class="text-app-muted"> · {m.brand}</span>{/if}</p>
          <p class="truncate text-sm text-app-muted">{m.subject === 'animal' ? 'Veterinaria' : 'Personas'} · {m.route}{m.control !== 'No' ? ` · ${m.control}` : ''}{m.presentations.length ? ` · ${m.presentations.join(', ')}` : ''}</p>
        </div>
        {#if canWrite}
          <div class="flex gap-1">
            <button type="button" class="btn-ghost" aria-label="Editar {m.name}" onclick={() => edit(m)}><Icon name="edit" size={16} /></button>
            <button type="button" class="btn-ghost text-app-danger" aria-label="Eliminar {m.name}" onclick={() => ((removing = m), del.reset())}><Icon name="trash" size={16} /></button>
          </div>
        {/if}
      </li>
    {/each}
  </ul>
{/if}

<Modal open={editing !== null} title={editing?.id ? 'Editar medicamento' : 'Nuevo medicamento'} onclose={() => (editing = null)} wide>
  {#if editing}
    {@const f = editing.f}
    <form id="cm-form" class="grid gap-3 sm:grid-cols-2" onsubmit={submit}>
      <div class="sm:col-span-2"><label class="label" for="cm-name">Denominación genérica *</label><input id="cm-name" class="field" bind:value={f.name} /></div>
      <div><label class="label" for="cm-brand">Marca</label><input id="cm-brand" class="field" bind:value={f.brand} /></div>
      <div><label class="label" for="cm-cat">Categoría</label><input id="cm-cat" class="field" bind:value={f.category} placeholder="Ej. Antibiótico" /></div>
      <div>
        <label class="label" for="cm-subject">Para</label>
        <select id="cm-subject" class="field" bind:value={f.subject}><option value="person">Personas</option><option value="animal">Animales</option></select>
      </div>
      {#if f.subject === 'animal'}
        <div><label class="label" for="cm-sp">Especies (separadas por coma)</label><input id="cm-sp" class="field" bind:value={f.species} placeholder="Perro, Gato" /></div>
      {/if}
      <div>
        <label class="label" for="cm-route">Vía</label>
        <select id="cm-route" class="field" bind:value={f.route}>{#each ROUTES as r}<option value={r}>{r}</option>{/each}</select>
      </div>
      <div>
        <label class="label" for="cm-ctl">Control</label>
        <select id="cm-ctl" class="field" bind:value={f.control}><option>No</option><option>Antibiótico</option><option>Fracción III</option></select>
      </div>
      <div class="sm:col-span-2"><label class="label" for="cm-pres">Presentaciones (una por línea)</label><textarea id="cm-pres" class="field" rows="3" bind:value={f.presentations}></textarea></div>
      <div class="sm:col-span-2"><label class="label" for="cm-dose">Dosis habitual</label><input id="cm-dose" class="field" bind:value={f.typical_dose} /></div>
      <div><label class="label" for="cm-mgkg">mg por kg (por toma)</label><input id="cm-mgkg" class="field" inputmode="decimal" bind:value={f.mg_per_kg} /></div>
      <div>
        <label class="label" for="cm-max">Máximo mg/kg al día</label>
        <input id="cm-max" class="field" inputmode="decimal" bind:value={f.max_mg_per_kg_day} aria-describedby="cm-max-h" />
        <p id="cm-max-h" class="hint">Si lo capturas, la receta avisa cuando la dosis diaria lo supere.</p>
      </div>
      <fieldset class="sm:col-span-2">
        <legend class="label">Concentraciones (para calcular mL)</legend>
        {#each f.concs as c, i (i)}
          <div class="mb-2 flex gap-2">
            <input class="field" aria-label="Nombre de la concentración" placeholder="Suspensión 250 mg/5 mL" bind:value={c.label} />
            <input class="field max-w-32" aria-label="mg por mL" inputmode="decimal" placeholder="mg/mL" bind:value={c.mg} />
            <button type="button" class="btn-ghost" aria-label="Quitar concentración" onclick={() => (f.concs = f.concs.filter((_, j) => j !== i))}><Icon name="x" size={16} /></button>
          </div>
        {/each}
        <button type="button" class="btn-secondary" onclick={() => (f.concs = [...f.concs, { label: '', mg: '' }])}><Icon name="plus" size={16} />Agregar concentración</button>
      </fieldset>
      <div class="sm:col-span-2"><label class="label" for="cm-notes">Notas</label><input id="cm-notes" class="field" bind:value={f.notes} /></div>
      <OpError op={save} class="sm:col-span-2" />
    </form>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={() => (editing = null)}>Cancelar</button>
    <button type="submit" form="cm-form" class="btn-primary" disabled={save.phase === 'loading'}>Guardar</button>
  {/snippet}
</Modal>

<ConfirmModal open={removing !== null} title="¿Eliminar este medicamento?" confirmLabel="Eliminar" op={del} onconfirm={confirmDelete} onclose={() => (removing = null)}>
  <p>«{removing?.name}» dejará de aparecer en el buscador. Las recetas ya emitidas no cambian.</p>
</ConfirmModal>
