import { describe, expect, it, vi } from 'vitest';
import { loadWithRetry } from './lazyRetry';

describe('loadWithRetry', () => {
  it('resolves after transient failures', async () => {
    const ok = { default: function OkPage() { return null; } };
    let calls = 0;
    const result = await loadWithRetry(
      async () => {
        calls += 1;
        if (calls < 3) throw new Error('network');
        return ok;
      },
      { retries: 2, retryDelayMs: 1, attemptTimeoutMs: 5_000, label: 'тест' },
    );
    expect(calls).toBe(3);
    expect(result).toBe(ok);
  });

  it('throws a recovery message after exhausting retries', async () => {
    await expect(
      loadWithRetry(
        async () => {
          throw new Error('boom');
        },
        { retries: 1, retryDelayMs: 1, label: 'пользователи' },
      ),
    ).rejects.toThrow(/Не удалось открыть «пользователи»/);
  });

  it('times out a hung import', async () => {
    vi.useFakeTimers();
    const pending = loadWithRetry(() => new Promise(() => {}), {
      retries: 0,
      attemptTimeoutMs: 40,
      label: 'зависшая',
    });
    const assertion = expect(pending).rejects.toThrow(/Не удалось открыть «зависшая»/);
    await vi.advanceTimersByTimeAsync(50);
    await assertion;
    vi.useRealTimers();
  });
});
