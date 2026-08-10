import { fmtDate, fmtNumber } from '@/lib/format';
import type { FailedLogin } from './systemTypes';

export function SystemSecurityTab({ failed }: { failed: FailedLogin[] }) {
  return (
    <div className="tab-panel active" id="tab-security" role="tabpanel">
      <section className="card" id="failedLoginsSection">
        <details className="section-details" id="failedLoginsDetails" open>
          <summary className="card-title" style={{ color: 'var(--red)' }}>
            ■ Неуспешные попытки входа{' '}
            <span id="failedLoginsCount">({failed.length ? failed.length : 'нет'})</span>
          </summary>
          <div id="failedLoginsHost">
            {!failed.length ? (
              <p className="auth-fails-empty empty">Нет неуспешных попыток</p>
            ) : (
              <div className="table-wrap">
                <table className="auth-fails-table">
                  <thead>
                    <tr>
                      <th scope="col">Логин</th>
                      <th scope="col">IP</th>
                      <th scope="col">Count</th>
                      <th scope="col">Первая</th>
                      <th scope="col">Последняя</th>
                      <th scope="col">Блок</th>
                    </tr>
                  </thead>
                  <tbody>
                    {failed.map((f, i) => (
                      <tr key={i}>
                        <td>{f.username}</td>
                        <td>{f.ip}</td>
                        <td>{fmtNumber(f.count)}</td>
                        <td>{fmtDate(f.first_at)}</td>
                        <td>{fmtDate(f.last_at)}</td>
                        <td>
                          {f.locked ? (
                            <span className="badge-locked">locked</span>
                          ) : (
                            '—'
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </details>
      </section>
    </div>
  );
}
