import { useCallback, useEffect, useId, useRef, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import {
  deleteHunt,
  listMyHunts,
  runHunt,
  updateHunt,
  type SavedHunt,
} from '@/api/hunts';
import { huntMapHref } from '@/pages/Hunts/huntMapState';
import {
  huntCardSubtitle,
  huntCardTitle,
  huntLastRunLabel,
  huntQueryLabel,
  huntScheduleSummary,
} from '@/pages/Hunts/huntDisplay';
import { AdminLayout } from '@/components/AdminLayout';
import { EmptyState, ContentSkeleton } from '@/components/Skeleton';
import { useToast } from '@/components/Toast';
import { COPY } from '@/lib/glossary';
import { fmtDate, fmtNumber } from '@/lib/format';
import './hunts.css';

const DEFAULT_SCHEDULE = {
  enabled: false,
  interval_min: 60,
  edge_threshold: 0,
  edge_ratio: 3,
};

const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

function useDialogA11y<T extends HTMLElement>(open: boolean, onDismiss: () => void) {
  const dialogRef = useRef<T | null>(null);
  const onDismissRef = useRef(onDismiss);
  onDismissRef.current = onDismiss;

  useEffect(() => {
    if (!open) return;
    const root = dialogRef.current;
    if (!root) return;
    const panel = root;
    const prev = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const nodes = () => Array.from(panel.querySelectorAll<HTMLElement>(FOCUSABLE));
    const preferred = panel.querySelector<HTMLElement>('input, button');
    (preferred ?? nodes()[0] ?? panel).focus();

    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        e.preventDefault();
        onDismissRef.current();
        return;
      }
      if (e.key !== 'Tab') return;
      const list = nodes();
      if (list.length === 0) {
        e.preventDefault();
        panel.focus();
        return;
      }
      const first = list[0];
      const last = list[list.length - 1];
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    }
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('keydown', onKey);
      prev?.focus();
    };
  }, [open]);

  return dialogRef;
}

function HuntEditModal({
  hunt,
  busy,
  onClose,
  onSave,
}: {
  hunt: SavedHunt;
  busy: boolean;
  onClose: () => void;
  onSave: (next: SavedHunt) => void;
}) {
  const titleId = useId();
  const [draft, setDraft] = useState<SavedHunt>(() => ({
    ...hunt,
    schedule: { ...DEFAULT_SCHEDULE, ...hunt.schedule },
  }));
  const dialogRef = useDialogA11y<HTMLFormElement>(true, onClose);

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!draft.name.trim() || busy) return;
    onSave({
      ...draft,
      name: draft.name.trim(),
      schedule: { ...DEFAULT_SCHEDULE, ...draft.schedule },
    });
  }

  return (
    <div
      className="modal-backdrop show save-hunt-backdrop"
      role="presentation"
      onClick={(e) => {
        if (e.target === e.currentTarget && !busy) onClose();
      }}
    >
      <form
        ref={dialogRef}
        className="modal save-hunt-modal hunt-edit-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        onClick={(e) => e.stopPropagation()}
        onSubmit={onSubmit}
      >
        <h3 id={titleId}>Настройки охоты</h3>
        <p className="save-hunt-lead">Имя, заметки и порог расписания.</p>

        <div className="field">
          <label htmlFor="hunt-edit-name">Название</label>
          <input
            id="hunt-edit-name"
            type="text"
            autoComplete="off"
            required
            maxLength={120}
            value={draft.name}
            disabled={busy}
            onChange={(e) => setDraft({ ...draft, name: e.target.value })}
          />
        </div>

        <div className="field">
          <label htmlFor="hunt-edit-notes">Заметки</label>
          <input
            id="hunt-edit-notes"
            type="text"
            autoComplete="off"
            maxLength={240}
            value={draft.notes || ''}
            disabled={busy}
            onChange={(e) => setDraft({ ...draft, notes: e.target.value })}
          />
        </div>

        <label className="hunt-check">
          <input
            type="checkbox"
            checked={draft.schedule?.enabled ?? false}
            disabled={busy}
            onChange={(e) =>
              setDraft({
                ...draft,
                schedule: { ...DEFAULT_SCHEDULE, ...draft.schedule, enabled: e.target.checked },
              })
            }
          />
          Расписание включено
        </label>

        <div className="hunt-edit-row">
          <div className="field">
            <label htmlFor="hunt-edit-interval">Интервал (мин)</label>
            <input
              id="hunt-edit-interval"
              type="number"
              min={15}
              max={1440}
              disabled={busy}
              value={draft.schedule?.interval_min ?? 60}
              onChange={(e) =>
                setDraft({
                  ...draft,
                  schedule: {
                    ...DEFAULT_SCHEDULE,
                    ...draft.schedule,
                    interval_min: Number(e.target.value),
                  },
                })
              }
            />
          </div>
          <div className="field">
            <label htmlFor="hunt-edit-threshold">Порог рёбер</label>
            <input
              id="hunt-edit-threshold"
              type="number"
              min={0}
              disabled={busy}
              value={draft.schedule?.edge_threshold ?? 0}
              onChange={(e) =>
                setDraft({
                  ...draft,
                  schedule: {
                    ...DEFAULT_SCHEDULE,
                    ...draft.schedule,
                    edge_threshold: Number(e.target.value),
                  },
                })
              }
            />
          </div>
          <div className="field">
            <label htmlFor="hunt-edit-ratio">Рост к прошлому запуску (×)</label>
            <input
              id="hunt-edit-ratio"
              type="number"
              min={1}
              step={0.1}
              disabled={busy}
              value={draft.schedule?.edge_ratio ?? 3}
              onChange={(e) =>
                setDraft({
                  ...draft,
                  schedule: {
                    ...DEFAULT_SCHEDULE,
                    ...draft.schedule,
                    edge_ratio: Number(e.target.value),
                  },
                })
              }
            />
          </div>
        </div>

        <div className="modal-actions">
          <button type="submit" className="btn primary" disabled={busy || !draft.name.trim()}>
            {busy ? 'Сохранение…' : 'Сохранить'}
          </button>
          <button type="button" className="btn" disabled={busy} onClick={onClose}>
            Отмена
          </button>
        </div>
      </form>
    </div>
  );
}

