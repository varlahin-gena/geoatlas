import { useCallback, useEffect, useRef, useState } from 'react';
import { authHeaders, isAbortError } from '@/api/client';
import { fetchGeoRanges } from '@/api/geo';
import type { ToastKind } from '@/components/Toast';
import { fmtNumber } from '@/lib/format';

export function useMapUploads(opts: {
  isAdmin: boolean;
  toast: (msg: string, kind?: ToastKind) => void;
  fetchData: () => void | Promise<void>;
}) {
  const { isAdmin, toast, fetchData } = opts;
  const logFileRef = useRef<HTMLInputElement>(null);
  const geoFileRef = useRef<HTMLInputElement>(null);
  const [geoIndexCount, setGeoIndexCount] = useState(0);
  const [geoUploadMaxBytes, setGeoUploadMaxBytes] = useState(512 * 1024 * 1024);

  useEffect(() => {
    if (!isAdmin) return;
    const controller = new AbortController();
    void fetchGeoRanges({ limit: 1 }, { signal: controller.signal })
      .then((data) => {
        if (controller.signal.aborted) return;
        setGeoIndexCount(Number(data.count) || 0);
        const maxB = Number(data.limits?.upload_max_bytes);
        if (Number.isFinite(maxB) && maxB > 0) setGeoUploadMaxBytes(maxB);
      })
      .catch((e) => {
        if (isAbortError(e)) return;
        /* ignore — upload still works with defaults */
      });
    return () => controller.abort();
  }, [isAdmin]);

  const uploadFile = useCallback(
    async (kind: 'logs' | 'geo', file: File) => {
      try {
        if (kind === 'geo') {
          const overBytes = file.size > geoUploadMaxBytes;
          const largeIndex = geoIndexCount >= 400_000;
          const largeFile = file.size >= 32 * 1024 * 1024;
          if (overBytes) {
            toast(
              `Файл ${fmtNumber(file.size)} байт больше лимита ${fmtNumber(geoUploadMaxBytes)} (GEOIP_UPLOAD_MAX_BYTES)`,
              'error',
            );
            return;
          }
          if (largeIndex || largeFile) {
            const ok = window.confirm(
              [
                'Повторная загрузка большого GeoIP при уже заполненном индексе всё ещё может быть тяжёлой по RAM.',
                `Сейчас в базе ≈ ${fmtNumber(geoIndexCount)} диапазонов, файл ${(file.size / (1024 * 1024)).toFixed(1)} МиБ.`,
                'Backend теперь обрабатывает такой сценарий экономнее, но если данные уже есть в ClickHouse, перезаливать их не нужно. Продолжить?',
              ].join('\n'),
            );
            if (!ok) return;
          }
        }
        const res = await fetch(kind === 'logs' ? '/upload-logs' : '/upload-geo', {
          method: 'POST',
          credentials: 'same-origin',
          headers: authHeaders({
            'Content-Type': kind === 'logs' ? 'text/plain' : 'text/csv',
          }),
          body: file,
        });
        const ct = res.headers.get('content-type') || '';
        const data = ct.includes('application/json')
          ? await res.json().catch(() => ({}))
          : await res.text();
        if (!res.ok) {
          const msg =
            typeof data === 'object' && data && 'error' in data
              ? String((data as { error: unknown }).error)
              : `HTTP ${res.status}`;
          throw new Error(msg);
        }
        toast(kind === 'logs' ? 'Логи загружены' : 'GeoIP загружен', 'success');
        if (kind === 'logs') void fetchData();
        if (kind === 'geo' && typeof data === 'object' && data && 'ranges' in data) {
          setGeoIndexCount(Number((data as { ranges: unknown }).ranges) || geoIndexCount);
        }
      } catch (e) {
        toast(e instanceof Error ? e.message : 'Ошибка загрузки', 'error');
      }
    },
    [geoUploadMaxBytes, geoIndexCount, toast, fetchData],
  );

  return {
    logFileRef,
    geoFileRef,
    geoIndexCount,
    geoUploadMaxBytes,
    uploadFile,
  };
}
