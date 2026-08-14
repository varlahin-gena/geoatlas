import { useEffect, useRef } from 'react';

export type UsePollingOptions = {
  /** Skip ticks while document.hidden (default true). */
  pauseWhenHidden?: boolean;
  /** Run `tick` once when polling starts (default true). Set false when a sibling effect already loads. */
  runImmediately?: boolean;
};

/**
 * Call `tick` immediately (unless `runImmediately: false`) and on an interval while `enabled`.
 * Aborts in-flight work via AbortSignal when a new tick starts or on cleanup.
 * By default pauses while the tab is hidden to cut wasted API/CH load.
 */
export function usePolling(
  tick: (signal: AbortSignal) => void | Promise<void>,
  intervalMs: number,
  enabled = true,
  options?: UsePollingOptions,
): void {
  const tickRef = useRef(tick);
  tickRef.current = tick;
  const pauseWhenHidden = options?.pauseWhenHidden !== false;
  const runImmediately = options?.runImmediately !== false;

  useEffect(() => {
    if (!enabled) return;
    let inFlight: AbortController | null = null;
    let id = 0;

    const run = () => {
      if (pauseWhenHidden && typeof document !== 'undefined' && document.hidden) return;
      inFlight?.abort();
      const controller = new AbortController();
      inFlight = controller;
      void Promise.resolve(tickRef.current(controller.signal)).catch(() => {
        /* tick handles errors */
      });
    };

    const onVisibility = () => {
      if (!pauseWhenHidden) return;
      if (!document.hidden) run();
    };

    if (runImmediately) run();
    id = window.setInterval(run, intervalMs);
    document.addEventListener('visibilitychange', onVisibility);
    return () => {
      inFlight?.abort();
      window.clearInterval(id);
      document.removeEventListener('visibilitychange', onVisibility);
    };
  }, [enabled, intervalMs, pauseWhenHidden, runImmediately]);
}
