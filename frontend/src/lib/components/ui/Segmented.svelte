<script lang="ts" generics="T extends string">
  /**
   * A row of pill buttons where exactly one is on: a filter, a view or a period. It is a group of toggle buttons (aria-pressed),
   * which is what these are: none of them owns a tab panel, so they are not announced as tabs. Real tabs (with a panel) use SectionTabs.
   */
  interface Option {
    value: T;
    label: string;
    disabled?: boolean;
  }
  interface Props {
    options: readonly Option[];
    value: T;
    /** what the group chooses, read aloud before the options */
    label: string;
    onselect: (v: T) => void;
    /** the row may scroll sideways when it does not fit (periods, long filters) */
    scroll?: boolean;
    /** fills the width instead of hugging its content */
    block?: boolean;
    size?: 'sm' | 'md';
    class?: string;
  }
  let { options, value, label, onselect, scroll = false, block = false, size = 'md', class: cls = '' }: Props = $props();
</script>

<div class="{block ? 'flex' : 'inline-flex'} max-w-full min-w-0 gap-1 rounded-full bg-app-ink/5 p-1 {scroll ? 'overflow-x-auto' : ''} {cls}" role="group" aria-label={label}>
  {#each options as o (o.value)}
    <button
      type="button"
      aria-pressed={value === o.value}
      disabled={o.disabled}
      class="whitespace-nowrap rounded-full text-sm font-medium transition disabled:cursor-default disabled:opacity-60 {size === 'sm' ? 'px-3.5 py-1.5' : 'min-h-9 px-4'} {value === o.value ? 'bg-app-panel text-app-ink shadow-sm' : 'text-app-muted hover:text-app-ink'}"
      onclick={() => onselect(o.value)}>{o.label}</button>
  {/each}
</div>
