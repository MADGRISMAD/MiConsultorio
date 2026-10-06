<script lang="ts">
  import { EASE, observeOnce, reducedMotion } from './motion';

  interface Props {
    text: string;
    tag?: 'h1' | 'h2' | 'h3' | 'p';
    class?: string;
    accent?: string;
    delay?: number;
    stagger?: number;
  }
  /**
   * Headline that rises in word by word from behind a mask.
   * Wrap words in *asterisks* to set them in the italic accent; "\n" breaks the line.
   */
  let { text, tag = 'h2', class: cls = '', accent = 'italic', delay = 0, stagger = 0.06 }: Props = $props();

  const lines = $derived.by(() => {
    let n = 0;
    return text.split('\n').map((line) =>
      line.split('*').flatMap((seg, si) =>
        seg
          .split(' ')
          .filter(Boolean)
          .map((word) => ({ word, accent: si % 2 === 1, i: n++ }))
      )
    );
  });

  let shown = $state(false);
  const reduce = reducedMotion();

  function inView(node: HTMLElement) {
    const stop = observeOnce(node, () => (shown = true), '0px 0px -10% 0px');
    return { destroy: stop };
  }

  const hidden = reduce ? 'opacity:0' : 'transform:translateY(110%) rotate(4deg)';
  const visible = reduce ? 'opacity:1' : 'transform:none';
</script>

<svelte:element this={tag} class={cls}>
  <span class="sr-only">{text.replace(/\*/g, '')}</span>
  <span aria-hidden="true" class="block" use:inView>
    {#each lines as line}
      <span class="block">
        {#each line as w (w.i)}
          <span class="inline-block overflow-hidden pb-[0.12em] -mb-[0.12em] align-bottom"
            ><span
              class="inline-block will-change-transform {w.accent ? accent : ''}"
              style="{shown ? visible : hidden}; transition: transform 1s {EASE} {delay + w.i * stagger}s, opacity 1s {EASE} {delay + w.i * stagger}s"
              >{w.word}</span
            ></span
          >{' '}
        {/each}
      </span>
    {/each}
  </span>
</svelte:element>
