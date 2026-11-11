import type { FormEvent } from 'react';
import { fmtDate, fmtNumber } from '@/lib/format';
import {
  fmtBytes,
  fmtDropAt,
  fmtLag,
  fmtPercent,
  fmtUptime,
  num,
} from './systemFormat';
import type { ConnectorPipelineRow } from '@/api/systemTypes';
import type { Retention } from './systemTypes';

export function SystemPipelineTab({
  syslogEps,
  syslogStageStatus,
  syslogng,
  syslogngDropsPerSec,
  syslogngFifo,
  rate,
  ingest,
  ingestStageStatus,
  buffered,
  qDepth,
  qCap,
  queuePct,
  dropsPerSec,
  bufferDropsPerSec,
  storage,
  qBytes,
  qBytesCap,
  lastDropAt,
  parseErr1h,
  uptimeSec,
  connectors,
  retention,
  setRetention,
  saveRetention,
}: {
  syslogEps: number;
  syslogStageStatus: 'ok' | 'warn' | 'bad';
  syslogng: Record<string, number>;
  syslogngDropsPerSec: number;
  syslogngFifo: number;
  rate: Record<string, number>;
  ingest: Record<string, number>;
  ingestStageStatus: 'ok' | 'warn' | 'bad';
  buffered: number;
  qDepth: number;
  qCap: number;
  queuePct: number;
  dropsPerSec: number;
  bufferDropsPerSec: number;
  storage: Record<string, Record<string, number>>;
  qBytes: number;
  qBytesCap: number;
  lastDropAt: string;
  parseErr1h: number;
  uptimeSec: number;
  connectors: ConnectorPipelineRow[];
  retention: Retention;
  setRetention: (r: Retention) => void;
  saveRetention: (e: FormEvent) => void | Promise<void>;
}) {
  const syslogCap = syslogngFifo * 2;
  const syslogQueuePct = syslogCap > 0 ? num(syslogng.queued) / syslogCap : 0;
  return (
    <div className="tab-panel active" role="tabpanel">
      <section className="card card-compact">
        <h3 className="card-title">Конвейер</h3>
        <div className="pipeline">
          <div className={`pipeline-stage ${syslogStageStatus}`}>
            <div className="stage-name">Syslog-NG</div>
            <div className="stage-value">{fmtNumber(syslogEps)} eps</div>
            <div className="stage-meta">
              udp: {fmtNumber(rate.udp_events_per_sec)}/s · tcp:{' '}
              {fmtNumber(rate.tcp_events_per_sec)}/s
              {num(syslogng.queued) > 0
                ? ` · q: ${fmtNumber(syslogng.queued)}${
                    syslogCap > 0 ? ` (${fmtPercent(syslogQueuePct)})` : ''
                  }`
                : ''}
              {num(syslogng.dropped_total) > 0
                ? ` · drop: ${fmtNumber(syslogng.dropped_total)} (${fmtNumber(syslogngDropsPerSec)}/s)`
                : ''}
            </div>
          </div>
          <div className="pipeline-arrow">→</div>
          <div className={`pipeline-stage ${ingestStageStatus}`}>
            <div className="stage-name">Backend Ingest</div>
            <div className="stage-value">
              {fmtNumber(rate.events_per_sec || 0)} eps
            </div>
            <div className="stage-meta">
              conn: {fmtNumber(ingest.connections)}, buf: {fmtNumber(buffered)}, q:{' '}
              {fmtNumber(qDepth)}/{fmtNumber(qCap)}
              {qCap > 0 ? ` (${fmtPercent(queuePct)})` : ''}
              {num(ingest.dropped_total) > 0
                ? `, drop: ${fmtNumber(ingest.dropped_total)} (${fmtNumber(dropsPerSec)}/s)`
                : ''}
              {num(ingest.buffer_drops_total) > 0
                ? `, buffer_drop: ${fmtNumber(ingest.buffer_drops_total)} (${fmtNumber(bufferDropsPerSec)}/s)`
                : ''}
              {num(ingest.circuit_open) >= 1 ? ', circuit: open' : ''}
            </div>
          </div>
          <div className="pipeline-arrow">→</div>
          <div className="pipeline-stage ok">
            <div className="stage-name">ClickHouse</div>
            <div className="stage-value">
              {fmtNumber(storage.traffic_logs?.row_count)}
            </div>
            <div className="stage-meta">строк в БД</div>
          </div>
        </div>
      </section>

      {connectors.length > 0 ? (
        <section className="card card-compact">
          <h3 className="card-title">По коннекторам</h3>
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Syslog processed (udp/tcp)</th>
                  <th>Syslog q / drop</th>
                  <th>Backend EPS</th>
                  <th>Backend recv</th>
                  <th>Drop / auth</th>
                  <th>Last event</th>
                </tr>
              </thead>
              <tbody>
                {connectors.map((c) => (
                  <tr key={c.id}>
                    <td>
                      <code>{c.id}</code>
                    </td>
                    <td>
                      {fmtNumber(c.syslog_udp_processed)} / {fmtNumber(c.syslog_tcp_processed)}
                    </td>
                    <td>
                      {fmtNumber(c.syslog_queued)} / {fmtNumber(c.syslog_dropped)}
                    </td>
                    <td>{fmtNumber(c.received_per_sec)}</td>
                    <td>{fmtNumber(c.received_total)}</td>
                    <td>
                      {fmtNumber(c.dropped_total)} / {fmtNumber(c.auth_rejected_total)}
                    </td>
                    <td>{c.last_event_at ? fmtDate(c.last_event_at) : '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p className="hint">
            Управление портами — в разделе Система → Коннекторы. Syslog processed — накопленные
            счётчики source; Backend EPS — received/s с атрибуцией по маркеру; recv — накопительно.
          </p>
        </section>
      ) : null}

      <section className="row cols-2">
        <div className="card card-compact">
          <h3 className="card-title">Ingest</h3>
          <div className="kv-grid cols-2">
            {(
              [
                ['Лаг', fmtLag(ingest.lag_sec)],
                [
                  'Queue',
                  `${fmtNumber(qDepth)} / ${fmtNumber(qCap)}${
                    qCap > 0 ? ` (${fmtPercent(queuePct)})` : ''
                  }`,
                ],
                [
                  'Queue bytes',
                  qBytesCap > 0
                    ? `${fmtBytes(qBytes)} / ${fmtBytes(qBytesCap)}`
                    : fmtBytes(qBytes),
                ],
                ['Buffered', fmtNumber(buffered)],
                [
                  'Dropped',
                  `${fmtNumber(ingest.dropped_total)} (${fmtNumber(dropsPerSec)}/s)`,
                ],
                [
                  'Buffer drops',
                  `${fmtNumber(ingest.buffer_drops_total)} (${fmtNumber(bufferDropsPerSec)}/s)`,
                ],
                ['Last drop', fmtDropAt(lastDropAt)],
                ['Received', fmtNumber(ingest.received_total)],
                ['Inserted', fmtNumber(ingest.inserted_total)],
                ['Skipped', fmtNumber(ingest.skipped_total)],
                ['Parse err.', fmtNumber(ingest.parse_errors_total)],
                ['Connections', fmtNumber(ingest.connections)],
                ['Parse (1h)', fmtNumber(parseErr1h)],
                ['Circuit', num(ingest.circuit_open) >= 1 ? 'open' : 'closed'],
                ['Uptime', fmtUptime(uptimeSec)],
                ['syslog-ng queued', fmtNumber(syslogng.queued)],
                [
                  'syslog-ng dropped',
                  `${fmtNumber(syslogng.dropped_total)} (${fmtNumber(syslogngDropsPerSec)}/s)`,
                ],
              ] as const
            ).map(([k, v]) => (
              <div className="kv-row" key={k}>
                <span className="k">{k}</span>
                <span className="v">{v}</span>
              </div>
            ))}
          </div>
        </div>
        <div className="card card-compact">
          <h3 className="card-title">Хранилище</h3>
          <div className="kv-grid cols-2">
            <div className="kv-row">
              <span className="k">traffic_logs</span>
              <span className="v">
                {fmtNumber(storage.traffic_logs?.row_count)} /{' '}
                {fmtBytes(storage.traffic_logs?.bytes_on_disk)}
              </span>
            </div>
            <div className="kv-row">
              <span className="k">active parts</span>
              <span className="v">{fmtNumber(storage.clickhouse?.active_parts)}</span>
            </div>
            <div className="kv-row">
              <span className="k">geo_ranges</span>
              <span className="v">
                {fmtNumber(storage.geo_ranges?.row_count)} диапазонов
              </span>
            </div>
          </div>
        </div>
      </section>

      <section className="card card-compact">
        <h3 className="card-title">Срок хранения</h3>
        <form className="form-row" onSubmit={saveRetention}>
          {(
            [
              ['traffic_logs_days', 'Логи трафика (дни)', 'retTrafficLogs'],
              ['edges_days', 'Агрегаты рёбер (дни)', 'retEdges'],
              ['parse_errors_days', 'Ошибки парсинга (дни)', 'retParseErrors'],
              ['system_metrics_days', 'Метрики системы (дни)', 'retSystemMetrics'],
            ] as const
          ).map(([key, label, id]) => (
            <div className="field" key={key}>
              <label htmlFor={id}>{label}</label>
              <input
                id={id}
                name={key}
                type="number"
                min={1}
                max={730}
                value={Number(retention[key] ?? 1)}
                onChange={(e) =>
                  setRetention({ ...retention, [key]: Number(e.target.value) })
                }
              />
              <span className="field-hint">сейчас: {Number(retention[key] ?? 0)} дн.</span>
            </div>
          ))}
          <button type="submit" className="btn primary">
            Сохранить
          </button>
        </form>
        <p className="hint">
          Уменьшение TTL удалит старые партиции при следующем merge/drop в ClickHouse.
          {retention.updated_at ? ` Обновлено: ${fmtDate(retention.updated_at)}` : ''}
        </p>
      </section>
    </div>
  );
}
