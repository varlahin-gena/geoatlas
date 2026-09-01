import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import {
  deleteHunt,
  huntMapHref,
  listMyHunts,
  runHunt,
  updateHunt,
  type SavedHunt,
} from '@/api/hunts';
import { AdminLayout } from '@/components/AdminLayout';
import { useToast } from '@/components/Toast';
import { fmtDate, fmtNumber } from '@/lib/format';
import './hunts.css';

const DEFAULT_SCHEDULE = {
  enabled: false,
  interval_min: 60,
  edge_threshold: 0,
  edge_ratio: 3,
};

export default function HuntsPage() {
  const { toast } = useToast();
  const [rows, setRows] = useState<SavedHunt[]>([]);
  const [loading, setLoading] = useState(true);
  const [running, setRunning] = useState<string | null>(null);
  const [editing, setEditing] = useState<SavedHunt | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await listMyHunts();
      setRows(data.hunts || []);
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось загрузить hunts', 'error');
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    document.title = 'ГеоАтлас — Saved hunts';
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function onRun(id: string) {
    setRunning(id);
    try {
      const data = await runHunt(id);
      if (data.hunt) {
        setRows((prev) => prev.map((h) => (h.id === id ? data.hunt! : h)));
      }
      const run = data.run;
      if (run?.skipped) {
        toast(`Пропуск: ${run.skipped}`, 'warn');
      } else if (run?.breach) {
        toast(`Порог превышен: ${fmtNumber(run.edge_count)} рёбер`, 'warn');
      } else {
        toast(`Готово: ${fmtNumber(run?.edge_count ?? 0)} рёбер`, 'success');
      }
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка запуска', 'error');
    } finally {
      setRunning(null);
    }
  }

  async function onDelete(id: string) {
    if (!confirm('Удалить hunt?')) return;
    try {
      await deleteHunt(id);
      setRows((prev) => prev.filter((h) => h.id !== id));
      toast('Удалено', 'success');
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка удаления', 'error');
    }
  }

  async function onSaveEdit(e: FormEvent) {
    e.preventDefault();
    if (!editing) return;
    try {
      const data = await updateHunt(editing.id, editing);
      if (data.hunt) {
        setRows((prev) => prev.map((h) => (h.id === editing.id ? data.hunt! : h)));
      }
      setEditing(null);
      toast('Сохранено', 'success');
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Ошибка сохранения', 'error');
    }
  }

  return (
    <AdminLayout title="Saved hunts">
      <div className="page-content-inner wide">
        <p className="page-lead">
          Сохранённые запросы карты с полным контекстом (период, группировка, фильтры). Расписание
          запускает snapshot-прогон; при превышении порога создаётся алерт{' '}
          <code>hunt_threshold</code> в журнале аномалий.
        </p>
        <p className="hint">
          Сохранить текущий вид карты — кнопка «Hunt» на карте или{' '}
          <Link to="/">вернуться на карту</Link>.
        </p>

        {loading ? <p className="hint">Загрузка…</p> : null}

        <div className="hunts-grid">
          {rows.map((h) => (
            <article key={h.id} className="card card-compact hunt-card">
              <header className="hunt-card-head">
                <h3>{h.name}</h3>
                {h.schedule?.enabled ? <span className="hunt-badge">по расписанию</span> : null}
              </header>
              {h.notes ? <p className="hint">{h.notes}</p> : null}
              <dl className="hunt-meta">
                <div>
                  <dt>Период</dt>
                  <dd>{h.map?.period || '1d'}</dd>
                </div>
                <div>
                  <dt>Группировка</dt>
                  <dd>{h.map?.group_by || 'city'}</dd>
                </div>
                <div>
                  <dt>Запрос</dt>
                  <dd>{h.map?.query || '—'}</dd>
                </div>
                <div>
                  <dt>Последний run</dt>
                  <dd>{h.last_run_at ? fmtDate(h.last_run_at) : '—'}</dd>
                </div>
              </dl>
              {h.runs?.length ? (
                <p className="hint">
                  Последний результат: {fmtNumber(h.runs[h.runs.length - 1]?.edge_count ?? 0)} рёбер
                  {h.runs[h.runs.length - 1]?.breach ? ' · порог превышен' : ''}
                </p>
              ) : null}
              <div className="hunt-actions">
                <Link to={huntMapHref(h)} className="btn sm">
                  На карте
                </Link>
                <button type="button" className="btn sm" disabled={running === h.id} onClick={() => void onRun(h.id)}>
                  {running === h.id ? '…' : 'Запустить'}
                </button>
                <button type="button" className="btn sm" onClick={() => setEditing({ ...h, schedule: { ...DEFAULT_SCHEDULE, ...h.schedule } })}>
                  Настройки
                </button>
                <button type="button" className="btn sm" onClick={() => void onDelete(h.id)}>
                  Удалить
                </button>
              </div>
            </article>
          ))}
        </div>

        {!loading && !rows.length ? (
          <p className="hint">Пока нет saved hunts. Сохраните текущий вид на карте.</p>
        ) : null}

        {editing ? (
          <form className="card card-compact hunt-edit-form" onSubmit={(e) => void onSaveEdit(e)}>
            <h3 className="card-title">Настройки: {editing.name}</h3>
            <label>
              Заметки
              <input
                value={editing.notes || ''}
                onChange={(e) => setEditing({ ...editing, notes: e.target.value })}
              />
            </label>
            <label className="hunt-check">
              <input
                type="checkbox"
                checked={editing.schedule?.enabled ?? false}
                onChange={(e) =>
                  setEditing({
                    ...editing,
                    schedule: { ...DEFAULT_SCHEDULE, ...editing.schedule, enabled: e.target.checked },
                  })
                }
              />
              Расписание включено
            </label>
            <div className="hunt-edit-row">
              <label>
                Интервал (мин)
                <input
                  type="number"
                  min={15}
                  max={1440}
                  value={editing.schedule?.interval_min ?? 60}
                  onChange={(e) =>
                    setEditing({
                      ...editing,
                      schedule: { ...DEFAULT_SCHEDULE, ...editing.schedule, interval_min: Number(e.target.value) },
                    })
                  }
                />
              </label>
              <label>
                Порог рёбер
                <input
                  type="number"
                  min={0}
                  value={editing.schedule?.edge_threshold ?? 0}
                  onChange={(e) =>
                    setEditing({
                      ...editing,
                      schedule: { ...DEFAULT_SCHEDULE, ...editing.schedule, edge_threshold: Number(e.target.value) },
                    })
                  }
                />
              </label>
              <label>
                Ratio vs prev
                <input
                  type="number"
                  min={1}
                  step={0.1}
                  value={editing.schedule?.edge_ratio ?? 3}
                  onChange={(e) =>
                    setEditing({
                      ...editing,
                      schedule: { ...DEFAULT_SCHEDULE, ...editing.schedule, edge_ratio: Number(e.target.value) },
                    })
                  }
                />
              </label>
            </div>
            <div className="hunt-actions">
              <button type="submit" className="btn primary">
                Сохранить
              </button>
              <button type="button" className="btn" onClick={() => setEditing(null)}>
                Отмена
              </button>
            </div>
          </form>
        ) : null}
      </div>
    </AdminLayout>
  );
}
