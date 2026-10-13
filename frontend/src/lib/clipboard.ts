import { toast } from '$lib/toast.svelte';

/**
 * Copies text to the clipboard. Shows `ok` as a toast when given (a screen that marks the button itself passes none) and, when
 * the browser refuses, `fail` as an error. Resolves to whether it was copied.
 */
export async function copyText(text: string, msg: { ok?: string; fail?: string } = {}): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    if (msg.ok) toast.show(msg.ok);
    return true;
  } catch {
    toast.show(msg.fail ?? 'No se pudo copiar. Selecciona el texto y cópialo a mano.', 'error');
    return false;
  }
}
