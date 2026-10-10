<script lang="ts">
  import { saveAll } from '$lib/saveall.svelte';
  import { Loader } from '$lib/loader.svelte';
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { api } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { ComplianceItem, Legal } from '$lib/types';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';

  let items = $state<ComplianceItem[] | null>(null);
  let legal = $state<Legal | null>(null);
  const ld = new Loader('No se pudo cargar el cumplimiento.');
  const saveOp = new Op();

  async function load() {
    await ld.run(
      async () => {
        [items, legal] = await Promise.all([api.legal.compliance(), api.legal.get()]);
      },
      { reset: true }
    );
  }
  void load();

  async function save(e?: SubmitEvent) {
    e?.preventDefault();
    if (!legal) return;
    if (await saveOp.run(async () => (legal = await api.legal.save($state.snapshot(legal) as Legal)))) {
      toast.show('Datos legales guardados');
      items = await api.legal.compliance().catch(() => items);
    }
  }

  const pending = $derived(items?.filter((i) => i.status === 'todo').length ?? 0);
  $effect(() => saveAll.register(() => save()));
</script>

{#if ld.error}
  <Alert>{ld.error} <button type="button" class="ml-2 underline" onclick={load}>Reintentar</button></Alert>
{:else if !items || !legal}
  <div class="card"><LoadingRows /></div>
{:else}
  <div class="grid gap-4">
    <section class="card p-6">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <h2 class="display text-2xl">Lista de verificación</h2>
        <span class="pill {pending ? 'pill-warn' : 'pill-ok'}">{pending ? `${pending} pendiente${pending === 1 ? '' : 's'}` : 'Todo en orden'}</span>
      </div>
      <p class="mt-1 text-sm text-app-muted">Lo que la normatividad mexicana pide a un consultorio y qué parte ya cubre Caresia. Es una guía operativa, no una asesoría legal: confírmala con tu asesor.</p>
      <ul class="mt-4 divide-y divide-app-ink/10">
        {#each items as it (it.key)}
          <li class="flex items-start gap-3 py-3.5">
            <span class="mt-0.5 grid h-7 w-7 flex-none place-items-center rounded-full {it.status === 'ok' ? 'bg-app-accent/15 text-app-accent' : it.status === 'todo' ? 'bg-app-warning/15 text-app-warning' : 'bg-app-ink/8 text-app-muted'}">
              <Icon name={it.status === 'ok' ? 'check' : it.status === 'todo' ? 'alert' : 'info'} size={15} stroke={2.2} />
            </span>
            <div class="min-w-0 flex-1">
              <p class="text-[15px] font-semibold">{it.label}</p>
              <p class="text-sm text-app-muted">{it.detail}</p>
            </div>
            {#if (it.status === 'todo' || it.key === 'arco_requests') && it.link && !it.link.startsWith('/ajustes')}
              <a href={it.link} class="btn-secondary shrink-0">Atender</a>
            {/if}
          </li>
        {/each}
      </ul>
    </section>

    <form class="card p-6" onsubmit={save}>
      <h2 class="display text-2xl">Datos legales del establecimiento</h2>
      <p class="mt-1 text-sm text-app-muted">Se imprimen en las recetas y en el aviso de privacidad.</p>
      <div class="mt-5 grid gap-4 sm:grid-cols-2">
        <div class="sm:col-span-2"><p class="section-title">Responsable sanitario</p></div>
        <div>
          <label class="label" for="lg-rname">Nombre</label>
          <input id="lg-rname" class="field" bind:value={legal.responsible_name} autocomplete="off" />
        </div>
        <div>
          <label class="label" for="lg-rlic">Cédula profesional</label>
          <input id="lg-rlic" class="field" inputmode="numeric" bind:value={legal.responsible_license} />
        </div>
        <div class="sm:col-span-2">
          <label class="label" for="lg-rinst">Institución que expidió su título</label>
          <input id="lg-rinst" class="field" bind:value={legal.responsible_institution} />
        </div>
        <div class="sm:col-span-2">
          <label class="label" for="lg-notice">Aviso de funcionamiento o licencia sanitaria <span class="font-normal text-app-muted">(folio)</span></label>
          <input id="lg-notice" class="field" bind:value={legal.operating_notice} />
          <p class="hint">Lo registras ante COFEPRIS o la autoridad sanitaria de tu estado. Si eres veterinario, anota el registro que te aplique.</p>
        </div>

        <div class="mt-2 sm:col-span-2"><p class="section-title">Aviso de privacidad · derechos ARCO</p></div>
        <div>
          <label class="label" for="lg-pc">Persona o área que atiende solicitudes</label>
          <input id="lg-pc" class="field" bind:value={legal.privacy_contact} placeholder="Administración" />
        </div>
        <div>
          <label class="label" for="lg-pe">Correo</label>
          <input id="lg-pe" class="field" type="email" bind:value={legal.privacy_email} />
        </div>
        <div>
          <label class="label" for="lg-pp">Teléfono</label>
          <input id="lg-pp" class="field" type="tel" bind:value={legal.privacy_phone} />
        </div>
        <div>
          <label class="label" for="lg-pa">Domicilio</label>
          <input id="lg-pa" class="field" bind:value={legal.privacy_address} placeholder="Si lo dejas vacío se usa el del consultorio" />
        </div>
      </div>
      <OpError op={saveOp} class="mt-4" />
      {#if !saveAll.active}<div class="mt-5"><button type="submit" class="btn-primary" disabled={saveOp.phase === 'loading'}>{#if saveOp.phase === 'loading'}<span class="spin"></span>{/if}Guardar datos legales</button></div>{/if}
    </form>
  </div>
{/if}
