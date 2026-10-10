/**
 * One save button for a whole settings page. Each form on the page registers its own save here;
 * the sticky bar of Ajustes runs them all, in order. While any is registered, the forms hide their own buttons.
 */
class SaveAll {
  private fns = new Set<() => Promise<unknown> | unknown>();
  count = $state(0);
  busy = $state(false);

  get active(): boolean {
    return this.count > 0;
  }

  /** returns the function that unregisters (use it as an $effect cleanup) */
  register(fn: () => Promise<unknown> | unknown): () => void {
    this.fns.add(fn);
    this.count = this.fns.size;
    return () => {
      this.fns.delete(fn);
      this.count = this.fns.size;
    };
  }

  async run(): Promise<void> {
    if (this.busy) return;
    this.busy = true;
    try {
      for (const fn of [...this.fns]) await fn();
    } finally {
      this.busy = false;
    }
  }
}

export const saveAll = new SaveAll();
