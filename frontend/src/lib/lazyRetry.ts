import { lazy, type ComponentType, type LazyExoticComponent } from 'react';
import { promiseTimeout } from './promiseTimeout';

export type LazyFactory<T extends ComponentType<unknown>> = () => Promise<{ default: T }>;

const DEFAULT_RETRIES = 2;
const DEFAULT_RETRY_DELAY_MS = 400;
/** Per-attempt ceiling so a hung chunk fetch cannot leave Suspense forever. */
const DEFAULT_ATTEMPT_TIMEOUT_MS = 12_000;

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => {
    setTimeout(resolve, ms);
  });
}

export type LazyRetryOptions = {
  retries?: number;
  retryDelayMs?: number;
  attemptTimeoutMs?: number;
  /** Short label for error messages (route / page name). */
  label?: string;
};

/** Retry + timeout wrapper used by `lazyRetry` (exported for unit tests). */
export async function loadWithRetry<T extends ComponentType<unknown>>(
  factory: LazyFactory<T>,
  opts?: LazyRetryOptions,
): Promise<{ default: T }> {
  const retries = opts?.retries ?? DEFAULT_RETRIES;
  const retryDelayMs = opts?.retryDelayMs ?? DEFAULT_RETRY_DELAY_MS;
  const attemptTimeoutMs = opts?.attemptTimeoutMs ?? DEFAULT_ATTEMPT_TIMEOUT_MS;
  const label = opts?.label ?? 'страница';

  let lastError: unknown;
  for (let attempt = 0; attempt <= retries; attempt++) {
    try {
      return await promiseTimeout(
        factory(),
        attemptTimeoutMs,
        `Таймаут загрузки модуля (${label}, ${attemptTimeoutMs} мс)`,
      );
    } catch (err) {
      lastError = err;
      if (attempt < retries) {
        await delay(retryDelayMs * (attempt + 1));
      }
    }
  }
  const detail =
    lastError instanceof Error && lastError.message
      ? lastError.message
      : 'не удалось загрузить код страницы';
  throw new Error(`Не удалось открыть «${label}»: ${detail}. Обновите страницу.`);
}

/**
 * Like `React.lazy`, but retries transient chunk failures and rejects on timeout
 * so ErrorBoundary can show a recovery UI instead of an infinite Suspense fallback.
 */
export function lazyRetry<T extends ComponentType<unknown>>(
  factory: LazyFactory<T>,
  opts?: LazyRetryOptions,
): LazyExoticComponent<T> {
  return lazy(() => loadWithRetry(factory, opts));
}
