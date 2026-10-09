<script lang="ts">
  import { Loader } from '$lib/loader.svelte';
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { ago } from '$lib/format';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import { PLATFORM_ROLES, ROLES, type Person, type PlatformRole } from '$lib/types';
  import ConfirmModal from '../ConfirmModal.svelte';
  import Modal from '../Modal.svelte';
  import Avatar from '../ui/Avatar.svelte';
  import Icon from '../ui/Icon.svelte';
  import LoadingRows from '../ui/LoadingRows.svelte';
  import PageHeader from '../ui/PageHeader.svelte';
  import Pill from '../ui/Pill.svelte';
  import RolePill from '../ui/RolePill.svelte';

  let people = $state<Person[]>([]);
  const ld = new Loader('No se pudo leer el equipo.');

  async function load() {
    await ld.run(async () => {
        people = (await api.platform.staff()).people;
    });
  }
  onMount(load);

  const me = $derived(session.user?.userId);
  // Solo un permanente puede restablecer la contraseña de otro permanente.
  const mePermanent = $derived(people.some((p) => p.id === me && p.permanent));
  const active = $derived(people.filter((p) => !p.disabled));
  const inactive = $derived(people.filter((p) => p.disabled));

  let addOpen = $state(false);
  let form = $state({ name: '', email: '', username: '', password: '', role: 'platform_support' as PlatformRole });
  const addOp = new Op();
  function openAdd() {
    form = { name: '', email: '', username: '', password: '', role: 'platform_support' };
    addOp.reset();
    addOpen = true;
  }
  async function submitAdd(e: SubmitEvent) {
    e.preventDefault();
    if (!(await addOp.run(() => api.platform.addStaff(form)))) return;
    addOpen = false;
    toast.show(`${form.name} se agregó al equipo`);
    await load();
  }

  let roleChange = $state<{ person: Person; role: PlatformRole } | null>(null);
  const roleOp = new Op();
  async function confirmRole() {
    const rc = roleChange;
    if (!rc) return;
    if (await roleOp.run(() => api.platform.updateStaff(rc.person.id, { role: rc.role }))) {
      roleChange = null;
      toast.show('Rol actualizado');
      await load();
    }
  }

  let toggle = $state<{ person: Person; disable: boolean } | null>(null);
  const toggleOp = new Op();
  async function confirmToggle() {
    const t = toggle;
    if (!t) return;
    if (await toggleOp.run(() => (t.disable ? api.platform.deactivateStaff(t.person.id) : api.platform.reactivateStaff(t.person.id)))) {
      toggle = null;
      toast.show(t.disable ? 'Cuenta desactivada' : 'Cuenta reactivada');
      await load();
    }
  }

  let pwFor = $state<Person | null>(null);
  let newPassword = $state('');
  const pwOp = new Op();
  async function submitPw(e: SubmitEvent) {
    e.preventDefault();
    const target = pwFor;
    if (!target) return;
    if (await pwOp.run(() => api.platform.resetStaffPassword(target.id, newPassword))) {
      pwFor = null;
      toast.show('Contraseña restablecida');
    }
  }
</script>

