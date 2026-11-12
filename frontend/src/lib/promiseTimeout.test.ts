import { describe, expect, it, vi } from 'vitest';
import { promiseTimeout } from './promiseTimeout';

describe('promiseTimeout', () => {
  it('resolves when the promise wins', async () => {
    await expect(promiseTimeout(Promise.resolve(42), 1000, 'late')).resolves.toBe(42);
  });

  it('rejects when time runs out', async () => {
    vi.useFakeTimers();
    const pending = promiseTimeout(new Promise<number>(() => {}), 50, 'timed out');
    const assertion = expect(pending).rejects.toThrow('timed out');
    await vi.advanceTimersByTimeAsync(50);
    await assertion;
    vi.useRealTimers();
  });
});
