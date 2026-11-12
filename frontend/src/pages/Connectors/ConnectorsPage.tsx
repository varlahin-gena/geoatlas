import { FormEvent, useCallback, useEffect, useId, useRef, useState } from 'react';
import {
  applyConnectors,
  createConnector,
  deleteConnector,
  listConnectors,
  updateConnector,
  type Connector,
  type ConnectorProtocols,
} from '@/api/connectors';
import { fetchSystemStats } from '@/api/system';
import { SettingsLayout } from '@/components/SettingsLayout';
import { EmptyState, TableSkeleton } from '@/components/Skeleton';
import { useToast } from '@/components/Toast';
import {
  CONNECTOR_LIVE_WINDOW_MS,
  connectorLiveClass,
  connectorLiveKind,
  connectorLiveLabel,
  connectorLiveTitle,
  connectorProtocolsLabel,
} from './connectorDisplay';
import './connectors.css';

const LIVE_POLL_MS = 15_000;

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

function DeleteConnectorModal({
  connector,
  busy,
  onClose,
  onConfirm,
}: {
  connector: Connector;
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
        <h3 id={titleId}>Удалить коннектор?</h3>
        <p className="hint" style={{ marginBottom: 16 }}>
          «{connector.name}» ({connector.id}) будет удалён. Чтобы закрыть порт, нажмите «Применить».
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

function EditConnectorModal({
  connector,
  busy,
  portMin,
  portMax,
  onClose,
  onSave,
}: {
  connector: Connector;
  busy: boolean;
  portMin: number;
  portMax: number;
  onClose: () => void;
  onSave: (patch: {
    name: string;
    port: number;
    protocols: ConnectorProtocols;
  }) => void;
}) {
  const titleId = useId();
  const dialogRef = useDialogA11y<HTMLFormElement>(true, onClose);
  const [name, setName] = useState(connector.name);
  const [port, setPort] = useState(String(connector.port));
  const [protocols, setProtocols] = useState<ConnectorProtocols>(
    connector.protocols === 'udp' || connector.protocols === 'tcp' ? connector.protocols : 'both',
  );

  return (
    <div
      className="modal-backdrop show"
      role="presentation"
      onClick={(e) => {
        if (e.target === e.currentTarget && !busy) onClose();
      }}
    >
      <form
        ref={dialogRef}
        className="modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        onClick={(e) => e.stopPropagation()}
        onSubmit={(e) => {
          e.preventDefault();
          if (busy) return;
          onSave({
            name: name.trim(),
            port: Number(port),
            protocols,
          });
        }}
      >
        <h3 id={titleId}>Изменить коннектор</h3>
        <p className="hint" style={{ marginBottom: 16 }}>
          ID <code>{connector.id}</code> нельзя сменить. После сохранения нажмите «Применить».
        </p>
        <div className="field">
          <label htmlFor="connector-edit-name">Имя</label>
          <input
            id="connector-edit-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            maxLength={128}
            disabled={busy}
            autoFocus
          />
        </div>
        <div className="field">
          <label htmlFor="connector-edit-port">Порт</label>
          <input
            id="connector-edit-port"
            value={port}
            onChange={(e) => setPort(e.target.value)}
            type="number"
            min={connector.builtin ? 514 : portMin > 0 ? portMin : 1024}
            max={connector.builtin ? 514 : portMax > 0 ? portMax : 65535}
            required
            disabled={busy || !!connector.builtin}
            title={connector.builtin ? 'У встроенного коннектора порт всегда 514' : undefined}
          />
          {!connector.builtin && portMin > 0 ? (
            <span className="field-hint">
              {portMin}–{portMax}
            </span>
          ) : null}
        </div>
        <div className="field">
          <label htmlFor="connector-edit-protocols">Протоколы</label>
          <select
            id="connector-edit-protocols"
            value={protocols}
            onChange={(e) => setProtocols(e.target.value as ConnectorProtocols)}
            disabled={busy}
          >
            <option value="both">UDP+TCP</option>
            <option value="udp">UDP</option>
            <option value="tcp">TCP</option>
          </select>
        </div>
        <div className="modal-actions">
          <button type="button" className="btn" disabled={busy} onClick={onClose}>
            Отмена
          </button>
          <button type="submit" className="btn primary" disabled={busy}>
            {busy ? 'Сохранение…' : 'Сохранить'}
          </button>
        </div>
      </form>
    </div>
  );
}

export default function ConnectorsPage() {
  const { toast } = useToast();
  const [rows, setRows] = useState<Connector[]>([]);
  const [loading, setLoading] = useState(true);
  const [maxConnectors, setMaxConnectors] = useState(1);
  const [extraPorts, setExtraPorts] = useState({ min: 0, max: 0 });
  const [applyBusy, setApplyBusy] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [rowBusy, setRowBusy] = useState<string | null>(null);
  const [lastEventById, setLastEventById] = useState<Record<string, string>>({});
  const [pendingDelete, setPendingDelete] = useState<Connector | null>(null);
  const [deleteBusy, setDeleteBusy] = useState(false);
  const [pendingEdit, setPendingEdit] = useState<Connector | null>(null);
  const [editBusy, setEditBusy] = useState(false);
  const [id, setId] = useState('');
  const [name, setName] = useState('');
  const [port, setPort] = useState('');
  const [protocols, setProtocols] = useState<ConnectorProtocols>('both');

  const refreshLive = useCallback(async () => {
    try {
      const stats = await fetchSystemStats();
      const map: Record<string, string> = {};
      for (const c of stats.connectors || []) {
        if (c.id && c.last_event_at) map[c.id] = c.last_event_at;
      }
      setLastEventById(map);
    } catch {
      /* soft: keep previous live map */
    }
  }, []);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await listConnectors();
      setRows(data.connectors || []);
      setMaxConnectors(data.max_connectors || 1);
      setExtraPorts(data.extra_ports || { min: 0, max: 0 });
      await refreshLive();
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось загрузить коннекторы', 'error');
    } finally {
      setLoading(false);
    }
  }, [toast, refreshLive]);

  useEffect(() => {
    document.title = 'ГеоАтлас — Коннекторы';
    void load();
  }, [load]);

  useEffect(() => {
    const timer = window.setInterval(() => {
      void refreshLive();
    }, LIVE_POLL_MS);
    return () => window.clearInterval(timer);
  }, [refreshLive]);

  async function onCreate(e: FormEvent) {
    e.preventDefault();
    try {
      await createConnector({
        id: id.trim(),
        name: name.trim(),
        port: Number(port),
        protocols,
      });
      setId('');
      setName('');
      setPort('');
      setProtocols('both');
      toast('Коннектор добавлен — примените изменения', 'success');
      setDirty(true);
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Не удалось добавить коннектор', 'error');
    }
  }

  function setEnabledLocal(connectorId: string, enabled: boolean) {
    setRows((prev) => prev.map((row) => (row.id === connectorId ? { ...row, enabled } : row)));
  }

  async function onToggle(c: Connector) {
    const next = !c.enabled;
    setRowBusy(c.id);
    setEnabledLocal(c.id, next);
    try {
      await updateConnector(c.id, { enabled: next });
      setDirty(true);
      toast(next ? 'Включено — примените изменения' : 'Выключено — примените изменения', 'success');
    } catch (err) {
      setEnabledLocal(c.id, c.enabled);
      toast(err instanceof Error ? err.message : 'Не удалось изменить статус', 'error');
    } finally {
      setRowBusy(null);
    }
  }

  async function onRestart(c: Connector) {
    setRowBusy(c.id);
    try {
      setEnabledLocal(c.id, false);
      await updateConnector(c.id, { enabled: false });
      setEnabledLocal(c.id, true);
      await updateConnector(c.id, { enabled: true });
      setDirty(true);
      toast('Перезапуск записан — примените изменения', 'success');
      await refreshLive();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Не удалось перезапустить', 'error');
      await load();
    } finally {
      setRowBusy(null);
    }
  }

  async function confirmDelete() {
    if (!pendingDelete || deleteBusy) return;
    setDeleteBusy(true);
    setRowBusy(pendingDelete.id);
    try {
      await deleteConnector(pendingDelete.id);
      setDirty(true);
      toast('Удалено — примените изменения', 'success');
      setPendingDelete(null);
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Не удалось удалить коннектор', 'error');
    } finally {
      setDeleteBusy(false);
      setRowBusy(null);
    }
  }

  async function confirmEdit(patch: {
    name: string;
    port: number;
    protocols: ConnectorProtocols;
  }) {
    if (!pendingEdit || editBusy) return;
    setEditBusy(true);
    setRowBusy(pendingEdit.id);
    try {
      const body: Parameters<typeof updateConnector>[1] = {
        name: patch.name,
        protocols: patch.protocols,
      };
      if (!pendingEdit.builtin) {
        body.port = patch.port;
      }
      await updateConnector(pendingEdit.id, body);
      setDirty(true);
      toast('Сохранено — примените изменения', 'success');
      setPendingEdit(null);
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Не удалось изменить коннектор', 'error');
    } finally {
      setEditBusy(false);
      setRowBusy(null);
    }
  }

  async function onApply() {
    if (!dirty) return;
    setApplyBusy(true);
    try {
      const res = await applyConnectors();
      if (res.ok && res.apply?.reloaded) {
        toast('Конфиг применён, приём перезагружен', 'success');
        setDirty(false);
      } else if (res.apply?.reload_error) {
        toast(`Конфиг записан, перезагрузка: ${res.apply.reload_error}`, 'error');
        setDirty(false);
      } else {
        toast('Конфиг записан', 'success');
        setDirty(false);
      }
      await refreshLive();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Не удалось применить конфиг', 'error');
    } finally {
      setApplyBusy(false);
    }
  }

  const portRangeHint =
    extraPorts.min > 0
      ? `Доп. порты ${extraPorts.min}–${extraPorts.max} (514 — у встроенного). Лимит: ${maxConnectors}.`
      : `Сейчас доступен только порт 514 (встроенный).`;

  const applyTitle = dirty
    ? 'Записать конфиг приёма и перезагрузить.'
    : 'Нет неприменённых изменений';

  const applyButton = (
    <button
      type="button"
      className="btn primary"
      disabled={!dirty || applyBusy}
      title={applyTitle}
      onClick={() => void onApply()}
    >
      {applyBusy ? 'Применение…' : 'Применить'}
    </button>
  );

  return (
    <SettingsLayout title="Коннекторы" plainTitle>
      <div className="page-content-inner narrow">
        <p className="page-lead">
          Порты приёма логов. После правок нажмите «Применить», чтобы открыть или закрыть порт.
        </p>

        <p className="connectors-summary" role="status">
          <span>
            {rows.length}/{maxConnectors}{' '}
            {maxConnectors === 1 ? 'коннектор' : 'коннекторов'}
          </span>
          <span aria-hidden="true">·</span>
          <span className="hint">{portRangeHint}</span>
        </p>

        <details className="connectors-details">
          <summary>Как это устроено</summary>
          <p>
            Каждый коннектор — отдельный UDP/TCP порт. Метрики потока смотрите в Мониторинг →
            Pipeline. Статус «Жив» означает события за последние{' '}
            {Math.round(CONNECTOR_LIVE_WINDOW_MS / 60_000)} мин.
          </p>
          {extraPorts.min > 0 ? (
            <p>
              Лимит {maxConnectors} = встроенный 514 + порты{' '}
              <code>
                {extraPorts.min}–{extraPorts.max}
              </code>{' '}
              из <code>GA_CONNECTOR_EXTRA_PORTS</code> (compose публикует этот range заранее).
              После смены range/портов при необходимости обновите firewall (
              <code>scripts/sync-syslog-firewall.sh</code> или <code>GA_SYSLOG_ALLOW_FROM</code>).
            </p>
          ) : (
            <p>
              Чтобы добавить порты, задайте <code>GA_CONNECTOR_EXTRA_PORTS</code> и опубликуйте
              диапазон в compose; для firewall —{' '}
              <code>scripts/sync-syslog-firewall.sh</code> или <code>GA_SYSLOG_ALLOW_FROM</code>.
            </p>
          )}
        </details>

        {dirty ? (
          <div className="connectors-dirty" role="status">
            <span>Есть неприменённые изменения</span>
            {applyButton}
          </div>
        ) : (
          <div className="connectors-toolbar">{applyButton}</div>
        )}

        <div className="card">
          <div className="table-wrap">
            <table className="connectors-table">
              <thead>
                <tr>
                  <th scope="col">Имя</th>
                  <th scope="col">ID</th>
                  <th scope="col">Порт</th>
                  <th scope="col">Протоколы</th>
                  <th scope="col">Статус</th>
                  <th scope="col">
                    <span className="visually-hidden">Действия</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {loading ? (
                  <TableSkeleton rows={4} cols={6} />
                ) : rows.length === 0 ? (
                  <tr>
                    <td colSpan={6} className="empty">
                      <EmptyState
                        compact
                        title="Нет коннекторов"
                        description="Добавьте порт приёма в форме ниже."
                      />
                    </td>
                  </tr>
                ) : (
                  rows.map((c) => {
                    const busy = rowBusy === c.id;
                    const kind = connectorLiveKind(c.enabled, lastEventById[c.id]);
                    const liveLabel = connectorLiveLabel(kind);
                    return (
                      <tr key={c.id}>
                        <td>
                          <div className="conn-name">
                            <span title={c.name}>{c.name}</span>
                            {c.builtin ? <span className="connectors-builtin">Встроенный</span> : null}
                          </div>
                        </td>
                        <td className="mono" title={c.id}>
                          {c.id}
                        </td>
                        <td className="mono">{c.port}</td>
                        <td>{connectorProtocolsLabel(c.protocols)}</td>
                        <td>
                          <button
                            type="button"
                            className={`btn sm ${connectorLiveClass(kind)}`}
                            disabled={busy}
                            aria-pressed={c.enabled}
                            aria-label={`${c.name}: ${liveLabel}. ${connectorLiveTitle(kind)}`}
                            title={connectorLiveTitle(kind)}
                            onClick={() => void onToggle(c)}
                          >
                            {liveLabel}
                          </button>
                        </td>
                        <td className="actions">
                          <div className="connectors-row-actions">
                            <button
                              type="button"
                              className="btn sm ghost"
                              disabled={busy}
                              title="Имя, порт и протоколы"
                              onClick={() => setPendingEdit(c)}
                            >
                              Изменить
                            </button>
                            <button
                              type="button"
                              className="btn sm ghost"
                              disabled={busy}
                              title="Выкл → вкл в конфиге"
                              onClick={() => void onRestart(c)}
                            >
                              Перезапуск
                            </button>
                            {!c.builtin ? (
                              <button
                                type="button"
                                className="btn sm danger ghost"
                                disabled={busy}
                                onClick={() => setPendingDelete(c)}
                              >
                                Удалить
                              </button>
                            ) : null}
                          </div>
                        </td>
                      </tr>
                    );
                  })
                )}
              </tbody>
            </table>
          </div>
        </div>

        <section className="card connectors-form">
          <h2>Добавить коннектор</h2>
          <form className="form-row" onSubmit={(e) => void onCreate(e)}>
            <div className="field field-name">
              <label htmlFor="connector-name">Имя</label>
              <input
                id="connector-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="МСЭ офис"
                required
              />
            </div>
            <div className="field field-port">
              <label htmlFor="connector-port">Порт</label>
              <input
                id="connector-port"
                value={port}
                onChange={(e) => setPort(e.target.value)}
                type="number"
                min={extraPorts.min > 0 ? extraPorts.min : 1024}
                max={extraPorts.max > 0 ? extraPorts.max : 65535}
                required
              />
              {extraPorts.min > 0 ? (
                <span className="field-hint">
                  {extraPorts.min}–{extraPorts.max}
                </span>
              ) : null}
            </div>
            <div className="field field-protocols">
              <label htmlFor="connector-protocols">Протоколы</label>
              <select
                id="connector-protocols"
                value={protocols}
                onChange={(e) => setProtocols(e.target.value as ConnectorProtocols)}
              >
                <option value="both">UDP+TCP</option>
                <option value="udp">UDP</option>
                <option value="tcp">TCP</option>
              </select>
            </div>
            <div className="field field-id">
              <label htmlFor="connector-id">ID</label>
              <input
                id="connector-id"
                value={id}
                onChange={(e) => setId(e.target.value)}
                placeholder="fw-a"
                required
                pattern="[a-z][a-z0-9_-]{0,31}"
                title="латиница, цифры, _ -; с буквы; до 32 символов"
                autoComplete="off"
              />
            </div>
            <div className="field field-submit">
              <span className="field-submit-label" aria-hidden="true">
                &nbsp;
              </span>
              <button type="submit" className="btn" disabled={rows.length >= maxConnectors}>
                Добавить
              </button>
            </div>
          </form>
        </section>
      </div>

      {pendingEdit ? (
        <EditConnectorModal
          connector={pendingEdit}
          busy={editBusy}
          portMin={extraPorts.min}
          portMax={extraPorts.max}
          onClose={() => {
            if (!editBusy) setPendingEdit(null);
          }}
          onSave={(patch) => void confirmEdit(patch)}
        />
      ) : null}

      {pendingDelete ? (
        <DeleteConnectorModal
          connector={pendingDelete}
          busy={deleteBusy}
          onClose={() => {
            if (!deleteBusy) setPendingDelete(null);
          }}
          onConfirm={() => void confirmDelete()}
        />
      ) : null}
    </SettingsLayout>
  );
}
