import { useCallback, useEffect, useState } from 'react';
import { apiFetch } from '@/api/client';
import { useToast } from '@/components/Toast';
import { fmtDate, fmtNumber } from '@/lib/format';
import { fmtBytes } from './systemFormat';
import type { BackupCatalog, BackupEntry } from './systemTypes';

export function SystemBackupTab() {
  const { toast } = useToast();
  const [cat, setCat] = useState<BackupCatalog | null>(null);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);

  const load = useCallback(async () => {
    try {
      const data = await apiFetch<BackupCatalog>('/api/system/backups');
      setCat(data);
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка списка бэкапов', 'error');
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (cat?.status?.state !== 'running') return;
    const id = window.setInterval(() => void load(), 3000);
    return () => window.clearInterval(id);
  }, [cat?.status?.state, load]);

  const create = async () => {
    setCreating(true);
    try {
      await apiFetch('/api/system/backups', { method: 'POST', body: '{}' });
      toast('Бэкап запущен', 'success');
      await load();
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось запустить бэкап', 'error');
    } finally {
      setCreating(false);
    }
  };

  const status = cat?.status;
  const running = status?.state === 'running' || creating;
  const backups: BackupEntry[] = cat?.backups || [];

  return (
    <div className="tab-panel active" id="tab-backup" role="tabpanel">
      <section className="card card-compact">
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 12,
            flexWrap: 'wrap',
          }}
        >
          <h3 className="card-title" style={{ margin: 0 }}>
            Резервное копирование
          </h3>
          <button
            type="button"
            className="btn primary"
            disabled={running || cat?.enabled === false || cat?.dir_ready === false}
            onClick={() => void create()}
          >
            {running ? 'Выполняется…' : 'Создать бэкап'}
          </button>
        </div>
        <p className="hint">
          Native ClickHouse BACKUP на том clickhouse-backups. Restore — CLI на хосте:{' '}
          <code>./scripts/restore-clickhouse.sh &lt;name&gt;</code>
        </p>
        {loading && !cat ? <p className="hint">Загрузка…</p> : null}
        {cat ? (
          <div className="kv-grid cols-2">
            <div className="kv-row">
              <span className="k">Статус</span>
              <span className="v">
                {status?.state || '—'}
                {status?.name ? ` · ${status.name}` : ''}
              </span>
            </div>
            <div className="kv-row">
              <span className="k">Сообщение</span>
              <span className="v">{status?.message || '—'}</span>
            </div>
            <div className="kv-row">
              <span className="k">Каталог</span>
              <span className="v">{cat.dir_ready ? 'готов' : 'не смонтирован'}</span>
            </div>
            <div className="kv-row">
              <span className="k">Политика</span>
              <span className="v">
                keep {fmtNumber(cat.keep)} · edges {cat.include_edges ? 'да' : 'нет'} · auth{' '}
                {cat.include_auth ? 'да' : 'нет'}
              </span>
            </div>
          </div>
        ) : null}
        {cat?.hint ? <p className="hint">{cat.hint}</p> : null}
      </section>

      <section className="card card-compact">
        <h3 className="card-title">Список бэкапов</h3>
        {backups.length === 0 ? (
          <p className="hint">Пока нет бэкапов.</p>
        ) : (
          <div className="table-wrap">
            <table className="auth-fails-table">
              <thead>
                <tr>
                  <th scope="col">Имя</th>
                  <th scope="col">Создан</th>
                  <th scope="col">Размер</th>
                  <th scope="col">Auth</th>
                </tr>
              </thead>
              <tbody>
                {backups.map((b) => (
                  <tr key={b.name}>
                    <td>
                      <code>{b.name}</code>
                    </td>
                    <td>{b.created_at ? fmtDate(b.created_at) : '—'}</td>
                    <td>{fmtBytes(b.size_bytes || 0)}</td>
                    <td>{b.has_auth ? 'да' : 'нет'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}
