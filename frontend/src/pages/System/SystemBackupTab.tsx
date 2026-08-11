import { useCallback, useEffect, useState } from 'react';
import { apiFetch } from '@/api/client';
import { useToast } from '@/components/Toast';
import { fmtDate, fmtNumber } from '@/lib/format';
import { fmtBytes } from './systemFormat';
import type { BackupCatalog, BackupEntry, BackupSchedule } from './systemTypes';

const TZ_PRESETS = ['Europe/Moscow', 'UTC', 'Europe/Berlin', 'Asia/Yekaterinburg'];

function pad2(n: number) {
  return String(n).padStart(2, '0');
}

export function SystemBackupTab() {
  const { toast } = useToast();
  const [cat, setCat] = useState<BackupCatalog | null>(null);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [busyName, setBusyName] = useState<string | null>(null);
  const [savingSchedule, setSavingSchedule] = useState(false);
  const [sched, setSched] = useState<BackupSchedule>({
    enabled: false,
    hour: 2,
    minute: 30,
    timezone: 'Europe/Moscow',
    keep: 7,
    include_edges: true,
    include_auth: true,
  });
  const [tzCustom, setTzCustom] = useState(false);

  const load = useCallback(async () => {
    try {
      const data = await apiFetch<BackupCatalog>('/api/system/backups');
      setCat(data);
      if (data.schedule) {
        const tz = data.schedule.timezone || 'Europe/Moscow';
        setTzCustom(!TZ_PRESETS.includes(tz));
        setSched({
          enabled: !!data.schedule.enabled,
          hour: data.schedule.hour ?? 2,
          minute: data.schedule.minute ?? 30,
          timezone: tz,
          keep: data.schedule.keep ?? 7,
          include_edges: data.schedule.include_edges !== false,
          include_auth: data.schedule.include_auth !== false,
          last_run_at: data.schedule.last_run_at,
          last_run_date: data.schedule.last_run_date,
        });
      }
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

  const saveSchedule = async () => {
    setSavingSchedule(true);
    try {
      await apiFetch<{ ok?: boolean; schedule?: BackupSchedule }>('/api/system/backup-schedule', {
        method: 'PUT',
        body: JSON.stringify({
          enabled: !!sched.enabled,
          hour: Number(sched.hour) || 0,
          minute: Number(sched.minute) || 0,
          timezone: (sched.timezone || 'UTC').trim(),
          keep: Number(sched.keep) || 7,
          include_edges: !!sched.include_edges,
          include_auth: !!sched.include_auth,
        }),
      });
      toast('Расписание сохранено', 'success');
      await load();
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось сохранить расписание', 'error');
    } finally {
      setSavingSchedule(false);
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
  const timeValue = `${pad2(sched.hour ?? 0)}:${pad2(sched.minute ?? 0)}`;

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
            <div className="kv-row">
              <span className="k">След. авто</span>
              <span className="v">{cat.next_run_at ? fmtDate(cat.next_run_at) : '—'}</span>
            </div>
          </div>
        ) : null}
        {cat?.hint ? <p className="hint">{cat.hint}</p> : null}
      </section>

      <section className="card card-compact">
        <h3 className="card-title">Расписание и политика</h3>
        <p className="hint">
          Ежедневный автобэкап в указанное локальное время. При включённом расписании внешний cron{' '}
          <code>scripts/backup-clickhouse.sh</code> не обязателен. Kill-switch:{' '}
          <code>BACKUP_ENABLED=0</code>.
        </p>
        <div className="kv-grid cols-2" style={{ marginTop: 12 }}>
          <div className="kv-row">
            <span className="k">Автобэкап</span>
            <span className="v">
              <label className="side-toggle" style={{ margin: 0 }}>
                <input
                  type="checkbox"
                  checked={!!sched.enabled}
                  onChange={(e) => setSched((s) => ({ ...s, enabled: e.target.checked }))}
                />
                <span>{sched.enabled ? 'вкл' : 'выкл'}</span>
              </label>
            </span>
          </div>
          <div className="kv-row">
            <span className="k">Время</span>
            <span className="v">
              <input
                type="time"
                value={timeValue}
                onChange={(e) => {
                  const [hh, mm] = (e.target.value || '02:30').split(':');
                  setSched((s) => ({
                    ...s,
                    hour: Math.min(23, Math.max(0, Number(hh) || 0)),
                    minute: Math.min(59, Math.max(0, Number(mm) || 0)),
                  }));
                }}
              />
            </span>
          </div>
          <div className="kv-row">
            <span className="k">Часовой пояс</span>
            <span className="v">
              {tzCustom ? (
                <span style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
                  <input
                    type="text"
                    style={{ minWidth: 200 }}
                    value={sched.timezone || ''}
                    onChange={(e) => setSched((s) => ({ ...s, timezone: e.target.value }))}
                    placeholder="IANA, напр. Asia/Tomsk"
                    aria-label="Часовой пояс IANA"
                  />
                  <button
                    type="button"
                    className="btn"
                    onClick={() => {
                      setTzCustom(false);
                      setSched((s) => ({ ...s, timezone: 'Europe/Moscow' }));
                    }}
                  >
                    Из списка
                  </button>
                </span>
              ) : (
                <select
                  value={sched.timezone || 'Europe/Moscow'}
                  aria-label="Часовой пояс"
                  onChange={(e) => {
                    const v = e.target.value;
                    if (v === '__custom__') {
                      setTzCustom(true);
                      return;
                    }
                    setSched((s) => ({ ...s, timezone: v }));
                  }}
                >
                  {TZ_PRESETS.map((tz) => (
                    <option key={tz} value={tz}>
                      {tz}
                    </option>
                  ))}
                  <option value="__custom__">Другой (IANA)…</option>
                </select>
              )}
            </span>
          </div>
          <div className="kv-row">
            <span className="k">Глубина (keep)</span>
            <span className="v">
              <input
                type="number"
                min={1}
                max={90}
                value={sched.keep ?? 7}
                onChange={(e) =>
                  setSched((s) => ({ ...s, keep: Math.min(90, Math.max(1, Number(e.target.value) || 1)) }))
                }
              />
            </span>
          </div>
          <div className="kv-row">
            <span className="k">Edges</span>
            <span className="v">
              <label className="side-toggle" style={{ margin: 0 }}>
                <input
                  type="checkbox"
                  checked={!!sched.include_edges}
                  onChange={(e) => setSched((s) => ({ ...s, include_edges: e.target.checked }))}
                />
                <span>traffic_edges_*</span>
              </label>
            </span>
          </div>
          <div className="kv-row">
            <span className="k">Auth tarball</span>
            <span className="v">
              <label className="side-toggle" style={{ margin: 0 }}>
                <input
                  type="checkbox"
                  checked={!!sched.include_auth}
                  onChange={(e) => setSched((s) => ({ ...s, include_auth: e.target.checked }))}
                />
                <span>*.auth.tgz (/app/data)</span>
              </label>
            </span>
          </div>
          <div className="kv-row">
            <span className="k">Последний авто</span>
            <span className="v">
              {sched.last_run_at ? fmtDate(sched.last_run_at) : '—'}
              {sched.last_run_date ? ` (${sched.last_run_date})` : ''}
            </span>
          </div>
        </div>
        <div style={{ marginTop: 12 }}>
          <button
            type="button"
            className="btn primary"
            disabled={savingSchedule || cat?.enabled === false}
            onClick={() => void saveSchedule()}
          >
            {savingSchedule ? 'Сохранение…' : 'Сохранить расписание'}
          </button>
        </div>
      </section>

      <section className="card card-compact">
        <h3 className="card-title">Список бэкапов</h3>
        <p className="hint">
          Колонка <strong>Auth</strong> — есть ли рядом снимок <code>/app/data</code> (users,
          retention, feeds) как <code>*.auth.tgz</code>. Это не трафик.
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
