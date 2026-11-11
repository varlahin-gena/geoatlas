import { useCallback, useEffect, useId, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  deleteAllParseErrors,
  deleteParseErrors,
  listParseErrors,
  type ParseErrorRow,
} from '@/api/parseErrors';
import { SettingsLayout } from '@/components/SettingsLayout';
import { EmptyState, TableSkeleton } from '@/components/Skeleton';
import { useToast } from '@/components/Toast';
import { fmtDate, fmtNumber } from '@/lib/format';
import {
  parseErrorReasonCounts,
  parseErrorReasonLabel,
  shortParseErrorRaw,
} from './parseErrorDisplay';
import './parseErrors.css';

type DeleteMode = { type: 'ids'; ids: string[] } | { type: 'all' };

function useDialogA11y<T extends HTMLElement>(open: boolean, onDismiss: () => void) {
  const ref = useRef<T | null>(null);
  useEffect(() => {
    if (!open) return;
    const node = ref.current;
    node?.focus();
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onDismiss();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open, onDismiss]);
  return ref;
}

function ConfirmDeleteModal({
  mode,
  busy,
  onClose,
  onConfirm,
}: {
  mode: DeleteMode;
  busy: boolean;
  onClose: () => void;
  onConfirm: () => void;
}) {
  const titleId = useId();
  const dialogRef = useDialogA11y<HTMLDivElement>(true, onClose);
  const isAll = mode.type === 'all';
  const count = mode.type === 'ids' ? mode.ids.length : 0;

  return (
    <div
      className="modal-backdrop show"
      role="presentation"
      onClick={(e) => {
        if (e.target === e.currentTarget && !busy) onClose();
      }}
    >
      <div
        ref={dialogRef}
        className="modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        onClick={(e) => e.stopPropagation()}
      >
        <h3 id={titleId}>{isAll ? 'Удалить все записи?' : 'Удалить выбранные?'}</h3>
        <p className="hint" style={{ marginBottom: 16 }}>
          {isAll
            ? 'Все нераспознанные строки будут удалены. Это нельзя отменить.'
            : `Будет удалено записей: ${fmtNumber(count)}. Это нельзя отменить.`}
        </p>
        <div className="modal-actions">
          <button type="button" className="btn" disabled={busy} onClick={onClose}>
            Отмена
          </button>
          <button type="button" className="btn danger" disabled={busy} onClick={onConfirm}>
            {busy ? 'Удаление…' : isAll ? 'Удалить все' : 'Удалить'}
          </button>
        </div>
      </div>
    </div>
  );
}

function RawCell({ raw }: { raw?: string }) {
  const [open, setOpen] = useState(false);
  const full = (raw || '').trim();
  const preview = shortParseErrorRaw(full);
  if (!full) return <span>—</span>;
  const needsToggle = full.replace(/\s+/g, ' ').trim().length > 72;

  return (
    <div className="pe-raw">
      {!open ? (
        <span className="pe-raw-preview" title={full}>
          {preview}
        </span>
      ) : (
        <pre className="pe-raw-full">{full}</pre>
      )}
      {needsToggle ? (
        <button
          type="button"
          className="pe-raw-toggle"
          aria-expanded={open}
          onClick={() => setOpen((v) => !v)}
        >
          {open ? 'Свернуть' : 'Показать полностью'}
        </button>
      ) : null}
    </div>
  );
}

function ReasonCell({ reason }: { reason?: string }) {
  const [open, setOpen] = useState(false);
  const label = parseErrorReasonLabel(reason);
  const full = (reason || '').trim();
  const showToggle = Boolean(full && full !== label);

  return (
    <div className="pe-reason">
      <span title={full || undefined}>{label}</span>
      {showToggle ? (
        <>
          {' '}
          <button
            type="button"
            className="pe-raw-toggle"
            aria-expanded={open}
            onClick={() => setOpen((v) => !v)}
          >
            {open ? 'Скрыть' : 'Подробнее'}
          </button>
          {open ? <span className="pe-reason-full">{full}</span> : null}
        </>
      ) : null}
    </div>
  );
}

