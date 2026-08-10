import { fmtDate, fmtNumber } from '@/lib/format';
import { capacityTone, fmtBytes, fmtLag, num } from './systemFormat';
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
  return (
    <div className="tab-panel active" id="tab-overview" role="tabpanel">
      {showInstallProfile ? (
        <section className="card card-compact" id="installProfileSection">
          <h3 className="card-title">
            Профиль установки{' '}
            <span className="profile-badge" id="profileBadge">
              {stats?.install_profile?.profile_label ||
                stats?.install_profile?.profile ||
                '—'}
            </span>
          </h3>
          <div className="capacity-meter" id="capacityMeter">
            <div className="meter-label">
              Нагрузка к ёмкости:{' '}
              <span id="capacityLabel">
                {fmtNumber(Math.round(eps))} / {fmtNumber(epsMax)} eps
              </span>
            </div>
            <div className="capacity-track">
              <div
                className={`capacity-fill ${capacityTone(capPct)}`}
                id="capacityFill"
                style={{ width: `${Math.min(150, Math.min(100, capPct))}%` }}
              />
            </div>
            <div className="capacity-hint" id="capacityHint">
              {capPct > 125
                ? 'Нагрузка превышает расчётную ёмкость профиля — рассмотрите upgrade или ./scripts/tune-resources.sh'
                : 'Нагрузка близка к лимиту профиля'}
            </div>
          </div>
        </section>
      ) : null}

      <section className="row cols-2">
        <div className="card card-compact">
          <h3 className="card-title">Health компонентов</h3>
          <div className="health-grid" id="componentHealthGrid">
            {([
              {
                name: 'Backend',
                health: backendHealth,
                meta: `goroutines: ${fmtNumber(stats?.backend_info?.num_goroutine)} · heap: ${
                  stats?.backend_info?.heap_alloc_mb != null
                    ? `${Number(stats.backend_info.heap_alloc_mb).toFixed(1)} MB`
                    : '—'
                }`,
              },
              {
                name: 'Ingest',
                health: ingestHealth,
                meta: `conn: ${fmtNumber(ingest.connections)} · lag: ${fmtLag(ingest.lag_sec)}`,
              },
            ] as const).map((item) => {
              const h = item.health || {};
              let stateText = String(h.state_text || 'unknown');
              let css: 'ok' | 'warn' | 'bad' = 'warn';
              if (h.up != null) {
                const up = Number(h.up);
                stateText = up >= 1 ? 'up' : 'down';
                css = up >= 1 ? 'ok' : 'bad';
              } else if (h.state != null) {
                const st = Number(h.state);
                if (st > 0) css = 'ok';
                else if (st < 0) css = 'bad';
                else css = 'warn';
                if (h.state_text == null) stateText = String(h.state);
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
          <div className="container-strip" id="containersRow">
            {CONTAINERS.map((name) => {
              const c = stats?.containers?.[name];
              const up = num(c?.mem_bytes) > 0;
              return (
                <div key={name} className={`container-chip${up ? ' up' : ''}`}>
                  <span className="dot" />
                  <span className="name">{name}</span>
                  <span className="metrics">
                    CPU {c?.cpu_pct != null ? `${Number(c.cpu_pct).toFixed(1)}%` : '—'} · Mem{' '}
                    {c?.mem_bytes != null ? fmtBytes(c.mem_bytes) : '—'}
                  </span>
                </div>
              );
            })}
          </div>
        </div>
      </section>

      <section className="card card-compact edges-card" id="edgesAggSection">
        <h3 className="card-title">
          Агрегаты рёбер{' '}
          <span className="profile-badge" id="edgesAggBadge">
            {edgesBadge}
          </span>
        </h3>
        <div className="kv-grid cols-3" id="edgesAggPrimary">
          <div className="kv-row">
            <span className="k">Raw / agg</span>
            <span className="v">
              {fmtNumber(edges?.raw_rows)} / {fmtNumber(edges?.agg_rows)}
            </span>
          </div>
          <div className="kv-row">
            <span className="k">Карта</span>
            <span className="v">{edges?.map_source || '—'}</span>
          </div>
          <div className="kv-row">
            <span className="k">Backfill</span>
            <span className="v">
              {edges?.days_total
                ? `${fmtNumber(edges.days_done)} / ${fmtNumber(edges.days_total)}`
                : '—'}
            </span>
          </div>
        </div>
        <div className="capacity-hint" id="edgesAggHint">
          {edgesHint}
        </div>
        <details
          className="details-toggle"
          id="edgesAggDetails"
          open={edges?.state === 'running' || edges?.state === 'error'}
        >
          <summary>+ Подробности</summary>
          <div className="kv-grid cols-2" id="edgesAggSecondary">
            <div className="kv-row">
              <span className="k">Сообщение</span>
              <span className="v">{edges?.message || '—'}</span>
            </div>
            <div className="kv-row">
              <span className="k">prefer_agg</span>
              <span className="v">{edges?.prefer_agg ? 'да' : 'нет'}</span>
            </div>
            <div className="kv-row">
              <span className="k">geo prefer_agg</span>
              <span className="v">{edges?.geo_prefer_agg ? 'да' : 'нет'}</span>
            </div>
            <div className="kv-row">
              <span className="k">Обновлено</span>
              <span className="v">{fmtDate(edges?.updated_at)}</span>
            </div>
            <div className="kv-row">
              <span className="k">Старт</span>
              <span className="v">{fmtDate(edges?.started_at)}</span>
            </div>
          </div>
        </details>
      </section>

      <div className="footer-info" id="footerInfoOverview">
        обновлено: {updatedAt ? updatedAt.toLocaleString('ru-RU') : '—'}
        {stats?.backend_info
          ? ` · backend heap: ${Number(stats.backend_info.heap_alloc_mb || 0).toFixed(1)} MB · goroutines: ${fmtNumber(stats.backend_info.num_goroutine)}`
          : ''}
      </div>
    </div>
  );
}
