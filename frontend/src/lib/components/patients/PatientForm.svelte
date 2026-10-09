<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { api } from '$lib/api';
  import { ownersApi } from '$lib/api/owners';
  import { dateShort } from '$lib/format';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { FieldValues, Patient, PatientInput, PatientSchema, Subject } from '$lib/types';
  import type { OwnerListItem } from '$lib/types/owners';
  import DynamicFields from '../DynamicFields.svelte';
  import OwnerPicker from './OwnerPicker.svelte';
  import Icon from '../ui/Icon.svelte';
  import LoadingRows from '../ui/LoadingRows.svelte';
  import { ageFrom, ageText } from './util';

  /** Pass the patient to edit; leave empty to register a new one. */
  let { patient = null }: { patient?: Patient | null } = $props();

  /* svelte-ignore state_referenced_locally */
  const editing = !!patient;
  const today = new Date().toISOString().slice(0, 10);

  let schema = $state<PatientSchema | null>(null);
  let loadError = $state('');

  // ----- form state (seeded once from `patient`) -----
  /* svelte-ignore state_referenced_locally */
  let subject = $state<Subject>(patient?.subject ?? 'person');
  /* svelte-ignore state_referenced_locally */
  let f = $state({
    names: patient?.names ?? '',
    last_names: patient?.last_names ?? '',
    sex: patient?.sex ?? '',
    birth_date: patient?.birth_date?.slice(0, 10) ?? '',
    curp: patient?.curp ?? '',
    phone: patient?.phone ?? '',
    email: patient?.email ?? '',
    address: patient?.address ?? '',
    guardian_name: patient?.guardian_name ?? '',
    guardian_relation: patient?.guardian_relation ?? '',
    guardian_phone: patient?.guardian_phone ?? '',
    guardian_email: patient?.guardian_email ?? '',
    owner_birth_date: ''
  });
  // animals carry the owner's surnames unless someone writes others
  /* svelte-ignore state_referenced_locally */
  let lastTouched = $state(!!patient?.last_names);
  // animals: the owner's name is typed as given names + surnames (stored together as guardian_name)
  let ownerNames = $state('');
  let ownerSurnames = $state('');
  function splitName(full: string): [string, string] {
    const w = full.trim().split(/\s+/).filter(Boolean);
    if (w.length < 2) return [w.join(' '), ''];
    const k = w.length >= 3 ? 2 : 1;
    return [w.slice(0, -k).join(' '), w.slice(-k).join(' ')];
  }
  /* svelte-ignore state_referenced_locally */
  if (patient?.subject === 'animal') [ownerNames, ownerSurnames] = splitName(patient.guardian_name ?? '');
  /* svelte-ignore state_referenced_locally */
  let profile = $state<FieldValues>({ ...(patient?.profile ?? {}) });
  let ack = $state(false);
  // animals: the owner picked from the clinic's list (their fields are read-only while one is picked)
  let owner = $state<OwnerListItem | null>(null);
  function pickOwner(o: OwnerListItem) {
    owner = o;
    f.guardian_name = o.name;
    [ownerNames, ownerSurnames] = splitName(o.name);
    f.guardian_phone = o.phone;
    f.guardian_email = o.email;
    if (o.address) f.address = o.address;
    f.owner_birth_date = o.birth_date ?? '';
  }
  function clearOwner() {
    owner = null;
    f.guardian_name = '';
    ownerNames = '';
    ownerSurnames = '';
    f.guardian_phone = '';
    f.guardian_email = '';
    f.owner_birth_date = '';
  }

  // the owner's full name follows the two fields; the pet takes the owner's surnames until someone writes others
  $effect(() => {
    if (animal && !owner) f.guardian_name = [ownerNames.trim(), ownerSurnames.trim()].filter(Boolean).join(' ');
  });
  $effect(() => {
    if (animal && !lastTouched) f.last_names = ownerSurnames.trim();
  });

  let errors = $state<Record<string, string>>({});
  let profileErrors = $state<Record<string, string>>({});
  const op = new Op();

  onMount(async () => {
    try {
      const s = await api.patients.schema();
      schema = s;
      if (!editing && !s.subjects.includes(subject)) subject = s.subjects[0] ?? 'person';
      // "+ Mascota" from an owner opens the form with that owner already picked
      const wanted = editing ? patient?.owner_id : page.url.searchParams.get('propietario');
      if (wanted && s.subjects.includes('animal')) {
        if (!editing) subject = 'animal';
        ownersApi.get(wanted).then(pickOwner, () => {});
      }
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'No se pudo cargar el formulario.';
    }
  });

  const animal = $derived(subject === 'animal');
  const mixed = $derived(!editing && (schema?.subjects.length ?? 0) > 1);
  const fields = $derived(schema?.profile[subject] ?? []);
  const sexOptions = $derived(animal ? ['Hembra', 'Macho'] : ['Mujer', 'Hombre', 'Otro']);
  const age = $derived(ageFrom(f.birth_date));
  const minor = $derived(!animal && age !== null && age < 18);
  const guardianRequired = $derived(animal || minor);
  const noticeDate = $derived(patient?.privacy_notice_at ?? null);

  function pickSubject(s: Subject) {
    if (s === subject) return;
    subject = s;
    f.sex = '';
    f.curp = '';
    profile = {};
    errors = {};
    profileErrors = {};
  }

  const empty = (v: unknown) => v == null || (typeof v === 'string' && !v.trim()) || (Array.isArray(v) && v.length === 0);

  function validate(): boolean {
    const e: Record<string, string> = {};
    const pe: Record<string, string> = {};
    if (!f.names.trim()) e.names = animal ? 'Escribe el nombre del animal.' : 'Escribe el nombre del paciente.';
    if (!animal && !f.last_names.trim()) e.last_names = 'Escribe los apellidos del paciente.';
    if (!f.sex) e.sex = 'Elige una opción.';
    if (f.birth_date && (age === null || f.birth_date > today)) e.birth_date = 'La fecha no puede ser futura.';
    const curp = f.curp.trim().toUpperCase();
    if (!animal && curp && !/^[A-Z0-9]{18}$/.test(curp)) e.curp = 'La CURP tiene 18 letras y números, sin espacios.';
    if (f.email.trim() && !/^\S+@\S+\.\S+$/.test(f.email.trim())) e.email = 'Revisa el correo electrónico.';
    if (f.guardian_email.trim() && !/^\S+@\S+\.\S+$/.test(f.guardian_email.trim())) e.guardian_email = 'Revisa el correo electrónico.';
    if (guardianRequired) {
      const who = animal ? 'del propietario' : 'del tutor';
      if (animal && !owner) {
        if (!ownerNames.trim() || !ownerSurnames.trim()) e.guardian_name = 'Escribe el nombre y los apellidos del propietario.';
      } else if (!f.guardian_name.trim()) e.guardian_name = `Escribe el nombre ${who}.`;
      if (!f.guardian_phone.trim()) e.guardian_phone = `Escribe el teléfono ${who}.`;
    }
    for (const fd of fields) {
      if (fd.required && fd.type !== 'bool' && empty(profile[fd.key])) pe[fd.key] = 'Este dato es obligatorio.';
    }
    if (!noticeDate && !ack) e.ack = 'Confirma que el paciente recibió el aviso de privacidad.';
    errors = e;
    profileErrors = pe;
    return !Object.keys(e).length && !Object.keys(pe).length;
  }

  async function focusFirstError() {
    await tick();
    const order = ['names', 'last_names', 'sex', 'birth_date', 'curp', 'email', 'guardian_name', 'guardian_phone', 'guardian_email', 'ack'];
    const key = order.find((k) => errors[k]);
    const id = key === 'guardian_name' && animal ? 'pf-owner_names' : key ? `pf-${key}` : `pf-p-${Object.keys(profileErrors)[0]}`;
    const el = document.getElementById(id) ?? document.getElementById(`${id}-l`);
    el?.scrollIntoView({ block: 'center', behavior: 'smooth' });
    (el as HTMLElement | null)?.focus?.({ preventScroll: true });
  }

  async function submit(ev: SubmitEvent) {
    ev.preventDefault();
    if (!validate()) {
      await focusFirstError();
      return;
    }
    const body: PatientInput = {
      subject,
      names: f.names.trim(),
      last_names: f.last_names.trim(),
      sex: f.sex,
      birth_date: f.birth_date,
      curp: animal ? '' : f.curp.trim().toUpperCase(),
      phone: (animal ? f.guardian_phone : f.phone).trim(),
      email: (animal ? f.guardian_email : f.email).trim(),
      address: f.address.trim(),
      owner_birth_date: animal ? f.owner_birth_date : undefined,
      guardian_name: f.guardian_name.trim(),
      guardian_relation: animal ? '' : f.guardian_relation.trim(),
      guardian_phone: f.guardian_phone.trim(),
      guardian_email: f.guardian_email.trim(),
      owner_id: animal ? (owner?.id ?? null) : null,
      profile,
      privacy_ack: true
    };
    let saved: Patient | null = null;
    const ok = await op.run(async () => {
      saved = patient ? await api.patients.update(patient.id, body) : await api.patients.create(body);
    });
    if (!ok || !saved) return;
    toast.show(editing ? 'Expediente actualizado' : 'Paciente registrado');
    await goto(`/pacientes/${(saved as Patient).id}`);
  }

  const chip = (on: boolean) =>
    `inline-flex min-h-10 items-center gap-2 rounded-full border px-4 text-sm font-medium transition ${on ? 'border-app-primary bg-app-primary/10 text-app-primary' : 'border-app-ink/15 text-app-muted hover:border-app-ink/30 hover:text-app-ink'}`;
