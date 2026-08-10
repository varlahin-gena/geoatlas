import { useEffect, useState } from 'react';
import { apiFetch } from '@/api/client';
import { AdminLayout } from '@/components/AdminLayout';
import { useToast } from '@/components/Toast';

interface ParseResult {
  line?: string;
  ok?: boolean;
  skipped?: boolean;
  vendor?: string;
  reason?: string;
  log?: Record<string, unknown>;
}

export default function ParserTestPage() {
  const { toast } = useToast();
  const [text, setText] = useState('');
  const [samples, setSamples] = useState<Record<string, string>>({});
  const [results, setResults] = useState<ParseResult[]>([]);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    document.title = 'ГеоАтлас — Тест парсеров';
    try {
      const pre = localStorage.getItem('nm.parserTest.prefill');
      if (pre) {
        setText(pre);
        localStorage.removeItem('nm.parserTest.prefill');
      }
    } catch {
      /* ignore */
    }
    apiFetch<{ samples?: Record<string, string> }>('/api/parse-samples')
      .then((d) => setSamples(d.samples || {}))
      .catch(() => {});
  }, []);

  async function run() {
    setBusy(true);
    try {
      const data = await apiFetch<{ results?: ParseResult[] }>('/api/parse-test', {
        method: 'POST',
        body: JSON.stringify({ text }),
      });
      setResults(data.results || []);
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка', 'error');
    } finally {
      setBusy(false);
    }
  }

  return (
    <AdminLayout title="Тест парсеров">
      <div className="page-content-inner">
        <h1>Тест парсеров</h1>
        <p className="page-lead">
          Строки прогоняются через тот же реестр парсеров, что и боевой ingest (
          <code>/api/parse-test</code>).
        </p>
        <div className="card">
          <div className="chrome-section-head" style={{ marginBottom: 8 }}>
            <span>Примеры</span>
          </div>
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 12 }}>
            {Object.keys(samples).map((k) => (
              <button key={k} type="button" className="btn" onClick={() => setText(samples[k])}>
                {k}
              </button>
            ))}
            {Object.keys(samples).length > 1 ? (
              <button
                type="button"
                className="btn"
                onClick={() => setText(Object.values(samples).join('\n'))}
              >
                все примеры
              </button>
            ) : null}
          </div>
          <label className="field-label" htmlFor="parseText" style={{ display: 'block', marginBottom: 6 }}>
            СТРОКИ ЛОГОВ (ПО ОДНОЙ В СТРОКЕ)
          </label>
          <textarea
            id="parseText"
            rows={12}
            style={{ width: '100%' }}
            value={text}
            onChange={(e) => setText(e.target.value)}
            placeholder="Вставьте строки логов…"
          />
          <div style={{ marginTop: 8, display: 'flex', gap: 8 }}>
            <button type="button" className="btn primary" disabled={busy} onClick={() => void run()}>
              Проверить
            </button>
            <button type="button" className="btn" disabled={busy} onClick={() => { setText(''); setResults([]); }}>
              Очистить
            </button>
          </div>
        </div>
        <div className="card" style={{ marginTop: 16 }}>
          <h2>Результат</h2>
          {!results.length ? (
            <p className="empty">Нет результатов</p>
          ) : (
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th scope="col">Статус</th>
                    <th scope="col">Вендор</th>
                    <th scope="col">Причина</th>
                    <th scope="col">Строка</th>
                  </tr>
                </thead>
                <tbody>
                  {results.map((r, i) => (
                    <tr key={i}>
                      <td>{r.ok ? 'parsed' : r.skipped ? 'skipped' : 'error'}</td>
                      <td>{r.vendor || '—'}</td>
                      <td>{r.reason || '—'}</td>
                      <td className="raw">{r.line || ''}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    </AdminLayout>
  );
}
