/**
 * The loading state of a screen: «loading» until the first load ends, and the message of the last failure.
 * `run` does the try / catch / finally that every list repeats.
 *
 * A screen that loads again while an earlier load is still in flight (a search that changes, a timer) keeps only the
 * newest: the older one is told so through `current()` and neither its error nor its «loading = false» is applied.
 */
export class Loader {
  loading = $state(true);
  error = $state('');
  private fallback: string;
  private seq = 0;

  /** `loading: false` for a screen that only loads when something happens (it starts idle). */
  constructor(fallback: string, opts: { loading?: boolean } = {}) {
    this.fallback = fallback;
    this.loading = opts.loading ?? true;
  }

  /**
   * Runs the work. A success clears the error, a failure keeps its message (or the fallback).
   * `reset` shows the loading state again and clears the error before starting (a reload the person asked for).
   * The work receives `current()`: false once a newer run started, so it can drop an answer that came late.
   */
  async run(work: (current: () => boolean) => Promise<unknown>, opts: { reset?: boolean } = {}): Promise<boolean> {
    const my = ++this.seq;
    const current = () => my === this.seq;
    if (opts.reset) {
      this.loading = true;
      this.error = '';
    }
    try {
      await work(current);
      if (current()) this.error = '';
      return true;
    } catch (e) {
      if (current()) this.error = e instanceof Error ? e.message : this.fallback;
      return false;
    } finally {
      if (current()) this.loading = false;
    }
  }
}
