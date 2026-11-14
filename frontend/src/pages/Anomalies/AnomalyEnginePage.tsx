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
import { useToast } from '@/components/Toast';
import { fmtBytes, fmtDate, fmtNumber } from '@/lib/format';
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

function formatScanDuration(raw?: string): string {
  if (!raw) return '—';
  const m = /^([\d.]+)\s*ms$/i.exec(raw.trim());
  if (m) return `${m[1]} мс`;
  return raw;
}

function fieldPreviewValue(field: ThresholdFieldMeta, value: number): string {
  if (field.unit === 'bytes') return fmtBytes(value);
  if (field.step && field.step < 1) return String(value).replace('.', ',');
  return fmtNumber(value);
}

function StatusPanel({ status }: { status: AnomalyScanStatus | null }) {
  if (!status) return null;
  return (
    <section className="card card-compact anomaly-engine-status">
      <h3 className="card-title">Состояние</h3>
      <dl className="anomaly-engine-dl">
        <div>
          <dt>Привыкание к трафику</dt>
          <dd>{status.learning ? 'идёт' : 'готово'}</dd>
        </div>
        <div>
          <dt>Сети предприятия</dt>
          <dd>{fmtNumber(status.enterprise_nets ?? 0)}</dd>
        </div>
        {(status.enterprise_nets ?? 0) === 0 ? (
          <div>
            <dt>Подсказка</dt>
            <dd className="hint">
              Отметьте сети в <Link to="/geo-ranges">базе GeoIP</Link> → «Сети предприятия», иначе
              поиск аномалий не запускается.
            </dd>
          </div>
        ) : null}
        <div>
          <dt>Последняя успешная проверка</dt>
          <dd>{status.last_ok ? fmtDate(status.last_ok) : '—'}</dd>
        </div>
        <div>
          <dt>Длительность</dt>
          <dd>{formatScanDuration(status.last_duration)}</dd>
        </div>
        <div>
          <dt>Найдено за последний проход</dt>
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
      {status.learning ? (
        <p className="anomaly-learning-note" role="status">
          Пока идёт привыкание: проверки «Новая страна» и «Регулярные короткие сессии» отключены.
          Остальное уже работает. Длительность задаётся в параметрах ниже.
        </p>
      ) : null}
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
      {field.unit === 'bytes' ? (
        <span className="field-hint">≈ {fmtBytes(value)}</span>
      ) : null}
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
  const fieldsId = useId();
  const [adviceOpen, setAdviceOpen] = useState(false);
  const [fieldsOpen, setFieldsOpen] = useState(false);
  const preview = detector.fields
    .slice(0, 2)
    .map((f) => `${f.label}: ${fieldPreviewValue(f, thresholds[f.key])}`)
    .join(' · ');

  return (
    <article className={`thresholds-detector${fieldsOpen ? ' is-editing' : ''}`}>
      <header className="thresholds-detector-head">
        <div className="thresholds-detector-titles">
          <h4 className="thresholds-detector-title">{detector.title}</h4>
          <div className="thresholds-detector-meta">
            <code className="thresholds-code" title={detector.code}>
              {detector.code}
            </code>
            <span className="thresholds-window">{detector.window}</span>
          </div>
          {!fieldsOpen && preview ? (
            <p className="thresholds-detector-summary">{preview}</p>
          ) : null}
        </div>
        <div className="thresholds-detector-actions">
          <button
            type="button"
            className="btn sm ghost"
            aria-expanded={adviceOpen}
            aria-controls={adviceId}
            onClick={() => setAdviceOpen((v) => !v)}
          >
            {adviceOpen ? 'Скрыть справку' : 'Справка'}
          </button>
          <button
            type="button"
            className="btn sm"
            aria-expanded={fieldsOpen}
            aria-controls={fieldsId}
            onClick={() => setFieldsOpen((v) => !v)}
          >
            {fieldsOpen ? 'Свернуть' : 'Изменить'}
          </button>
        </div>
      </header>

      {adviceOpen ? (
        <div id={adviceId} className="thresholds-detector-advice">
          <p>{detector.summary}</p>
          <p>
            <strong>Рекомендации.</strong> {detector.advice}
          </p>
        </div>
      ) : null}

      {fieldsOpen ? (
        <div id={fieldsId} className="form-row thresholds-detector-fields">
          {detector.fields.map((field) => (
            <ThresholdField
              key={field.key}
              field={field}
              value={thresholds[field.key]}
              onChange={(v) => onChange({ [field.key]: v })}
            />
          ))}
        </div>
      ) : null}
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
  const [openGroup, setOpenGroup] = useState(DETECTOR_THRESHOLD_GROUPS[0]?.id || 'scanning');
  const set = (patch: Partial<AnomalyThresholds>) => onChange({ ...thresholds, ...patch });

  return (
    <section className="card card-compact anomaly-thresholds-panel">
      <div className="anomaly-thresholds-head">
        <div>
          <h3 className="card-title">Чувствительность</h3>
          <p className="hint">
            Профиль установки: <strong>{profile || 'medium'}</strong>.
            {customized
              ? ' Сейчас используются переопределённые значения.'
              : ' Показаны значения по умолчанию профиля.'}{' '}
            Откройте категорию и нажмите «Изменить», чтобы править числа.
          </p>
        </div>
        <button type="button" className="btn sm" onClick={onReset} disabled={!defaults}>
          Вернуть значения профиля
        </button>
      </div>

      <div className="thresholds-categories">
        {DETECTOR_THRESHOLD_GROUPS.map((group) => {
          const expanded = openGroup === group.id;
          const panelId = `threshold-group-${group.id}`;
          return (
            <section
              key={group.id}
              className={`thresholds-category${expanded ? ' is-open' : ''}`}
            >
              <header className="thresholds-category-head">
                <button
                  type="button"
                  className="thresholds-category-toggle"
                  aria-expanded={expanded}
                  aria-controls={panelId}
                  onClick={() => setOpenGroup(expanded ? '' : group.id)}
                >
                  <span className="thresholds-category-title">{group.title}</span>
                  <span className="thresholds-category-count">
                    {group.detectors.length}{' '}
                    {group.detectors.length === 1 ? 'проверка' : 'проверки'}
                  </span>
                  <span className="thresholds-category-chevron" aria-hidden="true">
                    {expanded ? '▾' : '▸'}
                  </span>
                </button>
                {expanded ? (
                  <p className="thresholds-category-blurb">{group.blurb}</p>
                ) : null}
              </header>
              {expanded ? (
                <div id={panelId} className="thresholds-category-grid">
                  {group.detectors.map((detector) => (
                    <DetectorCard
                      key={detector.id}
                      detector={detector}
                      thresholds={thresholds}
                      onChange={set}
                    />
                  ))}
                </div>
              ) : null}
            </section>
          );
        })}
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
      toast(e instanceof Error ? e.message : 'Не удалось загрузить настройки аномалий', 'error');
    } finally {
      setLoading(false);
    }
  }, [applyView, toast]);

  useEffect(() => {
    document.title = 'ГеоАтлас — Настройки аномалий';
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
    toast(
      `Пороги профиля ${view.install_profile || 'medium'} — нажмите «Сохранить»`,
      'success',
    );
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
      toast('Настройки аномалий сохранены', 'success');
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Не удалось сохранить настройки', 'error');
    } finally {
      setSaving(false);
    }
  };

  return (
    <AdminLayout title="Настройки аномалий">
      <div className="page-intro">
        <p>
          Как часто искать аномалии и насколько чувствительны проверки. Журнал алертов — на странице{' '}
          <Link to="/anomalies">Аномалии</Link>. Сети предприятия задаются в{' '}
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
            <div className="anomaly-engine-sticky-actions">
              <button type="submit" className="btn primary" disabled={saving}>
                {saving ? 'Сохранение…' : 'Сохранить'}
              </button>
              <button
                type="button"
                className="btn"
                onClick={() => void load()}
                disabled={loading || saving}
                title="Отменить несохранённые правки и загрузить настройки с сервера"
              >
                Отменить правки
              </button>
            </div>

            <h3 className="card-title">Параметры</h3>

            <label className="anomaly-include-acked">
              <input
                type="checkbox"
                checked={settings.enabled}
                onChange={(e) => setSettings({ ...settings, enabled: e.target.checked })}
              />
              Искать аномалии
            </label>

            <div className="form-row">
              <label className="anomaly-engine-field">
                Как часто проверять (мин)
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
              <label className="anomaly-engine-field">
                Дней на привыкание к трафику
                <input
                  type="number"
                  min={1}
                  max={30}
                  value={settings.learning_days}
                  onChange={(e) => setSettings({ ...settings, learning_days: Number(e.target.value) })}
                />
              </label>
              <label className="anomaly-engine-field">
                Не повторять тот же алерт (часов)
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
              Учитывать частные адреса (10.x, 172.16.x, 192.168.x)
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
          </form>
        </>
      ) : null}
    </AdminLayout>
  );
}
