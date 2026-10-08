<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { ago } from '$lib/format';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import { CAPABILITY_ROWS, CLINIC_ROLES, ROLE_PERMISSIONS, ROLES, type ClinicRole, type Person, type Seats } from '$lib/types';
  import ConfirmModal from './ConfirmModal.svelte';
  import Modal from './Modal.svelte';
  import Avatar from './ui/Avatar.svelte';
  import Icon from './ui/Icon.svelte';
  import LoadingRows from './ui/LoadingRows.svelte';
  import PageHeader from './ui/PageHeader.svelte';
  import Pill from './ui/Pill.svelte';
  import RolePill from './ui/RolePill.svelte';

  let people = $state<Person[]>([]);
  let seats = $state<Seats | null>(null);
  let loading = $state(true);
  let loadError = $state('');

  async function load() {
    try {
      const r = await api.team();
      people = r.people;
      seats = r.seats;
      loadError = '';
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'No se pudo leer el equipo.';
    } finally {
      loading = false;
    }
  }
  onMount(load);

  const me = $derived(session.user?.userId);
  const active = $derived(people.filter((p) => !p.disabled));
  const inactive = $derived(people.filter((p) => p.disabled));
  const pct = (used: number, max: number | null) => (max ? Math.min(100, Math.round((used / max) * 100)) : 0);

  // ----- add a person -----
  let addOpen = $state(false);
  let form = $state({ name: '', email: '', username: '', phone: '', password: '', role: 'reception' as ClinicRole });
  const addOp = new Op();
  function openAdd() {
    form = { name: '', email: '', username: '', phone: '', password: '', role: 'reception' };
    addOp.reset();
    addOpen = true;
  }
  async function submitAdd(e: SubmitEvent) {
    e.preventDefault();
    if (!(await addOp.run(() => api.addMember(form)))) return;
    addOpen = false;
    toast.show(`${form.name} ya puede entrar`);
    await load();
  }

  // ----- change role -----
  let roleChange = $state<{ person: Person; role: ClinicRole } | null>(null);
  const roleOp = new Op();
  async function confirmRole() {
    const rc = roleChange;
    if (!rc) return;
    if (await roleOp.run(() => api.updateMember(rc.person.id, { role: rc.role }))) {
      roleChange = null;
      toast.show('Rol actualizado. Tendrá que volver a iniciar sesión.');
      await load();
    }
  }

  // ----- deactivate / reactivate -----
  let toggle = $state<{ person: Person; disable: boolean } | null>(null);
  const toggleOp = new Op();
  async function confirmToggle() {
    const t = toggle;
    if (!t) return;
    if (await toggleOp.run(() => (t.disable ? api.deactivateMember(t.person.id) : api.reactivateMember(t.person.id)))) {
      toggle = null;
      toast.show(t.disable ? 'Cuenta desactivada' : 'Cuenta reactivada');
      await load();
    }
  }

  // ----- reset password -----
  let pwFor = $state<Person | null>(null);
  let newPassword = $state('');
  const pwOp = new Op();
  async function submitPw(e: SubmitEvent) {
    e.preventDefault();
    const target = pwFor;
    if (!target) return;
    if (await pwOp.run(() => api.resetMemberPassword(target.id, newPassword))) {
      pwFor = null;
      toast.show('Contraseña restablecida');
    }
  }
</script>