</script>

{#snippet err(key: string)}
  {#if errors[key]}<p id="pf-{key}-h" class="mt-1 text-xs text-app-danger" role="alert">{errors[key]}</p>{/if}
{/snippet}

{#if loadError}
  <p class="alert" role="alert"><Icon name="alert" size={18} />{loadError}</p>
{:else if !schema}
  <div class="card"><LoadingRows /></div>
{:else}
  <form id="patient-form" class="flex flex-col gap-5 pb-24" onsubmit={submit} novalidate>
    <!-- ¿Persona o animal? / expediente -->
    {#if mixed || editing}
      <div class="order-first">
        {#if mixed}
          <div>
            <span class="label" id="pf-subject-l">¿Quién es el paciente?</span>
            <div class="flex flex-wrap gap-2" role="radiogroup" aria-labelledby="pf-subject-l">
              {#each schema.subjects as s}
                <button type="button" role="radio" aria-checked={subject === s} class={chip(subject === s)} onclick={() => pickSubject(s)}>
                  <Icon name={s === 'animal' ? 'paw' : 'user'} size={17} />{s === 'animal' ? 'Animal' : 'Persona'}
                </button>
              {/each}
            </div>
          </div>
        {:else if editing}
          <p class="flex items-center gap-2 text-sm text-app-muted "><Icon name={animal ? 'paw' : 'user'} size={17} />{animal ? 'Paciente animal' : 'Paciente persona'} · expediente #{patient?.file_number}</p>
        {/if}

      </div>
    {/if}

    <!-- Datos generales -->
    <section class="card p-5 sm:p-6" aria-labelledby="pf-general">
      <h2 id="pf-general" class="display mb-4 text-2xl">{animal ? 'Datos de la mascota' : 'Datos generales'}</h2>
      <div class="grid gap-4 sm:grid-cols-2">
        {#if animal}
          <div class="sm:col-span-2">
            <label class="label" for="pf-names">Nombre del animal <span class="text-app-danger" aria-hidden="true">*</span></label>
            <input id="pf-names" class="field" bind:value={f.names} maxlength="120" autocomplete="off" aria-invalid={!!errors.names} aria-describedby={errors.names ? 'pf-names-h' : undefined} />
            {@render err('names')}
          </div>
          <div class="sm:col-span-2">
            <label class="label" for="pf-last_names">Apellidos de la mascota <span class="font-normal text-app-muted">(se toman del propietario; puedes cambiarlos)</span></label>
            <input id="pf-last_names" class="field" bind:value={f.last_names} oninput={() => (lastTouched = true)} maxlength="120" autocomplete="off" placeholder="Se llenan con los apellidos del propietario" />
          </div>
        {:else}
          <div>
            <label class="label" for="pf-names">Nombre(s) <span class="text-app-danger" aria-hidden="true">*</span></label>
            <input id="pf-names" class="field" bind:value={f.names} maxlength="120" autocomplete="given-name" aria-invalid={!!errors.names} aria-describedby={errors.names ? 'pf-names-h' : undefined} />
            {@render err('names')}
          </div>
          <div>
            <label class="label" for="pf-last_names">Apellido(s) <span class="text-app-danger" aria-hidden="true">*</span></label>
            <input id="pf-last_names" class="field" bind:value={f.last_names} maxlength="120" autocomplete="family-name" aria-invalid={!!errors.last_names} aria-describedby={errors.last_names ? 'pf-last_names-h' : undefined} />
            {@render err('last_names')}
          </div>
        {/if}

        <div>
          <span class="label" id="pf-sex-l">Sexo <span class="text-app-danger" aria-hidden="true">*</span></span>
          <div class="flex flex-wrap gap-2" role="radiogroup" aria-labelledby="pf-sex-l" id="pf-sex" tabindex="-1">
            {#each sexOptions as o}
              <button type="button" role="radio" aria-checked={f.sex === o} class={chip(f.sex === o)} onclick={() => (f.sex = o)}>{o}</button>
            {/each}
          </div>
          {@render err('sex')}
        </div>

        <div>
          <label class="label" for="pf-birth_date">{animal ? 'Fecha de nacimiento aproximada' : 'Fecha de nacimiento'}{#if age !== null && !errors.birth_date} <span class="font-normal text-app-muted">({ageText(age)})</span>{/if}</label>
          <input id="pf-birth_date" class="field" type="date" max={today} bind:value={f.birth_date} aria-invalid={!!errors.birth_date} aria-describedby={errors.birth_date ? 'pf-birth_date-h' : animal ? 'pf-birth_date-t' : undefined} />
          {#if animal && !errors.birth_date}<p id="pf-birth_date-t" class="hint">Si no la sabes, pon una fecha aproximada según la edad que te indique el propietario.</p>{/if}
          {@render err('birth_date')}
        </div>

        {#if !animal}
          <div class="sm:col-span-2">
            <label class="label" for="pf-curp">CURP <span class="font-normal text-app-muted">(opcional)</span></label>
            <input id="pf-curp" class="field font-mono uppercase" bind:value={f.curp} oninput={() => (f.curp = f.curp.toUpperCase())} maxlength="18" autocomplete="off" aria-invalid={!!errors.curp} aria-describedby={errors.curp ? 'pf-curp-h' : 'pf-curp-t'} />
            {#if !errors.curp}<p id="pf-curp-t" class="hint">18 caracteres. Si el paciente no la tiene a la mano, puedes agregarla después.</p>{/if}
            {@render err('curp')}
          </div>
        {/if}
      </div>
    </section>

    <!-- Contacto (de una mascota es el del propietario, abajo) -->
    {#if !animal}
    <section class="card p-5 sm:p-6" aria-labelledby="pf-contact">
      <h2 id="pf-contact" class="display mb-4 text-2xl">Contacto</h2>
      <div class="grid gap-4 sm:grid-cols-2">
        <div>
          <label class="label" for="pf-phone">Teléfono</label>
          <input id="pf-phone" class="field" type="tel" bind:value={f.phone} maxlength="20" autocomplete="tel" />
        </div>
        <div>
          <label class="label" for="pf-email">Correo electrónico</label>
          <input id="pf-email" class="field" type="email" bind:value={f.email} maxlength="160" autocomplete="email" aria-invalid={!!errors.email} aria-describedby={errors.email ? 'pf-email-h' : undefined} />
          {@render err('email')}
        </div>
        <div class="sm:col-span-2">
          <label class="label" for="pf-address">Domicilio</label>
          <input id="pf-address" class="field" bind:value={f.address} maxlength="240" autocomplete="street-address" />
        </div>
      </div>
    </section>

    {/if}

    <!-- Responsable / tutor / propietario -->
    <section class="card p-5 sm:p-6 {animal ? 'order-first' : ''}" aria-labelledby="pf-guardian">
      <h2 id="pf-guardian" class="display text-2xl">{animal ? 'Primero, el propietario' : 'Responsable o tutor'}</h2>
      <p class="mb-4 mt-1 text-sm text-app-muted">
        {#if animal}Busca al propietario: si ya tiene otras mascotas, se agrupan con él. Si es nuevo, escribe sus datos. Es quien autoriza la atención y recibe las indicaciones.
        {:else if minor}El paciente es menor de edad ({ageText(age)}): los datos del padre, madre o tutor son obligatorios.
        {:else}Opcional para personas adultas. Se vuelve obligatorio si el paciente es menor de 18 años.{/if}
      </p>
      {#if animal}
        <div class="mb-4"><OwnerPicker selected={owner} onpick={pickOwner} onclear={clearOwner} id="pf-owner-search" /></div>
      {/if}
      <div class="grid gap-4 sm:grid-cols-2">
        {#if animal}
          <div>
            <label class="label" for="pf-owner_names">Nombre(s) del propietario <span class="text-app-danger" aria-hidden="true">*</span></label>
            <input id="pf-owner_names" class="field" bind:value={ownerNames} readonly={!!owner} maxlength="100" autocomplete="off" aria-invalid={!!errors.guardian_name} aria-describedby={errors.guardian_name ? 'pf-guardian_name-h' : undefined} />
          </div>
          <div>
            <label class="label" for="pf-owner_surnames">Apellido(s) del propietario <span class="text-app-danger" aria-hidden="true">*</span></label>
            <input id="pf-owner_surnames" class="field" bind:value={ownerSurnames} readonly={!!owner} maxlength="100" autocomplete="off" aria-invalid={!!errors.guardian_name} />
          </div>
          <div class="sm:col-span-2">{@render err('guardian_name')}</div>
        {:else}
        <div>
          <label class="label" for="pf-guardian_name">Nombre completo{#if guardianRequired} <span class="text-app-danger" aria-hidden="true">*</span>{/if}</label>
          <input id="pf-guardian_name" class="field" bind:value={f.guardian_name} maxlength="160" autocomplete="off" aria-invalid={!!errors.guardian_name} aria-describedby={errors.guardian_name ? 'pf-guardian_name-h' : undefined} />
          {@render err('guardian_name')}
        </div>
        {/if}
        <div>
          <label class="label" for="pf-guardian_phone">Teléfono{#if guardianRequired} <span class="text-app-danger" aria-hidden="true">*</span>{/if}</label>
          <input id="pf-guardian_phone" class="field" type="tel" bind:value={f.guardian_phone} readonly={!!owner} maxlength="20" autocomplete="off" aria-invalid={!!errors.guardian_phone} aria-describedby={errors.guardian_phone ? 'pf-guardian_phone-h' : undefined} />
          {@render err('guardian_phone')}
        </div>
        {#if !animal}
          <div>
            <label class="label" for="pf-guardian_relation">Parentesco</label>
            <input id="pf-guardian_relation" class="field" bind:value={f.guardian_relation} maxlength="60" placeholder="Madre, padre, tutor legal…" autocomplete="off" />
          </div>
        {/if}
        <div>
          <label class="label" for="pf-guardian_email">Correo electrónico</label>
          <input id="pf-guardian_email" class="field" type="email" bind:value={f.guardian_email} readonly={!!owner} maxlength="160" autocomplete="off" aria-invalid={!!errors.guardian_email} aria-describedby={errors.guardian_email ? 'pf-guardian_email-h' : undefined} />
          {@render err('guardian_email')}
        </div>
        {#if animal}
          <div class="sm:col-span-2">
            <label class="label" for="pf-address">Domicilio</label>
            <input id="pf-address" class="field" bind:value={f.address} maxlength="240" autocomplete="street-address" />
          </div>
          <div>
            <label class="label" for="pf-owner_birth">Fecha de nacimiento del propietario <span class="font-normal text-app-muted">(opcional)</span></label>
            <input id="pf-owner_birth" class="field" type="date" max={today} bind:value={f.owner_birth_date} aria-describedby="pf-owner_birth-t" />
            <p id="pf-owner_birth-t" class="hint">Solo informativa; no se usa para nada más.</p>
          </div>
        {/if}
      </div>
    </section>

    <!-- Datos clínicos del giro -->
    {#if fields.length}
      <section class="card p-5 sm:p-6" aria-labelledby="pf-clinical">
        <h2 id="pf-clinical" class="display mb-1 text-2xl">{animal ? 'Datos del animal' : 'Antecedentes y datos clínicos'}</h2>
        <p class="mb-5 text-sm text-app-muted">Lo que pide tu especialidad. Los campos con <span class="text-app-danger">*</span> son obligatorios.</p>
        <div class="space-y-6">
          {#key subject}
            <DynamicFields {fields} bind:values={profile} id="pf-p" errors={profileErrors} />
          {/key}
        </div>
      </section>
    {/if}

    <!-- Aviso de privacidad -->
    <section class="card p-5 sm:p-6" aria-labelledby="pf-privacy">
      <h2 id="pf-privacy" class="display mb-3 text-2xl">Aviso de privacidad</h2>
      {#if noticeDate}
        <p class="flex items-start gap-2 text-sm"><Icon name="shield" size={18} class="mt-0.5 flex-none text-app-accent" />Registrado el {dateShort(noticeDate)}{patient?.privacy_notice_by ? ` por ${patient.privacy_notice_by}` : ''}.</p>
      {:else}
        <label class="flex cursor-pointer items-start gap-3 rounded-xl border border-app-ink/15 p-4 transition hover:border-app-ink/30 {errors.ack ? 'border-app-danger' : ''}">
          <input id="pf-ack" type="checkbox" class="mt-1 h-5 w-5 flex-none accent-app-primary" bind:checked={ack} aria-invalid={!!errors.ack} aria-describedby={errors.ack ? 'pf-ack-h' : undefined} />
          <span class="text-sm">El paciente recibió el aviso de privacidad y otorga su consentimiento expreso para el tratamiento de sus datos personales y de salud.</span>
        </label>
        {@render err('ack')}
        <p class="hint">Conforme a la Ley Federal de Protección de Datos Personales en Posesión de los Particulares (LFPDPPP).</p>
      {/if}
    </section>
  </form>

  <!-- save bar -->
  <div class="page-in fixed inset-x-0 bottom-0 z-30 border-t border-app-ink/10 bg-app-panel/95 px-4 py-3 shadow-app backdrop-blur">
    <div class="mx-auto flex max-w-5xl flex-wrap items-center justify-end gap-2 lg:pl-72">
      <div class="mr-auto min-w-0 flex-1 text-sm" aria-live="polite">
        {#if op.phase === 'error'}<span class="flex items-center gap-2 text-app-danger" role="alert"><Icon name="alert" size={17} class="flex-none" />{op.message}</span>
        {:else if Object.keys(errors).length || Object.keys(profileErrors).length}<span class="text-app-danger">Revisa los campos marcados.</span>{/if}
      </div>
      <a class="btn-secondary" href={editing ? `/pacientes/${patient?.id}` : '/pacientes'}>Cancelar</a>
      <button type="submit" form="patient-form" class="btn-primary" disabled={op.phase === 'loading'}>
        {#if op.phase === 'loading'}<span class="spin"></span>{:else}<Icon name="check" size={18} stroke={2.2} />{/if}{editing ? 'Guardar cambios' : 'Registrar paciente'}
      </button>
    </div>
  </div>
{/if}
