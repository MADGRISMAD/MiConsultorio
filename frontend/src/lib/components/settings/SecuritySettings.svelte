<script lang="ts">
  import { Loader } from '$lib/loader.svelte';
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { securityApi } from '$lib/api/security';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { SecurityMember, TwoFactorPolicy } from '$lib/types/security';
  import ConfirmModal from '$lib/components/ConfirmModal.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import { session } from '$lib/session.svelte';

  const OPTIONS: { id: TwoFactorPolicy; label: string; about: string }[] = [
    { id: 'none', label: 'Opcional', about: 'Cada persona decide si la activa en Mi cuenta.' },
    { id: 'admins', label: 'Administradores', about: 'Quien administra el consultorio debe activarla.' },
    { id: 'clinical', label: 'Administradores y médicos', about: 'Quienes ven y escriben expedientes.' },
    { id: 'all', label: 'Todo el equipo', about: 'Todas las cuentas del consultorio.' }
  ];

  let policy = $state<TwoFactorPolicy | null>(null);
  let members = $state<SecurityMember[] | null>(null);
  const ld = new Loader('No se pudo cargar la seguridad.');
  const policyOp = new Op();
  const resetOp = new Op();
  let target = $state<SecurityMember | null>(null);

  async function load() {
    await ld.run(
      async () => {
        [policy, members] = await Promise.all([securityApi.policy(), securityApi.members()]);
      },
      { reset: true }
    );
  }
  void load();

  async function choose(next: TwoFactorPolicy) {
    const before = policy;
    policy = next;
    if (await policyOp.run(() => securityApi.setPolicy(next))) toast.show('Política guardada');
    else policy = before;
  }

  async function reset() {
    const t = target;
    if (!t) return;
    if (await resetOp.run(() => securityApi.resetMember(t.id))) {
      target = null;
      toast.show(`Se restableció la verificación de ${t.name}`);
      members = await securityApi.members().catch(() => members);
    }
  }

  const withoutIt = $derived(members?.filter((m) => !m.disabled && !m.two_factor_enabled).length ?? 0);
</script>

{#if ld.error}
  <Alert>{ld.error} <button type="button" class="ml-2 underline" onclick={load}>Reintentar</button></Alert>
{:else if !members || !policy}
  <div class="card"><LoadingRows /></div>
{:else}
  <div class="grid gap-4">
    <section class="card p-6">
      <h2 class="display text-2xl">Verificación en dos pasos</h2>
      <p class="mt-1 text-sm text-app-muted">
        Pide un código de una app de autenticación además de la contraseña. Quien deba usarla y aún no la tenga puede entrar, pero no verá expedientes, recetas ni archivos hasta activarla.
      </p>
      <fieldset class="mt-4 grid gap-2 sm:grid-cols-2" disabled={policyOp.phase === 'loading'}>
        <legend class="sr-only">¿A quién se le exige?</legend>
        {#each OPTIONS as o (o.id)}
          <label class="flex cursor-pointer items-start gap-3 rounded-xl border p-3.5 {policy === o.id ? 'border-app-primary bg-app-primary/5' : 'border-app-ink/10'}">
            <input type="radio" name="policy" class="mt-1" value={o.id} checked={policy === o.id} onchange={() => choose(o.id)} />
            <span>
              <span class="block text-[15px] font-semibold">{o.label}</span>
              <span class="block text-sm text-app-muted">{o.about}</span>
            </span>
          </label>
        {/each}
      </fieldset>
      <OpError op={policyOp} class="mt-3" />
      {#if policy !== 'none' && withoutIt > 0}
        <p class="mt-3 text-sm text-app-warning">{withoutIt} {withoutIt === 1 ? 'persona aún no la tiene activa' : 'personas aún no la tienen activa'}.</p>
      {/if}
    </section>

    <section class="card p-6">
      <h2 class="display text-2xl">Equipo</h2>
      <p class="mt-1 text-sm text-app-muted">Si alguien pierde su teléfono y sus códigos de recuperación, restablece su verificación: se cierran sus sesiones y podrá configurarla de nuevo. Queda registrado en la actividad.</p>
      <ul class="mt-4 divide-y divide-app-ink/10">
        {#each members as m (m.id)}
          <li class="flex flex-wrap items-center gap-3 py-3">
            <div class="min-w-0 flex-1">
              <p class="truncate text-[15px] font-semibold">{m.name}{#if m.disabled}<span class="ml-2 text-xs font-normal text-app-muted">desactivada</span>{/if}</p>
              <p class="truncate text-sm text-app-muted">{m.role_label} · {m.username}</p>
            </div>
            <span class="pill {m.two_factor_enabled ? 'pill-ok' : 'pill-warn'}">{m.two_factor_enabled ? 'Activada' : 'No activada'}</span>
            {#if m.two_factor_enabled && m.id !== session.user?.userId}
              <button type="button" class="btn-secondary" onclick={() => (target = m)}>Restablecer</button>
            {/if}
          </li>
        {/each}
      </ul>
    </section>
  </div>
{/if}

<ConfirmModal open={target !== null} title="¿Restablecer la verificación en dos pasos?" confirmLabel="Restablecer" op={resetOp} onconfirm={reset} onclose={() => (target = null)}>
  <p>
    Se quitará la verificación de <strong>{target?.name}</strong> y se cerrarán sus sesiones. Antes de hacerlo, confirma por otro medio que realmente es esa persona quien lo pide.
  </p>
</ConfirmModal>
