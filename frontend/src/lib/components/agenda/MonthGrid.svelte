<script lang="ts">
  import { STATUS_META, isActive, type Appt, type Professional, type TimeBlock } from '$lib/types/agenda';
  import { WEEKDAY_SHORT, fullName, monthDays, parseDay, proColor, todayStr } from './util';

  interface Props {
    cursor: string;
    appts: Appt[];
    blocks: TimeBlock[];
    pros: Professional[];
    onday: (day: string) => void;
    onopen: (a: Appt) => void;
  }
  let { cursor, appts, blocks, pros, onday, onopen }: Props = $props();

  const today = todayStr();
  const days = $derived(monthDays(cursor));
  const month = $derived(cursor.slice(0, 7));
  const byDay = $derived.by(() => {
    const m = new Map<string, Appt[]>();
    for (const a of appts) {
      if (!isActive(a)) continue;
      const l = m.get(a.date) ?? [];
      l.push(a);
      m.set(a.date, l);
    }
    return m;
  });
  const blocked = (d: string) => blocks.some((b) => b.date_from <= d && d <= b.date_to && !b.startHour);
  const MAX = 3;
</script>

<div class="overflow-x-auto">
  <div class="min-w-[34rem]">
    <div class="grid grid-cols-7 border-b border-app-ink/10">
      {#each WEEKDAY_SHORT as w}
        <div class="px-2 py-2 text-center font-mono text-[11px] uppercase tracking-[0.12em] text-app-muted">{w}</div>
      {/each}
    </div>
    <div class="grid grid-cols-7">
      {#each days as d (d)}
        {@const list = byDay.get(d) ?? []}
        <div class="min-h-[6.5rem] border-b border-l border-app-ink/10 p-1 first:border-l-0 [&:nth-child(7n+1)]:border-l-0 {d.slice(0, 7) === month ? '' : 'bg-app-ink/[0.03]'} {blocked(d) ? 'month-block' : ''}">
          <button type="button" class="mb-1 grid h-6 min-w-6 place-items-center rounded-full px-1 text-xs font-medium hover:bg-app-ink/8 {d === today ? 'bg-app-primary text-white hover:bg-app-primary' : d.slice(0, 7) === month ? '' : 'text-app-muted'}" onclick={() => onday(d)} aria-label="Ver el día {parseDay(d).toLocaleDateString('es-MX', { day: 'numeric', month: 'long' })}">{parseDay(d).getDate()}</button>
          {#each list.slice(0, MAX) as a (a.id)}
            <button type="button" class="mb-0.5 block w-full truncate rounded border-l-[3px] px-1 py-px text-left text-[11px] {STATUS_META[a.status].card}" style="border-left-color:{proColor(pros, a.professional_id)}" onclick={() => onopen(a)}>
              <span class="tabular-nums">{a.startHour}</span> {fullName(a)}
            </button>
          {/each}
          {#if list.length > MAX}
            <button type="button" class="px-1 text-[11px] font-medium text-app-primary hover:underline" onclick={() => onday(d)}>+{list.length - MAX} más</button>
          {/if}
        </div>
      {/each}
    </div>
  </div>
</div>

<style>
  .month-block {
    background-image: repeating-linear-gradient(135deg, rgb(var(--app-ink) / 0.1) 0 6px, rgb(var(--app-ink) / 0.02) 6px 12px);
  }
</style>
