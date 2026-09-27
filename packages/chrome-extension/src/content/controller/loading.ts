// Delays the loading indicator of the banner, so a page whose lines to
// hide are known quickly never shows it. Common UX guidance puts the delay
// at 200 to 500 milliseconds, and Nielsen's limits say that waits under a
// second need no feedback; the indicator appears without a user action, so
// it takes the long end.

/** The indicator shows once loading has lasted this many milliseconds. */
export const LOADING_DELAY_MS = 500;

/**
 * LoadingDelay tracks one stretch of loading across the runs of a
 * controller. visible(true) starts the stretch and reports whether it has
 * lasted the delay; visible(false) ends it at once. When the delay passes
 * during a stretch, it calls elapsed so the caller renders the banner again.
 */
export class LoadingDelay {
  readonly #delayMs: number;
  readonly #elapsed: () => void;
  #timer: ReturnType<typeof setTimeout> | null = null;
  #shown = false;

  constructor(delayMs: number, elapsed: () => void) {
    this.#delayMs = delayMs;
    this.#elapsed = elapsed;
  }

  visible(loading: boolean): boolean {
    if (!loading) {
      this.stop();
      return false;
    }
    if (!this.#shown && this.#timer === null) {
      this.#timer = setTimeout(() => {
        this.#timer = null;
        this.#shown = true;
        this.#elapsed();
      }, this.#delayMs);
    }
    return this.#shown;
  }

  /** stop ends the stretch and cancels a pending timer. */
  stop(): void {
    if (this.#timer !== null) clearTimeout(this.#timer);
    this.#timer = null;
    this.#shown = false;
  }
}
