import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import {
  fetchAnomalyEngineSettings,
  putAnomalyEngineSettings,
  type AnomalyEngineSettings,
  type AnomalyEngineSettingsView,
  type AnomalyScanStatus,
  type AnomalyThresholds,
} from '@/api/anomalies';
import { AdminLayout } from '@/components/AdminLayout';
import { ObserveSectionNav } from '@/components/ObserveSectionNav';
import { useToast } from '@/components/Toast';
import { fmtDate, fmtNumber } from '@/lib/format';
import './anomalies.css';

const DEFAULT_SETTINGS: AnomalyEngineSettings = {
  enabled: true,
  scan_interval_min: 5,
  learning_days: 3,
  suppress_hours: 24,
  include_private: false,
  new_country_min_share: 0.05,
};

function fmtShare(v: number | undefined): string {
  if (v == null || Number.isNaN(v)) return '—';
  return `${(v * 100).toFixed(1)}%`;
}

function StatusPanel({ status }: { status: AnomalyScanStatus | null }) {
  if (!status) return null;
  return (
    <section className="card card-compact anomaly-engine-status">
      <h3 className="card-title">Состояние сканера</h3>
      <dl className="anomaly-engine-dl">
        <div>
          <dt>Сканирование</dt>
          <dd>{status.enabled ? 'включено' : 'приостановлено'}</dd>
        </div>
        <div>
          <dt>Обучение</dt>
          <dd>{status.learning ? 'да' : 'нет'}</dd>
        </div>
        <div>
          <dt>Enterprise-сети</dt>
          <dd>{fmtNumber(status.enterprise_nets ?? 0)}</dd>
        </div>
        <div>
          <dt>Последний успешный тик</dt>
          <dd>{status.last_ok ? fmtDate(status.last_ok) : '—'}</dd>
        </div>
        <div>
          <dt>Длительность</dt>
          <dd>{status.last_duration || '—'}</dd>
        </div>
        <div>
          <dt>Вставлено</dt>
          <dd>{fmtNumber(status.last_inserted ?? 0)}</dd>
        </div>
        {status.last_skip ? (
          <div>
            <dt>Пропуск</dt>
            <dd>{status.last_skip}</dd>
          </div>
        ) : null}
        {status.last_error ? (
          <div>
            <dt>Ошибка</dt>
            <dd className="sev-high">{status.last_error}</dd>
          </div>
        ) : null}
      </dl>
    </section>
  );
}

function ThresholdsPanel({
  profile,
  thresholds,
}: {
  profile: string;
  thresholds: AnomalyThresholds | null;
}) {
  if (!thresholds) return null;
  return (
    <section className="card card-compact">
      <h3 className="card-title">Пороги детекторов (read-only)</h3>
      <p className="hint">
        Install profile: <strong>{profile || 'medium'}</strong>. Переопределение порогов по детекторам —
        в следующих версиях; сейчас только просмотр эффективных значений.
      </p>
      <dl className="anomaly-engine-dl thresholds-grid">
        <div>
          <dt>Port scan</dt>
          <dd>
            {thresholds.port_scan_ports} портов / {thresholds.port_scan_events} событий
          </dd>
        </div>
        <div>
          <dt>Horizontal scan</dt>
          <dd>
            {thresholds.horizontal_hosts} хостов / {thresholds.horizontal_events} событий
          </dd>
        </div>
        <div>
          <dt>Blocked surge</dt>
          <dd>
            ×{thresholds.surge_ratio}, min {fmtNumber(thresholds.surge_abs_min ?? 0)}, floor{' '}
            {fmtNumber(thresholds.surge_floor ?? 0)}
          </dd>
        </div>
        <div>
          <dt>Byte surge</dt>
          <dd>
            ×{thresholds.byte_surge_ratio}, min {fmtNumber(thresholds.byte_surge_abs_min ?? 0)}
          </dd>
        </div>
        <div>
          <dt>Beaconing</dt>
          <dd>
            ≥{thresholds.beacon_min_hours} ч, ≤{fmtNumber(thresholds.beacon_max_avg_bytes ?? 0)} B,
            regularity {thresholds.beacon_min_regularity}
          </dd>
        </div>
        <div>
          <dt>Lateral fanout</dt>
          <dd>
            {thresholds.lateral_hosts} хостов / {thresholds.lateral_events} событий
          </dd>
        </div>
        <div>
          <dt>New country</dt>
          <dd>
            min {fmtNumber(thresholds.new_country_min ?? 0)}, baseline{' '}
            {fmtNumber(thresholds.new_country_baseline ?? 0)}, share {fmtShare(thresholds.new_country_min_share)}
          </dd>
        </div>
        <div>
          <dt>Reputation peer</dt>
          <dd>min {fmtNumber(thresholds.rep_min_events ?? 0)} событий</dd>
        </div>
      </dl>
    </section>
  );
}

