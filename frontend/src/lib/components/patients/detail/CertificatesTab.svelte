<script lang="ts">
  import { Loader } from '$lib/loader.svelte';
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { certificatesApi, type Certificate, type CertKind } from '$lib/api/certificates';
  import { specialtyApi } from '$lib/api/specialty';
  import { dateShort, fullName } from '$lib/format';
  import { Op } from '$lib/op.svelte';
  import { printCertificate } from '$lib/print';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { Patient } from '$lib/types';
  import ConfirmModal from '../../ConfirmModal.svelte';
  import Modal from '../../Modal.svelte';
  import EmptyState from '../../ui/EmptyState.svelte';
  import Icon from '../../ui/Icon.svelte';
  import Pill from '../../ui/Pill.svelte';

  let { patient, canWrite, onchange = () => {} }: { patient: Patient; canWrite: boolean; onchange?: () => void } = $props();

  const vet = $derived(patient.subject === 'animal');
  const kind = $derived<CertKind>(vet ? 'veterinary' : 'medical');
  let list = $state<Certificate[]>([]);
  let purposes = $state<string[]>([]);
  const ld = new Loader('No se pudieron cargar los certificados.');

  async function load() {
    await ld.run(async () => {
      const r = await certificatesApi.list(patient.id);
      list = r.certificates;
      purposes = r.purposes[kind] ?? [];
    });
    onchange();
  }
  onMount(load);

  const expired = (c: Certificate) => !!c.valid_until && new Date(`${c.valid_until}T23:59:59`) < new Date();

  // ---- new certificate ----
  let adding = $state(false);
  const blank = () => ({ purpose: '', statement: '', findings: '', aptitude: '', restrictions: '', valid_days: 0, destination: '', microchip: '', vaccines: '' });
  let form = $state(blank());
  let edited = $state(false); // the text was typed by hand: do not overwrite it
  const addOp = new Op();

  const today = () => new Date().toLocaleDateString('es-MX', { day: 'numeric', month: 'long', year: 'numeric' });
  const profileText = (k: string) => String(patient.profile?.[k] ?? '').trim();

  function draft(): string {
    const who = fullName(patient);
    const clinic = session.clinic?.name ?? 'el consultorio';
    const purpose = form.purpose ? form.purpose.toLowerCase() : 'los fines que el interesado requiera';
    if (vet) {
      const species = [profileText('species') || profileText('especie'), profileText('breed') || profileText('raza')].filter(Boolean).join(', ');
      const owner = patient.guardian_name ? `, propiedad de ${patient.guardian_name}` : '';
      return `Por medio de la presente certifico que el animal «${patient.names}»${species ? ` (${species})` : ''}${owner}, fue examinado el día ${today()} en ${clinic}. Al momento de la exploración se encuentra clínicamente ${form.findings ? 'con los hallazgos descritos' : 'sano, sin signos aparentes de enfermedad infectocontagiosa'}. Se extiende el presente certificado a petición del propietario para fines de ${purpose}.`;
    }
    const age = patient.age != null ? `, de ${patient.age} ${patient.age === 1 ? 'año' : 'años'} de edad` : '';
    const fit = { apto: 'Se encuentra apto(a) para la actividad solicitada.', no_apto: 'No se encuentra apto(a) para la actividad solicitada.', restricciones: `Se encuentra apto(a) con las siguientes restricciones: ${form.restrictions || '(indicar)'}.` }[form.aptitude as 'apto'] ?? '';
    return `Por medio de la presente certifico que ${who}${age}, fue valorado(a) el día ${today()} en ${clinic}. ${form.findings ? 'A la exploración física: ' + form.findings.trim() + '. ' : 'A la exploración física se encuentra clínicamente sano(a). '}${fit} Se extiende el presente certificado a petición del interesado para fines ${purpose}.`.replace(/\s+/g, ' ');
  }
  function redraft() {
    if (!edited) form.statement = draft();
  }

  async function openAdd() {
    form = blank();
    form.purpose = purposes[0] ?? '';
    edited = false;
    addOp.reset();
    adding = true;
    if (vet) {
      try {
        const v = await specialtyApi.vaccinations(patient.id);
        const live = v.vaccinations.filter((x) => !x.voided_at);
        form.vaccines = live
          .slice(0, 12)
          .map((x) => `${x.name} (aplicada ${dateShort(x.applied_on)}${x.next_due ? `, próxima ${dateShort(x.next_due)}` : ''})`)
          .join('; ');
      } catch {
        /* the field stays empty and can be typed */
      }
    }
    form.statement = draft();
  }

  async function submit() {
    if (!form.purpose) return addOp.fail('Elige para qué es el certificado.');
    if (!form.statement.trim()) return addOp.fail('Escribe el texto del certificado.');
    let created: Certificate | null = null;
    if (!(await addOp.run(async () => (created = await certificatesApi.create(patient.id, { ...form, valid_days: Number(form.valid_days) || 0 }))))) return;
    adding = false;
    toast.show('Certificado emitido');
    await load();
    if (created) print(created);
  }

  // ---- print and void ----
  let busy = $state('');
  async function print(c: Certificate) {
    busy = c.id;
    try {
      await printCertificate(c.id);
    } catch (e) {
      toast.show(e instanceof Error ? e.message : 'No se pudo imprimir.', 'error');
    } finally {
      busy = '';
    }
  }
  let voiding = $state<Certificate | null>(null);
  let reason = $state('');
  const voidOp = new Op();
  async function confirmVoid() {
    const c = voiding;
    if (!c) return;
    if (!reason.trim()) return voidOp.fail('Escribe el motivo de la cancelación.');
    if (await voidOp.run(() => certificatesApi.void(c.id, reason.trim()))) {
      voiding = null;
      toast.show('Certificado cancelado');
      load();
    }
  }
