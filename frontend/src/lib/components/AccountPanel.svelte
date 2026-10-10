<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import { api } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import { saveAll } from '$lib/saveall.svelte';
  import { ROLES } from '$lib/types';
  import Avatar from './ui/Avatar.svelte';
  import Icon from './ui/Icon.svelte';
  import PageHeader from './ui/PageHeader.svelte';
  import RolePill from './ui/RolePill.svelte';
  import TwoFactorSection from './security/TwoFactorSection.svelte';

  let { embedded = false }: { /** inside the settings hub, which already shows the title */ embedded?: boolean } = $props();

  const u = $derived(session.user);
  /** inside Ajustes the single sticky button saves these forms */
  const own = $derived(embedded && saveAll.active);

  let name = $state(session.user?.name ?? '');
  let phone = $state('');
  const profileOp = new Op();
  async function saveProfile(e?: SubmitEvent) {
    e?.preventDefault();
    if (await profileOp.run(async () => session.setUser(await api.updateProfile(name, phone)))) toast.show('Datos actualizados');
  }

  // Professional data: printed on recetas and signed notes (cédula profesional)
  let title = $state(session.user?.professional?.title ?? '');
  let cedula = $state(session.user?.professional?.cedula ?? '');
  let institution = $state(session.user?.professional?.institution ?? '');
  let specialty = $state(session.user?.professional?.specialty_license ?? '');
  const proOp = new Op();
  const canPrescribe = $derived(u?.role === 'doctor' || u?.role === 'admin');
  async function savePro(e?: SubmitEvent) {
    e?.preventDefault();
    if (cedula.trim() && !/^\d{7,8}$/.test(cedula.trim())) return proOp.fail('La cédula profesional son 7 u 8 dígitos.');
    const ok = await proOp.run(async () =>
      session.setUser(await api.updateProfile(name, phone, { cedula: cedula.trim(), cedula_institution: institution.trim(), cedula_specialty: specialty.trim(), specialty_title: title.trim() }))
    );
    if (ok) toast.show('Datos profesionales guardados');
  }

  let current = $state('');
  let next = $state('');
  let again = $state('');
  const pwOp = new Op();
  $effect(() => {
    if (!embedded) return;
    return saveAll.register(async () => {
      await saveProfile();
      if (canPrescribe) await savePro();
    });
  });

  async function savePassword(e: SubmitEvent) {
    e.preventDefault();
    if (next !== again) return pwOp.fail('Las contraseñas nuevas no coinciden.');
    if (await pwOp.run(async () => session.setUser(await api.changePassword(current, next)))) {
      current = next = again = '';
      toast.show('Contraseña cambiada. Las demás sesiones se cerraron.');
    }
  }
</script>

{#if !embedded}<PageHeader title="Mi cuenta" subtitle="Tus datos y tu contraseña." />{/if}

<div class="grid gap-4 lg:grid-cols-2">
  <section class="card p-6">
    <div class="flex items-center gap-4">
      <Avatar name={u?.name || u?.username || '?'} size={56} />
      <div class="min-w-0">
        <p class="truncate text-lg font-semibold">{u?.name}</p>
        <p class="flex flex-wrap items-center gap-2 text-sm text-app-muted">{#if u}<RolePill role={u.role} long />{/if}{#if session.clinic}<span>{session.clinic.name}</span>{/if}</p>
      </div>
    </div>
    <dl class="mt-5 grid gap-3 border-t border-app-ink/10 pt-5 text-sm sm:grid-cols-2">
      <div><dt class="section-title">Correo</dt><dd class="mt-1 break-all">{u?.email || '—'}</dd></div>
      <div><dt class="section-title">Usuario</dt><dd class="mt-1">{u?.username}</dd></div>
    </dl>
    <p class="mt-3 text-sm text-app-muted">{u ? ROLES[u.role].about : ''}</p>

    <form class="mt-6 grid gap-3.5 border-t border-app-ink/10 pt-5" onsubmit={saveProfile}>
      <div>
        <label class="label" for="acc-name">Nombre</label>
        <input id="acc-name" class="field" bind:value={name} required minlength="2" autocomplete="name" />
      </div>
      <div>
        <label class="label" for="acc-phone">Celular <span class="font-normal text-app-muted">(opcional)</span></label>
        <input id="acc-phone" class="field" type="tel" bind:value={phone} autocomplete="tel" />
      </div>
      <OpError op={profileOp} />
      {#if !own}<div><button type="submit" class="btn-primary" disabled={profileOp.phase === 'loading'}>Guardar datos</button></div>{/if}
    </form>
  </section>

  {#if canPrescribe}
    <section class="card p-6 lg:col-span-2">
      <h2 class="display text-2xl">Datos profesionales</h2>
      <p class="mt-1 text-sm text-app-muted">Aparecen en tus recetas y en las notas que firmas. Sin cédula profesional e institución no se pueden emitir recetas.</p>
      <form class="mt-5 grid gap-3.5 sm:grid-cols-2" onsubmit={savePro}>
        <div>
          <label class="label" for="pro-title">Profesión o especialidad</label>
          <input id="pro-title" class="field" bind:value={title} placeholder="Médico Cirujano, Cirujano Dentista, Médico Veterinario Zootecnista…" />
        </div>
        <div>
          <label class="label" for="pro-cedula">Cédula profesional</label>
          <input id="pro-cedula" class="field" inputmode="numeric" bind:value={cedula} maxlength="8" placeholder="7 u 8 dígitos" />
        </div>
        <div>
          <label class="label" for="pro-inst">Institución que expidió el título</label>
          <input id="pro-inst" class="field" bind:value={institution} placeholder="Universidad Nacional Autónoma de México" />
        </div>
        <div>
          <label class="label" for="pro-spec">Cédula de especialidad <span class="font-normal text-app-muted">(si tienes)</span></label>
          <input id="pro-spec" class="field" bind:value={specialty} />
        </div>
        <OpError op={proOp} class="sm:col-span-2" />
        {#if !own}<div class="sm:col-span-2"><button type="submit" class="btn-primary" disabled={proOp.phase === 'loading'}>Guardar datos profesionales</button></div>{/if}
      </form>
    </section>
  {/if}

  <section class="card p-6">
    <h2 class="display text-2xl">Cambiar contraseña</h2>
    <p class="mt-1 text-sm text-app-muted">Al cambiarla, tus sesiones en otros dispositivos se cierran.</p>
    <form class="mt-5 grid gap-3.5" onsubmit={savePassword}>
      <div>
        <label class="label" for="pw-cur">Contraseña actual</label>
        <input id="pw-cur" class="field" type="password" bind:value={current} required autocomplete="current-password" />
      </div>
      <div>
        <label class="label" for="pw-n1">Contraseña nueva</label>
        <input id="pw-n1" class="field" type="password" bind:value={next} required minlength="8" maxlength="72" autocomplete="new-password" />
        <p class="hint">Mínimo 8 caracteres; una frase sirve.</p>
      </div>
      <div>
        <label class="label" for="pw-n2">Repite la contraseña nueva</label>
        <input id="pw-n2" class="field" type="password" bind:value={again} required autocomplete="new-password" />
      </div>
      <OpError op={pwOp} />
      <div><button type="submit" class="btn-primary" disabled={pwOp.phase === 'loading'}>Cambiar contraseña</button></div>
    </form>
  </section>

  <div class="lg:col-span-2"><TwoFactorSection /></div>
</div>
