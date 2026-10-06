<script lang="ts">
  import { specialties } from './data';
  import MarqueeRow from './MarqueeRow.svelte';
</script>

<section aria-label="Especialidades compatibles" class="border-y border-ink/15 py-10 sm:py-14">
  <p class="mb-6 text-center font-mono text-[11px] uppercase tracking-[0.16em] text-ink-soft">Funciona para cualquier consultorio que atiende con cita</p>
  <MarqueeRow base={-3} items={specialties.slice(0, 5)} />
  <MarqueeRow base={3} items={specialties.slice(5)} outline />
</section>
