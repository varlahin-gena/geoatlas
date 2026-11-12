import { FormEvent, useCallback, useEffect, useId, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { apiFetchRaw } from '@/api/client';
import {
  createReputationFeed,
  deleteReputationList,
  listReputationCatalog,
  listReputationFeeds,
  listReputationLists,
  refreshReputation,
  type ReputationFeed as Feed,
  type ReputationList as RepList,
} from '@/api/reputation';
import { useAuth } from '@/auth/AuthContext';
import { AdminLayout } from '@/components/AdminLayout';
import { EmptyState, TableSkeleton } from '@/components/Skeleton';
import { useToast } from '@/components/Toast';
import { fmtDate, fmtNumber } from '@/lib/format';
import {
  REPUTATION_CATEGORY_OPTIONS,
  REPUTATION_FORMAT_OPTIONS,
  reputationCategoryLabel,
  reputationFormatLabel,
  shortReputationUrl,
} from './reputationDisplay';
import './reputation.css';

type ActiveRow = {
  name: string;
  category: string;
  format: string;
  url: string;
  count: number;
  source: string;
  updated_at?: string;
  last_error: string;
};

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

function UrlCell({ url }: { url: string }) {
  const short = shortReputationUrl(url);
  if (!url) return <span>—</span>;
  const safe = /^https?:\/\//i.test(url);
  if (safe) {
    return (
      <a href={url} target="_blank" rel="noreferrer" title={url}>
        {short}
      </a>
    );
  }
  return <span title={url}>{short}</span>;
}

function DeleteListModal({
  name,
  busy,
  onClose,
  onConfirm,
}: {
  name: string;
  busy: boolean;
  onClose: () => void;
  onConfirm: () => void;
}) {
  const titleId = useId();
  const dialogRef = useDialogA11y<HTMLDivElement>(true, onClose);

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
        <h3 id={titleId}>Удалить список?</h3>
        <p className="hint" style={{ marginBottom: 16 }}>
          «{name}» будет снят. URL-фид с тем же именем тоже удалится. Это нельзя отменить.
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

export default function ReputationPage() {
  const { toast } = useToast();
  const { reputationEnabled } = useAuth();
  const navigate = useNavigate();
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [feeds, setFeeds] = useState<Feed[]>([]);
  const [lists, setLists] = useState<RepList[]>([]);
  const [catalog, setCatalog] = useState<Feed[]>([]);
  const [loading, setLoading] = useState(true);
  const [name, setName] = useState('');
  const [url, setUrl] = useState('');
  const [category, setCategory] = useState('attacks');
  const [format, setFormat] = useState('netset');
  const [refreshBusy, setRefreshBusy] = useState(false);
  const [deleteBusy, setDeleteBusy] = useState(false);
  const [pendingDelete, setPendingDelete] = useState<string | null>(null);
  const [uploadName, setUploadName] = useState('');
  const [lastRefreshAt, setLastRefreshAt] = useState<string | null>(null);

  const activeNames = useMemo(
    () => new Set(feeds.map((f) => f.name).filter(Boolean)),
    [feeds],
  );

  const catalogAvailable = useMemo(
    () => catalog.filter((f) => f?.name && !activeNames.has(f.name)),
    [catalog, activeNames],
  );

  const activeRows = useMemo((): ActiveRow[] => {
    const byFeed: Record<string, Feed> = {};
    const byList: Record<string, RepList> = {};
    feeds.forEach((f) => {
      if (f?.name) byFeed[f.name] = f;
    });
    lists.forEach((l) => {
      if (l?.name) byList[l.name] = l;
    });
    return Object.keys({ ...byFeed, ...byList })
      .sort()
      .map((n) => {
        const f = byFeed[n];
        const l = byList[n];
        return {
          name: n,
          category: f?.category || l?.category || '',
          format: f ? f.format || 'netset' : '',
          url: f?.url || '',
          count: l?.count || 0,
          source: l?.source || (f ? 'url' : ''),
          updated_at: l?.updated_at,
          last_error: l?.last_error || '',
        };
      });
  }, [feeds, lists]);

  const totalRanges = useMemo(
    () => lists.reduce((s, x) => s + (x.count || 0), 0),
    [lists],
  );

  const errorCount = useMemo(
    () => activeRows.filter((r) => r.last_error).length,
    [activeRows],
  );

  const showFormatCol = useMemo(
    () => activeRows.some((r) => r.format && r.format !== 'netset'),
    [activeRows],
  );

  const showErrorCol = errorCount > 0;

  const load = useCallback(async () => {
    try {
      const [f, l, c] = await Promise.all([
        listReputationFeeds(),
        listReputationLists(),
        listReputationCatalog(),
      ]);
      setFeeds(f.feeds || []);
      setLists(l.lists || []);
      setCatalog(c.feeds || []);
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось загрузить списки репутации', 'error');
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    document.title = 'ГеоАтлас — Репутация IP';
    if (!reputationEnabled) {
      navigate('/', { replace: true });
      return;
    }
    void load();
  }, [load, reputationEnabled, navigate]);

  async function addFeedBody(body: {
    name: string;
    url: string;
    category: string;
    format: string;
  }) {
    await createReputationFeed(body);
  }

  async function onAddFeed(e: FormEvent) {
    e.preventDefault();
    try {
      await addFeedBody({
        name: name.trim(),
        url: url.trim(),
        category: category.trim() || 'unknown',
        format: format || 'netset',
      });
      toast(`Фид «${name.trim()}» добавлен. Нажмите «Скачать фиды сейчас».`, 'success');
      setName('');
      setUrl('');
      void load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Не удалось добавить фид', 'error');
    }
  }

  async function refreshFeeds() {
    if (refreshBusy) return;
    setRefreshBusy(true);
    toast('Скачивание фидов… это может занять несколько минут', 'info');
    try {
      const data = await refreshReputation(true);
      const updated = Array.isArray(data.updated) ? data.updated : [];
      const skipped = Array.isArray(data.skipped) ? data.skipped : [];
      const failNames = Array.isArray(data.failed) ? data.failed : [];
      const errMap = data.errors && typeof data.errors === 'object' ? data.errors : {};
      const counts = data.counts && typeof data.counts === 'object' ? data.counts : {};

      const lines: string[] = [];
      if (updated.length) {
        let total = 0;
        const perFeed = updated.map((n) => {
          if (String(n).startsWith('removed:')) {
            return `${String(n).slice('removed:'.length)} (удалён устаревший)`;
          }
          const c = Number(counts[n]) || 0;
          total += c;
          return `${n} — ${fmtNumber(c)}`;
        });
        lines.push(`Обновлено ${updated.length} (диапазонов: ${fmtNumber(total)})`);
        lines.push(perFeed.join('\n'));
      } else {
        lines.push('Обновлено: 0');
      }
      if (skipped.length) {
        lines.push(`Без изменений: ${skipped.length} (${skipped.join(', ')})`);
      }
      if (failNames.length) {
        lines.push(`Ошибок: ${failNames.length}`);
        lines.push(
          failNames
            .map((n) => `${n} — ${errMap[n] ? String(errMap[n]) : 'неизвестная ошибка'}`)
            .join('\n'),
        );
      }
      setLastRefreshAt(new Date().toISOString());
      await load();
      toast(lines.join('\n'), failNames.length ? 'error' : 'success');
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось скачать фиды', 'error');
    } finally {
      setRefreshBusy(false);
    }
  }

  async function confirmDelete() {
    if (!pendingDelete || deleteBusy) return;
    setDeleteBusy(true);
    try {
      await deleteReputationList(pendingDelete);
      toast(`Удалено: ${pendingDelete}`, 'success');
      setPendingDelete(null);
      void load();
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось удалить список', 'error');
    } finally {
      setDeleteBusy(false);
    }
  }

  async function onUploadFile(file: File) {
    setUploadName(file.name);
    try {
      const res = await apiFetchRaw('/upload-reputation', {
        method: 'POST',
        headers: { 'Content-Type': 'text/csv' },
        body: await file.text(),
      });
      const data = (await res.json().catch(() => ({}))) as {
        error?: string;
        ranges?: number;
      };
      if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
      toast(`Загружено диапазонов: ${fmtNumber(data.ranges || 0)}`, 'success');
      void load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Не удалось загрузить файл', 'error');
    } finally {
      setUploadName('');
    }
  }

  const activeColSpan = 5 + (showFormatCol ? 1 : 0) + (showErrorCol ? 1 : 0);

  return (
    <AdminLayout
      title="Репутация IP"
      actions={
        <>
          <button type="button" className="btn" onClick={() => void load()} disabled={loading}>
            Обновить таблицу
          </button>
          <button
            type="button"
            className="btn primary"
            disabled={refreshBusy}
            onClick={() => void refreshFeeds()}
          >
            {refreshBusy ? 'Скачивание…' : 'Скачать фиды сейчас'}
          </button>
        </>
      }
    >
      <div className="page-content-inner">
        <p className="page-lead">
          Подключите списки вредоносных IP — они обновляются по расписанию и участвуют в проверке
          репутации на карте и в аномалиях.
        </p>

        <p className="reputation-summary" role="status">
          {activeRows.length ? (
            <>
              <span>
                {fmtNumber(activeRows.length)}{' '}
                {activeRows.length === 1 ? 'список' : activeRows.length < 5 ? 'списка' : 'списков'}
              </span>
              <span>· {fmtNumber(totalRanges)} диапазонов</span>
              {errorCount ? (
                <span className="hint" style={{ color: 'var(--red)' }}>
                  · с ошибкой: {errorCount}
                </span>
              ) : null}
              {lastRefreshAt ? (
                <span className="hint">· скачано {fmtDate(lastRefreshAt)}</span>
              ) : null}
            </>
          ) : (
            <span className="hint">
              Списки ещё не загружены — добавьте фид, выберите пресет или загрузите CSV.
            </span>
          )}
        </p>

        <details className="reputation-details">
          <summary>Как это устроено</summary>
          <p>
            URL-фиды хранятся на сервере и подтягиваются по расписанию или кнопкой «Скачать фиды
            сейчас». После добавления фида списки нужно скачать, иначе диапазоны появятся не сразу.
          </p>
          <p>
            Форматы: <code>netset</code> / plain, <code>spamhaus_json</code>, <code>csv_ip</code>.
          </p>
        </details>

        <div className="card reputation-card reputation-form">
          <h2>Добавить фид</h2>
          <form className="form-row" onSubmit={(e) => void onAddFeed(e)}>
            <div className="field">
              <label htmlFor="rep-feed-name">Имя</label>
              <input
                id="rep-feed-name"
                required
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="my_blocklist"
                autoComplete="off"
              />
            </div>
            <div className="field field-url">
              <label htmlFor="rep-feed-url">URL</label>
              <input
                id="rep-feed-url"
                required
                type="url"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                placeholder="https://…"
                autoComplete="off"
              />
            </div>
            <div className="field">
              <label htmlFor="rep-feed-cat">Категория</label>
              <select
                id="rep-feed-cat"
                value={category}
                onChange={(e) => setCategory(e.target.value)}
              >
                {REPUTATION_CATEGORY_OPTIONS.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </select>
            </div>
            <div className="field">
              <label htmlFor="rep-feed-fmt">Формат</label>
              <select
                id="rep-feed-fmt"
                value={format}
                onChange={(e) => setFormat(e.target.value)}
              >
                {REPUTATION_FORMAT_OPTIONS.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </select>
            </div>
            <button type="submit" className="btn primary">
              Добавить
            </button>
          </form>

          <div className="reputation-upload">
            <span className="reputation-upload-label">Или загрузите CSV вручную</span>
            <input
              ref={fileInputRef}
              type="file"
              accept=".csv,text/csv,text/plain"
              hidden
              onChange={(e) => {
                const file = e.target.files?.[0];
                e.target.value = '';
                if (file) void onUploadFile(file);
              }}
            />
            <button
              type="button"
              className="btn"
              onClick={() => fileInputRef.current?.click()}
            >
              Выбрать CSV…
            </button>
            {uploadName ? <span className="reputation-upload-name">{uploadName}</span> : null}
          </div>
        </div>

        <div className="card reputation-card">
          <h2>Активные списки</h2>
          <div className="table-wrap">
            <table className="reputation-table">
              <thead>
                <tr>
                  <th scope="col">Имя</th>
                  <th scope="col">Категория</th>
                  {showFormatCol ? <th scope="col">Формат</th> : null}
                  <th scope="col">Источник</th>
                  <th scope="col" className="num">
                    Диапазонов
                  </th>
                  <th scope="col">Обновлён</th>
                  {showErrorCol ? <th scope="col">Ошибка</th> : null}
                  <th scope="col">
                    <span className="visually-hidden">Действия</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {loading ? (
                  <TableSkeleton cols={activeColSpan} rows={4} />
                ) : !activeRows.length ? (
                  <tr>
                    <td colSpan={activeColSpan} className="empty">
                      <EmptyState
                        compact
                        title="Нет активных списков"
                        description="Добавьте URL-фид выше, выберите пресет из каталога или загрузите CSV."
                      />
                    </td>
                  </tr>
                ) : (
                  activeRows.map((r) => (
                    <tr key={r.name}>
                      <td className="rep-name">{r.name}</td>
                      <td>{reputationCategoryLabel(r.category)}</td>
                      {showFormatCol ? <td>{reputationFormatLabel(r.format)}</td> : null}
                      <td className="rep-url">
                        <UrlCell url={r.url} />
                      </td>
                      <td className="num">{fmtNumber(r.count)}</td>
                      <td>{fmtDate(r.updated_at)}</td>
                      {showErrorCol ? (
                        <td className="rep-error" title={r.last_error || undefined}>
                          {r.last_error || '—'}
                        </td>
                      ) : null}
                      <td className="actions">
                        <button
                          type="button"
                          className="btn sm danger ghost"
                          onClick={() => setPendingDelete(r.name)}
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

        <div className="card reputation-card">
          <h2>Каталог</h2>
          <p className="hint">Пресеты, которых ещё нет среди активных списков.</p>
          <div className="table-wrap">
            <table className="reputation-table">
              <thead>
                <tr>
                  <th scope="col">Имя</th>
                  <th scope="col">Категория</th>
                  <th scope="col">Формат</th>
                  <th scope="col">Источник</th>
                  <th scope="col">
                    <span className="visually-hidden">Действия</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {loading ? (
                  <TableSkeleton cols={5} rows={4} />
                ) : !catalog.length ? (
                  <tr>
                    <td colSpan={5} className="empty">
                      <EmptyState
                        compact
                        title="Каталог пуст"
                        description="Пресеты фидов недоступны на этом сервере."
                      />
                    </td>
                  </tr>
                ) : !catalogAvailable.length ? (
                  <tr>
                    <td colSpan={5} className="empty">
                      <EmptyState
                        compact
                        title="Все пресеты уже подключены"
                        description="Активные фиды покрывают весь каталог — остаётся скачать списки или добавить свой URL."
                      />
                    </td>
                  </tr>
                ) : (
                  catalogAvailable.map((c) => (
                    <tr key={c.name}>
                      <td className="rep-name">{c.name}</td>
                      <td>{reputationCategoryLabel(c.category)}</td>
                      <td>{reputationFormatLabel(c.format || 'netset')}</td>
                      <td className="rep-url">
                        <UrlCell url={c.url || ''} />
                      </td>
                      <td className="actions">
                        <button
                          type="button"
                          className="btn sm"
                          onClick={async () => {
                            try {
                              await addFeedBody({
                                name: c.name,
                                url: c.url || '',
                                category: c.category || 'unknown',
                                format: c.format || 'netset',
                              });
                              toast(
                                `«${c.name}» из каталога. Нажмите «Скачать фиды сейчас».`,
                                'success',
                              );
                              void load();
                            } catch (err) {
                              toast(
                                err instanceof Error ? err.message : 'Не удалось добавить пресет',
                                'error',
                              );
                            }
                          }}
                        >
                          Добавить
                        </button>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>
      </div>

      {pendingDelete ? (
        <DeleteListModal
          name={pendingDelete}
          busy={deleteBusy}
          onClose={() => {
            if (!deleteBusy) setPendingDelete(null);
          }}
          onConfirm={() => void confirmDelete()}
        />
      ) : null}
    </AdminLayout>
  );
}
