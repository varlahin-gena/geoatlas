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
  const [busyName, setBusyName] = useState<string | null>(null);

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

  const attach = async (b: BackupEntry) => {
    if (
      !confirm(
        `Подключить «${b.name}» для просмотра на карте?\n\n` +
          `Данные попадут в shadow-таблицы nm_bak_* (live и ingest не трогаются). ` +
          `На карте переключите источник на «Бэкап».`,
      )
    ) {
      return;
    }
    setBusyName(b.name);
    try {
      await apiFetch(`/api/system/backups/${encodeURIComponent(b.name)}/attach`, {
        method: 'POST',
        body: '{}',
      });
      toast('Подключение запущено', 'success');
      await load();
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось подключить', 'error');
    } finally {
      setBusyName(null);
    }
  };

  const detach = async (b: BackupEntry) => {
    if (
      !confirm(
        `Отключить «${b.name}»?\n\nShadow-таблицы nm_bak_* будут удалены. Live-данные и файлы бэкапа на диске останутся.`,
      )
    ) {
      return;
    }
    setBusyName(b.name);
    try {
      await apiFetch(`/api/system/backups/${encodeURIComponent(b.name)}/detach`, {
        method: 'POST',
        body: '{}',
      });
      toast('Отключение запущено', 'success');
      await load();
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось отключить', 'error');
    } finally {
      setBusyName(null);
    }
  };

  const remove = async (b: BackupEntry) => {
    if (b.attached) {
      toast('Сначала отключите бэкап', 'error');
      return;
    }
    if (!confirm(`Удалить бэкап «${b.name}» с диска? Это необратимо.`)) {
      return;
    }
    setBusyName(b.name);
    try {
      await apiFetch(`/api/system/backups/${encodeURIComponent(b.name)}`, {
        method: 'DELETE',
      });
      toast('Бэкап удалён', 'success');
      await load();
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось удалить', 'error');
    } finally {
      setBusyName(null);
    }
  };

  const status = cat?.status;
  const running = status?.state === 'running' || creating || busyName !== null;
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
            {status?.state === 'running' || creating ? 'Выполняется…' : 'Создать бэкап'}
          </button>
        </div>
        <p className="hint">
          Native ClickHouse BACKUP на том clickhouse-backups. «Подключить» копирует данные в
          shadow-таблицы <code>nm_bak_*</code> — live и ingest не меняются. На карте переключатель
          Live / Бэкап. «Отключить» удаляет только shadow. Полный appliance restore (включая auth):{' '}
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
            <div className="kv-row">
              <span className="k">Подключён</span>
              <span className="v">{cat.attached ? <code>{cat.attached}</code> : 'нет'}</span>
            </div>
          </div>
        ) : null}
        {cat?.hint ? <p className="hint">{cat.hint}</p> : null}
      </section>

      <section className="card card-compact">
        <h3 className="card-title">Список бэкапов</h3>
        <p className="hint">
          Колонка <strong>Auth</strong> — есть ли рядом снимок <code>/app/data</code> (users,
          retention, feeds) как <code>*.auth.tgz</code>. Это не трафик: «нет» значит tarball не
          записался (часто из‑за ошибки прав на ранних попытках) или бэкап оборвался после
          ClickHouse BACKUP. На «Подключить / Отключить» auth не влияет.
        </p>
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
                  <th scope="col" title="Снимок /app/data (*.auth.tgz), не ClickHouse-таблицы">
                    Auth
                  </th>
                  <th scope="col">Состояние</th>
                  <th scope="col">Действия</th>
                </tr>
              </thead>
              <tbody>
                {backups.map((b) => {
                  const rowBusy = running;
                  return (
                    <tr key={b.name}>
                      <td>
                        <code>{b.name}</code>
                      </td>
                      <td>{b.created_at ? fmtDate(b.created_at) : '—'}</td>
                      <td>{fmtBytes(b.size_bytes || 0)}</td>
                      <td>{b.has_auth ? 'да' : 'нет'}</td>
                      <td>{b.attached ? 'подключён' : '—'}</td>
                      <td>
                        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                          {b.attached ? (
                            <button
                              type="button"
                              className="btn"
                              disabled={rowBusy}
                              onClick={() => void detach(b)}
                            >
                              Отключить
                            </button>
                          ) : (
                            <button
                              type="button"
                              className="btn primary"
                              disabled={rowBusy}
                              onClick={() => void attach(b)}
                            >
                              Подключить
                            </button>
                          )}
                          <button
                            type="button"
                            className="btn danger"
                            disabled={rowBusy || !!b.attached}
                            onClick={() => void remove(b)}
                            title={b.attached ? 'Сначала отключите' : undefined}
                          >
                            Удалить
                          </button>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}
