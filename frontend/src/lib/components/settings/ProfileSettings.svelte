<script lang="ts">
  import { publicOrigin } from '$lib/site';
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { profileApi, type ClinicProfile, type ProfileCompleteness, type ProfileService, type SurveySummary } from '$lib/api/profile';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import { dateShort, moneyCents } from '$lib/format';
  import LoadingRows from '../ui/LoadingRows.svelte';
  import Stars from '../ui/Stars.svelte';
  import ProfileMedia from './ProfileMedia.svelte';

  interface Props {
    /** Qué parte se ve: la página y sus fotos, el directorio, o las opiniones (Google Maps, encuesta y respuestas). */
    tab?: 'pagina' | 'directorio' | 'opiniones';
    /** Dirección definida en «Reservas» que aún no se ha recargado aquí. */
    slugOverride?: string;
    onstate?: (v: { enabled: boolean; hidden: boolean; score: number; survey: boolean }) => void;
    goto?: (tab: string) => void;
  }
  let { tab = 'pagina', slugOverride = '', onstate, goto }: Props = $props();

  let p = $state<ClinicProfile | null>(null);
  let slug = $state('');
  let reviewUrl = $state('');
  let results = $state<SurveySummary | null>(null);
  let services = $state<ProfileService[]>([]);
  let completeness = $state<ProfileCompleteness>({ score: 0, items: [] });
  let states = $state<string[]>([]);
  let payMethods = $state<string[]>([]);
  // listas que se escriben separadas por comas
  let insText = $state('');
  let langText = $state('');
  const toList = (t: string) => t.split(',').map((x) => x.trim()).filter(Boolean);
  function take(r: { profile: ClinicProfile; slug: string; review_url: string; services: ProfileService[]; states: string[]; payment_methods: string[]; completeness: ProfileCompleteness }) {
    completeness = r.completeness;
    p = r.profile;
    slug = r.slug;
    reviewUrl = r.review_url;
    services = r.services;
    states = r.states;
    payMethods = r.payment_methods;
    insText = r.profile.insurances.join(', ');
    langText = r.profile.languages.join(', ');
  }
  function togglePay(m: string, on: boolean) {
    if (!p) return;
    p.payment_methods = on ? [...p.payment_methods.filter((x) => x !== m), m] : p.payment_methods.filter((x) => x !== m);
  }
  // respuesta a opiniones
  let replies = $state<Record<string, string>>({});
  let replying = $state('');
  async function sendReply(id: string) {
    replying = id;
    try {
      await profileApi.reply(id, replies[id] ?? '');
      toast.show((replies[id] ?? '').trim() ? 'Respuesta publicada' : 'Respuesta quitada');
    } catch (e) {
      toast.show(e instanceof Error ? e.message : 'No se pudo guardar la respuesta.');
    }
    replying = '';
  }
  let loadError = $state('');
  const saveOp = new Op();

  onMount(async () => {
    try {
      const [r, s] = await Promise.all([profileApi.get(), profileApi.surveys()]);
      take(r);
      results = s;
      replies = Object.fromEntries(s.recent.map((x) => [x.id, x.reply]));
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'No se pudieron cargar los ajustes.';
    }
  });

  const mySlug = $derived(slugOverride || slug);
  const link = $derived(mySlug ? `${publicOrigin()}/${mySlug}` : '');
  $effect(() => {
    if (p) onstate?.({ enabled: p.enabled, hidden: p.directory_hidden, score: completeness.score, survey: p.survey_enabled });
  });

  async function save(e: SubmitEvent) {
    e.preventDefault();
    if (!p) return;
    if (await saveOp.run(async () => {
      const body = { ...($state.snapshot(p) as ClinicProfile), insurances: toList(insText), languages: toList(langText), public_services: services.filter((x) => x.public).map((x) => x.id) };
      take(await profileApi.save(body));
    })) toast.show('Perfil y encuesta guardados');
  }
</script>

