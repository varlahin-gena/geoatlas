import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import * as authApi from '@/api/auth';
import { authHeaders, isAbortError } from '@/api/client';
import { fetchGeoRanges } from '@/api/geo';
import type { AuthUser } from '@/api/types';
import type { ToastKind } from '@/components/Toast';
import { fmtNumber } from '@/lib/format';
import {
  buildGeoCurlSnippet,
  GEO_UPLOAD_LARGE_BYTES,
  readLocalDismissed,
  shouldShowGeoWizard,
  shouldSkipToGeoWizardDone,
  writeLocalDismissed,
  type DryRunPreview,
  type GeoStatus,
  type GeoWizardStep,
} from './geoWizard';

const POLL_MS = 2000;
const POLL_MAX = 90; // ~3 min

export function useGeoWizard(opts: {
  isAdmin: boolean;
  user: AuthUser | null;
  uiAuthEnabled: boolean;
  toast: (msg: string, kind?: ToastKind) => void;
  refreshUser: () => Promise<AuthUser | null>;
  onGeoReady?: () => void | Promise<void>;
}) {
  const { isAdmin, user, uiAuthEnabled, toast, refreshUser, onGeoReady } = opts;

  const [geo, setGeo] = useState<GeoStatus | null>(null);
  const [heldOpen, setHeldOpen] = useState(false);
  const [step, setStep] = useState<GeoWizardStep>('why');
  const [busy, setBusy] = useState(false);
  const [preview, setPreview] = useState<DryRunPreview | null>(null);
  const [pendingFile, setPendingFile] = useState<File | null>(null);
  const [pollNote, setPollNote] = useState('');
  const fileRef = useRef<HTMLInputElement>(null);
  const dismissedLocal = readLocalDismissed();

  const dismissed = Boolean(user?.geo_wizard_dismissed) || (!uiAuthEnabled && dismissedLocal);

  const curlSnippet = useMemo(
    () => buildGeoCurlSnippet(typeof window !== 'undefined' ? window.location.origin : 'http://127.0.0.1'),
    [],
  );

  const loadGeoStatus = useCallback(
    async (signal?: AbortSignal) => {
      if (!isAdmin) return null;
      try {
        const data = await fetchGeoRanges({ limit: 1 }, { signal });
        if (signal?.aborted) return null;
        const next: GeoStatus = {
          count: Number(data.count) || 0,
          indexReady: data.index_ready !== false,
          uploadMaxBytes: Number(data.limits?.upload_max_bytes) || 512 * 1024 * 1024,
          uploadMaxRanges: Number(data.limits?.upload_max_ranges) || 0,
        };
        setGeo(next);
        return next;
      } catch (e) {
        if (isAbortError(e)) return null;
        return null;
      }
    },
    [isAdmin],
  );

  useEffect(() => {
    if (!isAdmin) return;
    const controller = new AbortController();
    void loadGeoStatus(controller.signal);
    return () => controller.abort();
  }, [isAdmin, loadGeoStatus]);

  const autoShow = shouldShowGeoWizard({
    isAdmin,
    dismissed,
    geoCount: geo ? geo.count : null,
  });

  useEffect(() => {
    if (autoShow) setHeldOpen(true);
  }, [autoShow]);

  const open = useCallback(() => {
    setHeldOpen(true);
    setStep('why');
    setPreview(null);
    setPendingFile(null);
    setPollNote('');
  }, []);

  const persistDismiss = useCallback(
    async (value: boolean) => {
      writeLocalDismissed(value);
      if (!uiAuthEnabled) return;
      try {
        await authApi.setGeoWizardDismissed(value);
        await refreshUser();
      } catch (e) {
        toast(e instanceof Error ? e.message : 'Не удалось сохранить настройку', 'error');
      }
    },
    [uiAuthEnabled, refreshUser, toast],
  );

  const moreUpload = useCallback(() => {
    setStep('upload');
    setPreview(null);
    setPendingFile(null);
    setPollNote('');
  }, []);

  const dismiss = useCallback(async () => {
    setHeldOpen(false);
    await persistDismiss(true);
  }, [persistDismiss]);

  const closeAfterSuccess = useCallback(async () => {
    setHeldOpen(false);
    await persistDismiss(true);
    if (onGeoReady) await onGeoReady();
  }, [persistDismiss, onGeoReady]);

  const waitForGeo = useCallback(async () => {
    setPollNote('Ждём появления диапазонов в базе…');
    for (let i = 0; i < POLL_MAX; i++) {
      const st = await loadGeoStatus();
      if (st && st.count > 0 && st.indexReady) {
        setPollNote(`Готово: ${fmtNumber(st.count)} диапазонов`);
        setStep('done');
        return true;
      }
      if (st && st.count > 0 && !st.indexReady) {
        setPollNote(`База есть (${fmtNumber(st.count)}), индекс ещё загружается…`);
      }
      await new Promise((r) => setTimeout(r, POLL_MS));
    }
    setPollNote('Таймаут ожидания. Обновите карту или откройте «База GeoIP».');
    setStep('done');
    return false;
  }, [loadGeoStatus]);

  const runDryRun = useCallback(
    async (file: File) => {
      setBusy(true);
      setPreview(null);
      try {
        if (geo && geo.uploadMaxBytes > 0 && file.size > geo.uploadMaxBytes) {
          throw new Error(
            `Файл ${fmtNumber(file.size)} байт больше лимита ${fmtNumber(geo.uploadMaxBytes)}`,
          );
        }
        if (file.size >= GEO_UPLOAD_LARGE_BYTES) {
          toast(
            'Файл большой — надёжнее залить через curl с сервера (см. шаг «С сервера»). Dry-run в браузере может оборваться.',
            'warn',
          );
        }
        const res = await fetch('/upload-geo?dry_run=1', {
          method: 'POST',
          credentials: 'same-origin',
          headers: authHeaders({ 'Content-Type': 'text/csv' }),
          body: file,
        });
        const data = await res.json().catch(() => ({}));
        if (!res.ok) {
          const msg =
            typeof data === 'object' && data && 'error' in data
              ? String((data as { error: unknown }).error)
              : `HTTP ${res.status}`;
          throw new Error(msg);
        }
        const ranges = Number((data as { ranges?: unknown }).ranges) || 0;
        const sample = Array.isArray((data as { sample?: unknown }).sample)
          ? ((data as { sample: DryRunPreview['sample'] }).sample || [])
          : [];
        setPendingFile(file);
        setPreview({ ranges, sample });
        toast(`Проверка OK: ${fmtNumber(ranges)} диапазонов`, 'success');
      } catch (e) {
        toast(e instanceof Error ? e.message : 'Ошибка dry-run', 'error');
      } finally {
        setBusy(false);
      }
    },
    [geo, toast],
  );

  const commitUpload = useCallback(async () => {
    if (!pendingFile) {
      toast('Сначала выберите CSV и выполните проверку', 'error');
      return;
    }
    setBusy(true);
    try {
      const res = await fetch('/upload-geo', {
        method: 'POST',
        credentials: 'same-origin',
        headers: authHeaders({ 'Content-Type': 'text/csv' }),
        body: pendingFile,
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        const msg =
          typeof data === 'object' && data && 'error' in data
            ? String((data as { error: unknown }).error)
            : `HTTP ${res.status}`;
        throw new Error(msg);
      }
      const ranges = Number((data as { ranges?: unknown }).ranges) || 0;
      toast(`GeoIP загружен: ${fmtNumber(ranges)} диапазонов`, 'success');
      setStep('done');
      setBusy(false);
      await waitForGeo();
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка загрузки', 'error');
      setBusy(false);
    }
  }, [pendingFile, toast, waitForGeo]);

  const visible = heldOpen;

  // Curl/install filled geo while the intro is still open — skip to done.
  // Do not run on `upload`: that is "Ещё загрузка" / a second file.
  useEffect(() => {
    if (!visible || !geo || !shouldSkipToGeoWizardDone(step, geo)) return;
    setStep('done');
    setPollNote(`База уже на месте: ${fmtNumber(geo.count)} диапазонов`);
  }, [visible, geo, step]);

  return {
    visible,
    step,
    setStep,
    busy,
    geo,
    preview,
    pendingFile,
    pollNote,
    curlSnippet,
    fileRef,
    open,
    moreUpload,
    dismiss,
    closeAfterSuccess,
    runDryRun,
    commitUpload,
    waitForGeo,
    reloadStatus: loadGeoStatus,
    indexLoading: geo != null && !geo.indexReady && geo.count === 0,
  };
}
