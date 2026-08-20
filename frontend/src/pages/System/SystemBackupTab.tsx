import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  attachBackup,
  createBackup,
  deleteBackup,
  detachBackup,
  fetchBackups,
  fetchDRHistory,
  putBackupSchedule,
} from '@/api/system';
import { useToast } from '@/components/Toast';
import { formatDRMessage } from '@/lib/auditFormat';
import { fmtDate, fmtNumber } from '@/lib/format';
import { usePolling } from '@/lib/usePolling';
import { fmtBytes } from './systemFormat';
import type { BackupCatalog, BackupEntry, BackupSchedule, DREvent } from './systemTypes';

const TZ_PRESETS = ['Europe/Moscow', 'UTC', 'Europe/Berlin', 'Asia/Yekaterinburg'];

function pad2(n: number) {
  return String(n).padStart(2, '0');
}

function normalizeSched(s: BackupSchedule | undefined | null): BackupSchedule {
  return {
    enabled: !!s?.enabled,
    hour: s?.hour ?? 2,
    minute: s?.minute ?? 30,
    timezone: (s?.timezone || 'Europe/Moscow').trim(),
    keep: s?.keep ?? 7,
    include_edges: s?.include_edges !== false,
    include_auth: s?.include_auth !== false,
    updated_at: s?.updated_at,
    last_run_at: s?.last_run_at,
    last_run_date: s?.last_run_date,
  };
}

function scheduleSignature(s: BackupSchedule): string {
  return [
    s.enabled ? '1' : '0',
    String(s.hour ?? 0),
    String(s.minute ?? 0),
    (s.timezone || '').trim(),
    String(s.keep ?? 7),
    s.include_edges ? '1' : '0',
    s.include_auth ? '1' : '0',
  ].join('|');
}

function formatLocalTime(s: BackupSchedule): string {
  return `${pad2(s.hour ?? 0)}:${pad2(s.minute ?? 0)} ${s.timezone || 'UTC'}`;
}

