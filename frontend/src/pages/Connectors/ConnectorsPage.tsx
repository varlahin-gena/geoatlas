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
import { SettingsLayout } from '@/components/SettingsLayout';
import { EmptyState, TableSkeleton } from '@/components/Skeleton';
import { useToast } from '@/components/Toast';

export default function ConnectorsPage() {
  const { toast } = useToast();
  const [rows, setRows] = useState<Connector[]>([]);
  const [loading, setLoading] = useState(true);
  const [maxConnectors, setMaxConnectors] = useState(1);
  const [extraPorts, setExtraPorts] = useState({ min: 0, max: 0 });
  const [applyBusy, setApplyBusy] = useState(false);
  const [id, setId] = useState('');
  const [name, setName] = useState('');
  const [port, setPort] = useState('');
  const [protocols, setProtocols] = useState<ConnectorProtocols>('both');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await listConnectors();
      setRows(data.connectors || []);
      setMaxConnectors(data.max_connectors || 1);
      setExtraPorts(data.extra_ports || { min: 0, max: 0 });
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка', 'error');
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    document.title = 'ГеоАтлас — Коннекторы';
    void load();
  }, [load]);

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
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Ошибка', 'error');
    }
  }

  async function onToggle(c: Connector) {
    try {
      await updateConnector(c.id, { enabled: !c.enabled });
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Ошибка', 'error');
    }
  }

  async function onDelete(c: Connector) {
    if (c.builtin) return;
    if (!window.confirm(`Удалить коннектор «${c.name}» (${c.id})?`)) return;
    try {
      await deleteConnector(c.id);
      toast('Удалено. Примените конфиг.', 'success');
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Ошибка', 'error');
    }
  }

  async function onApply() {
    setApplyBusy(true);
    try {
      const res = await applyConnectors();
      if (res.ok && res.apply?.reloaded) {
        toast('Конфиг syslog-ng применён и перезагружен', 'success');
      } else if (res.apply?.reload_error) {
        toast(`Конфиг записан, reload: ${res.apply.reload_error}`, 'error');
      } else {
        toast('Конфиг записан', 'success');
      }
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

  return (
    <SettingsLayout
      title="Коннекторы"
      actions={
        <button type="button" className="btn primary" disabled={applyBusy} onClick={() => void onApply()}>
          {applyBusy ? 'Применение…' : 'Применить'}
        </button>
      }
    >
      <p className="muted" style={{ marginTop: 0 }}>
        Syslog-коннекторы — отдельные UDP/TCP порты приёма. После создания/изменения нажмите «Применить»,
        чтобы перегенерировать syslog-ng и сделать reload. Метрики — на странице Мониторинг → Pipeline.
      </p>
      <p className="muted">{portHint}</p>
      <p className="muted">
        Host firewall: после добавления портов выполните sync (
        <code>scripts/sync-syslog-firewall.sh</code>) или откройте порты вручную с{' '}
        <code>GA_SYSLOG_ALLOW_FROM</code>.
      </p>

      <div className="table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th>Имя</th>
              <th>ID</th>
              <th>Порт</th>
              <th>Протоколы</th>
              <th>Статус</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <TableSkeleton rows={4} cols={6} />
            ) : rows.length === 0 ? (
              <tr>
                <td colSpan={6}>
                  <EmptyState title="Нет коннекторов" />
                </td>
              </tr>
            ) : (
              rows.map((c) => (
                <tr key={c.id}>
                  <td>{c.name}</td>
                  <td>
                    <code>{c.id}</code>
                    {c.builtin ? ' · builtin' : ''}
                  </td>
                  <td>{c.port}</td>
                  <td>{c.protocols}</td>
                  <td>{c.enabled ? 'вкл' : 'выкл'}</td>
                  <td className="row-actions">
                    <button type="button" className="btn ghost" onClick={() => void onToggle(c)}>
                      {c.enabled ? 'Выкл' : 'Вкл'}
                    </button>
                    {!c.builtin ? (
                      <button type="button" className="btn ghost danger" onClick={() => void onDelete(c)}>
                        Удалить
                      </button>
                    ) : null}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      <section className="card" style={{ marginTop: '1.5rem' }}>
        <h3 className="card-title">Добавить коннектор</h3>
        <form className="form-grid" onSubmit={(e) => void onCreate(e)}>
          <label>
            ID
            <input value={id} onChange={(e) => setId(e.target.value)} placeholder="fw-a" required pattern="[a-z][a-z0-9_-]{0,31}" />
          </label>
          <label>
            Имя
            <input value={name} onChange={(e) => setName(e.target.value)} placeholder="МСЭ офис" required />
          </label>
          <label>
            Порт
            <input value={port} onChange={(e) => setPort(e.target.value)} type="number" min={1024} max={65535} required />
          </label>
          <label>
            Протоколы
            <select value={protocols} onChange={(e) => setProtocols(e.target.value as ConnectorProtocols)}>
              <option value="both">UDP+TCP</option>
              <option value="udp">UDP</option>
              <option value="tcp">TCP</option>
            </select>
          </label>
          <div className="form-actions">
            <button type="submit" className="btn primary" disabled={rows.length >= maxConnectors}>
              Добавить
            </button>
          </div>
        </form>
      </section>
    </SettingsLayout>
  );
}
