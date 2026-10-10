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
  import { CAPABILITY_ROWS, CLINIC_ROLES, ROLE_PERMISSIONS, ROLES, type ClinicRole, type Person, type Seats } from '$lib/types';
  import ConfirmModal from './ConfirmModal.svelte';
  import Modal from './Modal.svelte';
  import AreaPicker from './team/AreaPicker.svelte';
  import PermissionsModal from './team/PermissionsModal.svelte';
  import { CLINIC_KINDS } from '$lib/types';
  import Avatar from './ui/Avatar.svelte';
  import Icon from './ui/Icon.svelte';
  import LoadingRows from './ui/LoadingRows.svelte';
  import PageHeader from './ui/PageHeader.svelte';
  import Pill from './ui/Pill.svelte';
  import RolePill from './ui/RolePill.svelte';

  let people = $state<Person[]>([]);
  let seats = $state<Seats | null>(null);
  const ld = new Loader('No se pudo leer el equipo.');

  async function load() {
    await ld.run(async () => {
        const r = await api.team();
        people = r.people;
        seats = r.seats;
    });
  }
  onMount(load);

  const me = $derived(session.user?.userId);
  const active = $derived(people.filter((p) => !p.disabled));
  const inactive = $derived(people.filter((p) => p.disabled));
  const pct = (used: number, max: number | null) => (max ? Math.min(100, Math.round((used / max) * 100)) : 0);

  // ----- add a person -----
  let addOpen = $state(false);
  let form = $state({ name: '', email: '', username: '', phone: '', password: '', role: 'reception' as ClinicRole, areas: [] as string[] });
  const addOp = new Op();
  function openAdd() {
    form = { name: '', email: '', username: '', phone: '', password: '', role: 'reception', areas: [] };
    addOp.reset();
    addOpen = true;
  }
  async function submitAdd(e: SubmitEvent) {
    e.preventDefault();
    if (!(await addOp.run(() => api.addMember({ ...form, areas: form.role === 'doctor' ? form.areas : [] })))) return;
    addOpen = false;
    toast.show(`${form.name} ya puede entrar`);
    await load();
  }

  // ----- areas of a professional -----
  let areasFor = $state<Person | null>(null);
  let areasValue = $state<string[]>([]);
  const areasOp = new Op();
  function openAreas(p: Person) {
    areasValue = [...(p.areas ?? [])];
    areasOp.reset();
    areasFor = p;
  }
  async function saveAreas() {
    const p = areasFor;
    if (!p) return;
    if (await areasOp.run(() => api.updateMember(p.id, { areas: areasValue }))) {
      areasFor = null;
      toast.show('Áreas actualizadas');
      await load();
    }
  }
  const areaText = (p: Person) => (p.areas ?? []).map((a) => CLINIC_KINDS[a as keyof typeof CLINIC_KINDS]?.label ?? a).join(', ');

  // ----- change role -----
  let permsFor = $state<Person | null>(null);
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
    <h2 class="display text-2xl">Plan {seats.plan_name}</h2>
    <p class="text-sm text-app-muted">{seats.used_users} {seats.used_users === 1 ? 'persona entra' : 'personas entran'} a la app. Las cuentas de administración no cuentan para el límite.{#if !session.cobros} Las cuentas de caja se usan en la sección de Cobros, que viene con el plan Crecimiento.{/if}</p>
    <div class="mt-4 grid gap-4 sm:grid-cols-3">
      {#each [{ label: 'Especialistas', used: seats.used_doctors, max: seats.max_doctors }, { label: 'Recepcionistas', used: seats.used_reception, max: seats.max_reception }, { label: 'Cajeros', used: seats.used_cashiers, max: seats.max_cashiers }] as r}
        <div>
          <p class="flex items-baseline justify-between text-sm"><span class="font-medium">{r.label}</span><strong class="font-mono">{r.max === null ? `${r.used} · sin límite` : `${r.used} de ${r.max}`}</strong></p>
          {#if r.max !== null}
            <div class="mt-1.5 h-2 overflow-hidden rounded-full bg-app-ink/10" aria-hidden="true">
              <div class="h-full rounded-full {pct(r.used, r.max) >= 100 ? 'bg-app-warning' : 'bg-app-primary'}" style="width: {pct(r.used, r.max)}%"></div>
            </div>
            {#if r.used >= r.max}<p class="mt-1 text-xs text-app-warning">Lugares llenos: desactiva a alguien o cambia de plan.</p>{/if}
          {/if}
        </div>
      {/each}
    </div>
  </section>
{/if}

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
            <p class="flex flex-wrap items-center gap-2 font-semibold">
              <span class="truncate">{p.name}</span>
              {#if self}<Pill tone="info">Tú</Pill>{/if}
              {#if (p.permissions_extra?.length ?? 0) + (p.permissions_denied?.length ?? 0) > 0}<Pill tone="info">Permisos personalizados</Pill>{/if}
              {#if p.role === 'doctor' && (p.areas?.length ?? 0) > 0}<Pill tone="info">{areaText(p)}</Pill>{/if}
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
              {#if p.role === 'doctor' && (session.clinic?.specialties.length ?? 0) > 0}<button type="button" class="icon-btn" title="Áreas de atención" aria-label="Áreas de {p.name}" onclick={() => openAreas(p)}><Icon name="stethoscope" size={18} /></button>{/if}
              {#if p.role !== 'admin'}<button type="button" class="icon-btn" title="Permisos" aria-label="Permisos de {p.name}" onclick={() => (permsFor = p)}><Icon name="shield" size={18} /></button>{/if}
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
    {#if form.role === 'doctor'}<div class="sm:col-span-2"><AreaPicker bind:value={form.areas} /></div>{/if}
  </form>
  {#if addOp.phase === 'error'}
    <Alert class="mt-4">{addOp.message}</Alert>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={() => (addOpen = false)}>Cancelar</button>
    <button type="submit" form="add-member" class="btn-primary" disabled={addOp.phase === 'loading'}>
      {#if addOp.phase === 'loading'}<span class="spin"></span>{/if}Agregar
    </button>
  {/snippet}
</Modal>

<Modal open={areasFor !== null} title="Áreas de {areasFor?.name ?? ''}" onclose={() => (areasFor = null)}>
  <AreaPicker bind:value={areasValue} />
  <OpError op={areasOp} class="mt-4" />
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={() => (areasFor = null)}>Cancelar</button>
    <button type="button" class="btn-primary" disabled={areasOp.phase === 'loading'} onclick={saveAreas}>Guardar</button>
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
    <Alert class="mt-4">{pwOp.message}</Alert>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={() => (pwFor = null)}>Cancelar</button>
    <button type="submit" form="pw-member" class="btn-primary" disabled={pwOp.phase === 'loading'}>
      {#if pwOp.phase === 'loading'}<span class="spin"></span>{/if}Guardar
    </button>
  {/snippet}
</Modal>

<PermissionsModal
  person={permsFor}
  cobros={session.cobros}
  onclose={() => (permsFor = null)}
  onsaved={(saved) => {
    people = people.map((x) => (x.id === saved.id ? { ...x, ...saved } : x));
    permsFor = null;
  }}
/>
