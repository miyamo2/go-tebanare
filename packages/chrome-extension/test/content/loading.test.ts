import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { LOADING_DELAY_MS, LOADING_MIN_MS, LoadingDelay } from '../../src/content/controller/loading.js';

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

describe('LoadingDelay', () => {
  it('shows after the delay and calls changed once', () => {
    const changed = vi.fn();
    const d = new LoadingDelay(LOADING_DELAY_MS, LOADING_MIN_MS, changed);
    expect(d.visible(true)).toBe(false);
    vi.advanceTimersByTime(LOADING_DELAY_MS - 1);
    expect(d.visible(true)).toBe(false);
    expect(changed).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(changed).toHaveBeenCalledTimes(1);
    expect(d.visible(true)).toBe(true);
    vi.advanceTimersByTime((LOADING_DELAY_MS + LOADING_MIN_MS) * 2);
    expect(changed).toHaveBeenCalledTimes(1);
    expect(d.visible(true)).toBe(true);
  });

  it('keeps the start of a stretch while loading goes on', () => {
    const d = new LoadingDelay(100, 100, () => {});
    d.visible(true);
    vi.advanceTimersByTime(60);
    d.visible(true);
    vi.advanceTimersByTime(40);
    expect(d.visible(true)).toBe(true);
  });

  it('never shows a stretch that ends within the delay', () => {
    const changed = vi.fn();
    const d = new LoadingDelay(100, 100, changed);
    d.visible(true);
    vi.advanceTimersByTime(60);
    expect(d.visible(false)).toBe(false);
    vi.advanceTimersByTime(300);
    expect(changed).not.toHaveBeenCalled();
  });

  it('keeps a shown indicator for the minimum, then asks for a render', () => {
    const changed = vi.fn();
    const d = new LoadingDelay(100, 100, changed);
    d.visible(true);
    vi.advanceTimersByTime(120);
    expect(changed).toHaveBeenCalledTimes(1);
    // Loading ended 20 ms after the indicator appeared.
    expect(d.visible(false)).toBe(true);
    vi.advanceTimersByTime(79);
    expect(d.visible(false)).toBe(true);
    expect(changed).toHaveBeenCalledTimes(1);
    vi.advanceTimersByTime(1);
    expect(changed).toHaveBeenCalledTimes(2);
    expect(d.visible(false)).toBe(false);
    // The next stretch waits the delay again.
    expect(d.visible(true)).toBe(false);
  });

  it('hides at once when loading ends after the minimum', () => {
    const changed = vi.fn();
    const d = new LoadingDelay(100, 100, changed);
    d.visible(true);
    vi.advanceTimersByTime(250);
    expect(d.visible(false)).toBe(false);
    expect(changed).toHaveBeenCalledTimes(1);
  });

  it('keeps showing when loading starts again within the minimum', () => {
    const changed = vi.fn();
    const d = new LoadingDelay(100, 100, changed);
    d.visible(true);
    vi.advanceTimersByTime(100);
    expect(d.visible(false)).toBe(true);
    vi.advanceTimersByTime(50);
    expect(d.visible(true)).toBe(true);
    vi.advanceTimersByTime(100);
    // The minimum passed with loading on, so nothing needs a render.
    expect(changed).toHaveBeenCalledTimes(1);
    expect(d.visible(true)).toBe(true);
  });

  it('cancels the timers on stop', () => {
    const changed = vi.fn();
    const d = new LoadingDelay(100, 100, changed);
    d.visible(true);
    d.stop();
    vi.advanceTimersByTime(300);
    expect(changed).not.toHaveBeenCalled();
    d.visible(true);
    vi.advanceTimersByTime(100);
    d.visible(false);
    d.stop();
    expect(d.visible(false)).toBe(false);
    vi.advanceTimersByTime(300);
    expect(changed).toHaveBeenCalledTimes(1);
  });
});