export default function ParseErrorsPage() {
  const { toast } = useToast();
  const [rows, setRows] = useState<ParseErrorRow[]>([]);
  const [limit, setLimit] = useState('100');
  const [search, setSearch] = useState('');
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(true);
  const [deleteBusy, setDeleteBusy] = useState(false);
  const [pendingDelete, setPendingDelete] = useState<DeleteMode | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await listParseErrors({ limit, search });
      setRows(data.errors || []);
      setSelected(new Set());
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось загрузить ошибки парсинга', 'error');
    } finally {
      setLoading(false);
    }
  }, [limit, search, toast]);

  useEffect(() => {
    document.title = 'ГеоАтлас — Ошибки парсинга';
  }, []);

  useEffect(() => {
    const t = window.setTimeout(() => void load(), 300);
    return () => window.clearTimeout(t);
  }, [load]);

  const showVendor = useMemo(() => rows.some((r) => Boolean(r.vendor?.trim())), [rows]);
  const clusters = useMemo(() => parseErrorReasonCounts(rows).slice(0, 4), [rows]);
  const tableCols = 5 + (showVendor ? 1 : 0);

  async function deleteIds(ids: string[]) {
    if (!ids.length) return;
    await deleteParseErrors(ids);
    setRows((prev) => prev.filter((r) => !ids.includes(r.id)));
    setSelected(new Set());
    toast(`Удалено записей: ${fmtNumber(ids.length)}`, 'success');
  }

  async function confirmPendingDelete() {
    if (!pendingDelete || deleteBusy) return;
    setDeleteBusy(true);
    try {
      if (pendingDelete.type === 'all') {
        await deleteAllParseErrors();
        setRows([]);
        setSelected(new Set());
        toast('Таблица очищена', 'success');
      } else {
        await deleteIds(pendingDelete.ids);
      }
      setPendingDelete(null);
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось удалить', 'error');
    } finally {
      setDeleteBusy(false);
    }
  }

  function openInParserTest(raw: string) {
    try {
      localStorage.setItem('nm.parserTest.prefill', raw || '');
    } catch {
      /* ignore */
    }
    window.open('/parser-test', '_blank', 'noopener,noreferrer');
  }

  const allSelected = rows.length > 0 && selected.size === rows.length;

  return (
    <SettingsLayout title="Ошибки парсинга" plainTitle>
      <div className="page-content-inner">
        <p className="page-lead">
          Строки логов, которые не удалось разобрать. Хранятся 7 дней; лишнее можно удалить вручную.
        </p>

        <p className="parse-errors-summary" role="status">
          {rows.length ? (
            <>
              <span>{fmtNumber(rows.length)} на экране</span>
              {search.trim() ? <span className="hint">· фильтр «{search.trim()}»</span> : null}
            </>
          ) : loading ? (
            <span className="hint">Загрузка…</span>
          ) : (
            <span className="hint">Нет записей в выборке</span>
          )}
        </p>

        {clusters.length > 1 ? (
          <div className="parse-errors-clusters" aria-label="Частые причины">
            {clusters.map((c) => (
              <span key={c.label} className="parse-errors-chip">
                <strong>{fmtNumber(c.count)}</strong>
                {c.label}
              </span>
            ))}
          </div>
        ) : null}

        <div className="parse-errors-toolbar">
          <div className="field parse-errors-search">
            <label htmlFor="parse-errors-search">Поиск</label>
            <input
              id="parse-errors-search"
              type="search"
              placeholder="По строке или причине…"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>
          <div className="field parse-errors-limit">
            <label htmlFor="parse-errors-limit">Показать</label>
            <select
              id="parse-errors-limit"
              value={limit}
              onChange={(e) => setLimit(e.target.value)}
            >
              {[50, 100, 200, 500].map((n) => (
                <option key={n} value={String(n)}>
                  {n}
                </option>
              ))}
            </select>
          </div>
          <button type="button" className="btn" onClick={() => void load()} disabled={loading}>
            {loading ? 'Обновление…' : 'Обновить'}
          </button>
          <div className="parse-errors-actions">
            <button
              type="button"
              className="btn"
              disabled={!selected.size || deleteBusy}
              onClick={() => setPendingDelete({ type: 'ids', ids: [...selected] })}
            >
              Удалить выбранные{selected.size ? ` (${fmtNumber(selected.size)})` : ''}
            </button>
            <button
              type="button"
              className="btn danger"
              disabled={!rows.length || deleteBusy}
              onClick={() => setPendingDelete({ type: 'all' })}
            >
              Удалить все
            </button>
          </div>
        </div>

        <div className="table-wrap card">
          <table className="parse-errors-table">
            <thead>
              <tr>
                <th scope="col">
                  <input
                    type="checkbox"
                    checked={allSelected}
                    aria-label="Выбрать все"
                    onChange={(e) => {
                      setSelected(e.target.checked ? new Set(rows.map((r) => r.id)) : new Set());
                    }}
                  />
                </th>
                <th scope="col">Время</th>
                {showVendor ? <th scope="col">Вендор</th> : null}
                <th scope="col">Причина</th>
                <th scope="col">Строка</th>
                <th scope="col">
                  <span className="visually-hidden">Действия</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {loading && !rows.length ? (
                <TableSkeleton cols={tableCols} rows={5} />
              ) : !rows.length ? (
                <tr>
                  <td colSpan={tableCols} className="empty">
                    <EmptyState
                      compact
                      title="Нет нераспознанных строк"
                      description="Ошибки появятся здесь, если syslog не удалось разобрать. Можно проверить пресеты в тесте парсеров."
                      action={
                        <Link className="btn" to="/parser-test">
                          Тест парсеров
                        </Link>
                      }
                    />
                  </td>
                </tr>
              ) : (
                rows.map((r) => (
                  <tr key={r.id}>
                    <td>
                      <input
                        type="checkbox"
                        checked={selected.has(r.id)}
                        aria-label={`Выбрать запись ${fmtDate(r.timestamp)}`}
                        onChange={(e) => {
                          setSelected((prev) => {
                            const n = new Set(prev);
                            if (e.target.checked) n.add(r.id);
                            else n.delete(r.id);
                            return n;
                          });
                        }}
                      />
                    </td>
                    <td className="pe-time">{fmtDate(r.timestamp)}</td>
                    {showVendor ? <td>{r.vendor || '—'}</td> : null}
                    <td>
                      <ReasonCell reason={r.reason} />
                    </td>
                    <td>
                      <RawCell raw={r.raw} />
                    </td>
                    <td className="actions">
                      <button
                        type="button"
                        className="btn sm ghost"
                        title="Открыть строку в тесте парсеров"
                        onClick={() => openInParserTest(r.raw || '')}
                      >
                        В тест
                      </button>
                      <button
                        type="button"
                        className="btn sm danger ghost"
                        aria-label="Удалить запись"
                        title="Удалить запись"
                        disabled={deleteBusy}
                        onClick={() => setPendingDelete({ type: 'ids', ids: [r.id] })}
                      >
                        Удалить
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {pendingDelete ? (
        <ConfirmDeleteModal
          mode={pendingDelete}
          busy={deleteBusy}
          onClose={() => {
            if (!deleteBusy) setPendingDelete(null);
          }}
          onConfirm={() => void confirmPendingDelete()}
        />
      ) : null}
    </SettingsLayout>
  );
}
