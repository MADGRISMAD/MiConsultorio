<script lang="ts">
  import { goto } from '$app/navigation';
  import { api } from '$lib/api';
  import { summarizeHours } from '$lib/clinic';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { theme } from '$lib/theme.svelte';
  import { CLINIC_KINDS, CLINIC_ROLES, ROLES, type ClinicKind, type ClinicRole, type ClinicSettings, type Person, type Seats } from '$lib/types';
  import Spinner from '$lib/components/Spinner.svelte';
  import ClinicFields from '$lib/components/setup/ClinicFields.svelte';
  import HoursEditor from '$lib/components/setup/HoursEditor.svelte';
  import SpecialtyPicker from '$lib/components/setup/SpecialtyPicker.svelte';
  import Avatar from '$lib/components/ui/Avatar.svelte';
  import Brand from '$lib/components/ui/Brand.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';

  const steps = ['Tu consultorio', 'Datos', 'Horario', 'Equipo', 'Listo'];
  let step = $state(0);

  // ---- guard: only a clinic administrator sees the wizard ----
  $effect(() => {
    if (session.status === 'anonymous') goto('/login', { replaceState: true });
    else if (session.status === 'authenticated') {
      if (session.isPlatform || session.user?.role !== 'admin') goto(session.home, { replaceState: true });
      else if (!session.user.setupPending) goto('/ajustes?s=negocio', { replaceState: true });
    }
  });
  $effect(() => {
    if (session.status === 'authenticated') void session.loadClinic();
  });

  // ---- draft, filled once from the saved clinic ----
  let ready = $state(false);
  let kind = $state<ClinicKind>('GENERAL_MEDICAL');
  let specialties = $state<ClinicKind[]>([]);
  let name = $state('');
  let phone = $state('');
  let address = $state('');
  let settings = $state<ClinicSettings>({ hours: {} as ClinicSettings['hours'], appointment_minutes: 30 });
  $effect(() => {
    const c = session.clinic;
    if (!c || ready) return;
    ({ kind, name } = c);
    specialties = [...c.specialties];
    phone = c.phone_number;
    address = c.address;
    settings = structuredClone($state.snapshot(c.settings));
    ready = true;
  });

  // ---- team step ----
  let seats = $state<Seats | null>(null);
  let added = $state<Person[]>([]);
  let member = $state({ name: '', email: '', username: '', password: '', role: 'reception' as ClinicRole });
  const addOp = new Op();
  $effect(() => {
    if (step === 3 && !seats) api.team().then((t) => (seats = t.seats), () => {});
  });
  async function addMember(e: SubmitEvent) {
    e.preventDefault();
    const m = { ...member, phone: '' };
    if (!(await addOp.run(async () => added.push((await api.addMember(m)).person)))) return;
    member = { name: '', email: '', username: '', password: '', role: member.role };
    seats = (await api.team()).seats;
  }

  // ---- navigation: each step saves what it collected ----
  const op = new Op();
  async function next() {
    const ok = await op.run(async () => {
      let saved = null;
      if (step === 0) saved = await api.updateClinic({ kind, specialties });
      else if (step === 1) saved = await api.updateClinic({ name, phone_number: phone, address });
      else if (step === 2) saved = await api.updateClinic({ settings: $state.snapshot(settings) });
      if (saved) session.setClinic(saved);
    });
    if (ok) step += 1;
  }
  async function finish() {
    if (await op.run(async () => session.setUser(await api.completeSetup()))) await goto('/', { replaceState: true });
  }

  const valid = $derived(step !== 1 || name.trim().length >= 2);
  const mainLabel = $derived(CLINIC_KINDS[kind].label);
</script>

<svelte:head><title>Configura tu consultorio · Caresia</title></svelte:head>

