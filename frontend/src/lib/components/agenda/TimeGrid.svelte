<script lang="ts">
  import { onMount } from 'svelte';
  import { STATUS_META, isActive, type Appt, type Professional, type TimeBlock } from '$lib/types/agenda';
  import { fmtShort, fromMin, fullName, layoutLanes, proColor, toMin, todayStr } from './util';

  interface Props {
    days: string[];
    appts: Appt[];
    blocks: TimeBlock[];
    pros: Professional[];
    range: [number, number];
    slot: number;
    canEdit: boolean;
    isClosed: (day: string) => boolean;
    oncreate: (date: string, start: string) => void;
    onopen: (a: Appt) => void;
    onmove: (a: Appt, date: string, start: string) => void;
    onblock: (b: TimeBlock) => void;
    /** a specialist without agenda-admin rights: clicking an empty time marks it as busy */
    onbusy?: (date: string, start: string) => void;
    onday?: (day: string) => void;
  }
  let { days, appts, blocks, pros, range, slot, canEdit, isClosed, oncreate, onopen, onmove, onblock, onbusy, onday }: Props = $props();

  const PPM = 1.4; // pixels per minute
  const lo = $derived(range[0]);
  const hi = $derived(range[1]);
  const height = $derived((hi - lo) * PPM);
  const hours = $derived(Array.from({ length: Math.ceil((hi - lo) / 60) }, (_, i) => lo + i * 60));
  const today = todayStr();

  let nowMin = $state(minutesNow());
  function minutesNow() {
    const d = new Date();
    return d.getHours() * 60 + d.getMinutes();
  }
  onMount(() => {
    const t = setInterval(() => (nowMin = minutesNow()), 30_000);
    const stop = (e: TouchEvent) => {
      if (drag?.active) e.preventDefault();
    };
    scroller?.addEventListener('touchmove', stop, { passive: false });
    // open the day at the current hour (or the first appointment)
    if (scroller && days.includes(today) && nowMin > lo + 60) scroller.scrollTop = Math.max(0, (nowMin - lo - 90) * PPM);
    return () => {
      clearInterval(t);
      scroller?.removeEventListener('touchmove', stop);
    };
  });

  let scroller = $state<HTMLDivElement>();
  let cols = $state<HTMLElement[]>([]);

  const byDay = $derived.by(() => {
    const m = new Map<string, Appt[]>();
    for (const d of days) m.set(d, []);
    for (const a of appts) m.get(a.date)?.push(a);
    return m;
  });
  const lanes = $derived.by(() => {
    const out = new Map<string, [number, number]>();
    for (const list of byDay.values()) for (const [k, v] of layoutLanes(list.filter(isActive))) out.set(k, v);
    return out;
  });
  const blocksOf = (d: string) => blocks.filter((b) => b.date_from <= d && d <= b.date_to);

  const movable = (a: Appt) => canEdit && (a.status === 'scheduled' || a.status === 'confirmed' || a.status === 'arrived');

  // ----- click on an empty slot -----
  function onColClick(e: MouseEvent, d: string) {
    if ((!canEdit && !onbusy) || justDragged || e.target !== e.currentTarget) return;
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    const min = lo + Math.floor((e.clientY - rect.top) / PPM / slot) * slot;
    const at = fromMin(Math.min(Math.max(min, lo), hi - slot));
    if (canEdit) oncreate(d, at);
    else onbusy?.(d, at);
  }

  // ----- drag to reschedule: mouse drags after a few pixels, touch after a long press -----
  interface Drag {
    appt: Appt;
    x: number;
    y: number;
    grab: number; // pixels between the card's top and the pointer
    touch: boolean;
    active: boolean;
    date: string;
    start: number;
    timer?: ReturnType<typeof setTimeout>;
  }
  let drag = $state<Drag | null>(null);
  let justDragged = false;

  function down(e: PointerEvent, a: Appt) {
    if (!movable(a) || (e.pointerType === 'mouse' && e.button !== 0)) return;
    const card = e.currentTarget as HTMLElement;
    const touch = e.pointerType !== 'mouse';
    drag = { appt: a, x: e.clientX, y: e.clientY, grab: e.clientY - card.getBoundingClientRect().top, touch, active: false, date: a.date, start: toMin(a.startHour) };
    if (touch) drag.timer = setTimeout(() => drag && ((drag.active = true), navigator.vibrate?.(15)), 380);
  }
  function move(e: PointerEvent) {
    if (!drag) return;
    if (!drag.active) {
      const dist = Math.hypot(e.clientX - drag.x, e.clientY - drag.y);
      if (drag.touch && dist > 10) {
        clearTimeout(drag.timer);
        drag = null;
      } else if (!drag.touch && dist > 5) drag.active = true;
      if (!drag?.active) return;
    }
    let i = cols.findIndex((c) => c && e.clientX >= c.getBoundingClientRect().left && e.clientX < c.getBoundingClientRect().right);
    if (i < 0) i = e.clientX < (cols[0]?.getBoundingClientRect().left ?? 0) ? 0 : days.length - 1;
    const rect = cols[i].getBoundingClientRect();
    const dur = toMin(drag.appt.endHour) - toMin(drag.appt.startHour);
    const raw = lo + (e.clientY - rect.top - drag.grab) / PPM;
    drag.date = days[i];
    drag.start = Math.min(Math.max(Math.round(raw / slot) * slot, lo), Math.max(lo, hi - dur));
  }
  function up() {
    if (!drag) return;
    clearTimeout(drag.timer);
    const d = drag;
    drag = null;
    if (!d.active) return;
    justDragged = true;
    setTimeout(() => (justDragged = false), 50);
    if (d.date !== d.appt.date || fromMin(d.start) !== d.appt.startHour) onmove(d.appt, d.date, fromMin(d.start));
  }
  function click(a: Appt) {
    if (!justDragged) onopen(a);
  }
  function cancelDrag(e: KeyboardEvent) {
    if (e.key === 'Escape' && drag) {
      clearTimeout(drag.timer);
      drag = null;
    }
  }

  const style = (a: Appt) => {
    const [lane, n] = lanes.get(a.id) ?? [0, 1];
    const top = (toMin(a.startHour) - lo) * PPM;
    const h = Math.max((toMin(a.endHour) - toMin(a.startHour)) * PPM, 22);
    return `top:${top}px;height:${h - 2}px;left:calc(${(lane / n) * 100}% + 2px);width:calc(${100 / n}% - 4px);border-left-color:${proColor(pros, a.professional_id)}`;
  };
  const blockStyle = (b: TimeBlock, d: string) => {
    const s = b.startHour && b.date_from <= d ? toMin(b.startHour) : lo;
    const e = b.endHour ? toMin(b.endHour) : hi;
    const top = (Math.max(s, lo) - lo) * PPM;
    return `top:${top}px;height:${Math.max((Math.min(e, hi) - Math.max(s, lo)) * PPM, 10)}px`;
  };
  const colTemplate = $derived(`grid-template-columns:3.25rem repeat(${days.length}, minmax(${days.length === 1 ? '12rem' : '6.75rem'}, 1fr));min-width:${days.length === 1 ? '0' : `${3.25 + days.length * 6.75}rem`}`);
