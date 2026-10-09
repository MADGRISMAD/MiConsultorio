<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import { api } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import { CAPABILITY_ROWS, ROLE_PERMISSIONS, ROLES, type ClinicRole, type Person } from '$lib/types';
  import Modal from '../Modal.svelte';
  import Icon from '../ui/Icon.svelte';

  interface Props {
    person: Person | null;
    /** the plan includes cobros: show the register permissions */
    cobros: boolean;
    onclose: () => void;
    onsaved: (p: Person) => void;
  }
  let { person, cobros, onclose, onsaved }: Props = $props();

  // capabilities that can be changed for one person (managing the team never can)
  const GRANTABLE = ['navAppointments', 'adminAppointments', 'navHistorials', 'adminHistorials', 'pos', 'posReports', 'posManage'];
  const NEEDS: Record<string, string> = { adminAppointments: 'navAppointments', adminHistorials: 'navHistorials', posReports: 'pos', posManage: 'pos' };
  const DEPENDENTS = (basic: string) => Object.entries(NEEDS).filter(([, b]) => b === basic).map(([d]) => d);

  const rows = $derived(CAPABILITY_ROWS.filter(([, k]) => GRANTABLE.includes(k) && (cobros || !['pos', 'posReports', 'posManage'].includes(k))));
  const defaults = (role: string) => ROLE_PERMISSIONS[role as ClinicRole] ?? [];
  const roleHas = (k: string) => (person ? defaults(person.role) : []).includes(k);

  let on = $state<Record<string, boolean>>({});
  const op = new Op();

  $effect(() => {
    if (!person) return;
    const eff = new Set(person.permissions ?? defaults(person.role));
    on = Object.fromEntries(GRANTABLE.map((k) => [k, eff.has(k)]));
    op.reset();
  });

  function flip(k: string, v: boolean) {
    on[k] = v;
    if (v && NEEDS[k]) on[NEEDS[k]] = true;
    if (!v) for (const d of DEPENDENTS(k)) on[d] = false;
  }

  const custom = $derived(GRANTABLE.some((k) => on[k] !== roleHas(k)));

  async function save() {
    const p = person;
    if (!p) return;
    const extra = GRANTABLE.filter((k) => on[k] && !roleHas(k));
    const denied = GRANTABLE.filter((k) => !on[k] && roleHas(k));
    let saved: Person | undefined;
    if (await op.run(async () => void (saved = (await api.updateMember(p.id, { permissions_extra: extra, permissions_denied: denied })).person))) {
      toast.show('Permisos actualizados');
      if (saved) onsaved(saved);
    }
  }
  function reset() {
    if (!person) return;
    on = Object.fromEntries(GRANTABLE.map((k) => [k, roleHas(k)]));
  }
</script>

<Modal open={!!person} title={person ? `Permisos de ${person.name}` : 'Permisos'} {onclose}>
  {#if person}
    <p class="text-sm text-app-muted">Parte del rol <strong class="text-app-ink">{ROLES[person.role]?.label}</strong>. Activa o quita lo que esta persona puede hacer; los demás integrantes con el mismo rol no cambian.</p>
    <ul class="mt-4 divide-y divide-app-ink/8 rounded-xl border border-app-ink/10">
      {#each rows as [label, k]}
        <li class="flex items-center justify-between gap-3 px-4 py-3">
          <label class="min-w-0 flex-1 cursor-pointer text-sm" for="perm-{k}">
            {label}
            {#if on[k] !== roleHas(k)}<span class="ml-1.5 rounded-full bg-app-primary/12 px-2 py-0.5 text-[11px] font-medium text-app-primary">{on[k] ? 'agregado' : 'quitado'}</span>{/if}
          </label>
          <input id="perm-{k}" type="checkbox" class="h-5 w-5 flex-none accent-[rgb(var(--app-primary))]" checked={on[k]} onchange={(e) => flip(k, e.currentTarget.checked)} />
        </li>
      {/each}
    </ul>
    <p class="hint">Para administrar algo hace falta poder verlo, así que se activan juntos. Administrar el equipo solo lo tiene el rol Administrador.</p>
    <OpError op={op} class="mt-3" />
  {/if}
  {#snippet footer()}
    {#if custom}<button type="button" class="btn-ghost mr-auto" onclick={reset}>Volver a los del rol</button>{/if}
    <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
    <button type="button" class="btn-primary" disabled={op.phase === 'loading'} onclick={save}>{#if op.phase === 'loading'}<span class="spin"></span>{/if}Guardar permisos</button>
  {/snippet}
</Modal>