<PageHeader title="Equipo" subtitle="Quién entra a tu consultorio y qué puede hacer cada persona.">
  {#snippet actions()}
    <button type="button" class="btn-primary" onclick={openAdd}><Icon name="user-plus" size={18} />Agregar persona</button>
  {/snippet}
</PageHeader>

{#if seats}
  <section class="card mb-4 p-5 sm:p-6" aria-label="Lugares del plan">
    <div class="flex flex-wrap items-baseline justify-between gap-2">
      <div>
        <h2 class="display text-2xl">Plan {seats.plan_name}</h2>
        <p class="text-sm text-app-muted">
          {seats.used_users} {seats.used_users === 1 ? 'persona entra' : 'personas entran'} a la app
          {#if seats.max_doctors !== null}· {seats.used_doctors} de {seats.max_doctors} médicos o especialistas{/if}
        </p>
      </div>
      <strong class="font-mono text-sm">{seats.max_users === null ? 'Sin límite de cuentas' : `${seats.used_users} de ${seats.max_users} cuentas`}</strong>
    </div>
    {#if seats.max_users !== null}
      <div class="mt-3 h-2 overflow-hidden rounded-full bg-app-ink/10" aria-hidden="true">
        <div class="h-full rounded-full {pct(seats.used_users, seats.max_users) >= 100 ? 'bg-app-warning' : 'bg-app-primary'}" style="width: {pct(seats.used_users, seats.max_users)}%"></div>
      </div>
      {#if seats.used_users >= seats.max_users}
        <p class="mt-3 text-sm text-app-warning">Ya usas todos los lugares. Desactiva a alguien o pide un plan mayor para agregar más personas.</p>
      {/if}
    {/if}
  </section>
{/if}

<section class="card overflow-hidden">
  {#if loading}
    <LoadingRows />
  {:else if loadError}
    <p class="alert m-5" role="alert"><Icon name="alert" size={18} />{loadError}</p>
  {:else}
    <ul class="divide-y divide-app-ink/8">
      {#each active as p (p.id)}
        {@const self = p.id === me}
        <li class="flex flex-wrap items-center gap-x-4 gap-y-3 px-5 py-4">
          <Avatar name={p.name} size={44} />
          <div class="min-w-0 flex-1 basis-48">
            <p class="flex flex-wrap items-center gap-2 font-semibold">
              <span class="truncate">{p.name}</span>
              {#if self}<Pill tone="info">Tú</Pill>{/if}
            </p>
            <p class="truncate text-sm text-app-muted">{p.email || `@${p.username}`} · {p.last_login_at ? `entró ${ago(p.last_login_at)}` : 'aún no ha entrado'}</p>
          </div>
          {#if self}
            <RolePill role={p.role} long />
          {:else}
            <label class="sr-only" for="role-{p.id}">Rol de {p.name}</label>
            <select
              id="role-{p.id}"
              class="field !min-h-9 w-auto !py-1 pr-8 text-sm"
              value={p.role}
              onchange={(e) => {
                const role = e.currentTarget.value as ClinicRole;
                e.currentTarget.value = p.role; // the list only changes once the server confirms
                roleOp.reset();
                roleChange = { person: p, role };
              }}
            >
              {#each CLINIC_ROLES as r}<option value={r}>{ROLES[r].label}</option>{/each}
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
            <div class="min-w-0 flex-1 basis-48">
              <p class="truncate font-semibold">{p.name}</p>
              <p class="truncate text-sm text-app-muted">{p.email || `@${p.username}`} · {ROLES[p.role]?.label}</p>
            </div>
            <Pill>Desactivada</Pill>
            <button type="button" class="btn-secondary !min-h-9" onclick={() => { toggleOp.reset(); toggle = { person: p, disable: false }; }}><Icon name="refresh" size={16} />Reactivar</button>
          </li>
        {/each}
      </ul>
    {/if}

    <p class="border-t border-app-ink/10 px-5 py-4 text-sm text-app-muted">
      Si alguien se va, desactívalo: deja de poder entrar al instante, aunque tenga la sesión abierta, y libera su lugar en el plan. Al cambiar un rol o una contraseña, la persona vuelve a iniciar sesión. Siempre debe quedar al menos un administrador activo.
    </p>
  {/if}
</section>

<section class="card mt-4 overflow-hidden" aria-label="Qué puede hacer cada rol">
  <div class="px-5 pb-2 pt-5"><h2 class="display text-2xl">Qué puede hacer cada rol</h2></div>
  <div class="relative overflow-x-auto">
    <table class="w-full min-w-[34rem] text-sm">
      <thead>
        <tr class="border-b border-app-ink/10">
          <th class="th">Acción</th>
          {#each CLINIC_ROLES as r}<th class="th text-center">{ROLES[r].short}</th>{/each}
        </tr>
      </thead>
      <tbody class="divide-y divide-app-ink/8">
        {#each CAPABILITY_ROWS as [label, perm]}
          <tr>
            <td class="td">{label}</td>
            {#each CLINIC_ROLES as r}
              <td class="td text-center">
                {#if ROLE_PERMISSIONS[r].includes(perm)}<Icon name="check" size={18} stroke={2.4} class="mx-auto text-app-accent" /><span class="sr-only">Sí</span>{:else}<span class="text-app-muted/50" aria-label="No">—</span>{/if}
              </td>
            {/each}
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</section>

<Modal open={addOpen} title="Agregar persona" onclose={() => (addOpen = false)}>
  <form id="add-member" class="grid gap-3.5 text-left sm:grid-cols-2" onsubmit={submitAdd}>
    <div class="sm:col-span-2">
      <label class="label" for="m-name">Nombre completo</label>
      <input id="m-name" class="field" bind:value={form.name} required minlength="2" autocomplete="off" />
    </div>
    <div>
      <label class="label" for="m-email">Correo</label>
      <input id="m-email" class="field" type="email" bind:value={form.email} required autocomplete="off" />
    </div>
    <div>
      <label class="label" for="m-user">Usuario</label>
      <input id="m-user" class="field" bind:value={form.username} required minlength="3" pattern="[A-Za-z0-9._\-]+" title="Letras, números, punto, guion y guion bajo" autocomplete="off" />
    </div>
    <div>
      <label class="label" for="m-phone">Celular <span class="font-normal text-app-muted">(opcional)</span></label>
      <input id="m-phone" class="field" type="tel" bind:value={form.phone} autocomplete="off" />
    </div>
    <div>
      <label class="label" for="m-pass">Contraseña inicial</label>
      <input id="m-pass" class="field" type="password" bind:value={form.password} required minlength="8" maxlength="72" autocomplete="new-password" />
    </div>
    <fieldset class="sm:col-span-2">
      <legend class="label">Rol</legend>
      <div class="grid gap-2">
        {#each CLINIC_ROLES as r}
          <label class="flex cursor-pointer items-start gap-3 rounded-xl border p-3 transition {form.role === r ? 'border-app-primary bg-app-primary/8' : 'border-app-ink/10 hover:border-app-ink/25'}">
            <input type="radio" name="role" class="mt-1 accent-[rgb(var(--app-primary))]" value={r} bind:group={form.role} />
            <span>
              <span class="block text-sm font-semibold">{ROLES[r].label}</span>
              <span class="block text-xs text-app-muted">{ROLES[r].about}</span>
            </span>
          </label>
        {/each}
      </div>
    </fieldset>
  </form>
  {#if addOp.phase === 'error'}
    <p class="alert mt-4" role="alert"><Icon name="alert" size={18} />{addOp.message}</p>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={() => (addOpen = false)}>Cancelar</button>
    <button type="submit" form="add-member" class="btn-primary" disabled={addOp.phase === 'loading'}>
      {#if addOp.phase === 'loading'}<span class="spin"></span>{/if}Agregar
    </button>
  {/snippet}
</Modal>

<ConfirmModal open={roleChange !== null} title="¿Cambiar el rol?" confirmLabel="Cambiar rol" op={roleOp} onconfirm={confirmRole} onclose={() => (roleChange = null)}>
  {#if roleChange}
    <p><strong class="text-app-ink">{roleChange.person.name}</strong> pasará de <strong class="text-app-ink">{ROLES[roleChange.person.role].label}</strong> a <strong class="text-app-ink">{ROLES[roleChange.role].label}</strong>.</p>
    <p class="mt-2 text-sm">{ROLES[roleChange.role].about} Su sesión se cierra y volverá a entrar con el rol nuevo.</p>
  {/if}
</ConfirmModal>

<ConfirmModal
  open={toggle !== null}
  title={toggle?.disable ? '¿Desactivar esta cuenta?' : '¿Reactivar esta cuenta?'}
  confirmLabel={toggle?.disable ? 'Desactivar' : 'Reactivar'}
  op={toggleOp}
  onconfirm={confirmToggle}
  onclose={() => (toggle = null)}
>
  {#if toggle}
    {#if toggle.disable}
      <p><strong class="text-app-ink">{toggle.person.name}</strong> dejará de poder entrar al instante, aunque tenga la sesión abierta. Podrás reactivarla cuando quieras.</p>
    {:else}
      <p><strong class="text-app-ink">{toggle.person.name}</strong> podrá volver a entrar como {ROLES[toggle.person.role].label}. Ocupará un lugar del plan.</p>
    {/if}
  {/if}
</ConfirmModal>

<Modal open={pwFor !== null} title="Restablecer contraseña" onclose={() => (pwFor = null)}>
  <form id="pw-member" class="text-left" onsubmit={submitPw}>
    <p class="mb-4 text-sm text-app-muted">Escribe una contraseña nueva para <strong class="text-app-ink">{pwFor?.name}</strong>. Su sesión actual se cerrará.</p>
    <label class="label" for="pw-new">Nueva contraseña</label>
    <input id="pw-new" class="field" type="password" bind:value={newPassword} required minlength="8" maxlength="72" autocomplete="new-password" />
    <p class="hint">Mínimo 8 caracteres.</p>
  </form>
  {#if pwOp.phase === 'error'}
    <p class="alert mt-4" role="alert"><Icon name="alert" size={18} />{pwOp.message}</p>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={() => (pwFor = null)}>Cancelar</button>
    <button type="submit" form="pw-member" class="btn-primary" disabled={pwOp.phase === 'loading'}>
      {#if pwOp.phase === 'loading'}<span class="spin"></span>{/if}Guardar
    </button>
  {/snippet}
</Modal>
