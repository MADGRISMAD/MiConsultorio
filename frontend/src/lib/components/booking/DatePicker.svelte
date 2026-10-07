<script lang="ts">
  import Icon from '$lib/components/ui/Icon.svelte';

  interface Props {
    /** first selectable day, YYYY-MM-DD */
    min: string;
    /** days after `min` that can still be booked */
    horizon: number;
    value: string;
    onpick: (date: string) => void;
    /** Days (YYYY-MM-DD) that still have a free slot in the shown month; null while unknown (nothing is disabled). */
    available?: string[] | null;
    /** Called with the shown month (YYYY-MM) at start and whenever it changes. */
    onmonth?: (month: string) => void;
  }
  let { min, horizon, value, onpick, available = null, onmonth }: Props = $props();

  const free = $derived(available ? new Set(available) : null);

  const MONTHS = ['enero', 'febrero', 'marzo', 'abril', 'mayo', 'junio', 'julio', 'agosto', 'septiembre', 'octubre', 'noviembre', 'diciembre'];
  const DAYS = ['L', 'M', 'X', 'J', 'V', 'S', 'D'];
  const DAY_NAMES = ['lunes', 'martes', 'miércoles', 'jueves', 'viernes', 'sábado', 'domingo'];

  const iso = (y: number, m: number, d: number) => `${y}-${String(m + 1).padStart(2, '0')}-${String(d).padStart(2, '0')}`;
  const [my, mm] = (() => {
    const [y, m] = min.split('-').map(Number);
    return [y, m - 1];
  })();
  let year = $state(my);
  let month = $state(mm);

  const max = $derived.by(() => {
    const [y, m, d] = min.split('-').map(Number);
    const t = new Date(Date.UTC(y, m - 1, d + horizon));
    return iso(t.getUTCFullYear(), t.getUTCMonth(), t.getUTCDate());
  });

  const cells = $derived.by(() => {
    const first = (new Date(Date.UTC(year, month, 1)).getUTCDay() + 6) % 7; // Monday first
    const days = new Date(Date.UTC(year, month + 1, 0)).getUTCDate();
    const out: (number | null)[] = Array(first).fill(null);
    for (let d = 1; d <= days; d++) out.push(d);
    return out;
  });

  const canPrev = $derived(iso(year, month, 1) > min.slice(0, 8) + '01');
  const canNext = $derived(iso(month === 11 ? year + 1 : year, (month + 1) % 12, 1) <= max);

  function shift(n: number) {
    const t = new Date(Date.UTC(year, month + n, 1));
    year = t.getUTCFullYear();
    month = t.getUTCMonth();
  }

  $effect(() => {
    onmonth?.(`${year}-${String(month + 1).padStart(2, '0')}`);
  });

  function label(d: number) {
    const dow = (new Date(Date.UTC(year, month, d)).getUTCDay() + 6) % 7;
    return `${DAY_NAMES[dow]} ${d} de ${MONTHS[month]}`;
  }
</script>

<div class="rounded-2xl border border-app-ink/10 bg-app-panel p-3 sm:p-4">
  <div class="mb-2 flex items-center justify-between">
    <button type="button" class="icon-btn disabled:opacity-30" onclick={() => shift(-1)} disabled={!canPrev} aria-label="Mes anterior">
      <Icon name="arrow-left" size={18} />
    </button>
    <p class="text-sm font-semibold capitalize" aria-live="polite">{MONTHS[month]} {year}</p>
    <button type="button" class="icon-btn disabled:opacity-30" onclick={() => shift(1)} disabled={!canNext} aria-label="Mes siguiente">
      <Icon name="arrow-right" size={18} />
    </button>
  </div>
  <div class="grid grid-cols-7 gap-1 text-center" role="grid" aria-label="Elige una fecha">
    {#each DAYS as d}
      <span class="py-1 font-mono text-[11px] uppercase text-app-muted" aria-hidden="true">{d}</span>
    {/each}
    {#each cells as d}
      {#if d === null}
        <span></span>
      {:else}
        {@const day = iso(year, month, d)}
        {@const full = !!free && day >= min && day <= max && !free.has(day)}
        {@const off = day < min || day > max || full}
        <button
          type="button"
          class="grid aspect-square min-h-10 place-items-center rounded-xl text-sm transition disabled:cursor-not-allowed disabled:text-app-muted/40
            {value === day ? 'bg-app-ink font-semibold text-app-surface' : day === min ? 'font-semibold text-app-primary hover:bg-app-ink/8' : 'hover:bg-app-ink/8'}"
          disabled={off}
          aria-label={full ? `${label(d)}, sin lugares` : label(d)}
          aria-pressed={value === day}
          onclick={() => onpick(day)}>{d}</button
        >
      {/if}
    {/each}
  </div>
</div>
