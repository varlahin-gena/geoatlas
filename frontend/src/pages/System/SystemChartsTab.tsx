import type { RefObject } from 'react';
import { PERIODS } from './systemTypes';

export function SystemChartsTab({
  period,
  onPeriodChange,
  chartEvents,
  chartLag,
  chartCpu,
  chartMem,
  chartBuffer,
  chartStorage,
}: {
  period: string;
  onPeriodChange: (period: string) => void;
  chartEvents: RefObject<HTMLDivElement | null>;
  chartLag: RefObject<HTMLDivElement | null>;
  chartCpu: RefObject<HTMLDivElement | null>;
  chartMem: RefObject<HTMLDivElement | null>;
  chartBuffer: RefObject<HTMLDivElement | null>;
  chartStorage: RefObject<HTMLDivElement | null>;
}) {
  return (
    <div className="tab-panel active" role="tabpanel">
      <div className="charts-period-bar" role="group" aria-label="Период графиков">
        <div className="period-tabs">
          {PERIODS.map(([v, label]) => (
            <button
              key={v}
              type="button"
              data-period={v}
              className={period === v ? 'active' : ''}
              onClick={() => onPeriodChange(v)}
            >
              {label}
            </button>
          ))}
        </div>
      </div>
      <section className="row cols-2">
        <div className="card chart-card">
          <div className="chart-host" ref={chartEvents} style={{ height: 240 }} />
        </div>
        <div className="card chart-card">
          <div className="chart-host" ref={chartLag} style={{ height: 240 }} />
        </div>
      </section>
      <section className="row cols-2">
        <div className="card chart-card">
          <div className="chart-host" ref={chartCpu} style={{ height: 280 }} />
        </div>
        <div className="card chart-card">
          <div className="chart-host" ref={chartMem} style={{ height: 280 }} />
        </div>
      </section>
      <section className="row cols-2">
        <div className="card chart-card">
          <div className="chart-host" ref={chartBuffer} style={{ height: 200 }} />
        </div>
        <div className="card chart-card">
          <div className="chart-host" ref={chartStorage} style={{ height: 200 }} />
        </div>
      </section>
    </div>
  );
}