<PageHeader title="Equipo de plataforma" subtitle="Las personas que administran Caresia: negocios, suscripciones y soporte.">
  {#snippet actions()}
    <button type="button" class="btn-primary" onclick={openAdd}><Icon name="user-plus" size={18} />Agregar persona</button>
  {/snippet}
</PageHeader>

<section class="card overflow-hidden">
  {#if ld.loading}
    <LoadingRows />
  {:else if ld.error}
    <Alert class="m-5">{ld.error}</Alert>
  {:else}
    <ul class="divide-y divide-app-ink/8">
      {#each active as p (p.id)}
        {@const self = p.id === me}
        <li class="flex flex-wrap items-center gap-x-4 gap-y-3 px-5 py-4">
          <Avatar name={p.name} size={44} />
          <div class="min-w-0 flex-1 basis-48">
            <p class="flex flex-wrap items-center gap-2 font-semibold"><span class="truncate">{p.name}</span>{#if self}<Pill tone="info">Tú</Pill>{/if}{#if p.permanent}<Pill tone="ok">Permanente</Pill>{/if}</p>
            <p class="truncate text-sm text-app-muted">{p.email} · {p.last_login_at ? `entró ${ago(p.last_login_at)}` : 'aún no ha entrado'}</p>
          </div>
          {#if self}
            <RolePill role={p.role} long />
          {:else if p.permanent}
            <RolePill role={p.role} long />
            {#if mePermanent}
              <button type="button" class="icon-btn" title="Restablecer contraseña" aria-label="Restablecer la contraseña de {p.name}" onclick={() => { newPassword = ''; pwOp.reset(); pwFor = p; }}><Icon name="key" size={18} /></button>
            {/if}
          {:else}
            <label class="sr-only" for="srole-{p.id}">Rol de {p.name}</label>
            <select id="srole-{p.id}" class="field !min-h-9 w-auto !py-1 pr-8 text-sm" value={p.role} onchange={(e) => { const role = e.currentTarget.value as PlatformRole; e.currentTarget.value = p.role; roleOp.reset(); roleChange = { person: p, role }; }}>
              {#each PLATFORM_ROLES as r}<option value={r}>{ROLES[r].label}</option>{/each}
            </select>
            <div class="flex gap-1">
              <button type="button" class="icon-btn" title="Restablecer contraseña" aria-label="Restablecer la contraseña de {p.name}" onclick={() => { newPassword = ''; pwOp.reset(); pwFor = p; }}><Icon name="key" size={18} /></button>
              <button type="button" class="icon-btn danger" title="Desactivar" aria-label="Desactivar a {p.name}" onclick={() => { toggleOp.reset(); toggle = { person: p, disable: true }; }}><Icon name="ban" size={18} /></button>
            </div>
          {/if}
        </li>
      {/each}
    </ul>
    {#if inactive.length}
      <div class="border-t border-app-ink/10 bg-app-elevated px-5 py-2.5"><p class="section-title">Desactivadas</p></div>
      <ul class="divide-y divide-app-ink/8">
        {#each inactive as p (p.id)}
          <li class="flex flex-wrap items-center gap-x-4 gap-y-3 px-5 py-4 opacity-75">
            <Avatar name={p.name} size={44} />
            <div class="min-w-0 flex-1 basis-48"><p class="truncate font-semibold">{p.name}</p><p class="truncate text-sm text-app-muted">{p.email} · {ROLES[p.role]?.label}</p></div>
            <Pill>Desactivada</Pill>
            <button type="button" class="btn-secondary !min-h-9" onclick={() => { toggleOp.reset(); toggle = { person: p, disable: false }; }}><Icon name="refresh" size={16} />Reactivar</button>
          </li>
        {/each}
      </ul>
    {/if}
    <p class="border-t border-app-ink/10 px-5 py-4 text-sm text-app-muted">
      <strong class="text-app-ink">Administrador</strong>: negocios, suscripciones, pagos y este equipo. <strong class="text-app-ink">Soporte</strong>: consulta negocios y su actividad, sin cambiar nada. Siempre debe quedar al menos un administrador activo.
    </p>
  {/if}
</section>

<Modal open={addOpen} title="Agregar a la plataforma" onclose={() => (addOpen = false)}>
  <form id="add-staff" class="grid gap-3.5 text-left sm:grid-cols-2" onsubmit={submitAdd}>
    <div class="sm:col-span-2"><label class="label" for="s-name">Nombre completo</label><input id="s-name" class="field" bind:value={form.name} required minlength="2" autocomplete="off" /></div>
    <div><label class="label" for="s-email">Correo</label><input id="s-email" class="field" type="email" bind:value={form.email} required autocomplete="off" /></div>
    <div><label class="label" for="s-user">Usuario</label><input id="s-user" class="field" bind:value={form.username} required minlength="3" pattern="[A-Za-z0-9._\-]+" autocomplete="off" /></div>
    <div class="sm:col-span-2"><label class="label" for="s-pass">Contraseña inicial</label><input id="s-pass" class="field" type="password" bind:value={form.password} required minlength="8" maxlength="72" autocomplete="new-password" /></div>
    <fieldset class="sm:col-span-2">
      <legend class="label">Rol</legend>
      <div class="grid gap-2">
        {#each PLATFORM_ROLES as r}
          <label class="flex cursor-pointer items-start gap-3 rounded-xl border p-3 transition {form.role === r ? 'border-app-primary bg-app-primary/8' : 'border-app-ink/10 hover:border-app-ink/25'}">
            <input type="radio" name="srole" class="mt-1 accent-[rgb(var(--app-primary))]" value={r} bind:group={form.role} />
            <span><span class="block text-sm font-semibold">{ROLES[r].label}</span><span class="block text-xs text-app-muted">{ROLES[r].about}</span></span>
          </label>
        {/each}
      </div>
    </fieldset>
  </form>
  <OpError op={addOp} class="mt-4" />
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={() => (addOpen = false)}>Cancelar</button>
    <button type="submit" form="add-staff" class="btn-primary" disabled={addOp.phase === 'loading'}>{#if addOp.phase === 'loading'}<span class="spin"></span>{/if}Agregar</button>
  {/snippet}
</Modal>

<ConfirmModal open={roleChange !== null} title="¿Cambiar el rol?" confirmLabel="Cambiar rol" op={roleOp} onconfirm={confirmRole} onclose={() => (roleChange = null)}>
  {#if roleChange}<p><strong class="text-app-ink">{roleChange.person.name}</strong> pasará a <strong class="text-app-ink">{ROLES[roleChange.role].label}</strong>. Su sesión se cierra y volverá a entrar con el rol nuevo.</p>{/if}
</ConfirmModal>

<ConfirmModal open={toggle !== null} title={toggle?.disable ? '¿Desactivar esta cuenta?' : '¿Reactivar esta cuenta?'} confirmLabel={toggle?.disable ? 'Desactivar' : 'Reactivar'} op={toggleOp} onconfirm={confirmToggle} onclose={() => (toggle = null)}>
  {#if toggle}<p>{toggle.disable ? `${toggle.person.name} dejará de poder entrar al instante.` : `${toggle.person.name} podrá volver a entrar.`}</p>{/if}
</ConfirmModal>

<Modal open={pwFor !== null} title="Restablecer contraseña" onclose={() => (pwFor = null)}>
  <form id="pw-staff" class="text-left" onsubmit={submitPw}>
    <p class="mb-4 text-sm text-app-muted">Contraseña nueva para <strong class="text-app-ink">{pwFor?.name}</strong>. Su sesión actual se cerrará.</p>
    <label class="label" for="spw">Nueva contraseña</label>
    <input id="spw" class="field" type="password" bind:value={newPassword} required minlength="8" maxlength="72" autocomplete="new-password" />
  </form>
  <OpError op={pwOp} class="mt-4" />
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={() => (pwFor = null)}>Cancelar</button>
    <button type="submit" form="pw-staff" class="btn-primary" disabled={pwOp.phase === 'loading'}>{#if pwOp.phase === 'loading'}<span class="spin"></span>{/if}Guardar</button>
  {/snippet}
</Modal>