</script>

<svelte:window onpointermove={move} onpointerup={up} onpointercancel={up} onkeydown={cancelDrag} />

<div bind:this={scroller} class="isolate max-h-[calc(100dvh-17rem)] min-h-[24rem] overflow-auto overscroll-x-contain rounded-[inherit]" role="presentation">
  <div class="grid" style={colTemplate}>
    <!-- header -->
    <div class="sticky left-0 top-0 z-30 border-b border-app-ink/10 bg-app-panel"></div>
    {#each days as d (d)}
      <div class="sticky top-0 z-20 border-b border-l border-app-ink/10 bg-app-panel px-2 py-2 text-center">
        {#if onday}
          <button type="button" class="rounded-lg px-2 py-0.5 text-sm font-medium hover:bg-app-ink/5 {d === today ? 'text-app-primary' : ''}" onclick={() => onday(d)}>{fmtShort(d)}</button>
        {:else}
          <span class="text-sm font-medium {d === today ? 'text-app-primary' : ''}">{fmtShort(d)}</span>
        {/if}
      </div>
    {/each}

    <!-- hour gutter -->
    <div class="sticky left-0 z-10 border-app-ink/10 bg-app-panel" style="height:{height}px">
      <div class="relative h-full">
        {#each hours as h (h)}
          <span class="absolute right-1.5 -translate-y-1/2 font-mono text-[10.5px] text-app-muted tabular-nums" style="top:{(h - lo) * PPM}px;{h === lo ? 'transform:none' : ''}">{fromMin(h)}</span>
        {/each}
      </div>
    </div>

    <!-- day columns -->
    {#each days as d, i (d)}
      <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
      <div
        bind:this={cols[i]}
        data-day={d}
        class="relative border-l border-app-ink/10 {isClosed(d) ? 'bg-app-ink/[0.035]' : ''} {canEdit || onbusy ? 'cursor-cell' : ''}"
        style="height:{height}px;background-image:linear-gradient(to bottom, transparent calc(100% - 1px), rgb(var(--app-ink) / 0.09) 0);background-size:100% {60 * PPM}px"
        onclick={(e) => onColClick(e, d)}
      >
        {#each blocksOf(d) as b (b.id)}
          {#if canEdit || onbusy}
            <button type="button" class="block-stripes absolute inset-x-0 z-[1] overflow-hidden px-1.5 py-1 text-left text-[11px] font-medium text-app-muted" style={blockStyle(b, d)} onclick={() => onblock(b)} title="Bloqueo: {b.reason || 'sin motivo'}">
              {b.reason || 'Bloqueado'}
            </button>
          {:else}
            <div class="block-stripes absolute inset-x-0 z-[1] overflow-hidden px-1.5 py-1 text-[11px] font-medium text-app-muted" style={blockStyle(b, d)} title="Bloqueo: {b.reason || 'sin motivo'}">{b.reason || 'Bloqueado'}</div>
          {/if}
        {/each}

        {#each byDay.get(d) ?? [] as a (a.id)}
          {@const faded = !isActive(a)}
          <button
            type="button"
            class="absolute z-[2] overflow-hidden rounded-lg border-l-4 px-1.5 py-1 text-left text-[11.5px] leading-tight shadow-sm transition hover:brightness-95 focus-visible:z-10 {STATUS_META[a.status].card} {faded ? 'opacity-70' : ''} {movable(a) ? 'select-none' : ''} {drag?.active && drag.appt.id === a.id ? 'opacity-40' : ''}"
            style={style(a)}
            onpointerdown={(e) => down(e, a)}
            onclick={() => click(a)}
            aria-label="{a.startHour} {fullName(a)}, {STATUS_META[a.status].label}"
          >
            <span class="block truncate font-semibold tabular-nums">{a.startHour} {fullName(a)}</span>
            {#if toMin(a.endHour) - toMin(a.startHour) >= 40}
              <span class="block truncate text-app-muted">{a.service_name || a.details || STATUS_META[a.status].label}</span>
            {/if}
            {#if toMin(a.endHour) - toMin(a.startHour) >= 55 && (a.room || a.professional_name)}
              <span class="block truncate text-app-muted">{[a.professional_name, a.room].filter(Boolean).join(' · ')}</span>
            {/if}
          </button>
        {/each}

        {#if drag?.active && drag.date === d}
          {@const dur = toMin(drag.appt.endHour) - toMin(drag.appt.startHour)}
          <div class="pointer-events-none absolute inset-x-1 z-20 rounded-lg border-2 border-dashed border-app-primary bg-app-primary/20 px-1.5 py-1 text-[11.5px] font-semibold text-app-primary" style="top:{(drag.start - lo) * PPM}px;height:{dur * PPM - 2}px">
            {fromMin(drag.start)} – {fromMin(drag.start + dur)}
          </div>
        {/if}

        {#if d === today && nowMin >= lo && nowMin <= hi}
          <div class="pointer-events-none absolute inset-x-0 z-[5] flex items-center" style="top:{(nowMin - lo) * PPM}px" aria-hidden="true">
            <span class="-ml-1 h-2 w-2 rounded-full bg-app-danger"></span>
            <span class="h-px flex-1 bg-app-danger"></span>
          </div>
        {/if}
      </div>
    {/each}
  </div>
</div>

<style>
  .block-stripes {
    background-image: repeating-linear-gradient(135deg, rgb(var(--app-ink) / 0.13) 0 6px, rgb(var(--app-ink) / 0.03) 6px 12px);
    border-top: 1px solid rgb(var(--app-ink) / 0.2);
  }
  button {
    -webkit-touch-callout: none;
  }
</style>
