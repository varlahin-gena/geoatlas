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
  const [loadError, setLoadError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [editing, setEditing] = useState<Draft | null>(null);
  const [isCreate, setIsCreate] = useState(false);
  const [dryResult, setDryResult] = useState<ParseDryRunResult | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    try {
      const res = await listParsers();
      setItems(res.parsers || []);
    } catch (e) {
      const msg = e instanceof Error ? e.message : 'Не удалось загрузить парсеры';
      setLoadError(
        msg === 'internal server error'
          ? 'Не удалось загрузить правила. Убедитесь, что таблица parser_rules создана, затем обновите страницу.'
          : msg,
      );
      toast(msg === 'internal server error' ? 'Не удалось загрузить парсеры' : msg, 'error');
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
      toast(p.enabled ? 'Правило отключено' : 'Правило включено', 'success');
      await load();
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось обновить правило', 'error');
    } finally {
      setBusy(false);
    }
  }

  async function onDelete(p: ParserItem) {
    if (!p.editable) return;
    if (!window.confirm(`Удалить правило «${p.name}» (${p.id})?`)) return;
    setBusy(true);
    try {
      await deleteParser(p.id);
      toast('Правило удалено', 'success');
      if (editing?.id === p.id) setEditing(null);
      await load();
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось удалить правило', 'error');
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
        toast('Правило создано', 'success');
      } else {
        await updateParser(editing.id, {
          name: editing.name,
          vendor: editing.vendor,
          enabled: editing.enabled,
          priority: editing.priority,
          spec,
        });
        toast('Правило сохранено', 'success');
      }
      setIsCreate(false);
      await load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Не удалось сохранить правило', 'error');
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
      toast(err instanceof Error ? err.message : 'Не удалось проверить строку', 'error');
    } finally {
      setBusy(false);
    }
  }

  const toolbar = (
    <>
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
    </>
  );

  return (
    <SettingsLayout title="Парсеры" actions={toolbar}>
      <p className="page-lead parsers-lead">
        Здесь система учится разбирать строки логов МСЭ в поля карты (IP, порты, действие). Готовые
        шаблоны UserGate / FortiGate уже в списке — чаще всего достаточно отредактировать шаблон или
        скопировать его идею под свой формат.
      </p>

      <details className="parsers-details">
        <summary>Как добавить своё правило</summary>
        <ol>
          <li>
            Возьмите одну реальную строку лога (из «Ошибок парсинга» или с устройства).
          </li>
          <li>
            Нажмите «Добавить правило». Для CEF укажите производителя из заголовка строки (
            <code>CEF:0|Vendor|…</code>). Для KV — ключи, которые обязаны быть в строке.
          </li>
          <li>
            В «Полях» сопоставьте имена системы (<code>src</code>, <code>dst</code>, <code>act</code>…)
            с ключами в логе. Слева — наше поле, справа — как оно называется у вендора.
          </li>
          <li>
            Вставьте строку в «Проверку» и нажмите «Проверить». Если IP и действие появились —
            сохраните. Иначе поправьте сопоставление.
          </li>
        </ol>
        <p>
          Чем меньше приоритет, тем раньше правило пробуется. Cisco ASA/FTD встроены в код и здесь
          не редактируются. После сохранения правило сразу участвует в приёме логов — рестарт не
          нужен. Живой поток удобно смотреть на странице «Тест парсеров».
        </p>
      </details>

      {loadError ? (
        <div className="parsers-banner" role="alert">
          <div>
            <strong>Не удалось загрузить список</strong>
            <p>{loadError}</p>
          </div>
          <button type="button" className="btn" disabled={loading} onClick={() => void load()}>
            Повторить
          </button>
        </div>
      ) : null}

      <div className="card">
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Приоритет</th>
                <th>ID</th>
                <th>Имя</th>
                <th>Производитель</th>
                <th>Тип</th>
                <th>Источник</th>
                <th>Вкл.</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <TableLoadRows cols={8} rows={5} loading={loading} onRetry={() => void load()} />
              ) : loadError ? (
                <tr>
                  <td colSpan={8} className="empty">
                    <EmptyState
                      compact
                      title="Список недоступен"
                      description="Исправьте ошибку загрузки и нажмите «Повторить»."
                    />
                  </td>
                </tr>
              ) : sorted.length === 0 ? (
                <tr>
                  <td colSpan={8} className="empty">
                    <EmptyState
                      compact
                      title="Нет правил"
                      description="Нажмите «Добавить правило» и пройдите шаги из подсказки выше."
                    />
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
                        <span className="parsers-badge is-builtin">встроено</span>
                      ) : p.seed ? (
                        <span className="parsers-badge is-seed">шаблон</span>
                      ) : (
                        <span className="parsers-badge">своё</span>
                      )}
                    </td>
                    <td>
                      <input
                        type="checkbox"
                        checked={p.enabled}
                        disabled={!p.editable || busy}
                        onChange={() => void toggleEnabled(p)}
                        aria-label={p.enabled ? `Отключить ${p.name}` : `Включить ${p.name}`}
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
                              Редактировать
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
                          <span className="hint">только просмотр</span>
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
        <div className="card parsers-editor" style={{ marginTop: '1rem' }}>
          <h3>{isCreate ? 'Новое правило' : `Правило ${editing.id}`}</h3>
          <p className="parsers-editor-lead">
            {isCreate
              ? 'Заполните, кто шлёт логи и как называются поля → проверьте на реальной строке → сохраните.'
              : 'Меняйте сопоставление полей, проверяйте на строке лога, затем сохраняйте.'}
          </p>
          <form className="parsers-form-grid" onSubmit={(e) => void onSave(e)}>
            <div className="parsers-form-row">
              <label>
                ID
                <input
                  value={editing.id}
                  disabled={!isCreate || busy}
                  required
                  placeholder="my-firewall"
                  onChange={(e) => setEditing({ ...editing, id: e.target.value })}
                />
                <span className="field-hint">Латиницей, без пробелов. После создания не меняется.</span>
              </label>
              <label>
                Имя
                <input
                  value={editing.name}
                  required
                  disabled={busy}
                  placeholder="Мой МСЭ CEF"
                  onChange={(e) => setEditing({ ...editing, name: e.target.value })}
                />
                <span className="field-hint">Как правило видно в списке.</span>
              </label>
              <label>
                Производитель
                <input
                  value={editing.vendor}
                  disabled={busy}
                  placeholder="usergate"
                  onChange={(e) => setEditing({ ...editing, vendor: e.target.value })}
                />
                <span className="field-hint">Короткий ярлык устройства (не путать с CEF Vendor).</span>
              </label>
              <label>
                Приоритет
                <input
                  type="number"
                  value={editing.priority}
                  disabled={busy}
                  onChange={(e) => setEditing({ ...editing, priority: Number(e.target.value) || 0 })}
                />
                <span className="field-hint">Меньше число — раньше пробуем. Шаблоны: 100–200.</span>
              </label>
              <label>
                Тип
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
                  <option value="cef">CEF</option>
                  <option value="kv">KV (ключ=значение)</option>
                </select>
                <span className="field-hint">CEF — строки с заголовком CEF:0|…; KV — пары key=value.</span>
              </label>
              <label style={{ flexDirection: 'row', alignItems: 'center', gap: '0.5rem' }}>
                <input
                  type="checkbox"
                  checked={editing.enabled}
                  disabled={busy}
                  onChange={(e) => setEditing({ ...editing, enabled: e.target.checked })}
                />
                Включено
              </label>
            </div>

            {editing.kind === 'cef' ? (
              <label>
                Производитель в CEF
                <input
                  value={editing.cefVendor}
                  required
                  disabled={busy}
                  placeholder="Usergate"
                  onChange={(e) => setEditing({ ...editing, cefVendor: e.target.value })}
                />
                <span className="field-hint">
                  Точное имя из CEF-заголовка после первой «|», например Usergate или Fortinet.
                </span>
              </label>
            ) : (
              <label>
                Обязательные ключи (через запятую)
                <input
                  value={editing.requiredKeys}
                  required
                  disabled={busy}
                  placeholder="src,dst"
                  onChange={(e) => setEditing({ ...editing, requiredKeys: e.target.value })}
                />
                <span className="field-hint">
                  Правило сработает только если все эти ключи есть в строке.
                </span>
              </label>
            )}

            <label>
              Поля (JSON)
              <textarea
                rows={8}
                value={editing.fieldsJSON}
                disabled={busy}
                spellCheck={false}
                onChange={(e) => setEditing({ ...editing, fieldsJSON: e.target.value })}
              />
              <span className="field-hint">
                Слева поле GeoAtlas (<code>src</code>, <code>dst</code>, <code>act</code>…), справа —
                ключ в логе. Пример: <code>{`"src": "src", "bytes_sent": "in"`}</code>.
              </span>
            </label>
            <label>
              Преобразования (JSON)
              <textarea
                rows={4}
                value={editing.transformsJSON}
                disabled={busy}
                spellCheck={false}
                onChange={(e) => setEditing({ ...editing, transformsJSON: e.target.value })}
              />
              <span className="field-hint">
                Обычно хватает <code>{`{"time_unit":"auto"}`}</code>. Остальное — тонкая настройка
                (зоны, протокол, склейка правила).
              </span>
            </label>
            <label>
              Примеры (по одной строке)
              <textarea
                rows={3}
                value={editing.samplesText}
                disabled={busy}
                spellCheck={false}
                placeholder="Вставьте сюда образцы логов — пригодятся для проверки и пресетов"
                onChange={(e) => setEditing({ ...editing, samplesText: e.target.value })}
              />
            </label>

            <div className="parsers-dry-run">
              <h4>Проверка перед сохранением</h4>
              <p className="field-hint" style={{ marginTop: 0 }}>
                Вставьте строку лога и нажмите «Проверить». В ответе должны появиться IP и действие —
                тогда правило можно сохранять.
              </p>
              <label>
                Строка лога
                <textarea
                  rows={3}
                  value={editing.dryText}
                  disabled={busy}
                  spellCheck={false}
                  placeholder="CEF:0|Vendor|Product|… src=… dst=… act=…"
                  onChange={(e) => setEditing({ ...editing, dryText: e.target.value })}
                />
              </label>
              <button
                type="button"
                className="btn"
                disabled={busy || !editing.dryText}
                onClick={() => void onDryRun()}
              >
                Проверить
              </button>
              {dryResult ? <pre>{JSON.stringify(dryResult, null, 2)}</pre> : null}
            </div>

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
        </div>
      ) : null}
    </SettingsLayout>
  );
}
