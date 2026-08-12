import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { renderHook } from '@testing-library/react';
import { usePolling } from './usePolling';

describe('usePolling', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('runs immediately and on interval', () => {
    const tick = vi.fn();
    renderHook(() => usePolling(tick, 1000, true));
    expect(tick).toHaveBeenCalledTimes(1);
    vi.advanceTimersByTime(1000);
    expect(tick).toHaveBeenCalledTimes(2);
    vi.advanceTimersByTime(2000);
    expect(tick).toHaveBeenCalledTimes(4);
  });

  it('does not run when disabled', () => {
    const tick = vi.fn();
    renderHook(() => usePolling(tick, 1000, false));
    expect(tick).not.toHaveBeenCalled();
    vi.advanceTimersByTime(5000);
    expect(tick).not.toHaveBeenCalled();
  });

  it('aborts previous signal when a new tick starts', () => {
    const signals: AbortSignal[] = [];
    const tick = vi.fn((signal: AbortSignal) => {
      signals.push(signal);
    });
    const { unmount } = renderHook(() => usePolling(tick, 1000, true));
    expect(signals[0]?.aborted).toBe(false);
    vi.advanceTimersByTime(1000);
    expect(signals[0]?.aborted).toBe(true);
    expect(signals[1]?.aborted).toBe(false);
    unmount();
    expect(signals[1]?.aborted).toBe(true);
  });
});
