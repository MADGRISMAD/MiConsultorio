<script lang="ts">
  import { ownersApi } from '$lib/api/owners';
  import type { OwnerPet, OwnerRef } from '$lib/types/owners';
  import Icon from '../../ui/Icon.svelte';

  let { patientId }: { patientId: string } = $props();

  let owner = $state<OwnerRef | null>(null);
  let siblings = $state<OwnerPet[]>([]);

  $effect(() => {
    const id = patientId;
    owner = null;
    siblings = [];
    ownersApi.ofPatient(id).then(
      (r) => {
        if (id !== patientId) return;
        owner = r.owner;
        siblings = r.siblings;
      },
      () => {}
    );
  });
</script>

{#if owner}
  <section class="card p-5" aria-label="Propietario">
    <p class="section-title">Propietario</p>
    <p class="mt-1.5 flex items-center gap-2 font-semibold"><Icon name="user" size={17} class="text-app-primary" />{owner.name}</p>
    <p class="mt-0.5 text-sm text-app-muted">{[owner.phone, owner.email].filter(Boolean).join(' · ') || 'Sin datos de contacto'}</p>
    {#if siblings.length > 0}
      <p class="section-title mt-4">Otras mascotas de {owner.name.split(' ')[0]}</p>
      <ul class="mt-2 flex flex-wrap gap-2">
        {#each siblings as s (s.id)}
          <li>
            <a href="/pacientes/{encodeURIComponent(s.id)}" class="inline-flex min-h-9 items-center gap-2 rounded-full border border-app-ink/15 px-3.5 text-sm font-medium transition hover:border-app-primary hover:text-app-primary">
              <Icon name="paw" size={15} />{s.names}{#if s.species}<span class="font-normal text-app-muted">· {s.species}</span>{/if}
            </a>
          </li>
        {/each}
      </ul>
    {/if}
  </section>
{/if}
