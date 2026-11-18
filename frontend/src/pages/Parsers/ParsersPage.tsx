import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react';
import {
  createParser,
  deleteParser,
  dryRunParser,
  listParsers,
  updateParser,
  type ParserItem,
  type ParserKind,
  type ParserSpec,
  type ParseDryRunResult,
} from '@/api/parsers';
import { SettingsLayout } from '@/components/SettingsLayout';
import { EmptyState, TableLoadRows } from '@/components/Skeleton';
import { useToast } from '@/components/Toast';
import './parsers.css';

const emptyCEFSpec = (): ParserSpec => ({
  kind: 'cef',
  match: { cef_vendor: '' },
  fields: { src: 'src', dst: 'dst', spt: 'spt', dpt: 'dpt', act: 'act', proto: 'proto' },
  transforms: { time_unit: 'auto' },
  samples: [],
});

const emptyKVSpec = (): ParserSpec => ({
  kind: 'kv',
  match: { required_keys: ['src', 'dst'] },
  fields: { src: 'src', dst: 'dst', spt: 'spt', dpt: 'dpt', act: 'act', proto: 'proto', rule: 'rule' },
  samples: [],
});

type Draft = {
  id: string;
  name: string;
  vendor: string;
  enabled: boolean;
  priority: number;
  kind: ParserKind;
  cefVendor: string;
  requiredKeys: string;
  fieldsJSON: string;
  transformsJSON: string;
  samplesText: string;
  dryText: string;
};

function draftFromItem(p: ParserItem): Draft {
  const spec = p.spec ?? emptyCEFSpec();
  return {
    id: p.id,
    name: p.name,
    vendor: p.vendor,
    enabled: p.enabled,
    priority: p.priority,
    kind: spec.kind || 'cef',
    cefVendor: spec.match?.cef_vendor || '',
    requiredKeys: (spec.match?.required_keys || []).join(','),
    fieldsJSON: JSON.stringify(spec.fields || {}, null, 2),
    transformsJSON: JSON.stringify(spec.transforms || {}, null, 2),
    samplesText: (spec.samples || []).join('\n'),
    dryText: (spec.samples || [])[0] || '',
  };
}

function newDraft(): Draft {
  return {
    id: '',
    name: '',
    vendor: '',
    enabled: true,
    priority: 250,
    kind: 'cef',
    cefVendor: '',
    requiredKeys: 'src,dst',
    fieldsJSON: JSON.stringify(emptyCEFSpec().fields, null, 2),
    transformsJSON: JSON.stringify({ time_unit: 'auto' }, null, 2),
    samplesText: '',
    dryText: '',
  };
}

function buildSpec(d: Draft): ParserSpec {
  const fields = JSON.parse(d.fieldsJSON || '{}');
  const transforms = JSON.parse(d.transformsJSON || '{}');
  const samples = d.samplesText
    .split('\n')
    .map((s) => s.trim())
    .filter(Boolean);
  if (d.kind === 'kv') {
    return {
      kind: 'kv',
      match: {
        required_keys: d.requiredKeys
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean),
      },
      fields,
      transforms,
      samples,
    };
  }
  return {
    kind: 'cef',
    match: { cef_vendor: d.cefVendor.trim() },
    fields,
    transforms,
    samples,
  };
}

