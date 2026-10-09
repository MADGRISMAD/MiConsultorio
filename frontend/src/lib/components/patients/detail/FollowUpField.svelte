<script lang="ts">
  interface Props {
    date: string;
    time: string;
    /** shown under the field */
    hint?: string;
    id?: string;
  }
  let { date = $bindable(), time = $bindable(), hint = '', id = 'followup' }: Props = $props();
  const today = new Date().toISOString().slice(0, 10);
</script>

<fieldset class="rounded-2xl border border-app-ink/10 p-4">
  <legend class="section-title px-1">Cita de seguimiento recomendada <span class="font-normal normal-case text-app-muted">(opcional)</span></legend>
  <div class="grid gap-3 sm:grid-cols-2">
    <div>
      <label class="label" for="{id}-date">Fecha</label>
      <input id="{id}-date" type="date" class="field" min={today} bind:value={date} />
    </div>
    <div>
      <label class="label" for="{id}-time">Hora <span class="font-normal text-app-muted">(si la dejas vacía se toma el primer horario libre)</span></label>
      <input id="{id}-time" type="time" class="field" bind:value={time} disabled={!date} />
    </div>
  </div>
  <p class="hint">{hint || 'Se agrega a la agenda como pendiente de confirmar: recepción la confirma con el paciente.'}</p>
</fieldset>
