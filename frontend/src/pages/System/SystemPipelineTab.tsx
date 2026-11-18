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
            <div className="stage-name">Syslog</div>
            <div className="stage-value">{fmtNumber(syslogEps)} EPS</div>
            <div className="stage-meta">
              {num(syslogng.udp_processed) + num(syslogng.tcp_processed) === 0
                ? 'Метрики пусты — пересоздайте контейнер syslog-ng'
                : `UDP: ${fmtNumber(syslogng.udp_processed)} · TCP: ${fmtNumber(syslogng.tcp_processed)}`}
              {num(syslogng.queued) > 0
                ? ` · очередь: ${fmtNumber(syslogng.queued)}${
                    syslogCap > 0 ? ` (${fmtPercent(syslogQueuePct)})` : ''
                  }`
                : ''}
              {num(syslogng.dropped_total) > 0
                ? ` · потери: ${fmtNumber(syslogng.dropped_total)} (${fmtNumber(syslogngDropsPerSec)}/с)`
                : ''}
            </div>
          </div>
          <div className="pipeline-arrow">→</div>
          <div className={`pipeline-stage ${ingestStageStatus}`}>
            <div className="stage-name">Приём</div>
            <div className="stage-value">
              {fmtNumber(rate.events_per_sec || 0)} EPS
            </div>
            <div className="stage-meta">
              соединения: {fmtNumber(ingest.connections)}, буфер: {fmtNumber(buffered)}, очередь:{' '}
              {fmtNumber(qDepth)}/{fmtNumber(qCap)}
              {qCap > 0 ? ` (${fmtPercent(queuePct)})` : ''}
              {num(ingest.dropped_total) > 0
                ? `, потери: ${fmtNumber(ingest.dropped_total)} (${fmtNumber(dropsPerSec)}/с)`
                : ''}
              {num(ingest.buffer_drops_total) > 0
                ? `, потери буфера: ${fmtNumber(ingest.buffer_drops_total)} (${fmtNumber(bufferDropsPerSec)}/с)`
                : ''}
              {num(ingest.circuit_open) >= 1 ? ', запись на паузе' : ''}
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
                  <th>Принято syslog (UDP / TCP)</th>
                  <th>Очередь / потери syslog</th>
                  <th>Скорость приёма, EPS</th>
                  <th>Принято всего</th>
                  <th>Отброшено / отказ auth</th>
                  <th>Последнее событие</th>
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
            Порты настраиваются в Система → Коннекторы. Syslog — счётчики до приёма; скорость — EPS
            по коннектору; очередь и потери — буфер syslog.
          </p>
        </section>
      ) : null}

      <section className="row cols-2">
        <div className="card card-compact">
          <h3 className="card-title">Приём логов</h3>
          <div className="kv-grid cols-2">
            {(
              [
                ['Лаг', fmtLag(ingest.lag_sec)],
                [
                  'Очередь',
                  `${fmtNumber(qDepth)} / ${fmtNumber(qCap)}${
                    qCap > 0 ? ` (${fmtPercent(queuePct)})` : ''
                  }`,
                ],
                [
                  'Очередь, байты',
                  qBytesCap > 0
                    ? `${fmtBytes(qBytes)} / ${fmtBytes(qBytesCap)}`
                    : fmtBytes(qBytes),
                ],
                ['Буфер', fmtNumber(buffered)],
                [
                  'Потери',
                  `${fmtNumber(ingest.dropped_total)} (${fmtNumber(dropsPerSec)}/с)`,
                ],
                [
                  'Потери буфера',
                  `${fmtNumber(ingest.buffer_drops_total)} (${fmtNumber(bufferDropsPerSec)}/с)`,
                ],
                ['Последняя потеря', fmtDropAt(lastDropAt)],
                ['Принято', fmtNumber(ingest.received_total)],
                ['Записано', fmtNumber(ingest.inserted_total)],
                ['Пропущено', fmtNumber(ingest.skipped_total)],
                ['Ошибки разбора', fmtNumber(ingest.parse_errors_total)],
                ['Соединения', fmtNumber(ingest.connections)],
                ['Ошибки разбора (1 ч)', fmtNumber(parseErr1h)],
                ['Запись в БД', num(ingest.circuit_open) >= 1 ? 'на паузе' : 'активна'],
                ['Аптайм', fmtUptime(uptimeSec)],
                ['Очередь syslog', fmtNumber(syslogng.queued)],
                [
                  'Потери syslog',
                  `${fmtNumber(syslogng.dropped_total)} (${fmtNumber(syslogngDropsPerSec)}/с)`,
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
              <span className="k">Логи трафика</span>
              <span className="v">
                {fmtNumber(storage.traffic_logs?.row_count)} /{' '}
                {fmtBytes(storage.traffic_logs?.bytes_on_disk)}
              </span>
            </div>
            <div className="kv-row">
              <span className="k">Активные части</span>
              <span className="v">{fmtNumber(storage.clickhouse?.active_parts)}</span>
            </div>
            <div className="kv-row">
              <span className="k">Geo-диапазоны</span>
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
              ['edges_days', 'Сводки для карты (дни)', 'retEdges'],
              ['parse_errors_days', 'Ошибки разбора (дни)', 'retParseErrors'],
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
          При уменьшении срока старые данные удалятся при следующей очистке в ClickHouse.
          {retention.updated_at ? ` Обновлено: ${fmtDate(retention.updated_at)}` : ''}
        </p>
      </section>
    </div>
  );
}
