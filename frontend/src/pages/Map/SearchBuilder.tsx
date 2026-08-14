import { useEffect, useRef, useState } from 'react';
import { isAbortError } from '@/api/client';
import {
  createSearchTemplate,
  deleteSearchTemplate,
  listSearchTemplates,
  type SearchTemplate as Template,
} from '@/api/searchTemplates';
import { useAuth } from '@/auth/AuthContext';
import { useToast } from '@/components/Toast';
import type { SearchField } from '@/lib/search';
import {
  createDefaultBuilderGroup,
  createDefaultBuilderRow,
  fieldLabel,
  SEARCH_BUILDER_FIELDS,
  SEARCH_EXAMPLE_CHIPS,
  serializeSearchBuilderRows,
  type BuilderRow,
} from '@/lib/searchBuilder';

type Tab = 'builder' | 'mine' | 'all';

export function SearchBuilder({
  open,
  onOpenChange,
  search,
  onApply,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  search: string;
  onApply: (query: string) => void;
}) {
  const { isAdmin } = useAuth();
  const { toast } = useToast();
  const panelRef = useRef<HTMLDivElement>(null);
  const [tab, setTab] = useState<Tab>('builder');
  const [rows, setRows] = useState<BuilderRow[]>([createDefaultBuilderRow()]);
  const [mine, setMine] = useState<Template[]>([]);
  const [all, setAll] = useState<Template[]>([]);

  useEffect(() => {
    if (!open) return;
    function onDoc(e: MouseEvent) {
      const t = e.target as Node;
      if (panelRef.current?.contains(t)) return;
      const box = panelRef.current?.closest('.search-box');
      if (box?.contains(t)) return;
      onOpenChange(false);
    }
    document.addEventListener('mousedown', onDoc);
    return () => document.removeEventListener('mousedown', onDoc);
  }, [open, onOpenChange]);

  useEffect(() => {
    if (!open || (tab !== 'mine' && tab !== 'all')) return;
    const controller = new AbortController();
    void loadTemplates(tab, controller.signal);
    return () => controller.abort();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, tab]);

  async function loadTemplates(which: 'mine' | 'all', signal?: AbortSignal) {
    try {
      if (which === 'mine') {
        const data = await listSearchTemplates(undefined, { signal });
        if (signal?.aborted) return;
        setMine(data.templates || []);
      } else {
        const data = await listSearchTemplates('all', { signal });
        if (signal?.aborted) return;
        setAll(data.templates || []);
      }
    } catch (e) {
      if (isAbortError(e)) return;
      toast(e instanceof Error ? e.message : 'Не удалось загрузить шаблоны', 'error');
    }
  }

  function applyRows(next = rows) {
    const q = serializeSearchBuilderRows(next);
    onApply(q);
    onOpenChange(false);
  }

  function updateRow(idx: number, patch: Partial<BuilderRow>) {
    setRows((prev) => prev.map((r, i) => (i === idx ? ({ ...r, ...patch } as BuilderRow) : r)));
  }

  return (
    <div
      ref={panelRef}
      className={`search-builder-panel${open ? ' open' : ''}`}
      id="searchBuilderPanel"
    >
      <div className="search-builder-tabs" role="tablist">
        <button
          type="button"
          className={`search-builder-tab${tab === 'builder' ? ' active' : ''}`}
          onClick={() => setTab('builder')}
        >
          Конструктор
        </button>
        <button
          type="button"
          className={`search-builder-tab${tab === 'mine' ? ' active' : ''}`}
          onClick={() => setTab('mine')}
        >
          Мои запросы
        </button>
        {isAdmin ? (
          <button
            type="button"
            className={`search-builder-tab${tab === 'all' ? ' active' : ''}`}
            onClick={() => setTab('all')}
          >
            Все шаблоны
          </button>
        ) : null}
      </div>

      {tab === 'builder' ? (
        <div className="search-builder-tab-panel">
          <div className="search-builder-head">
            <div className="search-builder-title">Конструктор запроса</div>
            <div className="search-builder-actions">
              <button type="button" onClick={() => setRows((r) => [...r, createDefaultBuilderRow()])}>
                Добавить условие
              </button>
              <button type="button" onClick={() => setRows((r) => [...r, createDefaultBuilderGroup()])}>
                Добавить группу
              </button>
              <button type="button" className="primary" onClick={() => applyRows()}>
                Применить
              </button>
              <button
                type="button"
                onClick={() => {
                  setRows([createDefaultBuilderRow()]);
                  onApply('');
                }}
              >
                Сбросить
              </button>
            </div>
          </div>
          <div className="search-builder-chips" aria-label="Примеры запросов">
            {SEARCH_EXAMPLE_CHIPS.map((chip) => (
              <button
                key={chip.label}
                type="button"
                className="search-example-chip"
                onClick={() => {
                  onApply(chip.query);
                  onOpenChange(false);
                }}
              >
                {chip.label}
              </button>
            ))}
          </div>
          <div className="search-builder-hint">
            Подсказка: country:Россия AND device:fw1, NOT rule:block, (A OR B)
          </div>
          <div className="search-builder-rows">
            {rows.map((row, idx) => (
              <div key={idx} className="search-builder-row">
                {idx > 0 ? (
                  <select
                    className="search-builder-join"
                    value={row.joinWith}
                    onChange={(e) =>
                      updateRow(idx, { joinWith: e.target.value === 'OR' ? 'OR' : 'AND' })
                    }
                  >
                    <option value="AND">И</option>
                    <option value="OR">ИЛИ</option>
                  </select>
                ) : (
                  <span className="search-builder-join" style={{ visibility: 'hidden', width: 56 }}>
                    И
                  </span>
                )}
                <label className="search-builder-negate">
                  <input
                    type="checkbox"
                    checked={row.negate}
                    onChange={(e) => updateRow(idx, { negate: e.target.checked })}
                  />
                  НЕ
                </label>
                {row.kind === 'term' ? (
                  <>
                    <select
                      value={row.field}
                      onChange={(e) => updateRow(idx, { field: e.target.value as SearchField })}
                    >
                      {SEARCH_BUILDER_FIELDS.map((f) => (
                        <option key={f} value={f}>
                          {fieldLabel(f)}
                        </option>
                      ))}
                    </select>
                    <input
                      type="text"
                      className="search-builder-value"
                      placeholder="Значение"
                      value={row.value}
                      onChange={(e) => updateRow(idx, { value: e.target.value })}
                    />
                  </>
                ) : (
                  <div className="search-builder-group">
                    <div className="search-builder-group-head">
                      <span>Группа</span>
                      <label className="search-builder-group-op">
                        Связка
                        <select
                          value={row.op}
                          onChange={(e) =>
                            updateRow(idx, { op: e.target.value === 'OR' ? 'OR' : 'AND' })
                          }
                        >
                          <option value="AND">И</option>
                          <option value="OR">ИЛИ</option>
                        </select>
                      </label>
                    </div>
                    <div className="search-builder-group-children">
                      {row.children.map((child, cidx) => (
                        <div key={cidx} className="search-builder-row search-builder-row-nested">
                          <label className="search-builder-negate">
                            <input
                              type="checkbox"
                              checked={child.negate}
                              onChange={(e) => {
                                const children = row.children.slice();
                                children[cidx] = { ...child, negate: e.target.checked };
                                updateRow(idx, { children });
                              }}
                            />
                            НЕ
                          </label>
                          <select
                            value={child.field}
                            onChange={(e) => {
                              const children = row.children.slice();
                              children[cidx] = {
                                ...child,
                                field: e.target.value as SearchField,
                              };
                              updateRow(idx, { children });
                            }}
                          >
                            {SEARCH_BUILDER_FIELDS.map((f) => (
                              <option key={f} value={f}>
                                {fieldLabel(f)}
                              </option>
                            ))}
                          </select>
                          <input
                            type="text"
                            placeholder="Значение"
                            value={child.value}
                            onChange={(e) => {
                              const children = row.children.slice();
                              children[cidx] = { ...child, value: e.target.value };
                              updateRow(idx, { children });
                            }}
                          />
                        </div>
                      ))}
                    </div>
                    <div className="search-builder-group-actions">
                      <button
                        type="button"
                        onClick={() =>
                          updateRow(idx, {
                            children: [
                              ...row.children,
                              { negate: false, field: 'all', value: '' },
                            ],
                          })
                        }
                      >
                        + условие в группе
                      </button>
                    </div>
                  </div>
                )}
                <button
                  type="button"
                  className="search-builder-remove"
                  disabled={rows.length <= 1}
                  onClick={() => setRows((prev) => prev.filter((_, i) => i !== idx))}
                >
                  Удалить
                </button>
              </div>
            ))}
          </div>
          {search ? (
            <div className="search-builder-hint" style={{ marginTop: 8 }}>
              Текущий запрос: <code>{search}</code>
            </div>
          ) : null}
        </div>
      ) : null}

      {tab === 'mine' ? (
        <div className="search-builder-tab-panel">
          <div className="search-templates-toolbar">
            <button
              type="button"
              onClick={async () => {
                const name = window.prompt('Название шаблона');
                if (!name || !search.trim()) {
                  toast('Нужны название и непустой запрос', 'warn');
                  return;
                }
                try {
                  await createSearchTemplate({ name, query: search });
                  toast('Сохранено', 'success');
                  void loadTemplates('mine');
                } catch (e) {
                  toast(e instanceof Error ? e.message : 'Ошибка', 'error');
                }
              }}
            >
              Сохранить текущий
            </button>
            <button type="button" onClick={() => void loadTemplates('mine')}>
              Обновить
            </button>
          </div>
          <div className="search-templates-hint">
            Личные шаблоны хранятся на сервере и доступны только вашей учётной записи.
          </div>
          <div className="search-templates-list">
            {!mine.length ? (
              <div className="search-templates-empty">Пока нет шаблонов</div>
            ) : (
              mine.map((t) => (
                <div key={t.id} className="search-template-card">
                  <div className="search-template-card-head">
                    <div className="search-template-name">{t.name}</div>
                  </div>
                  <div className="search-template-query">{t.query}</div>
                  <div className="search-template-actions">
                    <button
                      type="button"
                      onClick={() => {
                        onApply(t.query);
                        onOpenChange(false);
                      }}
                    >
                      Применить
                    </button>
                    <button
                      type="button"
                      onClick={async () => {
                        if (!confirm('Удалить шаблон?')) return;
                        try {
                          await deleteSearchTemplate(t.id);
                          void loadTemplates('mine');
                        } catch (e) {
                          toast(e instanceof Error ? e.message : 'Ошибка', 'error');
                        }
                      }}
                    >
                      Удалить
                    </button>
                  </div>
                </div>
              ))
            )}
          </div>
        </div>
      ) : null}

      {tab === 'all' ? (
        <div className="search-builder-tab-panel">
          <div className="search-templates-toolbar">
            <button type="button" onClick={() => void loadTemplates('all')}>
              Обновить
            </button>
          </div>
          <div className="search-templates-hint">
            Шаблоны всех пользователей. Чужие можно только применить.
          </div>
          <div className="search-templates-list">
            {!all.length ? (
              <div className="search-templates-empty">Пусто</div>
            ) : (
              all.map((t) => (
                <div key={t.id} className="search-template-card">
                  <div className="search-template-card-head">
                    <div className="search-template-name">{t.name}</div>
                    <div className="search-template-author">{t.owner || t.username || ''}</div>
                  </div>
                  <div className="search-template-query">{t.query}</div>
                  <div className="search-template-actions">
                    <button
                      type="button"
                      onClick={() => {
                        onApply(t.query);
                        onOpenChange(false);
                      }}
                    >
                      Применить
                    </button>
                  </div>
                </div>
              ))
            )}
          </div>
        </div>
      ) : null}
    </div>
  );
}
