import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { LOADING_DELAY_MS, LoadingDelay } from '../../src/content/controller/loading.js';

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

describe('LoadingDelay', () => {
  it('shows after the delay and calls elapsed once', () => {
    const elapsed = vi.fn();
    const d = new LoadingDelay(LOADING_DELAY_MS, elapsed);
    expect(d.visible(true)).toBe(false);
    vi.advanceTimersByTime(LOADING_DELAY_MS - 1);
    expect(d.visible(true)).toBe(false);
    expect(elapsed).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(elapsed).toHaveBeenCalledTimes(1);
    expect(d.visible(true)).toBe(true);
    vi.advanceTimersByTime(LOADING_DELAY_MS * 2);
    expect(elapsed).toHaveBeenCalledTimes(1);
  });

  it('keeps the start of a stretch while loading goes on', () => {
    const d = new LoadingDelay(100, () => {});
    d.visible(true);
    vi.advanceTimersByTime(60);
    d.visible(true);
    vi.advanceTimersByTime(40);
    expect(d.visible(true)).toBe(true);
  });

  it('ends the stretch when loading stops, and starts over', () => {
    const elapsed = vi.fn();
    const d = new LoadingDelay(100, elapsed);
    d.visible(true);
    vi.advanceTimersByTime(60);
    expect(d.visible(false)).toBe(false);
    vi.advanceTimersByTime(100);
    expect(elapsed).not.toHaveBeenCalled();
    d.visible(true);
    vi.advanceTimersByTime(100);
    expect(elapsed).toHaveBeenCalledTimes(1);
    expect(d.visible(false)).toBe(false);
    expect(d.visible(true)).toBe(false);
  });

  it('cancels the timer on stop', () => {
    const elapsed = vi.fn();
    const d = new LoadingDelay(100, elapsed);
    d.visible(true);
    d.stop();
    vi.advanceTimersByTime(200);
    expect(elapsed).not.toHaveBeenCalled();
  });
});