function HuntDeleteModal({
  hunt,
  busy,
  onClose,
  onConfirm,
}: {
  hunt: SavedHunt;
  busy: boolean;
  onClose: () => void;
  onConfirm: () => void;
}) {
  const titleId = useId();
  const dialogRef = useDialogA11y<HTMLDivElement>(true, onClose);
  const title = huntCardTitle(hunt);

  return (
    <div
      className="modal-backdrop show save-hunt-backdrop"
      role="presentation"
      onClick={(e) => {
        if (e.target === e.currentTarget && !busy) onClose();
      }}
    >
      <div
        ref={dialogRef}
        className="modal save-hunt-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        onClick={(e) => e.stopPropagation()}
      >
        <h3 id={titleId}>{COPY.huntDeleteConfirm}</h3>
        <p className="save-hunt-lead">
          Охота «{title}» будет удалена. Это действие нельзя отменить.
        </p>
        <div className="modal-actions">
          <button type="button" className="btn" disabled={busy} onClick={onClose}>
            Отмена
          </button>
          <button type="button" className="btn danger" disabled={busy} onClick={onConfirm}>
            {busy ? 'Удаление…' : 'Удалить'}
          </button>
        </div>
      </div>
    </div>
  );
}

export default function HuntsPage() {
  const { toast } = useToast();
  const [rows, setRows] = useState<SavedHunt[]>([]);
  const [loading, setLoading] = useState(true);
  const [running, setRunning] = useState<string | null>(null);
  const [editing, setEditing] = useState<SavedHunt | null>(null);
  const [deleting, setDeleting] = useState<SavedHunt | null>(null);
  const [saving, setSaving] = useState(false);
  const [removing, setRemoving] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await listMyHunts();
      setRows(data.hunts || []);
    } catch (e) {
      toast(e instanceof Error ? e.message : COPY.huntsLoadFailed, 'error');
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    document.title = 'ГеоАтлас — Охоты';
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function onRun(id: string) {
    setRunning(id);
    try {
      const data = await runHunt(id);
      if (data.hunt) {
        setRows((prev) => prev.map((h) => (h.id === id ? data.hunt! : h)));
      }
      const run = data.run;
      if (run?.skipped) {
        toast(`Пропуск: ${run.skipped}`, 'warn');
      } else if (run?.breach) {
        toast(`Порог превышен: ${fmtNumber(run.edge_count)} рёбер`, 'warn');
      } else {
        toast(`Готово: ${fmtNumber(run?.edge_count ?? 0)} рёбер`, 'success');
      }
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка запуска', 'error');
    } finally {
      setRunning(null);
    }
  }

  async function onConfirmDelete() {
    if (!deleting || removing) return;
    setRemoving(true);
    try {
      await deleteHunt(deleting.id);
      setRows((prev) => prev.filter((h) => h.id !== deleting.id));
      setDeleting(null);
      toast('Удалено', 'success');
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка удаления', 'error');
    } finally {
      setRemoving(false);
    }
  }

  async function onSaveEdit(next: SavedHunt) {
    if (saving) return;
    setSaving(true);
    try {
      const data = await updateHunt(next.id, next);
      if (data.hunt) {
        setRows((prev) => prev.map((h) => (h.id === next.id ? data.hunt! : h)));
      }
      setEditing(null);
      toast('Сохранено', 'success');
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Ошибка сохранения', 'error');
    } finally {
      setSaving(false);
    }
  }

  return (
    <AdminLayout title="Охоты">
      <div className="page-content-inner wide">
        <p className="page-lead">
          Сохранённые виды карты. По расписанию или вручную — если порог превышен, алерт появится в{' '}
          <Link to="/anomalies">Аномалиях</Link>.
        </p>
        <p className="hint">
          Сохранить текущий вид — кнопка «Охота» на карте или{' '}
          <Link to="/">вернуться на карту</Link>.
        </p>

        {loading ? <ContentSkeleton label={COPY.huntsLoading} /> : null}

        {!loading && rows.length ? (
          <div className="table-wrap hunts-table-wrap">
            <table className="hunts-table">
              <thead>
                <tr>
                  <th scope="col">Охота</th>
                  <th scope="col">Запрос</th>
                  <th scope="col">Расписание</th>
                  <th scope="col">Последний запуск</th>
                  <th scope="col">
                    <span className="visually-hidden">Действия</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {rows.map((h) => {
                  const last = huntLastRunLabel(h);
                  const title = huntCardTitle(h);
                  const subtitle = huntCardSubtitle(h.map);
                  const lastRunText = !h.last_run_at
                    ? 'Ещё не запускали'
                    : (() => {
                        const run = h.runs?.length ? h.runs[h.runs.length - 1] : undefined;
                        const when = fmtDate(h.last_run_at);
                        if (!run) return when;
                        return `${when} · ${fmtNumber(run.edge_count ?? 0)} рёбер${
                          run.breach ? ' · порог превышен' : ''
                        }`;
                      })();
                  return (
                    <tr key={h.id}>
                      <td>
                        <div className="hunt-name-cell">
                          <strong className="hunt-title">{title}</strong>
                          <span className="hunt-subtitle">{subtitle}</span>
                          {h.schedule?.enabled ? (
                            <span className="hunt-badge">по расписанию</span>
                          ) : null}
                          {h.notes ? <span className="hint hunt-notes">{h.notes}</span> : null}
                        </div>
                      </td>
                      <td>{huntQueryLabel(h.map)}</td>
                      <td>{huntScheduleSummary(h)}</td>
                      <td title={last.title ? fmtDate(last.title) : undefined}>{lastRunText}</td>
                      <td className="actions">
                        <div className="hunt-actions">
                          <Link to={huntMapHref(h)} className="btn sm primary">
                            На карте
                          </Link>
                          <button
                            type="button"
                            className="btn sm"
                            disabled={running === h.id}
                            onClick={() => void onRun(h.id)}
                          >
                            {running === h.id ? '…' : 'Запустить'}
                          </button>
                          <details className="hunt-more">
                            <summary className="btn sm ghost">Ещё</summary>
                            <div className="hunt-more-menu" role="menu">
                              <button
                                type="button"
                                role="menuitem"
                                className="hunt-more-item"
                                onClick={(e) => {
                                  const root = e.currentTarget.closest(
                                    'details',
                                  ) as HTMLDetailsElement | null;
                                  if (root) root.open = false;
                                  setEditing({
                                    ...h,
                                    schedule: { ...DEFAULT_SCHEDULE, ...h.schedule },
                                  });
                                }}
                              >
                                Настройки
                              </button>
                              <button
                                type="button"
                                role="menuitem"
                                className="hunt-more-item danger"
                                onClick={(e) => {
                                  const root = e.currentTarget.closest(
                                    'details',
                                  ) as HTMLDetailsElement | null;
                                  if (root) root.open = false;
                                  setDeleting(h);
                                }}
                              >
                                Удалить
                              </button>
                            </div>
                          </details>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        ) : null}

        {!loading && !rows.length ? (
          <EmptyState
            title={COPY.huntsEmptyTitle}
            description={COPY.huntsEmptyDesc}
            action={
              <Link className="btn primary" to="/">
                Открыть карту
              </Link>
            }
          />
        ) : null}
      </div>

      {editing ? (
        <HuntEditModal
          hunt={editing}
          busy={saving}
          onClose={() => setEditing(null)}
          onSave={(next) => void onSaveEdit(next)}
        />
      ) : null}

      {deleting ? (
        <HuntDeleteModal
          hunt={deleting}
          busy={removing}
          onClose={() => setDeleting(null)}
          onConfirm={() => void onConfirmDelete()}
        />
      ) : null}
    </AdminLayout>
  );
}