<section aria-labelledby="pf-set">
  <h3 id="pf-set" class="sr-only">Página pública, directorio y opiniones</h3>
  {#if loadError}
    <Alert>{loadError}</Alert>
  {:else if !p}
    <LoadingRows />
  {:else}
    <form id="pf-form" onsubmit={save} class="grid gap-5">
      <div class="grid gap-5 {tab !== 'pagina' ? '!hidden' : ''}">
      <p class="text-sm text-app-muted">
        Una página para que tus pacientes te encuentren: qué ofreces, tu equipo, dónde estás y la opinión de quienes ya se atendieron. Comparte el enlace en Instagram, WhatsApp o Google.
      </p>
      <label class="flex cursor-pointer items-start gap-3 rounded-2xl p-4 ring-1 ring-app-ink/12 {p.enabled ? 'bg-app-accent/8' : ''}">
        <input type="checkbox" class="mt-0.5 h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={p.enabled} disabled={!mySlug} />
        <span>
          <span class="font-medium">Publicar la página del consultorio</span>
          <span class="block text-sm text-app-muted">{#if !mySlug}Necesita tu dirección en línea.{:else if p.enabled}Publicada: <a class="break-all font-medium text-app-primary underline" href={link} target="_blank" rel="noopener">{link}</a>{:else}Sin publicar: nadie puede verla todavía.{/if}</span>
        </span>
      </label>
      {#if !mySlug}
        <p class="note">Primero escribe tu dirección en línea. <button type="button" class="font-medium underline" onclick={() => goto?.('reservas')}>Ir a «Reservas»</button></p>
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

      </div>

      <div class="grid gap-5 {tab !== 'directorio' ? '!hidden' : ''}">
      <div class="rounded-2xl bg-app-primary/6 p-4">
        <p class="section-title mb-2">Directorio de Caresia</p>
        <p class="mb-3 text-sm text-app-muted">Todos los consultorios aparecen en el buscador de Caresia, donde los pacientes encuentran especialistas por especialidad y ciudad. <strong class="text-app-ink">Los que tienen su perfil completo salen primero; entre más datos falten, más abajo.</strong></p>

        <div class="mb-4 rounded-xl bg-app-panel p-4 ring-1 ring-app-ink/10">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <p class="font-medium">Tu perfil: <span class="display text-2xl">{completeness.score}</span><span class="text-app-muted"> / 100</span></p>
            {#if !p.directory_hidden}<a class="text-sm font-medium text-app-primary underline" href="{publicOrigin()}/directorio" target="_blank" rel="noopener">Verlo en el directorio</a>{/if}
          </div>
          <div class="mt-2 h-2 overflow-hidden rounded-full bg-app-ink/10" role="progressbar" aria-valuenow={completeness.score} aria-valuemin="0" aria-valuemax="100" aria-label="Qué tan completo está tu perfil"><div class="h-full rounded-full bg-app-accent transition-all" style="width:{completeness.score}%"></div></div>
          {#if completeness.items.some((i) => !i.done)}
            <p class="mt-3 text-sm font-medium">Para subir, te falta:</p>
            <ul class="mt-1.5 grid gap-1 text-sm sm:grid-cols-2">
              {#each completeness.items.filter((i) => !i.done) as it (it.label)}
                <li class="flex items-baseline justify-between gap-3 text-app-muted"><span>{it.label}</span><span class="font-mono text-xs">+{it.points}</span></li>
              {/each}
            </ul>
            <p class="hint mt-2">Se actualiza al guardar. Lo que llenes en otras pestañas (fotos, servicios, reservas) también cuenta.</p>
          {:else}
            <p class="mt-3 text-sm font-medium text-app-accent">Perfil completo: apareces entre los primeros.</p>
          {/if}
        </div>

        <label class="flex cursor-pointer items-start gap-3 text-sm">
          <input type="checkbox" class="mt-0.5 h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={p.directory_hidden} />
          <span><span class="font-medium">No mostrar mi consultorio en el directorio</span><span class="block text-app-muted">Dejas de aparecer en el buscador; tu página y tus reservas por enlace siguen igual.</span></span>
        </label>
        <div class="mt-3 grid gap-4 sm:grid-cols-3">
          <div>
            <label class="label" for="pf-state">Estado</label>
            <select id="pf-state" class="field" bind:value={p.state}><option value="">Elige…</option>{#each states as st (st)}<option value={st}>{st}</option>{/each}</select>
          </div>
          <div>
            <label class="label" for="pf-city">Ciudad</label>
            <input id="pf-city" class="field" maxlength="80" bind:value={p.city} placeholder="Ej. Tijuana" />
          </div>
          <div>
            <label class="label" for="pf-col">Colonia (opcional)</label>
            <input id="pf-col" class="field" maxlength="80" bind:value={p.neighborhood} placeholder="Ej. Zona Río" />
          </div>
          <div class="sm:col-span-2">
            <label class="label" for="pf-ins">Aseguradoras que aceptas</label>
            <input id="pf-ins" class="field" bind:value={insText} placeholder="GNP, AXA, MetLife" />
            <p class="hint">Sepáralas con comas.</p>
          </div>
          <div>
            <label class="label" for="pf-lang">Idiomas</label>
            <input id="pf-lang" class="field" bind:value={langText} placeholder="Español, Inglés" />
          </div>
        </div>
        <fieldset class="mt-4">
          <legend class="label">Formas de pago</legend>
          <div class="flex flex-wrap gap-x-5 gap-y-2">
            {#each payMethods as m (m)}
              <label class="flex cursor-pointer items-center gap-2 text-sm"><input type="checkbox" class="h-4 w-4 accent-[rgb(var(--app-primary))]" checked={p.payment_methods.includes(m)} onchange={(e) => togglePay(m, e.currentTarget.checked)} />{m}</label>
            {/each}
          </div>
        </fieldset>
        <fieldset class="mt-4">
          <legend class="label">Servicios que se muestran con su precio</legend>
          {#if services.length}
            <div class="grid gap-2 sm:grid-cols-2">
              {#each services as sv (sv.id)}
                <label class="flex cursor-pointer items-center gap-2 text-sm"><input type="checkbox" class="h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={sv.public} />{sv.name} · <span class="text-app-muted">{moneyCents(sv.price_cents)}</span></label>
              {/each}
            </div>
            <p class="hint mt-1">El precio sale del catálogo de servicios; el directorio muestra «desde» el más barato.</p>
          {:else}
            <p class="hint">Agrega servicios con precio en el catálogo para poder mostrarlos.</p>
          {/if}
        </fieldset>
      </div>

      </div>

      <div class="grid gap-5 {tab !== 'opiniones' ? '!hidden' : ''}">
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
      </div>

    </form>

    <div hidden={tab !== 'pagina'}><ProfileMedia /></div>

    {#if results}
      <div class="mt-8 {tab !== 'opiniones' ? '!hidden' : ''}">
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
                {#if r.public}
                  <div class="mt-2 grid gap-2">
                    <label class="sr-only" for="rp-{r.id}">Tu respuesta</label>
                    <textarea id="rp-{r.id}" class="field" rows="2" maxlength="600" placeholder="Responde en público (opcional)" bind:value={replies[r.id]}></textarea>
                    <div><button type="button" class="btn-secondary" disabled={replying === r.id} onclick={() => sendReply(r.id)}>{#if replying === r.id}<span class="spin"></span>{/if}Guardar respuesta</button></div>
                  </div>
                {/if}
              </li>
            {/each}
          </ul>
        {/if}
      </div>
    {/if}

    <div class="mt-10 pt-5 save-sticky">
      <OpError op={saveOp} class="mb-3" />
      <button type="submit" form="pf-form" class="btn-primary" disabled={saveOp.phase === 'loading'}>{#if saveOp.phase === 'loading'}<span class="spin"></span>{/if}Guardar cambios</button>
      <p class="hint mt-2">Guarda los textos, la página, el directorio, Google Maps y la encuesta. Las fotos y las respuestas a opiniones se guardan por separado.</p>
    </div>
  {/if}
</section>