export default function AnomalyEnginePage() {
  const { toast } = useToast();
  const [view, setView] = useState<AnomalyEngineSettingsView | null>(null);
  const [settings, setSettings] = useState<AnomalyEngineSettings>(DEFAULT_SETTINGS);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await fetchAnomalyEngineSettings();
      setView(data);
      if (data.settings) setSettings({ ...DEFAULT_SETTINGS, ...data.settings });
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось загрузить настройки движка', 'error');
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    document.title = 'ГеоАтлас — Движок аномалий';
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const onSave = async (e: FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      const data = await putAnomalyEngineSettings(settings);
      setView(data);
      if (data.settings) setSettings({ ...DEFAULT_SETTINGS, ...data.settings });
      toast('Настройки движка сохранены', 'success');
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Не удалось сохранить настройки', 'error');
    } finally {
      setSaving(false);
    }
  };

  return (
    <AdminLayout title="Движок аномалий">
      <ObserveSectionNav />
      <div className="page-intro">
        <p>
          Параметры сканера и подавления повторов. Журнал алертов — на странице{' '}
          <Link to="/anomalies">Аномалии</Link>. Enterprise-сети задаются в{' '}
          <Link to="/geo-ranges">базе GeoIP</Link>.
        </p>
      </div>

      {loading ? <p className="hint">Загрузка…</p> : null}

      {!loading && !view ? (
        <p className="hint warn-banner">
          Модуль аномалий недоступен (ANOMALY_ENABLED=false на сервере).
        </p>
      ) : null}

      {!loading && view ? (
        <>
          <StatusPanel status={view.status ?? null} />

          <form className="card card-compact anomaly-engine-form" onSubmit={(e) => void onSave(e)}>
            <h3 className="card-title">Параметры</h3>

            <label className="anomaly-include-acked">
              <input
                type="checkbox"
                checked={settings.enabled}
                onChange={(e) => setSettings({ ...settings, enabled: e.target.checked })}
              />
              Сканирование включено
            </label>

            <div className="form-row">
              <label>
                Интервал сканирования (мин)
                <input
                  type="number"
                  min={1}
                  max={1440}
                  value={settings.scan_interval_min}
                  onChange={(e) =>
                    setSettings({ ...settings, scan_interval_min: Number(e.target.value) })
                  }
                />
              </label>
              <label>
                Дней обучения baseline
                <input
                  type="number"
                  min={1}
                  max={30}
                  value={settings.learning_days}
                  onChange={(e) => setSettings({ ...settings, learning_days: Number(e.target.value) })}
                />
              </label>
              <label>
                Подавление ack (часов)
                <input
                  type="number"
                  min={1}
                  max={168}
                  value={settings.suppress_hours}
                  onChange={(e) => setSettings({ ...settings, suppress_hours: Number(e.target.value) })}
                />
              </label>
            </div>

            <div className="form-row">
              <label>
                Min share new country
                <input
                  type="number"
                  min={0.01}
                  max={1}
                  step={0.01}
                  value={settings.new_country_min_share}
                  onChange={(e) =>
                    setSettings({ ...settings, new_country_min_share: Number(e.target.value) })
                  }
                />
              </label>
            </div>

            <label className="anomaly-include-acked">
              <input
                type="checkbox"
                checked={settings.include_private}
                onChange={(e) => setSettings({ ...settings, include_private: e.target.checked })}
              />
              Учитывать частные IP (RFC1918) в детекторах
            </label>

            {settings.updated_at ? (
              <p className="hint">Сохранено: {fmtDate(settings.updated_at)}</p>
            ) : null}

            <div className="form-actions">
              <button type="submit" className="btn primary" disabled={saving}>
                {saving ? 'Сохранение…' : 'Сохранить'}
              </button>
              <button type="button" className="btn" onClick={() => void load()} disabled={loading || saving}>
                Обновить
              </button>
            </div>
          </form>

          <ThresholdsPanel profile={view.install_profile ?? 'medium'} thresholds={view.thresholds ?? null} />
        </>
      ) : null}
    </AdminLayout>
  );
}
