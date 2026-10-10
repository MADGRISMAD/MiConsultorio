<script lang="ts">
  import { saveAll } from '$lib/saveall.svelte';
  import { publicOrigin } from '$lib/site';
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { profileApi, type ClinicProfile, type SurveySummary } from '$lib/api/profile';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import { dateShort } from '$lib/format';
  import LoadingRows from '../ui/LoadingRows.svelte';
  import Stars from '../ui/Stars.svelte';
  import ProfileMedia from './ProfileMedia.svelte';

  let p = $state<ClinicProfile | null>(null);
  let slug = $state('');
  let reviewUrl = $state('');
  let results = $state<SurveySummary | null>(null);
  let loadError = $state('');
  const saveOp = new Op();

  onMount(async () => {
    try {
      const [r, s] = await Promise.all([profileApi.get(), profileApi.surveys()]);
      p = r.profile;
      slug = r.slug;
      reviewUrl = r.review_url;
      results = s;
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'No se pudieron cargar los ajustes.';
    }
  });

  const link = $derived(slug ? `${publicOrigin()}/clinica/${slug}` : '');

  async function save(e?: SubmitEvent) {
    e?.preventDefault();
    if (!p) return;
    if (await saveOp.run(async () => {
      const r = await profileApi.save($state.snapshot(p) as ClinicProfile);
      p = r.profile;
      slug = r.slug;
      reviewUrl = r.review_url;
    })) toast.show('Perfil y encuesta guardados');
  }
  $effect(() => saveAll.register(() => save()));
</script>