</script>

{#if ld.loading}
  <div class="card h-48 animate-pulse"></div>
{:else if ld.error}
  <Alert>{ld.error}</Alert>
{:else}
  <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
    <p class="max-w-xl text-sm text-app-muted">
      {vet ? 'Certificados veterinarios' : 'Certificados médicos'} de {patient.names}. Salen con tu nombre, cédula, folio y un código QR para verificarlos; los firmas a mano.
      {vet ? 'No sustituyen el certificado zoosanitario oficial.' : 'No son incapacidades.'}
    </p>
    {#if canWrite}<button type="button" class="btn-primary" onclick={openAdd}><Icon name="plus" size={18} />Nuevo certificado</button>{/if}
  </div>

  {#if list.length === 0}
    <div class="card"><EmptyState icon="file" title="Sin certificados" text="Aquí quedan los certificados que emitas a {patient.names}, con su folio.">
      {#if canWrite}<button type="button" class="btn-primary" onclick={openAdd}><Icon name="plus" size={18} />Nuevo certificado</button>{/if}
    </EmptyState></div>
  {:else}
    <ul class="space-y-3">
      {#each list as c (c.id)}
        <li class="card p-4 {c.voided_at ? 'opacity-70' : ''}">
          <div class="flex flex-wrap items-center gap-2">
            <span class="font-medium">Folio {String(c.folio).padStart(6, '0')} · {c.purpose}</span>
            {#if c.voided_at}<Pill tone="bad">Cancelado</Pill>{:else if expired(c)}<Pill tone="warn">Vencido</Pill>{:else}<Pill tone="ok">Vigente</Pill>{/if}
          </div>
          <p class="text-xs text-app-muted">Emitido {dateShort(c.issued_at)} por {c.author_name}{c.valid_until ? ` · vigente hasta ${dateShort(c.valid_until)}` : ''}</p>
          <p class="mt-1 line-clamp-3 text-sm">{c.statement}</p>
          {#if c.voided_at}<p class="mt-1 text-xs text-app-danger">Cancelado por {c.voided_by}: {c.void_reason}</p>{/if}
          <div class="mt-2 flex flex-wrap gap-1">
            <button type="button" class="btn-ghost" disabled={busy === c.id} onclick={() => print(c)}><Icon name="receipt" size={16} />Ver / imprimir</button>
            {#if canWrite && !c.voided_at}<button type="button" class="btn-ghost text-app-danger" onclick={() => { voiding = c; reason = ''; voidOp.reset(); }}><Icon name="ban" size={16} />Cancelar</button>{/if}
          </div>
        </li>
      {/each}
    </ul>
  {/if}
{/if}

<Modal open={adding} title={vet ? 'Nuevo certificado veterinario' : 'Nuevo certificado médico'} onclose={() => (adding = false)} wide>
  <form id="cert-form" class="space-y-4" onsubmit={(e) => { e.preventDefault(); submit(); }}>
    <div class="grid gap-4 sm:grid-cols-2">
      <div>
        <label class="label" for="ce-purpose">Para qué es</label>
        <select id="ce-purpose" class="field" bind:value={form.purpose} onchange={redraft}>{#each purposes as p}<option value={p}>{p}</option>{/each}</select>
      </div>
      <div>
        <label class="label" for="ce-valid">Vigencia</label>
        <select id="ce-valid" class="field" bind:value={form.valid_days}>
          <option value={0}>Sin fecha de vencimiento</option>
          {#each [15, 30, 90, 180, 365] as d}<option value={d}>{d} días</option>{/each}
        </select>
      </div>
      {#if !vet}
        <div>
          <label class="label" for="ce-apt">Aptitud <span class="font-normal text-app-muted">(opcional)</span></label>
          <select id="ce-apt" class="field" bind:value={form.aptitude} onchange={redraft}>
            <option value="">Sin especificar</option>
            <option value="apto">Apto(a)</option>
            <option value="no_apto">No apto(a)</option>
            <option value="restricciones">Apto(a) con restricciones</option>
          </select>
        </div>
        {#if form.aptitude === 'restricciones'}
          <div>
            <label class="label" for="ce-res">Restricciones</label>
            <input id="ce-res" class="field" maxlength="500" bind:value={form.restrictions} oninput={redraft} />
          </div>
        {/if}
      {:else}
        <div>
          <label class="label" for="ce-dest">Destino <span class="font-normal text-app-muted">(si viaja)</span></label>
          <input id="ce-dest" class="field" maxlength="200" bind:value={form.destination} />
        </div>
        <div>
          <label class="label" for="ce-chip">Microchip <span class="font-normal text-app-muted">(opcional)</span></label>
          <input id="ce-chip" class="field" maxlength="40" bind:value={form.microchip} />
        </div>
        <div class="sm:col-span-2">
          <label class="label" for="ce-vac">Vacunación y desparasitación</label>
          <textarea id="ce-vac" class="field" rows="2" maxlength="1000" bind:value={form.vaccines}></textarea>
          <p class="hint">Se llenó con lo registrado en el carnet; corrígelo si hace falta.</p>
        </div>
      {/if}
      <div class="sm:col-span-2">
        <label class="label" for="ce-find">{vet ? 'Hallazgos de la exploración' : 'Exploración'} <span class="font-normal text-app-muted">(opcional)</span></label>
        <textarea id="ce-find" class="field" rows="2" maxlength="1500" bind:value={form.findings} oninput={redraft}></textarea>
      </div>
      <div class="sm:col-span-2">
        <label class="label" for="ce-text">Texto del certificado</label>
        <textarea id="ce-text" class="field" rows="6" maxlength="3000" bind:value={form.statement} oninput={() => (edited = true)}></textarea>
        <p class="hint">Se arma solo con lo que llenaste; puedes editarlo. Tú eres quien certifica: revísalo antes de emitir. Al emitirlo se imprime con tu cédula, folio y código QR.</p>
        {#if edited}<button type="button" class="btn-ghost mt-1" onclick={() => { edited = false; form.statement = draft(); }}><Icon name="refresh" size={16} />Volver a generar el texto</button>{/if}
      </div>
    </div>
    <OpError op={addOp} />
  </form>
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={() => (adding = false)}>Cancelar</button>
    <button type="submit" form="cert-form" class="btn-primary" disabled={addOp.phase === 'loading'}>{#if addOp.phase === 'loading'}<span class="spin"></span>{/if}Emitir e imprimir</button>
  {/snippet}
</Modal>

<ConfirmModal open={!!voiding} title="Cancelar certificado" op={voidOp} onconfirm={confirmVoid} onclose={() => (voiding = null)} confirmLabel="Cancelar certificado">
  <p>Quedará en el expediente como cancelado y el código QR dirá que ya no es válido. No se puede deshacer.</p>
  <label class="label mt-4" for="cert-reason">Motivo</label>
  <input id="cert-reason" class="field" bind:value={reason} autocomplete="off" maxlength="200" />
</ConfirmModal>
