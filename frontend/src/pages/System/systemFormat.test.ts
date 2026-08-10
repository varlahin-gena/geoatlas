import { describe, expect, it } from 'vitest';
import {
  capacityTone,
  dropsTone,
  fmtBytes,
  pipelineIngestStatus,
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
});
