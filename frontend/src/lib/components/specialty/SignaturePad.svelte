<script lang="ts">
  import { onMount } from 'svelte';
  import Icon from '../ui/Icon.svelte';

  interface Props {
    /** PNG data URL of the signature, or '' while the pad is empty */
    value?: string;
    label?: string;
  }
  let { value = $bindable(''), label = 'Firma' }: Props = $props();

  const W = 640;
  const H = 220;
  let canvas = $state<HTMLCanvasElement>();
  let drawing = false;
  let last: [number, number] | null = null;
  let ink = $state(false);
  const uid = $props.id();

  function ctx() {
    return canvas?.getContext('2d') ?? null;
  }
  function blank() {
    const c = ctx();
    if (!c) return;
    c.fillStyle = '#fff';
    c.fillRect(0, 0, W, H);
    c.strokeStyle = '#111';
    c.lineWidth = 3;
    c.lineCap = 'round';
    c.lineJoin = 'round';
  }
  onMount(blank);

  function pos(ev: PointerEvent): [number, number] {
    const r = canvas!.getBoundingClientRect();
    return [((ev.clientX - r.left) / r.width) * W, ((ev.clientY - r.top) / r.height) * H];
  }
  function down(ev: PointerEvent) {
    ev.preventDefault();
    canvas!.setPointerCapture(ev.pointerId);
    drawing = true;
    last = pos(ev);
    const c = ctx()!;
    c.beginPath();
    c.arc(last[0], last[1], 1.4, 0, Math.PI * 2);
    c.fillStyle = '#111';
    c.fill();
  }
  function move(ev: PointerEvent) {
    if (!drawing || !last) return;
    ev.preventDefault();
    const c = ctx()!;
    const p = pos(ev);
    c.beginPath();
    c.moveTo(last[0], last[1]);
    c.lineTo(p[0], p[1]);
    c.stroke();
    last = p;
    ink = true;
  }
  function up() {
    if (!drawing) return;
    drawing = false;
    last = null;
    ink = true;
    value = canvas!.toDataURL('image/png');
  }
  export function clear() {
    blank();
    ink = false;
    value = '';
  }
  // the parent can reset the pad by clearing the bound value
  $effect(() => {
    if (value === '' && ink) {
      blank();
      ink = false;
    }
  });
</script>

<div>
  <p id="{uid}-l" class="label">{label}</p>
  <div class="overflow-hidden rounded-xl border-2 border-dashed border-app-ink/25 bg-white">
    <canvas
      bind:this={canvas}
      width={W}
      height={H}
      aria-labelledby="{uid}-l"
      aria-describedby="{uid}-h"
      class="block h-auto w-full cursor-crosshair touch-none"
      onpointerdown={down}
      onpointermove={move}
      onpointerup={up}
      onpointercancel={up}
    ></canvas>
  </div>
  <div class="mt-1.5 flex items-center justify-between gap-2">
    <p id="{uid}-h" class="hint !mt-0">Firma con el dedo, el lápiz o el mouse dentro del recuadro.</p>
    <button type="button" class="btn-ghost !min-h-8" disabled={!ink} onclick={clear}><Icon name="rotate" size={16} />Borrar</button>
  </div>
</div>
