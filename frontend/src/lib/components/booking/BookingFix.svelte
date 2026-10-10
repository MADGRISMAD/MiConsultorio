<script lang="ts">
  import { agendaApi } from '$lib/api/agenda';
  import Icon from '$lib/components/ui/Icon.svelte';

  let { slug }: { slug: string } = $props();
  let fix = $state<{ problem: string; title: string; text: string; to: string; action: string } | null>(null);

  // Only a signed-in team member gets an answer; for everyone else the call is refused and nothing shows.
  $effect(() => {
    const s = slug;
    fix = null;
    agendaApi
      .bookingCheck(s)
      .then((r) => (fix = r.problem ? r : null))
      .catch(() => (fix = null));
  });
</script>

{#if fix}
  <section class="card mt-4 border border-app-primary/30 px-6 py-6" aria-live="polite">
    <p class="text-xs font-semibold uppercase tracking-wide text-app-primary">Solo lo ves tú, porque tienes la sesión iniciada</p>
    <h2 class="display mt-2 text-2xl leading-tight">{fix.title}</h2>
    <p class="mt-2 text-app-muted">{fix.text}</p>
    <a href={fix.to} class="btn-primary mt-5 inline-flex items-center gap-2">{fix.action}<Icon name="arrow-right" size={16} /></a>
  </section>
{/if}
