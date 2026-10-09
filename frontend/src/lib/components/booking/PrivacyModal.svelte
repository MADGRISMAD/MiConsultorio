<script lang="ts">
  import Modal from '$lib/components/Modal.svelte';
  import PrivacyContent from '$lib/legal/PrivacyContent.svelte';

  let { open, onclose }: { open: boolean; onclose: () => void } = $props();
</script>

<!-- the privacy notice read in place, so nobody has to leave the form -->
<Modal {open} title="Aviso de privacidad" {onclose} wide>
  <div class="max-h-[65vh] overflow-y-auto pr-1 text-left"><PrivacyContent /></div>
  {#snippet footer()}<button type="button" class="btn-primary" onclick={onclose}>Entendido</button>{/snippet}
</Modal>
