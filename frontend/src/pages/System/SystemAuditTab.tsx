import { useCallback, useEffect, useState } from 'react';
import { fetchAuditLog } from '@/api/system';
import { useToast } from '@/components/Toast';
import { formatAuditRow } from '@/lib/auditFormat';
import { fmtDate } from '@/lib/format';
import type { AuditEvent } from './systemTypes';

const DEFAULT_LIMIT = 100;

export function SystemAuditTab() {
  const { toast } = useToast();
  const [items, setItems] = useState<AuditEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [actor, setActor] = useState('');
  const [action, setAction] = useState('');
  const [result, setResult] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetchAuditLog({
        limit: DEFAULT_LIMIT,
        actor: actor.trim() || undefined,
        action: action.trim() || undefined,
        result: result.trim() || undefined,
      });
      setItems(res.items || []);
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось загрузить журнал аудита', 'error');
    } finally {
      setLoading(false);
    }
  }, [action, actor, result, toast]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <div className="tab-panel active" role="tabpanel">
      <section className="card">
        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap' }}>
          <h3 className="card-title" style={{ margin: 0 }}>
            Журнал аудита
          </h3>
          <button type="button" className="btn" onClick={() => void load()} disabled={loading}>
            Обновить
          </button>
        </div>
        <p className="hint">
          Append-only журнал admin/security действий: входы, пользователи, токены, backup и system settings.
        </p>
        <div className="history-filters">
          <input
            className="input"
            placeholder="Учётная запись"
            value={actor}
            onChange={(e) => setActor(e.target.value)}
          />
          <input
            className="input"
            placeholder="Действие"
            value={action}
            onChange={(e) => setAction(e.target.value)}
          />
          <select className="input" value={result} onChange={(e) => setResult(e.target.value)}>
            <option value="">Любой результат</option>
            <option value="succeeded">succeeded</option>
            <option value="failed">failed</option>
          </select>
          <button type="button" className="btn" onClick={() => void load()}>
            Применить
          </button>
        </div>
        {loading ? <p className="hint">Загрузка…</p> : null}
        {!loading && !items.length ? <p className="auth-fails-empty empty">Событий пока нет</p> : null}
        {items.length ? (
          <div className="table-wrap">
            <table className="auth-fails-table">
              <thead>
                <tr>
                  <th scope="col">Время</th>
                  <th scope="col">Actor</th>
                  <th scope="col">Action</th>
                  <th scope="col">Resource</th>
                  <th scope="col">Result</th>
                  <th scope="col">Детали</th>
                  <th scope="col">IP</th>
                </tr>
              </thead>
              <tbody>
                {items.map((item, idx) => (
                  <tr key={`${item.timestamp || 't'}-${item.action || 'a'}-${idx}`}>
                    <td>{fmtDate(item.timestamp)}</td>
                    <td>{item.actor || 'system'}</td>
                    <td className="mono">{item.action || '—'}</td>
                    <td>
                      {item.resource_type || '—'}
                      {item.resource_id ? ` · ${item.resource_id}` : ''}
                    </td>
                    <td>{item.result || '—'}</td>
                    <td>{formatAuditRow(item)}</td>
                    <td>{item.ip || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}
      </section>
    </div>
  );
}
