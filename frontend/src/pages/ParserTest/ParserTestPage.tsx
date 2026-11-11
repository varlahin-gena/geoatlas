import { useEffect, useMemo, useState } from 'react';
import { fetchParseSamples, runParseTest } from '@/api/parseTest';
import { SettingsLayout } from '@/components/SettingsLayout';
import { useToast } from '@/components/Toast';
import { fmtNumber } from '@/lib/format';
import {
  parseFieldsLabel,
  parseRowStatus,
  parseRowStatusLabel,
  shortParseLine,
} from './parserTestDisplay';
import './parserTest.css';

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
  const [activePreset, setActivePreset] = useState<string | null>(null);
  const [fromParseErrors, setFromParseErrors] = useState(false);

  useEffect(() => {
    document.title = 'ГеоАтлас — Тест парсеров';
    try {
      const pre = localStorage.getItem('nm.parserTest.prefill');
      if (pre) {
        setText(pre);
        setFromParseErrors(true);
        localStorage.removeItem('nm.parserTest.prefill');
      }
    } catch {
      /* ignore */
    }
    fetchParseSamples()
      .then((d) => setSamples(d && typeof d === 'object' ? d : {}))
      .catch(() => {});
  }, []);

  const sampleKeys = useMemo(() => Object.keys(samples).sort(), [samples]);
  const firstPreset = sampleKeys[0];
  const canRun = Boolean(text.trim()) && !busy;

  async function run() {
    if (!text.trim()) return;
    setBusy(true);
    try {
      const res = await runParseTest(text);
      const data = (await res.json().catch(() => ({}))) as ParseTestResult & { error?: string };
      if (!res.ok) {
        toast(data.error || `HTTP ${res.status}`, 'error');
        return;
      }
      setResult(data);
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось проверить строки', 'error');
    } finally {
      setBusy(false);
    }
  }

  function applyPreset(key: string) {
    setText((samples[key] || []).join('\n'));
    setActivePreset(key);
    setFromParseErrors(false);
    setResult(null);
  }

  function applyAllPresets() {
    setText(Object.values(samples).flat().join('\n'));
    setActivePreset('__all__');
    setFromParseErrors(false);
    setResult(null);
  }

  const rows = result?.results || [];

  return (
    <SettingsLayout title="Тест парсеров" plainTitle>
      <div className="page-content-inner">
        <p className="page-lead parser-test-lead">
          Проверка строк тем же набором парсеров, что в приёме логов.
        </p>
        {fromParseErrors ? (
          <p className="parser-test-prefill" role="status">
            Строка подставлена из «Ошибок парсинга».
          </p>
        ) : null}

        <div className="card parser-test-card">
          <span className="parser-test-presets-label" id="parser-presets-label">
            Примеры
          </span>
          <div className="parser-test-presets" role="group" aria-labelledby="parser-presets-label">
            {sampleKeys.map((k) => (
              <button
                key={k}
                type="button"
                className={`btn sm${activePreset === k ? ' is-active' : ''}`}
                aria-pressed={activePreset === k}
                onClick={() => applyPreset(k)}
              >
                {k}
              </button>
            ))}
            {sampleKeys.length > 1 ? (
              <button
                type="button"
                className={`btn sm ghost${activePreset === '__all__' ? ' is-active' : ''}`}
                aria-pressed={activePreset === '__all__'}
                onClick={applyAllPresets}
              >
                Все примеры
              </button>
            ) : null}
          </div>

          <div className="parser-test-field">
            <label htmlFor="parseText">Строки логов (по одной в строке)</label>
            <textarea
              id="parseText"
              rows={12}
              value={text}
              onChange={(e) => {
                setText(e.target.value);
                setActivePreset(null);
                setFromParseErrors(false);
              }}
              placeholder="Вставьте строки логов…"
              spellCheck={false}
            />
          </div>

          <div className="parser-test-actions">
            <button
              type="button"
              className="btn primary"
              disabled={!canRun}
              onClick={() => void run()}
            >
              {busy ? 'Проверяю…' : 'Проверить'}
            </button>
            <button
              type="button"
              className="btn"
              disabled={busy || (!text && !result)}
              onClick={() => {
                setText('');
                setResult(null);
                setActivePreset(null);
                setFromParseErrors(false);
              }}
            >
              Очистить
            </button>
          </div>
        </div>

        <div className="card parser-test-card">
          <h2>Результат</h2>
          {!result ? (
            <div className="parser-test-empty">
              <p>Нет результатов — вставьте лог или выберите пример.</p>
              {firstPreset ? (
                <button type="button" className="btn" onClick={() => applyPreset(firstPreset)}>
                  Пример: {firstPreset}
                </button>
              ) : sampleKeys.length > 1 ? (
                <button type="button" className="btn" onClick={applyAllPresets}>
                  Все примеры
                </button>
              ) : null}
            </div>
          ) : (
            <>
              <div className="parser-test-summary" role="status">
                <span className="parser-test-chip">
                  <strong>{fmtNumber(result.received)}</strong>
                  получено
                </span>
                <span className="parser-test-chip is-ok">
                  <strong>{fmtNumber(result.parsed)}</strong>
                  разобрано
                </span>
                <span className="parser-test-chip">
                  <strong>{fmtNumber(result.skipped)}</strong>
                  пропущено
                </span>
                <span className={`parser-test-chip${(result.errors || 0) > 0 ? ' is-bad' : ''}`}>
                  <strong>{fmtNumber(result.errors)}</strong>
                  ошибок
                </span>
                {result.truncated ? (
                  <span className="parser-test-chip">
                    обрезано (макс. {fmtNumber(result.max_lines)})
                  </span>
                ) : null}
              </div>

              <div className="table-wrap">
                <table className="parser-test-table">
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
                      rows.map((r, i) => {
                        const status = parseRowStatus(r);
                        return (
                          <tr key={i}>
                            <td>
                              <span className={`parser-test-status is-${status}`}>
                                {parseRowStatusLabel(status)}
                              </span>
                            </td>
                            <td>{r.vendor || '—'}</td>
                            <td>{parseFieldsLabel(r)}</td>
                            <td>
                              <span className="parser-test-line" title={r.line || undefined}>
                                {shortParseLine(r.line)}
                              </span>
                            </td>
                          </tr>
                        );
                      })
                    )}
                  </tbody>
                </table>
              </div>
            </>
          )}
        </div>
      </div>
    </SettingsLayout>
  );
}