{#if session.status !== 'authenticated' || !ready}
  <Spinner />
{:else}
  <div class="app fixed inset-0 overflow-y-auto" data-theme={theme.mode} style="background-image: radial-gradient(ellipse 60% 420px at 50% 0%, rgb(var(--app-primary) / 0.1), transparent 75%); background-repeat: no-repeat">
    <div class="mx-auto flex min-h-full max-w-3xl flex-col px-4 py-6 sm:px-6">
      <header class="flex items-center justify-between">
        <a href="/" aria-label="Caresia"><Brand size={32} class="text-[1.05rem]" /></a>
        <div class="flex items-center gap-1">
          <button type="button" class="icon-btn" aria-label={theme.mode === 'dark' ? 'Cambiar a tema claro' : 'Cambiar a tema oscuro'} onclick={() => theme.toggle()}><Icon name={theme.mode === 'dark' ? 'sun' : 'moon'} size={20} /></button>
          {#if step < steps.length - 1}
            <button type="button" class="btn-ghost !min-h-9" onclick={finish} disabled={op.phase === 'loading'}>Configurar después</button>
          {/if}
        </div>
      </header>

      <div class="mt-8">
        <p class="font-mono text-[11px] uppercase tracking-[0.14em] text-app-muted">Paso {step + 1} de {steps.length} · {steps[step]}</p>
        <div class="mt-3 flex gap-1.5" role="progressbar" aria-valuemin="1" aria-valuemax={steps.length} aria-valuenow={step + 1} aria-label="Progreso de la configuración">
          {#each steps as _, i}
            <span class="h-1 flex-1 rounded-full transition-colors duration-500 {i <= step ? 'bg-app-primary' : 'bg-app-ink/12'}"></span>
          {/each}
        </div>
      </div>

      <main class="card page-in mt-6 flex-1 px-5 py-7 sm:px-9 sm:py-9" aria-live="polite">
        {#if step === 0}
          <h1 class="display text-[2.4rem] leading-none sm:text-5xl">Bienvenido a <em class="italic text-app-primary">Caresia</em></h1>
          <p class="mb-7 mt-3 text-[15px] text-app-muted">En unos minutos dejamos tu consultorio listo para atender. Empecemos por conocerlo.</p>
          <SpecialtyPicker bind:kind bind:specialties />
        {:else if step === 1}
          <h1 class="display text-[2.4rem] leading-none sm:text-5xl">Datos del <em class="italic text-app-primary">consultorio</em></h1>
          <p class="mb-7 mt-3 text-[15px] text-app-muted">Aparecen en tu panel y en tus documentos. Podrás cambiarlos cuando quieras.</p>
          <ClinicFields bind:name bind:phone bind:address />
        {:else if step === 2}
          <h1 class="display text-[2.4rem] leading-none sm:text-5xl">Horario y <em class="italic text-app-primary">citas</em></h1>
          <p class="mb-7 mt-3 text-[15px] text-app-muted">Ayuda a agendar sin empalmes y avisa cuando una cita cae fuera de horario.</p>
          <HoursEditor bind:settings />
        {:else if step === 3}
          <h1 class="display text-[2.4rem] leading-none sm:text-5xl">Tu <em class="italic text-app-primary">equipo</em></h1>
          <p class="mb-2 mt-3 text-[15px] text-app-muted">Agrega a quienes van a usar Caresia. Cada quien ve solo lo que le toca. Este paso es opcional: puedes hacerlo después en <strong class="font-semibold text-app-ink">Equipo</strong>.</p>
          {#if seats}
            <p class="mb-5 text-sm text-app-muted">Plan {seats.plan_name}: {seats.used_users} de {seats.max_users ?? '∞'} cuentas{seats.max_doctors !== null ? ` · ${seats.used_doctors} de ${seats.max_doctors} médicos` : ''}.</p>
          {/if}

          {#if added.length}
            <ul class="mb-5 divide-y divide-app-ink/8 rounded-xl border border-app-ink/10">
              {#each added as p (p.id)}
                <li class="flex items-center gap-3 px-4 py-3">
                  <Avatar name={p.name} size={36} />
                  <span class="min-w-0 flex-1"><span class="block truncate text-sm font-semibold">{p.name}</span><span class="block truncate text-xs text-app-muted">{p.email}</span></span>
                  <span class="pill pill-ok"><Icon name="check" size={12} stroke={2.6} />{ROLES[p.role].short}</span>
                </li>
              {/each}
            </ul>
          {/if}

          <form class="grid gap-3.5 rounded-xl bg-app-elevated p-4 sm:grid-cols-2" onsubmit={addMember}>
            <div class="sm:col-span-2"><label class="label" for="w-name">Nombre completo</label><input id="w-name" class="field" bind:value={member.name} required minlength="2" autocomplete="off" /></div>
            <div><label class="label" for="w-email">Correo</label><input id="w-email" class="field" type="email" bind:value={member.email} required autocomplete="off" /></div>
            <div><label class="label" for="w-user">Usuario</label><input id="w-user" class="field" bind:value={member.username} required minlength="3" pattern="[A-Za-z0-9._\-]+" title="Letras, números, punto, guion y guion bajo" autocomplete="off" /></div>
            <div><label class="label" for="w-pass">Contraseña inicial</label><input id="w-pass" class="field" type="password" bind:value={member.password} required minlength="8" maxlength="72" autocomplete="new-password" /></div>
            <div>
              <label class="label" for="w-role">Rol</label>
              <select id="w-role" class="field" bind:value={member.role}>{#each CLINIC_ROLES as r}<option value={r}>{ROLES[r].label}</option>{/each}</select>
            </div>
            <p class="text-xs text-app-muted sm:col-span-2">{ROLES[member.role].about}</p>
            {#if addOp.phase === 'error'}<p class="alert sm:col-span-2" role="alert"><Icon name="alert" size={18} />{addOp.message}</p>{/if}
            <div class="sm:col-span-2"><button type="submit" class="btn-secondary" disabled={addOp.phase === 'loading'}>{#if addOp.phase === 'loading'}<span class="spin"></span>{:else}<Icon name="user-plus" size={17} />{/if}Agregar a la lista</button></div>
          </form>
        {:else}
          <span class="grid h-14 w-14 place-items-center rounded-2xl bg-app-accent/14 text-app-accent"><Icon name="check" size={28} stroke={2.4} /></span>
          <h1 class="display mt-5 text-[2.4rem] leading-none sm:text-5xl">Todo <em class="italic text-app-primary">listo</em></h1>
          <p class="mb-6 mt-3 text-[15px] text-app-muted">Así quedó tu consultorio. Todo se puede cambiar después en <strong class="font-semibold text-app-ink">Configuración</strong>.</p>
          <dl class="grid gap-x-8 gap-y-4 rounded-xl bg-app-elevated p-5 text-sm sm:grid-cols-2">
            <div><dt class="section-title">Consultorio</dt><dd class="mt-1 font-semibold">{name}</dd></div>
            <div><dt class="section-title">Giro</dt><dd class="mt-1 font-semibold">{mainLabel}{specialties.length ? ` + ${specialties.map((s) => CLINIC_KINDS[s].label).join(', ')}` : ''}</dd></div>
            <div><dt class="section-title">Horario</dt><dd class="mt-1">{summarizeHours(settings)}</dd></div>
            <div><dt class="section-title">Citas de</dt><dd class="mt-1">{settings.appointment_minutes} minutos</dd></div>
            <div class="sm:col-span-2"><dt class="section-title">Equipo agregado</dt><dd class="mt-1">{added.length ? added.map((p) => p.name).join(', ') : 'Solo tú por ahora'}</dd></div>
          </dl>
          <ul class="mt-6 grid gap-2 text-sm sm:grid-cols-2">
            <li class="flex items-center gap-3 rounded-xl border border-app-ink/10 p-3"><Icon name="calendar" size={19} class="text-app-primary" />Agenda tu primera cita</li>
            <li class="flex items-center gap-3 rounded-xl border border-app-ink/10 p-3"><Icon name="folder" size={19} class="text-app-primary" />Registra a tus pacientes</li>
            <li class="flex items-center gap-3 rounded-xl border border-app-ink/10 p-3"><Icon name="users" size={19} class="text-app-primary" />Ajusta roles en Equipo</li>
            <li class="flex items-center gap-3 rounded-xl border border-app-ink/10 p-3 text-app-muted"><Icon name="cash" size={19} />Cobros e inventario <span class="badge-soon ml-auto">Pronto</span></li>
          </ul>
        {/if}

        {#if op.phase === 'error'}<p class="alert mt-6" role="alert"><Icon name="alert" size={18} />{op.message}</p>{/if}
      </main>

      <footer class="mt-5 flex items-center gap-3 pb-2">
        {#if step > 0 && step < steps.length - 1}
          <button type="button" class="btn-ghost" onclick={() => (step -= 1)} disabled={op.phase === 'loading'}><Icon name="arrow-left" size={17} />Atrás</button>
        {/if}
        <span class="flex-1"></span>
        {#if step < steps.length - 1}
          <button type="button" class="btn-primary !px-7" onclick={next} disabled={!valid || op.phase === 'loading'}>
            {#if op.phase === 'loading'}<span class="spin"></span>{/if}{step === 3 && added.length === 0 ? 'Omitir por ahora' : 'Continuar'}<Icon name="arrow-right" size={17} />
          </button>
        {:else}
          <button type="button" class="btn-primary !px-7" onclick={finish} disabled={op.phase === 'loading'}>
            {#if op.phase === 'loading'}<span class="spin"></span>{/if}Ir a mi consultorio<Icon name="arrow-right" size={17} />
          </button>
        {/if}
      </footer>
    </div>
  </div>
{/if}
