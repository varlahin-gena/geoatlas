import { useCallback, useEffect, useState } from 'react';
import { fetchRetention, fetchSystemStats } from '@/api/system';
import type { Retention, SystemStats } from '@/api/systemTypes';
import type { ToastKind } from '@/components/Toast';
import { usePolling } from '@/lib/usePolling';

const DEFAULT_RETENTION: Retention = {
  traffic_logs_days: 30,
  edges_days: 30,
  parse_errors_days: 7,
  system_metrics_days: 7,
};

export function useSystemStatsPolling(
  toast: (msg: string, kind?: ToastKind) => void,
  autoRefresh: boolean,
) {
  const [stats, setStats] = useState<SystemStats | null>(null);
  const [retention, setRetention] = useState<Retention>(DEFAULT_RETENTION);
  const [updatedAt, setUpdatedAt] = useState<Date | null>(null);

  const loadStats = useCallback(async () => {
    try {
      const data = await fetchSystemStats();
      setStats(data);
      setUpdatedAt(new Date());
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка stats', 'error');
    }
  }, [toast]);

  const loadRetention = useCallback(async () => {
    try {
      const data = await fetchRetention();
      if (data.retention) setRetention(data.retention);
    } catch {
      /* optional */
    }
  }, []);

  useEffect(() => {
    void loadRetention();
  }, [loadRetention]);

  useEffect(() => {
    if (!autoRefresh) void loadStats();
  }, [autoRefresh, loadStats]);

  usePolling(
    async () => {
      await loadStats();
    },
    5000,
    autoRefresh,
  );

  return {
    stats,
    retention,
    setRetention,
    updatedAt,
    loadStats,
    loadRetention,
  };
}
