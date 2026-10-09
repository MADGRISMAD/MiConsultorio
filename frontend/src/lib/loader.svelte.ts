/**
 * The loading state of a screen: «loading» until the first load ends, and the message of the last failure.
 * `run` does the try / catch / finally that every list repeats.
 */
export class Loader {
  loading = $state(true);
  error = $state('');
  private fallback: string;

  constructor(fallback: string) {
    this.fallback = fallback;
  }

  /** Runs the work; a success clears the error, a failure keeps its message (or the fallback). */
  async run(work: () => Promise<unknown>): Promise<boolean> {
    try {
      await work();
      this.error = '';
      return true;
    } catch (e) {
      this.error = e instanceof Error ? e.message : this.fallback;
      return false;
    } finally {
      this.loading = false;
    }
  }
}
