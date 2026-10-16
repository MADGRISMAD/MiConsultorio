<script lang="ts">
  import ButtonLabel from './ButtonLabel.svelte';
  import Icon from './Icon.svelte';
  import SectionHead from './SectionHead.svelte';
  import { demoHref, plans } from './data';
  import { btn, reveal } from './motion';

  const steps = [
    { title: 'Solicita una demo', text: 'Te mostramos Caresia con ejemplos de tu especialidad.' },
    { title: 'Configura tu consultorio', text: 'En Crecimiento y Pro te acompañamos en el alta, tus usuarios y sus permisos.' },
    { title: 'Empieza a atender', text: 'Registra pacientes y citas desde el primer día.' }
  ];


</script>

<section id="precios" class="relative isolate scroll-mt-24">
  <div class="mx-auto max-w-6xl px-5 py-16 sm:px-8 lg:py-24">
  <SectionHead title={'Planes claros,\n*sin letras chiquitas.*'} blurb="Pago mensual, sin plazo forzoso. Cancela cuando quieras." />
  <ul class="mt-12 grid gap-4 lg:grid-cols-3 lg:items-stretch">
    {#each plans as p, i}
      {@const dark = p.featured}
      <li class="flex">
        <div
          role="presentation"
          use:reveal={{ y: 40, delay: i * 0.08, margin: '0px 0px -10% 0px' }}
          class="group relative flex w-full flex-col overflow-hidden rounded-[26px] p-7 {dark ? 'l-deep bg-ink text-paper lg:-my-4 lg:py-11' : 'bg-panel ring-1 ring-ink/10'}"
        >
          <div class="relative flex items-center justify-between">
            <h3 class="font-display text-3xl">{p.name}</h3>
            {#if p.featured}<span class="rounded-full bg-signal px-3 py-1 font-mono text-[10px] uppercase tracking-[0.14em] text-white">Recomendado</span>{/if}
          </div>
          <p class="relative mt-1 font-mono text-[11px] uppercase tracking-[0.12em] {dark ? 'text-paper/65' : 'text-ink-faint'}">{p.tagline}</p>
          <p class="relative mt-5 flex items-baseline gap-2">
            <span class="font-display text-5xl leading-none tracking-[-0.02em] tabular-nums">{p.price}</span>
            <span class="font-mono text-[11px] uppercase tracking-[0.1em] {dark ? 'text-paper/65' : 'text-ink-faint'}">{p.period} + IVA</span>
          </p>
          <ul class="relative mt-6 flex-1 space-y-2.5 text-[15px]">
            {#each p.highlights as f}
              <li class="flex gap-3">
                <Icon name="check" class="mt-0.5 h-4 w-4 flex-none {dark ? 'text-signal' : 'text-mint'}" strokeWidth={2.4} />
                <span class={dark ? 'text-paper/90' : 'text-ink'}>{f}</span>
              </li>
            {/each}
          </ul>
          <details class="relative mt-5 text-[14px] {dark ? 'text-paper/80' : 'text-ink-soft'}">
            <summary class="cursor-pointer select-none py-1 font-medium underline decoration-current/30 underline-offset-4 hover:decoration-current">Ver todas las características</summary>
            <ul class="mt-3 space-y-2">
              {#if p.includes}<li class="font-semibold {dark ? 'text-paper' : 'text-ink'}">{p.includes}</li>{/if}
              {#each p.features as f}<li class="flex gap-2"><span aria-hidden="true">·</span>{f}</li>{/each}
            </ul>
          </details>
          <a href={demoHref} class="{btn} relative mt-7 h-12 px-6 text-[15px] {dark ? 'bg-signal text-white hover:bg-paper hover:text-ink focus-visible:ring-offset-ink' : 'bg-ink text-paper hover:bg-signal'}">
            <ButtonLabel>{p.cta}</ButtonLabel>
          </a>
        </div>
      </li>
    {/each}
  </ul>
  <p class="mt-8 text-center font-mono text-[11px] uppercase tracking-[0.14em] text-ink-faint">Precios en pesos mexicanos · IVA no incluido</p>

  <ol class="mt-16 grid gap-6 border-t border-ink/15 pt-10 md:grid-cols-3 md:gap-10">
    {#each steps as s, i}
      <li use:reveal={{ y: 20, delay: i * 0.08 }} class="flex gap-4">
        <span class="grid h-10 w-10 flex-none place-items-center rounded-full font-display text-xl italic text-signal ring-1 ring-ink/20">{i + 1}</span>
        <div>
          <h3 class="font-display text-2xl leading-tight">{s.title}</h3>
          <p class="mt-1.5 text-[15px] leading-relaxed text-ink-soft">{s.text}</p>
        </div>
      </li>
    {/each}
  </ol>
  </div>
</section>
