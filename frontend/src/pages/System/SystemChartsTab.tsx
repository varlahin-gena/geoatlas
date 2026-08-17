import type { RefObject } from 'react';

export function SystemChartsTab({
  chartEvents,
  chartLag,
  chartCpu,
  chartMem,
  chartBuffer,
  chartStorage,
}: {
  chartEvents: RefObject<HTMLDivElement | null>;
  chartLag: RefObject<HTMLDivElement | null>;
  chartCpu: RefObject<HTMLDivElement | null>;
  chartMem: RefObject<HTMLDivElement | null>;
  chartBuffer: RefObject<HTMLDivElement | null>;
  chartStorage: RefObject<HTMLDivElement | null>;
}) {
  return (
    <div className="tab-panel active" role="tabpanel">
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
