export type Phase = 'idle' | 'loading' | 'success' | 'error';

/** Tracks one async user action (loading / success / error + message) for the UI to render. */
export class Op {
  phase = $state<Phase>('idle');
  message = $state('');
  #timer: ReturnType<typeof setTimeout> | undefined;

  reset() {
    clearTimeout(this.#timer);
    this.phase = 'idle';
    this.message = '';
  }

  /** Runs `fn`; resolves to true on success. Errors are shown for a few seconds, then cleared. */
  async run(fn: () => Promise<unknown>, successMessage = ''): Promise<boolean> {
    clearTimeout(this.#timer);
    this.phase = 'loading';
    this.message = '';
    try {
      await fn();
      this.phase = 'success';
      this.message = successMessage;
      return true;
    } catch (e) {
      this.phase = 'error';
      this.message = e instanceof Error ? e.message : 'Ocurrió un error inesperado.';
      this.#timer = setTimeout(() => this.reset(), 4000);
      return false;
    }
  }

  fail(message: string) {
    clearTimeout(this.#timer);
    this.phase = 'error';
    this.message = message;
    this.#timer = setTimeout(() => this.reset(), 4000);
  }
}
