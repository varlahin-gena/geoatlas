import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react';

export type ToastKind = 'info' | 'success' | 'error' | 'warn' | string;

export interface ToastEntry {
  id: string;
  msg: string;
  kind: string;
  at: number;
}

const TOAST_STORE_KEY = 'nm.toasts.v1';
const TOAST_STORE_MAX = 30;
const TOAST_MSG_MAX = 2000;

function sanitize(entry: Partial<ToastEntry> | null | undefined): ToastEntry | null {
  if (!entry || typeof entry !== 'object') return null;
  const id = entry.id != null ? String(entry.id) : '';
  if (!/^[A-Za-z0-9_-]{1,64}$/.test(id)) return null;
  let kind = entry.kind != null ? String(entry.kind) : '';
  if (kind && !/^[A-Za-z0-9_-]{1,32}$/.test(kind)) kind = '';
  let msg = entry.msg == null ? '' : String(entry.msg);
  if (msg.length > TOAST_MSG_MAX) msg = msg.slice(0, TOAST_MSG_MAX) + '…';
  return { id, msg, kind, at: Number(entry.at) || 0 };
}

function readStore(): ToastEntry[] {
  try {
    const raw = sessionStorage.getItem(TOAST_STORE_KEY);
    if (!raw) return [];
    const arr = JSON.parse(raw);
    if (!Array.isArray(arr)) return [];
    return arr.map(sanitize).filter((e): e is ToastEntry => !!e);
  } catch {
    return [];
  }
}

function writeStore(items: ToastEntry[]) {
  try {
    if (!items.length) {
      sessionStorage.removeItem(TOAST_STORE_KEY);
      return;
    }
    sessionStorage.setItem(
      TOAST_STORE_KEY,
      JSON.stringify(items.slice(-TOAST_STORE_MAX)),
    );
  } catch {
    /* ignore */
  }
}

interface ToastContextValue {
  toasts: ToastEntry[];
  toast: (msg: string, kind?: ToastKind) => void;
  dismiss: (id: string) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastEntry[]>(() => readStore());

  useEffect(() => {
    writeStore(toasts);
  }, [toasts]);

  const toast = useCallback((msg: string, kind: ToastKind = '') => {
    const entry = sanitize({
      id: `t${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`,
      msg,
      kind: kind ? String(kind) : '',
      at: Date.now(),
    });
    if (!entry) return;
    setToasts((prev) => [...prev, entry]);
  }, []);

  const dismiss = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const value = useMemo(() => ({ toasts, toast, dismiss }), [toasts, toast, dismiss]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div id="toastHost" className="toast-host" aria-live="polite">
        {toasts.map((t) => (
          <div
            key={t.id}
            className={`toast${t.kind ? ` ${t.kind}` : ''}`}
            data-toast-id={t.id}
            role={t.kind === 'error' ? 'alert' : 'status'}
          >
            <button
              type="button"
              className="toast-close"
              aria-label="Закрыть"
              title="Закрыть"
              onClick={() => dismiss(t.id)}
            >
              ×
            </button>
            <div className="toast-body">{t.msg}</div>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error('useToast must be used within ToastProvider');
  return ctx;
}
