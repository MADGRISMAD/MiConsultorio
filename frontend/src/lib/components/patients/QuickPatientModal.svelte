<script lang="ts">
  import { api } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { Patient, PatientSchema, Subject } from '$lib/types';
  import type { OwnerListItem } from '$lib/types/owners';
  import OwnerPicker from './OwnerPicker.svelte';
  import Modal from '../Modal.svelte';
  import Icon from '../ui/Icon.svelte';

  interface Props {
    open: boolean;
    onclose: () => void;
    oncreated: (patient: Patient) => void;
  }
  let { open, onclose, oncreated }: Props = $props();

  let schema = $state<PatientSchema | null>(null);
  let schemaError = $state('');
  let subject = $state<Subject>('person');
  let names = $state('');
  let lastNames = $state('');
  let phone = $state('');
  let ownerName = $state('');
  let ownerSurnames = $state('');
  let privacyAck = $state(false);
  /** surnames of a full name: the last two words, or the last one of a two-word name */
  const surnamesOf = (full: string) => {
    const w = full.trim().split(/\s+/).filter(Boolean);
    return w.length < 2 ? '' : w.slice(w.length >= 3 ? -2 : -1).join(' ');
  };
  let ownerPhone = $state('');
  let owner = $state<OwnerListItem | null>(null); // an owner picked from the clinic's list
  let error = $state('');
  const op = new Op();
  const uid = $props.id();

  const subjects = $derived(schema?.subjects ?? []);
  const mixed = $derived(subjects.length > 1);
  const animal = $derived(subject === 'animal');

  // (re)start each time it opens
  $effect(() => {
    if (!open) return;
    names = lastNames = phone = ownerName = ownerSurnames = ownerPhone = error = '';
    privacyAck = false;
    owner = null;
    op.reset();
    if (schema) return;
    schemaError = '';
    api.patients.schema().then(
      (s) => {
        schema = s;
        subject = s.subjects[0] ?? 'person';
      },
      (e) => (schemaError = e instanceof Error ? e.message : 'No se pudo cargar el formulario.')
    );
  });
  $effect(() => {
    if (schema && !schema.subjects.includes(subject)) subject = schema.subjects[0] ?? 'person';
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    error = '';
    if (!names.trim()) return (error = animal ? 'Escribe el nombre del animal.' : 'Escribe el nombre del paciente.');
    if (!animal && !lastNames.trim()) return (error = 'Escribe los apellidos del paciente.');
    if (animal && !owner && (!ownerName.trim() || !ownerSurnames.trim() || !ownerPhone.trim())) return (error = 'Escribe el nombre, los apellidos y el teléfono del propietario.');
    if (!animal && !phone.trim()) return (error = 'Escribe un teléfono de contacto.');
    let created: Patient | null = null;
    const ok = await op.run(async () => {
      created = await api.patients.quick(
        animal
          ? owner
            ? { subject, names: names.trim(), last_names: surnamesOf(owner.name), owner_id: owner.id, privacy_ack: privacyAck }
            : { subject, names: names.trim(), last_names: ownerSurnames.trim(), guardian_name: `${ownerName.trim()} ${ownerSurnames.trim()}`, guardian_phone: ownerPhone.trim(), privacy_ack: privacyAck }
          : { subject, names: names.trim(), last_names: lastNames.trim(), phone: phone.trim(), privacy_ack: privacyAck }
      );
    });
    if (!ok || !created) return;
    toast.show('Paciente registrado. Su expediente quedó pendiente de completar.');
    oncreated(created);
  }
</script>

<Modal {open} title="Registro rápido" {onclose}>
  <p class="mb-4 flex items-start gap-2 rounded-xl bg-app-primary/8 px-3.5 py-3 text-sm text-app-muted">
    <Icon name="info" size={18} class="mt-0.5 flex-none text-app-primary" />
    Crea un expediente <strong class="font-semibold text-app-ink">incompleto</strong> con lo mínimo para agendar. El personal clínico lo completará después.
  </p>
  {#if schemaError}
    <p class="alert" role="alert"><Icon name="alert" size={18} />{schemaError}</p>
  {:else if !schema}
    <div class="h-24 animate-pulse rounded-xl bg-app-ink/8" role="status" aria-label="Cargando"></div>
  {:else}
    <form id="{uid}-form" class="grid gap-3 text-left sm:grid-cols-2" onsubmit={submit}>
      {#if mixed}
        <div class="sm:col-span-2">
          <span class="label" id="{uid}-s">¿Quién es el paciente?</span>
          <div class="flex gap-2" role="radiogroup" aria-labelledby="{uid}-s">
            {#each subjects as s}
              <button type="button" role="radio" aria-checked={subject === s} class="inline-flex min-h-10 items-center gap-2 rounded-full border px-4 text-sm font-medium transition {subject === s ? 'border-app-primary bg-app-primary/10 text-app-primary' : 'border-app-ink/15 text-app-muted hover:border-app-ink/30 hover:text-app-ink'}" onclick={() => (subject = s)}>
                <Icon name={s === 'animal' ? 'paw' : 'user'} size={17} />{s === 'animal' ? 'Animal' : 'Persona'}
              </button>
            {/each}
          </div>
        </div>
      {/if}
      {#if animal}
        <div class="sm:col-span-2">
          <OwnerPicker selected={owner} onpick={(o) => (owner = o)} onclear={() => (owner = null)} id="quick-owner-search" />
        </div>
        {#if !owner}
          <label class="block">
            <span class="label">Nombre(s) del propietario *</span>
            <input class="field" bind:value={ownerName} maxlength="100" autocomplete="given-name" />
          </label>
          <label class="block">
            <span class="label">Apellido(s) del propietario *</span>
            <input class="field" bind:value={ownerSurnames} maxlength="100" autocomplete="family-name" />
          </label>
          <label class="block">
            <span class="label">Teléfono del propietario *</span>
            <input class="field" type="tel" bind:value={ownerPhone} maxlength="20" autocomplete="tel" />
          </label>
        {/if}
        <label class="block sm:col-span-2">
          <span class="label">Ahora, el nombre del animal *</span>
          <input class="field" bind:value={names} maxlength="120" autocomplete="off" />
        </label>
      {:else}
        <label class="block">
          <span class="label">Nombre(s) *</span>
          <input class="field" bind:value={names} maxlength="120" autocomplete="off" />
        </label>
        <label class="block">
          <span class="label">Apellido(s) *</span>
          <input class="field" bind:value={lastNames} maxlength="120" autocomplete="off" />
        </label>
        <label class="block sm:col-span-2">
          <span class="label">Teléfono *</span>
          <input class="field" type="tel" bind:value={phone} maxlength="20" autocomplete="tel" />
        </label>
      {/if}
      <label class="flex cursor-pointer items-start gap-3 rounded-xl border border-app-ink/15 p-3.5 sm:col-span-2">
        <input id="{uid}-ack" type="checkbox" class="mt-1 h-5 w-5 flex-none accent-app-primary" bind:checked={privacyAck} />
        <span class="text-sm">{animal ? 'El propietario' : 'El paciente'} recibió el <strong class="font-semibold">aviso de privacidad</strong> y otorga su consentimiento para el tratamiento de sus datos personales y de salud. <span class="text-app-muted">(Si aún no, queda pendiente en el expediente.)</span></span>
      </label>
    </form>
    <div aria-live="polite">
      {#if error || op.phase === 'error'}<p class="alert mt-4" role="alert"><Icon name="alert" size={18} />{error || op.message}</p>{/if}
    </div>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
    <button type="submit" form="{uid}-form" class="btn-primary" disabled={!schema || op.phase === 'loading'}>
      {#if op.phase === 'loading'}<span class="spin"></span>{/if}Registrar
    </button>
  {/snippet}
</Modal>
