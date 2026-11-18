import { fmtDate, fmtNumber } from '@/lib/format';
import {
  capacityHint,
  capacityTone,
  containerLabel,
  edgesAggMessageLabel,
  edgesMapSourceLabel,
  fmtBytes,
  fmtLag,
  healthStateLabel,
  num,
} from './systemFormat';
import { CONTAINERS, type EdgesAgg, type SystemStats } from './systemTypes';

export function SystemOverviewTab({
  showInstallProfile,
  stats,
  eps,
  epsMax,
  capPct,
  backendHealth,
  ingestHealth,
  ingest,
  edges,
  edgesBadge,
  edgesHint,
  updatedAt,
}: {
  showInstallProfile: boolean;
  stats: SystemStats | null;
  eps: number;
  epsMax: number;
  capPct: number;
  backendHealth: Record<string, unknown>;
  ingestHealth: Record<string, unknown>;
  ingest: Record<string, number>;
  edges?: EdgesAgg;
  edgesBadge: string;
  edgesHint: string;
  updatedAt: Date | null;
}) {
  const backendMeta = [
    `потоки: ${fmtNumber(stats?.backend_info?.num_goroutine)}`,
    stats?.backend_info?.heap_alloc_mb != null
      ? `память: ${Number(stats.backend_info.heap_alloc_mb).toFixed(1)} МБ`
      : 'память: —',
    stats?.backend_info?.geo_index_mb != null
      ? `GeoIP: ${Number(stats.backend_info.geo_index_mb).toFixed(1)} МБ`
      : null,
    stats?.backend_info?.geo_index_ranges != null
      ? `диапазоны: ${fmtNumber(stats.backend_info.geo_index_ranges)}`
      : null,
  ]
    .filter(Boolean)
    .join(' · ');

  return (
    <div className="tab-panel active" role="tabpanel">
      {showInstallProfile ? (
        <section className="card card-compact">
          <h3 className="card-title">
            Профиль установки{' '}
            <span className="profile-badge">
              {String(
                stats?.install_profile?.profile_label ||
                  stats?.install_profile?.profile ||
                  '—',
              )}
            </span>
          </h3>
          <div className="capacity-meter">
            <div className="meter-label">
              Нагрузка к лимиту профиля (EPS):{' '}
              <span>
                {fmtNumber(Math.round(eps))} / {fmtNumber(epsMax)} EPS
              </span>
            </div>
            <div className="capacity-track">
              <div
                className={`capacity-fill ${capacityTone(capPct)}`}
                style={{ width: `${Math.min(150, Math.min(100, capPct))}%` }}
              />
            </div>
            <div className="capacity-hint">{capacityHint(capPct)}</div>
          </div>
        </section>
      ) : null}

      <section className="row cols-2">
        <div className="card card-compact">
          <h3 className="card-title">Состояние сервисов</h3>
          <div className="health-grid">
            {([
              {
                name: 'Сервер',
                health: backendHealth,
                meta: backendMeta,
              },
              {
                name: 'Приём логов',
                health: ingestHealth,
                meta: `соединения: ${fmtNumber(ingest.connections)} · лаг: ${fmtLag(ingest.lag_sec)}`,
              },
            ] as const).map((item) => {
              const h = item.health || {};
              let stateText = healthStateLabel(
                h.state_text != null ? String(h.state_text) : 'unknown',
              );
              let css: 'ok' | 'warn' | 'bad' = 'warn';
              if (h.up != null) {
                const up = Number(h.up);
                stateText = healthStateLabel(up >= 1 ? 'up' : 'down');
                css = up >= 1 ? 'ok' : 'bad';
              } else if (h.state != null) {
                const st = Number(h.state);
                if (st > 0) css = 'ok';
                else if (st < 0) css = 'bad';
                else css = 'warn';
                if (h.state_text == null) stateText = healthStateLabel(String(h.state));
              }
              return (
                <div key={item.name} className={`health-card ${css}`}>
                  <div className="health-head">
                    <span className="health-name">{item.name}</span>
                    <span className="health-state">{stateText}</span>
                  </div>
                  <div className="health-meta">{item.meta}</div>
                </div>
              );
            })}
          </div>
        </div>

        <div className="card card-compact">
          <h3 className="card-title">Контейнеры</h3>
          <div className="container-strip">
            {CONTAINERS.map((name) => {
              const c = stats?.containers?.[name];
              const up = num(c?.mem_bytes) > 0;
              return (
                <div key={name} className={`container-chip${up ? ' up' : ''}`}>
                  <span className="dot" />
                  <span className="name">{containerLabel(name)}</span>
                  <span className="metrics">
                    CPU {c?.cpu_pct != null ? `${Number(c.cpu_pct).toFixed(1)}%` : '—'} · память{' '}
                    {c?.mem_bytes != null ? fmtBytes(c.mem_bytes) : '—'}
                  </span>
                </div>
              );
            })}
          </div>
        </div>
      </section>

      <section className="card card-compact edges-card">
        <h3 className="card-title">
          Сводки для карты{' '}
          <span className="profile-badge">
            {edgesBadge}
          </span>
        </h3>
        <div className="kv-grid cols-3">
          <div className="kv-row">
            <span className="k">Логи / сводки</span>
            <span className="v">
              {fmtNumber(edges?.raw_rows)} / {fmtNumber(edges?.agg_rows)}
            </span>
          </div>
          <div className="kv-row">
            <span className="k">Источник карты</span>
            <span className="v">{edgesMapSourceLabel(edges?.map_source)}</span>
          </div>
          <div className="kv-row">
            <span className="k">Дозаполнение</span>
            <span className="v">
              {edges?.days_total
                ? `${fmtNumber(edges.days_done)} / ${fmtNumber(edges.days_total)}`
                : '—'}
            </span>
          </div>
        </div>
        <div className="capacity-hint">
          {edgesHint}
        </div>
        <details
          className="details-toggle"
          open={edges?.state === 'running' || edges?.state === 'error'}
        >
          <summary>Подробности</summary>
          <div className="kv-grid cols-2">
            <div className="kv-row">
              <span className="k">Статус</span>
              <span className="v">{edgesAggMessageLabel(edges?.message)}</span>
            </div>
            <div className="kv-row">
              <span className="k">Карта на сводках</span>
              <span className="v">{edges?.prefer_agg ? 'да' : 'нет'}</span>
            </div>
            <div className="kv-row">
              <span className="k">Гео на сводках</span>
              <span className="v">{edges?.geo_prefer_agg ? 'да' : 'нет'}</span>
            </div>
            <div className="kv-row">
              <span className="k">Обновлено</span>
              <span className="v">{fmtDate(edges?.updated_at)}</span>
            </div>
            <div className="kv-row">
              <span className="k">Начато</span>
              <span className="v">{fmtDate(edges?.started_at)}</span>
            </div>
          </div>
        </details>
      </section>

      <div className="footer-info">
        Обновлено: {updatedAt ? updatedAt.toLocaleString('ru-RU') : '—'}
      </div>
    </div>
  );
}
