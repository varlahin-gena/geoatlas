import { FormEvent, useCallback, useEffect, useState } from 'react';
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

/** Fresh last_event_at window for green «жив» status. */
const LIVE_WINDOW_MS = 5 * 60 * 1000;
const LIVE_POLL_MS = 15_000;

type LiveKind = 'on' | 'idle' | 'off';

function connectorLiveKind(enabled: boolean, lastEventAt: string | undefined, now = Date.now()): LiveKind {
  if (!enabled) return 'off';
  if (!lastEventAt) return 'idle';
  const t = Date.parse(lastEventAt);
  if (!Number.isFinite(t)) return 'idle';
  return now - t <= LIVE_WINDOW_MS ? 'on' : 'idle';
}

function liveButtonClass(kind: LiveKind): string {
  if (kind === 'on') return 'status-on';
  if (kind === 'idle') return 'status-idle';
  return 'status-off';
}

function liveButtonTitle(kind: LiveKind, enabled: boolean): string {
  if (kind === 'on') {
    return 'Жив: enabled и событие за последние 5 мин. Клик — выключить.';
  }
  if (kind === 'idle') {
    return enabled
      ? 'Включён в конфиге, но нет свежего трафика (5 мин). Клик — выключить.'
      : 'Клик — включить.';
  }
  return 'Выключен в конфиге. Клик — включить.';
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
      toast(e instanceof Error ? e.message : 'Ошибка', 'error');
    } finally {
      setLoading(false);
    }
  }, [toast, refreshLive]);

  useEffect(() => {
    document.title = 'ГеоАтлас — Коннекторы';
    void load();
  }, [load]);

  useEffect(() => {
    const id = window.setInterval(() => {
      void refreshLive();
    }, LIVE_POLL_MS);
    return () => window.clearInterval(id);
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
      toast('Коннектор добавлен. Нажмите «Применить», чтобы открыть порт в syslog-ng.', 'success');
      setDirty(true);
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Ошибка', 'error');
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
      toast(
        next
          ? 'Включено. Нажмите «Применить», чтобы открыть порт.'
          : 'Выключено. Нажмите «Применить», чтобы закрыть порт.',
        'success',
      );
    } catch (err) {
      setEnabledLocal(c.id, c.enabled);
      toast(err instanceof Error ? err.message : 'Ошибка', 'error');
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
      toast('Перезапуск: выкл → вкл. Нажмите «Применить», чтобы обновить syslog-ng.', 'success');
      await refreshLive();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Ошибка перезапуска', 'error');
      await load();
    } finally {
      setRowBusy(null);
    }
  }

  async function onDelete(c: Connector) {
    if (c.builtin) return;
    if (!window.confirm(`Удалить коннектор «${c.name}» (${c.id})?`)) return;
    setRowBusy(c.id);
    try {
      await deleteConnector(c.id);
      setDirty(true);
      toast('Удалено. Примените конфиг.', 'success');
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Ошибка', 'error');
    } finally {
      setRowBusy(null);
    }
  }

  async function onApply() {
    if (!dirty) return;
    setApplyBusy(true);
    try {
      const res = await applyConnectors();
      if (res.ok && res.apply?.reloaded) {
        toast('Конфиг syslog-ng применён и перезагружен', 'success');
        setDirty(false);
      } else if (res.apply?.reload_error) {
        toast(`Конфиг записан, reload: ${res.apply.reload_error}`, 'error');
        setDirty(false);
      } else {
        toast('Конфиг записан', 'success');
        setDirty(false);
      }
      await refreshLive();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Ошибка apply', 'error');
    } finally {
      setApplyBusy(false);
    }
  }

  const portHint =
    extraPorts.min > 0
      ? `Доп. порты: ${extraPorts.min}–${extraPorts.max} (плюс 514 для default). Лимит: ${maxConnectors}.`
      : `Только порт 514 (default). Задайте GA_CONNECTOR_EXTRA_PORTS и опубликуйте range в compose.`;
  const applyTitle = dirty
    ? 'Записывает syslog-ng и делает reload. После новых портов: scripts/sync-syslog-firewall.sh или GA_SYSLOG_ALLOW_FROM.'
    : 'Нет неприменённых изменений';

  return (
    <SettingsLayout title="Коннекторы">
      <div className="page-content-inner narrow">
        <p className="page-lead">
          Отдельные UDP/TCP порты приёма. После изменений нажмите «Применить»; метрики — Мониторинг →
          Pipeline.
        </p>
        <p className="hint">{portHint}</p>

        <div className="toolbar">
          <button
            type="button"
            className="btn primary"
            disabled={!dirty || applyBusy}
            title={applyTitle}
            onClick={() => void onApply()}
          >
            {applyBusy ? 'Применение…' : 'Применить'}
          </button>
        </div>

        <div className="card">
          <div className="table-wrap">
            <table>
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
                      <EmptyState compact title="Нет коннекторов" />
                    </td>
                  </tr>
                ) : (
                  rows.map((c) => {
                    const busy = rowBusy === c.id;
                    const kind = connectorLiveKind(c.enabled, lastEventById[c.id]);
                    return (
                      <tr key={c.id}>
                        <td>{c.name}</td>
                        <td className="mono">
                          {c.id}
                          {c.builtin ? ' · builtin' : ''}
                        </td>
                        <td className="mono">{c.port}</td>
                        <td>{c.protocols}</td>
                        <td>
                          <button
                            type="button"
                            className={`btn ${liveButtonClass(kind)}`}
                            disabled={busy}
                            aria-pressed={c.enabled}
                            title={liveButtonTitle(kind, c.enabled)}
                            onClick={() => void onToggle(c)}
                          >
                            {c.enabled ? 'ВКЛ' : 'ВЫКЛ'}
                          </button>
                        </td>
                        <td className="actions">
                          <button
                            type="button"
                            className="btn"
                            disabled={busy}
                            title="Выкл → вкл"
                            onClick={() => void onRestart(c)}
                          >
                            Перезапуск
                          </button>
                          {!c.builtin ? (
                            <button
                              type="button"
                              className="btn danger"
                              disabled={busy}
                              onClick={() => void onDelete(c)}
                            >
                              Удалить
                            </button>
                          ) : null}
                        </td>
                      </tr>
                    );
                  })
                )}
              </tbody>
            </table>
          </div>
        </div>

        <section className="card">
          <h2>Добавить коннектор</h2>
          <form className="form-row" onSubmit={(e) => void onCreate(e)}>
            <div className="field">
              <label htmlFor="connector-id">ID</label>
              <input
                id="connector-id"
                value={id}
                onChange={(e) => setId(e.target.value)}
                placeholder="fw-a"
                required
                pattern="[a-z][a-z0-9_-]{0,31}"
                title="латиница, цифры, _ -; с буквы; до 32 символов"
              />
            </div>
            <div className="field">
              <label htmlFor="connector-name">Имя</label>
              <input
                id="connector-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="МСЭ офис"
                required
              />
            </div>
            <div className="field">
              <label htmlFor="connector-port">Порт</label>
              <input
                id="connector-port"
                value={port}
                onChange={(e) => setPort(e.target.value)}
                type="number"
                min={1024}
                max={65535}
                required
              />
            </div>
            <div className="field">
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
            <button type="submit" className="btn" disabled={rows.length >= maxConnectors}>
              Добавить
            </button>
          </form>
        </section>
      </div>
    </SettingsLayout>
  );
}