<section aria-labelledby="pf-set">
  <h3 id="pf-set" class="section-title mb-3">Página pública del consultorio</h3>
  {#if loadError}
    <Alert>{loadError}</Alert>
  {:else if !p}
    <LoadingRows />
  {:else}
    <form id="pf-form" onsubmit={save} class="grid gap-5">
      <p class="text-sm text-app-muted">
        Una página para que tus pacientes te encuentren: qué ofreces, tu equipo, dónde estás y la opinión de quienes ya se atendieron. Comparte el enlace en Instagram, WhatsApp o Google.
      </p>
      <label class="flex cursor-pointer items-center gap-3 text-sm font-medium">
        <input type="checkbox" class="h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={p.enabled} />
        Publicar la página del consultorio
      </label>
      {#if !slug}
        <p class="note">Para publicarla, primero define la dirección de reserva en línea en «Agenda y reservas»: la página usa la misma dirección.</p>
      {:else if p.enabled}
        <p class="text-sm">Dirección: <a class="break-all font-medium text-app-primary underline" href={link} target="_blank" rel="noopener">{link}</a></p>
      {/if}
      <div class="grid gap-4 sm:grid-cols-2">
        <div class="sm:col-span-2">
          <label class="label" for="pf-tag">Frase corta</label>
          <input id="pf-tag" class="field" maxlength="120" bind:value={p.tagline} placeholder="Ej. Cuidamos la salud de tu familia" />
        </div>
        <div class="sm:col-span-2">
          <label class="label" for="pf-about">Sobre el consultorio</label>
          <textarea id="pf-about" class="field" rows="4" maxlength="1500" bind:value={p.about}></textarea>
        </div>
        <div>
          <label class="label" for="pf-hours">Horario (texto libre)</label>
          <textarea id="pf-hours" class="field" rows="3" maxlength="400" bind:value={p.hours_text} placeholder={'Lunes a viernes 9:00–19:00\nSábados 9:00–14:00'}></textarea>
        </div>
        <div class="grid content-start gap-4">
          <div>
            <label class="label" for="pf-wa">WhatsApp</label>
            <input id="pf-wa" class="field" maxlength="20" inputmode="tel" bind:value={p.whatsapp} placeholder="5215512345678" />
            <p class="hint">Con código de país, solo números.</p>
          </div>
          <div>
            <label class="label" for="pf-mail">Correo de contacto</label>
            <input id="pf-mail" class="field" type="email" maxlength="120" bind:value={p.contact_email} placeholder="contacto@miconsultorio.mx" />
          </div>
          <div>
            <label class="label" for="pf-web">Sitio web</label>
            <input id="pf-web" class="field" maxlength="200" bind:value={p.website} placeholder="https://" />
          </div>
        </div>
      </div>

      <div class="rounded-2xl bg-app-primary/6 p-4">
        <p class="section-title mb-2">Google Maps</p>
        <p class="mb-3 text-sm text-app-muted">Si tu consultorio ya aparece en Google Maps, conecta tu ficha para invitar a tus pacientes satisfechos a dejarte una reseña.</p>
        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label class="label" for="pf-maps">Enlace de tu ficha en Google Maps</label>
            <input id="pf-maps" class="field" maxlength="500" bind:value={p.maps_url} placeholder="https://maps.app.goo.gl/…" />
          </div>
          <div>
            <label class="label" for="pf-place">Place ID de Google</label>
            <input id="pf-place" class="field" maxlength="200" bind:value={p.google_place_id} placeholder="ChIJ…" />
            <p class="hint">Se obtiene gratis en la herramienta «Place ID Finder» de Google. Con él se arma el botón «Calificarnos en Google».</p>
          </div>
        </div>
        {#if reviewUrl}<p class="mt-2 text-sm">Enlace de reseña: <a class="break-all text-app-primary underline" href={reviewUrl} target="_blank" rel="noopener noreferrer">probar</a></p>{/if}
      </div>

      <div class="rounded-2xl bg-app-primary/6 p-4">
        <p class="section-title mb-2">Encuesta de satisfacción</p>
        <label class="flex cursor-pointer items-center gap-3 text-sm font-medium">
          <input type="checkbox" class="h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={p.survey_enabled} />
          Enviar una encuesta por correo después de cada consulta
        </label>
        <p class="hint mt-1">Solo a pacientes que aceptaron recibir correos. Una por visita y como máximo una al mes por paciente.</p>
        <div class="mt-3 grid gap-4 sm:grid-cols-2">
          <div>
            <label class="label" for="pf-delay">Enviar a las</label>
            <select id="pf-delay" class="field" bind:value={p.survey_delay_hours}>{#each [1, 2, 3, 6, 12, 24, 48] as h}<option value={h}>{h} {h === 1 ? 'hora' : 'horas'} de terminar la consulta</option>{/each}</select>
          </div>
          <div>
            <label class="label" for="pf-min">Invitar a Google desde</label>
            <select id="pf-min" class="field" bind:value={p.maps_min_rating}>{#each [3, 4, 5] as n}<option value={n}>{n} estrellas o más</option>{/each}</select>
          </div>
        </div>
        <label class="mt-3 flex cursor-pointer items-center gap-3 text-sm">
          <input type="checkbox" class="h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={p.show_reviews} />
          Mostrar la calificación y los comentarios autorizados en la página pública
        </label>
      </div>

    </form>

    <ProfileMedia />

    {#if results}
      <div class="mt-8">
        <h3 class="section-title mb-3">Opiniones de pacientes</h3>
        {#if results.stats.count === 0}
          <p class="text-sm text-app-muted">Aún no hay respuestas. {results.sent ? `Se han enviado ${results.sent} encuesta${results.sent === 1 ? '' : 's'}.` : ''}</p>
        {:else}
          <div class="flex flex-wrap items-center gap-4">
            <p class="display text-4xl">{results.stats.average.toFixed(1)}</p>
            <div><Stars value={results.stats.average} size={22} /><p class="text-sm text-app-muted">{results.stats.count} respuesta{results.stats.count === 1 ? '' : 's'} de {results.sent} enviada{results.sent === 1 ? '' : 's'}</p></div>
          </div>
          <ul class="mt-4 space-y-3">
            {#each results.recent.filter((r) => r.comment) as r}
              <li class="rounded-xl border border-app-ink/10 p-3">
                <div class="flex flex-wrap items-center gap-2"><Stars value={r.rating} size={14} /><span class="text-xs text-app-muted">{dateShort(r.date)}{r.professional ? ` · ${r.professional}` : ''}{r.public ? ' · visible en la página' : ''}</span></div>
                <p class="mt-1 text-sm">{r.comment}</p>
              </li>
            {/each}
          </ul>
        {/if}
      </div>
    {/if}

    <div class="mt-10 border-t border-app-ink/10 pt-5">
      <OpError op={saveOp} class="mb-3" />
      {#if !saveAll.active}<button type="submit" form="pf-form" class="btn-primary" disabled={saveOp.phase === 'loading'}>{#if saveOp.phase === 'loading'}<span class="spin"></span>{/if}Guardar cambios</button>{/if}
      <p class="hint mt-2">Guarda los textos, la página, Google Maps y la encuesta. Las fotos se guardan al elegirlas.</p>
    </div>
  {/if}
</section>
