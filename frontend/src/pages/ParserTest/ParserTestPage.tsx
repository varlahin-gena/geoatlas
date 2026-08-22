import { useEffect, useState } from 'react';
import { fetchParseSamples, runParseTest } from '@/api/parseTest';
import { AdminLayout } from '@/components/AdminLayout';
import { DataSectionNav } from '@/components/DataSectionNav';
import { useToast } from '@/components/Toast';
import { fmtNumber } from '@/lib/format';

interface ParseRow {
  n?: number;
  line?: string;
  parsed?: boolean;
  skipped?: boolean;
  vendor?: string;
  reason?: string;
  src_ip?: string;
  dst_ip?: string;
  action?: string;
  proto?: string;
}

interface ParseTestResult {
  received?: number;
  parsed?: number;
  skipped?: number;
  errors?: number;
  truncated?: boolean;
  max_lines?: number;
  results?: ParseRow[];
}

export default function ParserTestPage() {
  const { toast } = useToast();
  const [text, setText] = useState('');
  const [samples, setSamples] = useState<Record<string, string[]>>({});
  const [result, setResult] = useState<ParseTestResult | null>(null);
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
    // SoT: GET /api/parse-samples → map[vendor][]string (not wrapped).
    fetchParseSamples()
      .then((d) => setSamples(d && typeof d === 'object' ? d : {}))
      .catch(() => {});
  }, []);

  async function run() {
    if (!text.trim()) return;
    setBusy(true);
    try {
      // SoT: raw text/plain body (not JSON { text }).
      const res = await runParseTest(text);
      const data = (await res.json().catch(() => ({}))) as ParseTestResult & { error?: string };
      if (!res.ok) {
        toast(data.error || `HTTP ${res.status}`, 'error');
        return;
      }
      setResult(data);
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка', 'error');
    } finally {
      setBusy(false);
    }
  }

  const rows = result?.results || [];

  return (
    <AdminLayout title="Тест парсеров">
      <div className="page-content-inner">
        <DataSectionNav />
        <p className="page-lead">
          Строки прогоняются через тот же реестр парсеров, что и боевой ingest (
          <code>/api/parse-test</code>).
        </p>
        <div className="card">
          <div className="chrome-section-head" style={{ marginBottom: 8 }}>
            <span>Примеры</span>
          </div>
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 12 }}>
            {Object.keys(samples)
              .sort()
              .map((k) => (
                <button
                  key={k}
                  type="button"
                  className="btn"
                  onClick={() => setText((samples[k] || []).join('\n'))}
                >
                  {k}
                </button>
              ))}
            {Object.keys(samples).length > 1 ? (
              <button
                type="button"
                className="btn"
                onClick={() => setText(Object.values(samples).flat().join('\n'))}
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
              {busy ? 'Проверяю…' : 'Проверить'}
            </button>
            <button
              type="button"
              className="btn"
              disabled={busy}
              onClick={() => {
                setText('');
                setResult(null);
              }}
            >
              Очистить
            </button>
          </div>
        </div>
        <div className="card" style={{ marginTop: 16 }}>
          <h2>Результат</h2>
          {!result ? (
            <p className="empty">Нет результатов</p>
          ) : (
            <>
              <p className="hint">
                получено {fmtNumber(result.received)} · разобрано {fmtNumber(result.parsed)} ·
                skipped {fmtNumber(result.skipped)} · ошибок {fmtNumber(result.errors)}
                {result.truncated ? ` · обрезано (max ${result.max_lines})` : ''}
              </p>
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th scope="col">Статус</th>
                      <th scope="col">Вендор</th>
                      <th scope="col">Причина / поля</th>
                      <th scope="col">Строка</th>
                    </tr>
                  </thead>
                  <tbody>
                    {!rows.length ? (
                      <tr>
                        <td colSpan={4} className="empty">
                          Нет строк
                        </td>
                      </tr>
                    ) : (
                      rows.map((r, i) => (
                        <tr key={i}>
                          <td>
                            {r.parsed ? 'parsed' : r.skipped ? 'skipped' : 'error'}
                          </td>
                          <td>{r.vendor || '—'}</td>
                          <td>
                            {r.parsed
                              ? `${r.src_ip || '?'} → ${r.dst_ip || '?'} · ${r.action || '—'} · ${r.proto || '—'}`
                              : r.reason || '—'}
                          </td>
                          <td className="raw">{r.line || ''}</td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            </>
          )}
        </div>
      </div>
    </AdminLayout>
  );
}
