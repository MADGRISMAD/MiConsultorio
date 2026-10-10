<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import { api } from '$lib/api';
  import { summarizeHours } from '$lib/clinic';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import { CLINIC_KINDS, type ClinicKind, type ClinicSettings } from '$lib/types';
  import ConfirmModal from '$lib/components/ConfirmModal.svelte';
  import ClinicFields from '$lib/components/setup/ClinicFields.svelte';
  import HoursEditor from '$lib/components/setup/HoursEditor.svelte';
  import SpecialtyPicker from '$lib/components/setup/SpecialtyPicker.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';

  let ready = $state(false);
  let name = $state('');
  let phone = $state('');
  let address = $state('');
  let image = $state('');
  let kind = $state<ClinicKind>('GENERAL_MEDICAL');
  let specialties = $state<ClinicKind[]>([]);
  let settings = $state<ClinicSettings>({ hours: {} as ClinicSettings['hours'], appointment_minutes: 30 });

  $effect(() => {
    if (session.status === 'authenticated') void session.loadClinic();
  });
  $effect(() => {
    const c = session.clinic;
    if (!c || ready) return;
    ({ name, kind } = c);
    phone = c.phone_number;
    address = c.address;
    image = c.image_url ?? '';
    specialties = [...c.specialties];
    settings = structuredClone($state.snapshot(c.settings));
    ready = true;
  });

  // changing the giro asks first: the patients of a giro the clinic leaves stop showing (they are kept)
  const giros = (k: ClinicKind, s: ClinicKind[]) => [...new Set([k, ...s])];
  const savedGiros = $derived(session.clinic ? giros(session.clinic.kind, session.clinic.specialties) : []);
  const newGiros = $derived(giros(kind, specialties));
  const leaving = $derived(savedGiros.filter((g) => !newGiros.includes(g)));
  const joining = $derived(newGiros.filter((g) => !savedGiros.includes(g)));
  const giroChanged = $derived(!!session.clinic && (session.clinic.kind !== kind || leaving.length > 0 || joining.length > 0));
  let confirmGiro = $state(false);
  const label = (g: ClinicKind) => CLINIC_KINDS[g]?.label ?? g;
  const dataOp = new Op();
  const kindOp = new Op();
  const hoursOp = new Op();
  async function save(op: Op, patch: Parameters<typeof api.updateClinic>[0], done: string) {
    if (await op.run(async () => session.setClinic(await api.updateClinic(patch)))) toast.show(done);
  }
</script>

{#if !ready}
  <div class="card"><LoadingRows /></div>
{:else}
  <div class="grid gap-4">
    <form class="card p-4 sm:p-6" onsubmit={(e) => { e.preventDefault(); void save(dataOp, { name, phone_number: phone, address, image_url: image }, 'Datos guardados'); }}>
      <h2 class="display mb-5 text-3xl">Nombre y contacto</h2>
      <ClinicFields bind:name bind:phone bind:address bind:image />
      <OpError op={dataOp} class="mt-4" />
      <div class="save-sticky"><button type="submit" class="btn-primary" disabled={dataOp.phase === 'loading'}>Guardar datos</button></div>
    </form>

    <form class="card p-4 sm:p-6" onsubmit={(e) => { e.preventDefault(); if (giroChanged) { kindOp.reset(); confirmGiro = true; } else void save(kindOp, { kind, specialties }, 'Giro guardado'); }}>
      <h2 class="display mb-5 text-3xl">Giro y especialidades</h2>
      <SpecialtyPicker bind:kind bind:specialties />
      {#if session.user?.billing?.max_kinds != null}
        <p class="hint mt-2">Tu plan {session.user.billing.plan_name} permite hasta {session.user.billing.max_kinds} {session.user.billing.max_kinds === 1 ? 'giro' : 'giros'} ({newGiros.length} elegido{newGiros.length === 1 ? '' : 's'}).{#if newGiros.length > session.user.billing.max_kinds} <strong class="text-app-warning">Para guardar necesitas quitar giros o cambiar de plan.</strong>{/if}</p>
      {/if}
      <OpError op={kindOp} class="mt-4" />
      <div class="save-sticky"><button type="submit" class="btn-primary" disabled={kindOp.phase === 'loading'}>Guardar giro</button></div>
    </form>

    <form class="card p-4 sm:p-6" onsubmit={(e) => { e.preventDefault(); void save(hoursOp, { settings: $state.snapshot(settings) }, 'Horario guardado'); }}>
      <h2 class="display mb-1 text-3xl">Horario y citas</h2>
      <p class="mb-5 text-sm text-app-muted">Ahora: {summarizeHours(settings)} · citas de {settings.appointment_minutes} min</p>
      <HoursEditor bind:settings />
      <OpError op={hoursOp} class="mt-4" />
      <div class="save-sticky"><button type="submit" class="btn-primary" disabled={hoursOp.phase === 'loading'}>Guardar horario</button></div>
    </form>
  </div>
{/if}

<ConfirmModal
  open={confirmGiro}
  title="¿Cambiar el giro del consultorio?"
  confirmLabel="Sí, cambiar giro"
  op={kindOp}
  onclose={() => (confirmGiro = false)}
  onconfirm={async () => {
    await save(kindOp, { kind, specialties }, 'Giro guardado');
    if (kindOp.phase !== 'error') confirmGiro = false;
  }}
>
  {#if leaving.length}
    <p>Los pacientes de <strong class="text-app-ink">{leaving.map(label).join(', ')}</strong> dejarán de mostrarse en listas y búsquedas.</p>
    <p class="mt-2 text-sm"><strong class="text-app-ink">No se elimina nada:</strong> sus expedientes, citas y cobros se conservan, y vuelven a aparecer si más adelante activas de nuevo ese giro.</p>
  {/if}
  {#if joining.length}
    <p class="{leaving.length ? 'mt-2' : ''}">Se agregará <strong class="text-app-ink">{joining.map(label).join(', ')}</strong>: los formularios y secciones se ajustan a esa especialidad.</p>
  {/if}
  {#if !leaving.length && !joining.length}
    <p>El giro principal pasará a ser <strong class="text-app-ink">{label(kind)}</strong>.</p>
  {/if}
  <p class="mt-2 text-sm">Los pacientes que registres desde ahora pertenecen a los giros activos. Si trabajas con más de un giro, todos se muestran juntos.</p>
</ConfirmModal>
