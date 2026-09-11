import { useCallback, useEffect, useId, useState, type FormEvent } from 'react';
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
import { TriageSectionNav } from '@/components/ObserveSectionNav';
import { useToast } from '@/components/Toast';
import { fmtDate, fmtNumber } from '@/lib/format';
import {
  DETECTOR_THRESHOLD_GROUPS,
  type DetectorThresholdMeta,
  type ThresholdFieldMeta,
} from './anomalyThresholdMeta';
import './anomalies.css';

const DEFAULT_SETTINGS: AnomalyEngineSettings = {
  enabled: true,
  scan_interval_min: 5,
  learning_days: 3,
  suppress_hours: 24,
  include_private: false,
  new_country_min_share: 0.05,
};

function thresholdsFromView(view: AnomalyEngineSettingsView): Required<AnomalyThresholds> {
  const src = view.thresholds ?? view.threshold_defaults;
  return {
    port_scan_ports: src?.port_scan_ports ?? 50,
    port_scan_events: src?.port_scan_events ?? 100,
    horizontal_hosts: src?.horizontal_hosts ?? 40,
    horizontal_events: src?.horizontal_events ?? 80,
    surge_ratio: src?.surge_ratio ?? 5,
    surge_abs_min: src?.surge_abs_min ?? 200,
    surge_floor: src?.surge_floor ?? 20,
    new_country_min: src?.new_country_min ?? 5,
    new_country_baseline: src?.new_country_baseline ?? 10,
    new_country_min_share: src?.new_country_min_share ?? 0.05,
    rep_min_events: src?.rep_min_events ?? 3,
    byte_surge_ratio: src?.byte_surge_ratio ?? 5,
    byte_surge_abs_min: src?.byte_surge_abs_min ?? 100_000_000,
    byte_surge_floor: src?.byte_surge_floor ?? 5_000_000,
    beacon_min_hours: src?.beacon_min_hours ?? 10,
    beacon_max_avg_bytes: src?.beacon_max_avg_bytes ?? 300_000,
    beacon_min_regularity: src?.beacon_min_regularity ?? 0.55,
    lateral_hosts: src?.lateral_hosts ?? 25,
    lateral_events: src?.lateral_events ?? 50,
  };
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
        {(status.enterprise_nets ?? 0) === 0 ? (
          <div>
            <dt>Подсказка</dt>
            <dd className="hint">
              Отметьте сети в <Link to="/geo-ranges">базе GeoIP</Link> → «Сети предприятия», иначе
              сканер пропускает тики.
            </dd>
          </div>
        ) : null}
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

function ThresholdField({
  field,
  value,
  onChange,
}: {
  field: ThresholdFieldMeta;
  value: number;
  onChange: (v: number) => void;
}) {
  return (
    <label className="threshold-field">
      <span className="threshold-field-label">{field.label}</span>
      <input
        type="number"
        min={field.min ?? 1}
        max={field.max}
        step={field.step ?? 1}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
      />
      {field.hint ? <span className="field-hint">{field.hint}</span> : null}
    </label>
  );
}

function DetectorCard({
  detector,
  thresholds,
  onChange,
}: {
  detector: DetectorThresholdMeta;
  thresholds: Required<AnomalyThresholds>;
  onChange: (patch: Partial<AnomalyThresholds>) => void;
}) {
  const adviceId = useId();
  const [open, setOpen] = useState(false);

  return (
    <article className="thresholds-detector">
      <header className="thresholds-detector-head">
        <div className="thresholds-detector-titles">
          <h4 className="thresholds-detector-title">{detector.title}</h4>
          <div className="thresholds-detector-meta">
            <code className="thresholds-code">{detector.code}</code>
            <span className="thresholds-window">{detector.window}</span>
          </div>
        </div>
        <button
          type="button"
          className="btn sm thresholds-advice-toggle"
          aria-expanded={open}
          aria-controls={adviceId}
          onClick={() => setOpen((v) => !v)}
        >
          {open ? 'Скрыть' : 'Как настроить'}
        </button>
      </header>

      <p className="thresholds-detector-summary">{detector.summary}</p>

      {open ? (
        <p id={adviceId} className="thresholds-detector-advice">
          <strong>Рекомендации.</strong> {detector.advice}
        </p>
      ) : null}

      <div className="form-row thresholds-detector-fields">
        {detector.fields.map((field) => (
          <ThresholdField
            key={field.key}
            field={field}
            value={thresholds[field.key]}
            onChange={(v) => onChange({ [field.key]: v })}
          />
        ))}
      </div>
    </article>
  );
}

function ThresholdsPanel({
  profile,
  thresholds,
  defaults,
  customized,
  onChange,
  onReset,
}: {
  profile: string;
  thresholds: Required<AnomalyThresholds>;
  defaults: AnomalyThresholds | null;
  customized: boolean;
  onChange: (next: Required<AnomalyThresholds>) => void;
  onReset: () => void;
}) {
  const set = (patch: Partial<AnomalyThresholds>) => onChange({ ...thresholds, ...patch });

  return (
    <section className="card card-compact anomaly-thresholds-panel">
      <div className="anomaly-thresholds-head">
        <div>
          <h3 className="card-title">Пороги детекторов</h3>
          <p className="hint">
            Профиль установки: <strong>{profile || 'medium'}</strong>.
            {customized
              ? ' Сейчас используются переопределённые значения.'
              : ' Показаны значения по умолчанию профиля.'}{' '}
            Детекторы сгруппированы по смыслу; у каждого типа — краткое описание и кнопка «Как настроить».
          </p>
        </div>
        <button type="button" className="btn sm" onClick={onReset} disabled={!defaults}>
          Сбросить к профилю
        </button>
      </div>

      <div className="thresholds-categories">
        {DETECTOR_THRESHOLD_GROUPS.map((group) => (
          <section key={group.id} className="thresholds-category">
            <header className="thresholds-category-head">
              <h4 className="thresholds-category-title">{group.title}</h4>
              <p className="thresholds-category-blurb">{group.blurb}</p>
            </header>
            <div className="thresholds-category-grid">
              {group.detectors.map((detector) => (
                <DetectorCard
                  key={detector.id}
                  detector={detector}
                  thresholds={thresholds}
                  onChange={set}
                />
              ))}
            </div>
          </section>
        ))}
      </div>
    </section>
  );
}

export default function AnomalyEnginePage() {
  const { toast } = useToast();
  const [view, setView] = useState<AnomalyEngineSettingsView | null>(null);
  const [settings, setSettings] = useState<AnomalyEngineSettings>(DEFAULT_SETTINGS);
  const [thresholds, setThresholds] = useState<Required<AnomalyThresholds>>(() => thresholdsFromView({}));
  const [thresholdCustomized, setThresholdCustomized] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const applyView = useCallback((data: AnomalyEngineSettingsView) => {
    setView(data);
    if (data.settings) {
      setSettings({ ...DEFAULT_SETTINGS, ...data.settings });
      setThresholdCustomized(Boolean(data.settings.thresholds));
    }
    setThresholds(thresholdsFromView(data));
  }, []);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await fetchAnomalyEngineSettings();
      applyView(data);
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось загрузить настройки движка', 'error');
    } finally {
      setLoading(false);
    }
  }, [applyView, toast]);

  useEffect(() => {
    document.title = 'ГеоАтлас — Движок аномалий';
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const onThresholdChange = (next: Required<AnomalyThresholds>) => {
    setThresholds(next);
    setThresholdCustomized(true);
    setSettings((prev) => ({ ...prev, new_country_min_share: next.new_country_min_share }));
  };

  const onResetThresholds = () => {
    if (!view?.threshold_defaults) return;
    const defaults = thresholdsFromView({ threshold_defaults: view.threshold_defaults });
    setThresholds(defaults);
    setThresholdCustomized(false);
    setSettings((prev) => ({ ...prev, new_country_min_share: defaults.new_country_min_share }));
  };

  const onSave = async (e: FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      const payload: AnomalyEngineSettings = {
        ...settings,
        thresholds: thresholdCustomized ? thresholds : null,
      };
      const data = await putAnomalyEngineSettings(payload);
      applyView(data);
      toast('Настройки движка сохранены', 'success');
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Не удалось сохранить настройки', 'error');
    } finally {
      setSaving(false);
    }
  };

  return (
    <AdminLayout title="Движок аномалий">
      <TriageSectionNav />
      <div className="page-intro">
        <p>
          Параметры сканера, пороги детекторов и подавление повторов. Журнал алертов — на странице{' '}
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

            <ThresholdsPanel
              profile={view.install_profile ?? 'medium'}
              thresholds={thresholds}
              defaults={view.threshold_defaults ?? null}
              customized={thresholdCustomized}
              onChange={onThresholdChange}
              onReset={onResetThresholds}
            />

            <div className="form-actions">
              <button type="submit" className="btn primary" disabled={saving}>
                {saving ? 'Сохранение…' : 'Сохранить'}
              </button>
              <button type="button" className="btn" onClick={() => void load()} disabled={loading || saving}>
                Обновить
              </button>
            </div>
          </form>
        </>
      ) : null}
    </AdminLayout>
  );
}
