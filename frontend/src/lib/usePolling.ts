import { useEffect, useRef } from 'react';

/**
 * Call `tick` immediately and on an interval while `enabled`.
 * Aborts in-flight work via AbortSignal when a new tick starts or on cleanup.
 */
export function usePolling(
  tick: (signal: AbortSignal) => void | Promise<void>,
  intervalMs: number,
  enabled = true,
): void {
  const tickRef = useRef(tick);
  tickRef.current = tick;

  useEffect(() => {
    if (!enabled) return;
    let inFlight: AbortController | null = null;

    const run = () => {
      inFlight?.abort();
      const controller = new AbortController();
      inFlight = controller;
      void Promise.resolve(tickRef.current(controller.signal)).catch(() => {
        /* tick handles errors */
      });
    };

    run();
    const id = window.setInterval(run, intervalMs);
    return () => {
      inFlight?.abort();
      window.clearInterval(id);
    };
  }, [enabled, intervalMs]);
}
