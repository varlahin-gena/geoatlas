import { describe, expect, it } from 'vitest';
import {
  capacityHint,
  capacityTone,
  containerLabel,
  dropsTone,
  edgesAggBadge,
  edgesAggHint,
  edgesAggMessageLabel,
  edgesMapSourceLabel,
  fmtBytes,
  healthStateLabel,
  pipelineIngestStatus,
  pipelineSyslogStatus,
  queueTone,
} from './systemFormat';

describe('systemFormat', () => {
  it('fmtBytes formats units', () => {
    expect(fmtBytes(512)).toMatch(/Б$/);
    expect(fmtBytes(2048)).toMatch(/КБ$/);
    expect(fmtBytes(2 * 1024 * 1024)).toMatch(/МБ$/);
    expect(fmtBytes(2 * 1024 * 1024 * 1024)).toMatch(/ГБ$/);
  });

  it('queueTone thresholds', () => {
    expect(queueTone(0, 0)).toBe('ok');
    expect(queueTone(50, 100)).toBe('ok');
    expect(queueTone(75, 100)).toBe('warn');
    expect(queueTone(90, 100)).toBe('bad');
  });

  it('dropsTone thresholds', () => {
    expect(dropsTone(0, 0)).toBe('ok');
    expect(dropsTone(1, 0)).toBe('warn');
    expect(dropsTone(50, 50)).toBe('bad');
  });

  it('capacityTone thresholds', () => {
    expect(capacityTone(50)).toBe('ok');
    expect(capacityTone(90)).toBe('warn');
    expect(capacityTone(125)).toBe('bad');
  });

  it('pipelineIngestStatus', () => {
    expect(pipelineIngestStatus({}, {}, 0)).toBe('ok');
    expect(pipelineIngestStatus({ drops_per_sec: 1 }, {}, 0)).toBe('warn');
    expect(pipelineIngestStatus({ drops_per_sec: 100 }, {}, 0)).toBe('bad');
    expect(pipelineIngestStatus({}, {}, 0.9)).toBe('bad');
    expect(pipelineIngestStatus({}, { buffered_lines: 20000 }, 0)).toBe('warn');
  });

  it('pipelineSyslogStatus', () => {
    expect(pipelineSyslogStatus({}, undefined, 0, 0)).toBe('ok');
    expect(pipelineSyslogStatus({}, 0, 0, 0)).toBe('bad');
    expect(pipelineSyslogStatus({ dropped_total: 1 }, 1, 0, 0)).toBe('warn');
    expect(pipelineSyslogStatus({}, 1, 0, 1)).toBe('warn');
    expect(pipelineSyslogStatus({}, 1, 0, 100)).toBe('bad');
    expect(pipelineSyslogStatus({ queued: 80 }, 1, 50, 0)).toBe('warn');
    expect(pipelineSyslogStatus({ queued: 90 }, 1, 50, 0)).toBe('bad');
  });

  it('edges agg copy stays human-readable', () => {
    expect(edgesAggBadge({ state: 'ready' })).toBe('готово');
    expect(edgesAggBadge({ state: 'running', phase: 'backfill' })).toBe('в работе · заполнение');
    expect(edgesMapSourceLabel('edges_daily')).toBe('суточные сводки');
    expect(edgesMapSourceLabel('traffic_logs')).toBe('подробные логи');
    expect(edgesAggMessageLabel('up to date')).toMatch(/актуальны/i);
    expect(edgesAggHint({ state: 'ready', message: 'up to date' })).toMatch(/суточные/i);
    expect(edgesAggHint({ state: 'running', phase: 'schema' })).toMatch(/подробных логах/i);
  });

  it('overview health and capacity copy stays human-readable', () => {
    expect(healthStateLabel('up')).toBe('работает');
    expect(healthStateLabel('down')).toBe('недоступен');
    expect(healthStateLabel('stopped')).toBe('остановлен');
    expect(healthStateLabel('unknown')).toBe('неизвестно');
    expect(containerLabel('backend')).toBe('Сервер');
    expect(containerLabel('syslog-ng')).toBe('Syslog');
    expect(capacityHint(130)).toMatch(/выше лимита/i);
    expect(capacityHint(95)).toMatch(/близка к лимиту/i);
  });
});
