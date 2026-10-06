<script lang="ts">
  interface Props {
    class?: string;
    base?: string;
    sweep?: string;
    beats?: number;
  }
  let { class: cls = '', base = 'stroke-ink/10', sweep = 'stroke-signal', beats = 4 }: Props = $props();

  const BEAT = 300;
  const d = $derived.by(() => {
    let path = 'M0 60';
    for (let x = 0; x < beats * BEAT; x += BEAT) {
      path += ` L${x + 90} 60 Q${x + 105} 47 ${x + 120} 60 L${x + 140} 60 L${x + 148} 70 L${x + 158} 8 L${x + 168} 100 L${x + 177} 60 L${x + 200} 60 Q${x + 222} 38 ${x + 246} 60 L${x + BEAT} 60`;
    }
    return path;
  });
</script>

<!-- Heart-monitor trace with a sweeping highlight. Decorative. -->
<svg aria-hidden="true" viewBox="0 0 {beats * BEAT} 120" preserveAspectRatio="none" class={cls} fill="none">
  <path {d} class={base} stroke-width="1.5" vector-effect="non-scaling-stroke" stroke-linejoin="round" />
  <path {d} pathLength="1000" class="{sweep} ecg-sweep" stroke-width="2.5" vector-effect="non-scaling-stroke" stroke-linejoin="round" stroke-linecap="round" />
</svg>
