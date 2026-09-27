// Times the loading indicator of the banner. A page whose lines to hide are
// known within the delay never shows it. Once it shows, it stays for the
// minimum, so it never flashes for a few milliseconds. Nielsen puts the
// limit for an uninterrupted flow of thought at one second and finds that
// shorter waits need no feedback
// (https://www.nngroup.com/articles/response-times-3-important-limits/);
// both times stay well under it.

/** The indicator shows once loading has lasted this many milliseconds. */
export const LOADING_DELAY_MS = 300;
/** Once shown, the indicator stays at least this many milliseconds. */
export const LOADING_MIN_MS = 500;

/**
 * LoadingDelay tracks one stretch of loading across the runs of a
 * controller. visible(true) starts the stretch and reports whether it has
 * lasted the delay. visible(false) ends it, but an indicator that shows
 * stays until it has shown for the minimum. Whenever what visible reports
 * changes without a call (the delay or the minimum passes), it calls
 * changed so the caller renders the banner again.
 */
export class LoadingDelay {
  readonly #delayMs: number;
  readonly #minMs: number;
  readonly #changed: () => void;
  #delay: ReturnType<typeof setTimeout> | null = null;
  #min: ReturnType<typeof setTimeout> | null = null;
  #shown = false;
  // Loading ended before the indicator had shown for the minimum.
  #ended = false;

  constructor(delayMs: number, minMs: number, changed: () => void) {
    this.#delayMs = delayMs;
    this.#minMs = minMs;
    this.#changed = changed;
  }

  visible(loading: boolean): boolean {
    if (loading) {
      this.#ended = false;
      if (!this.#shown && this.#delay === null) this.#delay = setTimeout(() => this.#show(), this.#delayMs);
      return this.#shown;
    }
    if (this.#min !== null) {
      this.#ended = true;
      return true;
    }
    this.stop();
    return false;
  }

  /** stop hides the indicator at once, ends the stretch, and cancels the timers. */
  stop(): void {
    if (this.#delay !== null) clearTimeout(this.#delay);
    if (this.#min !== null) clearTimeout(this.#min);
    this.#delay = null;
    this.#min = null;
    this.#shown = false;
    this.#ended = false;
  }

  #show(): void {
    this.#delay = null;
    this.#shown = true;
    this.#min = setTimeout(() => {
      this.#min = null;
      // The caller's next visible(false) ends the stretch.
      if (this.#ended) this.#changed();
    }, this.#minMs);
    this.#changed();
  }
}