export default function ParsersPage() {
  const { toast } = useToast();
  const [items, setItems] = useState<ParserItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [editing, setEditing] = useState<Draft | null>(null);
  const [isCreate, setIsCreate] = useState(false);
  const [dryResult, setDryResult] = useState<ParseDryRunResult | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await listParsers();
      setItems(res.parsers || []);
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось загрузить парсеры', 'error');
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    void load();
  }, [load]);

  const sorted = useMemo(
    () => [...items].sort((a, b) => a.priority - b.priority || a.id.localeCompare(b.id)),
    [items],
  );

  async function toggleEnabled(p: ParserItem) {
    if (!p.editable) return;
    setBusy(true);
    try {
      await updateParser(p.id, { enabled: !p.enabled });
      toast(p.enabled ? 'Отключён' : 'Включён', 'success');
      await load();
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка обновления', 'error');
    } finally {
      setBusy(false);
    }
  }

  async function onDelete(p: ParserItem) {
    if (!p.editable) return;
    if (!window.confirm(`Удалить парсер «${p.name}» (${p.id})?`)) return;
    setBusy(true);
    try {
      await deleteParser(p.id);
      toast('Удалён', 'success');
      if (editing?.id === p.id) setEditing(null);
      await load();
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Ошибка удаления', 'error');
    } finally {
      setBusy(false);
    }
  }

  async function onSave(e: FormEvent) {
    e.preventDefault();
    if (!editing) return;
    setBusy(true);
    setDryResult(null);
    try {
      const spec = buildSpec(editing);
      if (isCreate) {
        await createParser({
          id: editing.id,
          name: editing.name,
          vendor: editing.vendor || editing.id,
          enabled: editing.enabled,
          priority: editing.priority,
          spec,
        });
        toast('Создан', 'success');
      } else {
        await updateParser(editing.id, {
          name: editing.name,
          vendor: editing.vendor,
          enabled: editing.enabled,
          priority: editing.priority,
          spec,
        });
        toast('Сохранён', 'success');
      }
      setIsCreate(false);
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Ошибка сохранения', 'error');
    } finally {
      setBusy(false);
    }
  }

  async function onDryRun() {
    if (!editing) return;
    setBusy(true);
    try {
      const spec = buildSpec(editing);
      const res = await dryRunParser({
        text: editing.dryText,
        spec,
        vendor: editing.vendor || editing.id || 'dry-run',
        name: editing.name || 'dry-run',
      });
      setDryResult(res);
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Dry-run failed', 'error');
    } finally {
      setBusy(false);
    }
  }

  return (
    <SettingsLayout title="Парсеры">
      <p className="page-lead parsers-lead">
        CEF/KV-правила в ClickHouse; Cisco ASA/FTD — встроенный код. Порядок = priority (меньше —
        раньше). После сохранения registry перезагружается без рестарта ingest.
      </p>

      <div className="parsers-toolbar">
        <button
          type="button"
          className="btn primary"
          disabled={busy}
          onClick={() => {
            setIsCreate(true);
            setEditing(newDraft());
            setDryResult(null);
          }}
        >
          Добавить правило
        </button>
        <button type="button" className="btn" disabled={busy || loading} onClick={() => void load()}>
          Обновить
        </button>
      </div>

      <div className="card">
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Priority</th>
                <th>ID</th>
                <th>Имя</th>
                <th>Vendor</th>
                <th>Kind</th>
                <th>Источник</th>
                <th>Вкл</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <TableLoadRows cols={8} rows={5} loading={loading} onRetry={() => void load()} />
              ) : sorted.length === 0 ? (
                <tr>
                  <td colSpan={8} className="empty">
                    <EmptyState compact title="Нет парсеров" description="Добавьте CEF/KV правило." />
                  </td>
                </tr>
              ) : (
                sorted.map((p) => (
                  <tr key={p.id}>
                    <td>{p.priority}</td>
                    <td>
                      <code>{p.id}</code>
                    </td>
                    <td>{p.name}</td>
                    <td>{p.vendor}</td>
                    <td>{p.kind || '—'}</td>
                    <td>
                      {p.source === 'builtin' ? (
                        <span className="parsers-badge is-builtin">код</span>
                      ) : p.seed ? (
                        <span className="parsers-badge is-seed">seed</span>
                      ) : (
                        <span className="parsers-badge">db</span>
                      )}
                    </td>
                    <td>
                      <input
                        type="checkbox"
                        checked={p.enabled}
                        disabled={!p.editable || busy}
                        onChange={() => void toggleEnabled(p)}
                        aria-label={`Включить ${p.id}`}
                      />
                    </td>
                    <td>
                      <div className="parsers-actions">
                        {p.editable ? (
                          <>
                            <button
                              type="button"
                              className="btn"
                              disabled={busy}
                              onClick={() => {
                                setIsCreate(false);
                                setEditing(draftFromItem(p));
                                setDryResult(null);
                              }}
                            >
                              Изменить
                            </button>
                            <button
                              type="button"
                              className="btn danger"
                              disabled={busy}
                              onClick={() => void onDelete(p)}
                            >
                              Удалить
                            </button>
                          </>
                        ) : (
                          <span className="hint">только код</span>
                        )}
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {editing ? (
        <div className="card" style={{ marginTop: '1rem' }}>
          <h3>{isCreate ? 'Новое правило' : `Правило ${editing.id}`}</h3>
          <form className="parsers-form-grid" onSubmit={(e) => void onSave(e)}>
            <div className="parsers-form-row">
              <label>
                ID
                <input
                  value={editing.id}
                  disabled={!isCreate || busy}
                  required
                  onChange={(e) => setEditing({ ...editing, id: e.target.value })}
                />
              </label>
              <label>
                Имя
                <input
                  value={editing.name}
                  required
                  disabled={busy}
                  onChange={(e) => setEditing({ ...editing, name: e.target.value })}
                />
              </label>
              <label>
                Vendor
                <input
                  value={editing.vendor}
                  disabled={busy}
                  onChange={(e) => setEditing({ ...editing, vendor: e.target.value })}
                />
              </label>
              <label>
                Priority
                <input
                  type="number"
                  value={editing.priority}
                  disabled={busy}
                  onChange={(e) => setEditing({ ...editing, priority: Number(e.target.value) || 0 })}
                />
              </label>
              <label>
                Kind
                <select
                  value={editing.kind}
                  disabled={busy}
                  onChange={(e) => {
                    const kind = e.target.value as ParserKind;
                    setEditing({
                      ...editing,
                      kind,
                      fieldsJSON: JSON.stringify(
                        (kind === 'kv' ? emptyKVSpec() : emptyCEFSpec()).fields,
                        null,
                        2,
                      ),
                    });
                  }}
                >
                  <option value="cef">cef</option>
                  <option value="kv">kv</option>
                </select>
              </label>
              <label style={{ flexDirection: 'row', alignItems: 'center', gap: '0.5rem' }}>
                <input
                  type="checkbox"
                  checked={editing.enabled}
                  disabled={busy}
                  onChange={(e) => setEditing({ ...editing, enabled: e.target.checked })}
                />
                Включён
              </label>
            </div>

            {editing.kind === 'cef' ? (
              <label>
                CEF Vendor
                <input
                  value={editing.cefVendor}
                  required
                  disabled={busy}
                  onChange={(e) => setEditing({ ...editing, cefVendor: e.target.value })}
                />
              </label>
            ) : (
              <label>
                Required keys (через запятую)
                <input
                  value={editing.requiredKeys}
                  required
                  disabled={busy}
                  onChange={(e) => setEditing({ ...editing, requiredKeys: e.target.value })}
                />
              </label>
            )}

            <label>
              Fields (JSON)
              <textarea
                rows={8}
                value={editing.fieldsJSON}
                disabled={busy}
                onChange={(e) => setEditing({ ...editing, fieldsJSON: e.target.value })}
              />
            </label>
            <label>
              Transforms (JSON)
              <textarea
                rows={6}
                value={editing.transformsJSON}
                disabled={busy}
                onChange={(e) => setEditing({ ...editing, transformsJSON: e.target.value })}
              />
            </label>
            <label>
              Samples (по строке)
              <textarea
                rows={4}
                value={editing.samplesText}
                disabled={busy}
                onChange={(e) => setEditing({ ...editing, samplesText: e.target.value })}
              />
            </label>

            <div className="parsers-actions">
              <button type="submit" className="btn primary" disabled={busy}>
                {busy ? 'Сохранение…' : 'Сохранить'}
              </button>
              <button
                type="button"
                className="btn"
                disabled={busy}
                onClick={() => {
                  setEditing(null);
                  setDryResult(null);
                }}
              >
                Закрыть
              </button>
            </div>
          </form>

          <div className="parsers-dry-run">
            <h4>Dry-run</h4>
            <label>
              Строка лога
              <textarea
                rows={3}
                value={editing.dryText}
                disabled={busy}
                onChange={(e) => setEditing({ ...editing, dryText: e.target.value })}
              />
            </label>
            <button type="button" className="btn" disabled={busy || !editing.dryText} onClick={() => void onDryRun()}>
              Прогнать
            </button>
            {dryResult ? (
              <pre>{JSON.stringify(dryResult, null, 2)}</pre>
            ) : null}
          </div>
        </div>
      ) : null}
    </SettingsLayout>
  );
}