export function SystemBackupTab() {
  const { toast } = useToast();
  const [cat, setCat] = useState<BackupCatalog | null>(null);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [busyName, setBusyName] = useState<string | null>(null);
  const [savingSchedule, setSavingSchedule] = useState(false);
  const [savedSched, setSavedSched] = useState<BackupSchedule>(normalizeSched(null));
  const [sched, setSched] = useState<BackupSchedule>(normalizeSched(null));
  const [tzCustom, setTzCustom] = useState(false);
  const [history, setHistory] = useState<DREvent[]>([]);

  const load = useCallback(async () => {
    try {
      const data = await fetchBackups();
      const historyRes = await fetchDRHistory({ limit: 50 });
      setCat(data);
      setHistory(historyRes.items || []);
      const next = normalizeSched(data.schedule);
      setSavedSched(next);
      setSched(next);
      setTzCustom(!TZ_PRESETS.includes(next.timezone || ''));
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка списка бэкапов', 'error');
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    void load();
  }, [load]);

  usePolling(
    async () => {
      await load();
    },
    3000,
    cat?.status?.state === 'running',
    { runImmediately: false },
  );

  const dirty = useMemo(
    () => scheduleSignature(sched) !== scheduleSignature(savedSched),
    [sched, savedSched],
  );

  const scheduleActive =
    !!cat?.enabled && !!cat?.dir_ready && !!savedSched.enabled && !dirty;
  const schedulePausedByEnv = !!savedSched.enabled && cat?.enabled === false;

  const create = async () => {
    setCreating(true);
    try {
      await createBackup();
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
      const res = await putBackupSchedule({
        enabled: !!sched.enabled,
        hour: Number(sched.hour) || 0,
        minute: Number(sched.minute) || 0,
        timezone: (sched.timezone || 'UTC').trim(),
        keep: Number(sched.keep) || 7,
        include_edges: !!sched.include_edges,
        include_auth: !!sched.include_auth,
      });
      const applied = normalizeSched(res.schedule || sched);
      setSavedSched(applied);
      setSched(applied);
      setTzCustom(!TZ_PRESETS.includes(applied.timezone || ''));
      toast(
        applied.enabled
          ? `Расписание применено: ежедневно в ${formatLocalTime(applied)}`
          : 'Расписание применено: автобэкап выключен',
        'success',
      );
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
          `На карте переключите источник на «Резервная копия».`,
      )
    ) {
      return;
    }
    setBusyName(b.name);
    try {
      await attachBackup(b.name);
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
      await detachBackup(b.name);
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
      await deleteBackup(b.name);
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
    <div className="tab-panel active" role="tabpanel">
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
          Прямой эфир / Резервная копия. «Отключить» удаляет только shadow. Полный appliance restore (включая auth):{' '}
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
              <span className="k">Расписание</span>
              <span className="v">
                {schedulePausedByEnv ? (
                  <>в файле вкл, но <code>BACKUP_ENABLED=0</code></>
                ) : dirty ? (
                  <>есть несохранённые изменения</>
                ) : savedSched.enabled ? (
                  <>
                    <strong>включено</strong> · ежедневно {formatLocalTime(savedSched)}
                  </>
                ) : (
                  <strong>выключено</strong>
                )}
              </span>
            </div>
            <div className="kv-row">
              <span className="k">След. авто</span>
              <span className="v">
                {scheduleActive && cat.next_run_at
                  ? fmtDate(cat.next_run_at, savedSched.timezone)
                  : '—'}
              </span>
            </div>
          </div>
        ) : null}
        {cat?.hint ? <p className="hint">{cat.hint}</p> : null}
      </section>

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
            Расписание и политика
          </h3>
          <span
            className="profile-badge"
            title={
              dirty
                ? 'Изменения ещё не сохранены'
                : schedulePausedByEnv
                  ? 'BACKUP_ENABLED=0 блокирует создание'
                  : savedSched.enabled
                    ? 'Автобэкап активен'
                    : 'Автобэкап выключен'
            }
          >
            {dirty
              ? 'не сохранено'
              : schedulePausedByEnv
                ? 'пауза (env)'
                : savedSched.enabled
                  ? 'расписание вкл'
                  : 'расписание выкл'}
          </span>
        </div>
        <p className="hint">
          Ежедневный автобэкап в указанное время выбранного пояса (в нём же «Применено» /
          «След. авто» / «Последний авто»). При включённом расписании внешний cron{' '}
          <code>scripts/backup-clickhouse.sh</code> не обязателен. Kill-switch:{' '}
          <code>BACKUP_ENABLED=0</code>.
        </p>
        <p className="hint" style={{ marginTop: 8 }}>
          {dirty ? (
            <>
              Черновик отличается от сохранённого. Нажмите «Сохранить расписание», чтобы применить.
            </>
          ) : savedSched.updated_at ? (
            <>
              Применено: {fmtDate(savedSched.updated_at, savedSched.timezone)}
              {savedSched.enabled
                ? ` · активно · ${formatLocalTime(savedSched)} · keep ${fmtNumber(savedSched.keep)}`
                : ' · автобэкап выключен'}
            </>
          ) : (
            <>Сохранённых правок из UI пока нет (действуют дефолты env).</>
          )}
        </p>
        <div className="kv-grid cols-2" style={{ marginTop: 12 }}>
          <div className="kv-row">
            <span className="k">Автобэкап</span>
            <span className="v">
              <label className="toggle" style={{ margin: 0 }}>
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
              <label className="toggle" style={{ margin: 0 }}>
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
              <label className="toggle" style={{ margin: 0 }}>
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
              {sched.last_run_at
                ? fmtDate(sched.last_run_at, savedSched.timezone || sched.timezone)
                : '—'}
              {sched.last_run_date ? ` (${sched.last_run_date})` : ''}
            </span>
          </div>
        </div>
        <div style={{ marginTop: 12, display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
          <button
            type="button"
            className="btn primary"
            disabled={savingSchedule || cat?.enabled === false || !dirty}
            onClick={() => void saveSchedule()}
          >
            {savingSchedule ? 'Сохранение…' : dirty ? 'Сохранить и применить' : 'Нет изменений'}
          </button>
          {dirty ? (
            <button
              type="button"
              className="btn"
              disabled={savingSchedule}
              onClick={() => {
                setSched(savedSched);
                setTzCustom(!TZ_PRESETS.includes(savedSched.timezone || ''));
              }}
            >
              Отменить правки
            </button>
          ) : null}
        </div>
      </section>

      <section className="card card-compact">
        <h3 className="card-title">Список бэкапов</h3>
        <p className="hint">
          Колонка <strong>Создан</strong> — в поясе расписания (
          <code>{savedSched.timezone || sched.timezone || 'UTC'}</code>
          ). Имя файла — то же локальное время с оффсетом (<code>…+0300</code>); старые{' '}
          <code>…Z</code> — UTC. <strong>Источник</strong> — вручную или по расписанию.{' '}
          <strong>Auth</strong> — снимок <code>/app/data</code> как <code>*.auth.tgz</code> (не
          трафик).
        </p>
        {backups.length === 0 ? (
          <p className="hint">Пока нет бэкапов.</p>
        ) : (
          <div className="table-wrap">
            <table className="auth-fails-table">
              <thead>
                <tr>
                  <th scope="col">Имя</th>
                  <th
                    scope="col"
                    title={`Время в поясе расписания ${savedSched.timezone || sched.timezone || 'UTC'}`}
                  >
                    Создан
                  </th>
                  <th scope="col">Источник</th>
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
                  const sourceLabel =
                    b.source === 'manual'
                      ? 'вручную'
                      : b.source === 'schedule'
                        ? 'по расписанию'
                        : '—';
                  return (
                    <tr key={b.name}>
                      <td>
                        <code>{b.name}</code>
                      </td>
                      <td>
                        {b.created_at
                          ? fmtDate(b.created_at, savedSched.timezone || sched.timezone)
                          : '—'}
                      </td>
                      <td>{sourceLabel}</td>
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
            DR-history
          </h3>
          <span className="hint">Последние 50 backup/attach/detach/delete/schedule событий</span>
        </div>
        {!history.length ? (
          <p className="auth-fails-empty empty">Событий пока нет</p>
        ) : (
          <div className="table-wrap">
            <table className="auth-fails-table">
              <thead>
                <tr>
                  <th scope="col">Время</th>
                  <th scope="col">Actor</th>
                  <th scope="col">Action</th>
                  <th scope="col">Target</th>
                  <th scope="col">Status</th>
                  <th scope="col">Message</th>
                </tr>
              </thead>
              <tbody>
                {history.map((item, idx) => (
                  <tr key={`${item.timestamp || 't'}-${item.action || 'a'}-${idx}`}>
                    <td>{fmtDate(item.timestamp, savedSched.timezone || sched.timezone)}</td>
                    <td>{item.actor || 'system'}</td>
                    <td className="mono">{item.action || '—'}</td>
                    <td>{item.target || '—'}</td>
                    <td>{item.status || '—'}</td>
                    <td>{formatDRMessage(item)}</td>
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
