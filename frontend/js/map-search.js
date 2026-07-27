'use strict';

const SEARCH_FIELD_DEFS = Object.freeze({
    all: { key: 'all', label: 'Все поля', aliases: ['all', 'any', 'text'] },
    ip: { key: 'ip', label: 'IP', aliases: ['ip', 'addr', 'address'] },
    country: { key: 'country', label: 'Страна', aliases: ['country', 'страна'] },
    city: { key: 'city', label: 'Город', aliases: ['city', 'город'] },
    rule: { key: 'rule', label: 'Правило', aliases: ['rule', 'policy', 'правило'] },
    device: { key: 'device', label: 'Устройство', aliases: ['device', 'host', 'fw', 'устройство'] },
    src: { key: 'src', label: 'Источник', aliases: ['src', 'source', 'from'] },
    dst: { key: 'dst', label: 'Назначение', aliases: ['dst', 'dest', 'destination', 'to'] },
    proto: { key: 'proto', label: 'Протокол', aliases: ['proto', 'protocol'] },
    zone: { key: 'zone', label: 'Зона', aliases: ['zone'] },
});

const SEARCH_BUILDER_FIELDS = ['all', 'ip', 'country', 'city', 'rule', 'device'];
const SEARCH_COMPLEXITY_MESSAGE = 'Этот запрос слишком вложенный для конструктора. Его можно выполнять, но удобнее редактировать прямо в строке.';
const SEARCH_EXAMPLE_CHIPS = Object.freeze([
    { label: 'country:Россия', query: 'country:Россия' },
    { label: 'rule:block', query: 'rule:block' },
    { label: 'NOT city:Москва', query: 'NOT city:Москва' },
    { label: 'country AND device', query: 'country:Россия AND device:fw1' },
    { label: '(A OR B) AND NOT', query: '(country:Россия OR country:Казахстан) AND NOT rule:allow' },
]);

let searchFieldAliasMap = null;

function getSearchFieldAliasMap() {
    if (searchFieldAliasMap) return searchFieldAliasMap;
    searchFieldAliasMap = {};
    Object.values(SEARCH_FIELD_DEFS).forEach(function (def) {
        def.aliases.forEach(function (alias) {
            searchFieldAliasMap[normalizeText(alias)] = def.key;
        });
    });
    return searchFieldAliasMap;
}

function canonicalSearchField(field) {
    if (!field) return 'all';
    return getSearchFieldAliasMap()[normalizeText(field)] || null;
}

function quoteSearchValue(raw) {
    const value = String(raw || '').trim();
    if (!value) return '';
    if (!/[\s()"']/u.test(value)) return value;
    return '"' + value.replace(/\\/g, '\\\\').replace(/"/g, '\\"') + '"';
}

function searchValueTokens(text) {
    const raw = String(text || '').trim();
    if (!raw) return [];
    const tokens = [];
    let i = 0;
    while (i < raw.length) {
        const ch = raw[i];
        if (/\s/.test(ch)) {
            i++;
            continue;
        }
        if (ch === '(') {
            tokens.push({ type: 'LPAREN', value: ch });
            i++;
            continue;
        }
        if (ch === ')') {
            tokens.push({ type: 'RPAREN', value: ch });
            i++;
            continue;
        }
        if (ch === ':') {
            tokens.push({ type: 'COLON', value: ch });
            i++;
            continue;
        }
        if (ch === '"') {
            i++;
            let value = '';
            let closed = false;
            while (i < raw.length) {
                const cur = raw[i];
                if (cur === '\\' && i + 1 < raw.length) {
                    value += raw[i + 1];
                    i += 2;
                    continue;
                }
                if (cur === '"') {
                    closed = true;
                    i++;
                    break;
                }
                value += cur;
                i++;
            }
            if (!closed) throw new Error('Незакрытая кавычка в поисковом запросе');
            tokens.push({ type: 'STRING', value: value });
            continue;
        }
        let word = '';
        while (i < raw.length) {
            const cur = raw[i];
            if (/\s/.test(cur) || cur === '(' || cur === ')' || cur === ':' || cur === '"') break;
            word += cur;
            i++;
        }
        const upper = word.toUpperCase();
        if (upper === 'AND' || upper === 'OR' || upper === 'NOT') {
            tokens.push({ type: upper, value: upper });
        } else {
            tokens.push({ type: 'WORD', value: word });
        }
    }
    return tokens;
}

function looksLikeAdvancedSearch(raw) {
    const text = String(raw || '').trim();
    if (!text) return false;
    if (/[()"]/u.test(text)) return true;
    if (/\b(AND|OR|NOT)\b/i.test(text)) return true;
    return /\b[\p{L}\p{N}_-]+\s*:/u.test(text);
}

function parseSearchQuery(raw) {
    const tokens = searchValueTokens(raw);
    let idx = 0;

    function peek() {
        return tokens[idx] || null;
    }

    function next() {
        return tokens[idx++] || null;
    }

    function startsPrimary(token) {
        return !!token && (token.type === 'WORD' || token.type === 'STRING' || token.type === 'LPAREN' || token.type === 'NOT');
    }

    function parseValueToken() {
        const token = next();
        if (!token || (token.type !== 'WORD' && token.type !== 'STRING')) {
            throw new Error('Ожидалось значение после поля поиска');
        }
        return token.value;
    }

    function parsePrimary() {
        const token = peek();
        if (!token) throw new Error('Ожидалось условие поиска');
        if (token.type === 'LPAREN') {
            next();
            const expr = parseOr();
            const closing = next();
            if (!closing || closing.type !== 'RPAREN') {
                throw new Error('Пропущена закрывающая скобка');
            }
            return expr;
        }
        if (token.type !== 'WORD' && token.type !== 'STRING') {
            throw new Error('Ожидалось условие поиска');
        }
        const head = next();
        if (head.type === 'WORD' && peek() && peek().type === 'COLON') {
            next();
            const field = canonicalSearchField(head.value);
            if (!field) {
                throw new Error('Неизвестное поле: ' + head.value);
            }
            return { type: 'TERM', field: field, value: parseValueToken() };
        }
        return { type: 'TERM', field: 'all', value: head.value };
    }

    function parseUnary() {
        if (peek() && peek().type === 'NOT') {
            next();
            return { type: 'NOT', expr: parseUnary() };
        }
        return parsePrimary();
    }

    function parseAnd() {
        let left = parseUnary();
        while (true) {
            const token = peek();
            if (token && token.type === 'AND') {
                next();
                left = { type: 'AND', left: left, right: parseUnary() };
                continue;
            }
            if (startsPrimary(token)) {
                left = { type: 'AND', left: left, right: parseUnary() };
                continue;
            }
            break;
        }
        return left;
    }

    function parseOr() {
        let left = parseAnd();
        while (peek() && peek().type === 'OR') {
            next();
            left = { type: 'OR', left: left, right: parseAnd() };
        }
        return left;
    }

    if (!tokens.length) return null;
    const ast = parseOr();
    if (idx < tokens.length) {
        throw new Error('Лишние токены в конце запроса');
    }
    return ast;
}

function searchLineCountryValues(line, pointMap) {
    const values = [line.src_country, line.dst_country];
    const srcP = pointMap[line.src];
    const dstP = pointMap[line.dst];
    if (srcP) values.push(srcP.country);
    if (dstP) values.push(dstP.country);
    const expanded = [];
    values.forEach(function (value) {
        if (!value) return;
        expanded.push(value);
        expanded.push(ruCountry(value));
    });
    return expanded;
}

function searchPointCountryValues(point) {
    if (!point || !point.country) return [];
    return [point.country, ruCountry(point.country)];
}

function getLineSearchFieldValues(line, pointMap) {
    const srcP = pointMap[line.src];
    const dstP = pointMap[line.dst];
    return {
        all: [
            line.src, line.dst, line.src_label, line.dst_label,
            line.rule, line.proto, line.device, line.last_action,
            line.src_zone, line.dst_zone, line.src_country, line.dst_country,
            ruCountry(line.src_country), ruCountry(line.dst_country),
            srcP && srcP.city, srcP && srcP.country, srcP && ruCountry(srcP.country), srcP && srcP.region, srcP && srcP.label,
            dstP && dstP.city, dstP && dstP.country, dstP && ruCountry(dstP.country), dstP && dstP.region, dstP && dstP.label,
        ],
        ip: [line.src, line.dst],
        country: searchLineCountryValues(line, pointMap),
        city: [srcP && srcP.city, dstP && dstP.city],
        rule: [line.rule],
        device: [line.device],
        src: [line.src, line.src_label, line.src_zone, line.src_country, ruCountry(line.src_country), srcP && srcP.city, srcP && srcP.country, srcP && ruCountry(srcP.country), srcP && srcP.region],
        dst: [line.dst, line.dst_label, line.dst_zone, line.dst_country, ruCountry(line.dst_country), dstP && dstP.city, dstP && dstP.country, dstP && ruCountry(dstP.country), dstP && dstP.region],
        proto: [line.proto],
        zone: [line.src_zone, line.dst_zone],
    };
}

function getPointSearchFieldValues(key, point) {
    return {
        all: [key, point.label, point.city, point.region, point.country, ruCountry(point.country)],
        ip: [key],
        country: searchPointCountryValues(point),
        city: [point.city],
        rule: [],
        device: [],
        src: [key, point.label],
        dst: [key, point.label],
        proto: [],
        zone: [],
    };
}

function valueIncludesQuery(values, query) {
    const needle = normalizeText(query);
    if (!needle) return true;
    return (values || []).some(function (value) {
        return normalizeText(value).includes(needle);
    });
}

function createSimpleSearchMatcher(raw) {
    const normalized = normalizeText(raw);
    return {
        matchesLine: function (line, pointMap) {
            return valueIncludesQuery(getLineSearchFieldValues(line, pointMap).all, normalized);
        },
        matchesPoint: function (key, point) {
            return valueIncludesQuery(getPointSearchFieldValues(key, point).all, normalized);
        },
    };
}

function evaluateSearchAst(ast, ctx) {
    if (!ast) return true;
    if (ast.type === 'TERM') {
        const fields = ctx.fieldValues[ast.field] || [];
        return valueIncludesQuery(fields, ast.value);
    }
    if (ast.type === 'NOT') return !evaluateSearchAst(ast.expr, ctx);
    if (ast.type === 'AND') return evaluateSearchAst(ast.left, ctx) && evaluateSearchAst(ast.right, ctx);
    if (ast.type === 'OR') return evaluateSearchAst(ast.left, ctx) || evaluateSearchAst(ast.right, ctx);
    return true;
}

function buildSearchMatcher(ast) {
    return {
        matchesLine: function (line, pointMap) {
            return evaluateSearchAst(ast, {
                kind: 'line',
                fieldValues: getLineSearchFieldValues(line, pointMap),
            });
        },
        matchesPoint: function (key, point) {
            return evaluateSearchAst(ast, {
                kind: 'point',
                fieldValues: getPointSearchFieldValues(key, point),
            });
        },
    };
}

function compileSearchQuery(raw) {
    const text = String(raw || '').trim();
    if (!text) {
        return {
            raw: '',
            mode: 'empty',
            matcher: null,
            ast: null,
            error: '',
            builderEditable: true,
        };
    }
    if (!looksLikeAdvancedSearch(text)) {
        return {
            raw: text,
            mode: 'simple',
            matcher: createSimpleSearchMatcher(text),
            ast: { type: 'TERM', field: 'all', value: text },
            error: '',
            builderEditable: true,
        };
    }
    try {
        const ast = parseSearchQuery(text);
        const rows = searchBuilderRowsFromAst(ast);
        return {
            raw: text,
            mode: 'advanced',
            matcher: buildSearchMatcher(ast),
            ast: ast,
            error: '',
            builderEditable: rows.editable,
            builderRows: rows.rows,
            builderReason: rows.reason || '',
        };
    } catch (err) {
        return {
            raw: text,
            mode: 'fallback',
            matcher: createSimpleSearchMatcher(text),
            ast: null,
            error: err && err.message ? err.message : 'Не удалось разобрать поисковый запрос',
            builderEditable: false,
            builderReason: err && err.message ? err.message : '',
        };
    }
}

function searchBuilderTermToRow(node, joinWith) {
    let term = node;
    let negate = false;
    if (term && term.type === 'NOT') {
        negate = true;
        term = term.expr;
    }
    if (!term || term.type !== 'TERM') return null;
    return {
        kind: 'term',
        joinWith: joinWith || 'AND',
        negate: negate,
        field: term.field || 'all',
        value: term.value || '',
        op: 'AND',
        children: [],
    };
}

function flattenOpChain(node, op) {
    if (!node) return [];
    if (node.type === op) {
        return flattenOpChain(node.left, op).concat(flattenOpChain(node.right, op));
    }
    return [node];
}

function nodeToBuilderItem(node, joinWith) {
    let negate = false;
    let n = node;
    if (n && n.type === 'NOT') {
        negate = true;
        n = n.expr;
    }
    const term = searchBuilderTermToRow(n, joinWith);
    if (term) {
        term.negate = negate || term.negate;
        return term;
    }
    if (!n || (n.type !== 'AND' && n.type !== 'OR')) return null;
    const parts = flattenOpChain(n, n.type);
    const children = [];
    for (let i = 0; i < parts.length; i++) {
        let childNeg = false;
        let child = parts[i];
        if (child && child.type === 'NOT') {
            childNeg = true;
            child = child.expr;
        }
        if (!child || child.type !== 'TERM') return null;
        children.push({
            negate: childNeg,
            field: child.field || 'all',
            value: child.value || '',
        });
    }
    if (!children.length) return null;
    return {
        kind: 'group',
        joinWith: joinWith || 'AND',
        negate: negate,
        field: 'all',
        value: '',
        op: n.type,
        children: children,
    };
}

function searchBuilderRowsFromAst(ast) {
    if (!ast) return { editable: true, rows: [] };
    if (ast.type === 'AND' || ast.type === 'OR') {
        const parts = flattenOpChain(ast, ast.type);
        const rows = [];
        for (let i = 0; i < parts.length; i++) {
            const item = nodeToBuilderItem(parts[i], i === 0 ? 'AND' : ast.type);
            if (!item) {
                return { editable: false, rows: [], reason: SEARCH_COMPLEXITY_MESSAGE };
            }
            rows.push(item);
        }
        if (!rows.length) {
            return { editable: false, rows: [], reason: SEARCH_COMPLEXITY_MESSAGE };
        }
        return { editable: true, rows: rows };
    }
    const single = nodeToBuilderItem(ast, 'AND');
    if (single) return { editable: true, rows: [single] };
    return { editable: false, rows: [], reason: SEARCH_COMPLEXITY_MESSAGE };
}

function createDefaultBuilderRow() {
    return {
        kind: 'term',
        joinWith: 'AND',
        negate: false,
        field: 'all',
        value: '',
        op: 'AND',
        children: [],
    };
}

function createDefaultBuilderGroup() {
    return {
        kind: 'group',
        joinWith: 'AND',
        negate: false,
        field: 'all',
        value: '',
        op: 'OR',
        children: [
            { negate: false, field: 'all', value: '' },
            { negate: false, field: 'all', value: '' },
        ],
    };
}

function cloneBuilderRows(rows) {
    return (rows || []).map(function (row) {
        return {
            kind: row.kind === 'group' ? 'group' : 'term',
            joinWith: row.joinWith === 'OR' ? 'OR' : 'AND',
            negate: !!row.negate,
            field: SEARCH_FIELD_DEFS[row.field] ? row.field : 'all',
            value: row.value || '',
            op: row.op === 'OR' ? 'OR' : 'AND',
            children: Array.isArray(row.children)
                ? row.children.map(function (child) {
                    return {
                        negate: !!child.negate,
                        field: SEARCH_FIELD_DEFS[child.field] ? child.field : 'all',
                        value: child.value || '',
                    };
                })
                : [],
        };
    });
}

function normalizeBuilderItem(row, idx) {
    const kind = row && row.kind === 'group' ? 'group' : 'term';
    const item = {
        kind: kind,
        joinWith: idx === 0 ? 'AND' : (row.joinWith === 'OR' ? 'OR' : 'AND'),
        negate: !!(row && row.negate),
        field: SEARCH_FIELD_DEFS[row && row.field] ? row.field : 'all',
        value: (row && row.value) || '',
        op: row && row.op === 'OR' ? 'OR' : 'AND',
        children: [],
    };
    if (kind === 'group') {
        const children = Array.isArray(row.children) && row.children.length
            ? row.children
            : [{ negate: false, field: 'all', value: '' }];
        item.children = children.map(function (child) {
            return {
                negate: !!child.negate,
                field: SEARCH_FIELD_DEFS[child.field] ? child.field : 'all',
                value: child.value || '',
            };
        });
    }
    return item;
}

function currentSearchBuilderState() {
    if (!currentSearchBuilderEditable && currentSearch) return [];
    if (currentSearchBuilderEditable && Array.isArray(currentSearchBuilderRows) && currentSearchBuilderRows.length) {
        return currentSearchBuilderRows.map(normalizeBuilderItem);
    }
    if (currentSearchMode === 'simple' && currentSearch) {
        return [normalizeBuilderItem({
            kind: 'term',
            joinWith: 'AND',
            negate: false,
            field: 'all',
            value: currentSearch,
        }, 0)];
    }
    return [createDefaultBuilderRow()];
}

function serializeBuilderTerm(term) {
    const value = String(term && term.value || '').trim();
    if (!value) return null;
    const field = SEARCH_FIELD_DEFS[term.field] ? term.field : 'all';
    const prefix = field === 'all' ? '' : field + ':';
    return (term.negate ? 'NOT ' : '') + prefix + quoteSearchValue(value);
}

function serializeSearchBuilderRows(rows) {
    const prepared = (rows || [])
        .map(function (row, idx) {
            const joinWith = idx === 0 ? '' : ((row.joinWith === 'OR') ? 'OR ' : 'AND ');
            if (row.kind === 'group') {
                const inner = (row.children || [])
                    .map(serializeBuilderTerm)
                    .filter(Boolean)
                    .join(row.op === 'OR' ? ' OR ' : ' AND ');
                if (!inner) return null;
                return joinWith + (row.negate ? 'NOT ' : '') + '(' + inner + ')';
            }
            const expr = serializeBuilderTerm(row);
            if (!expr) return null;
            return joinWith + expr;
        })
        .filter(Boolean);
    return prepared.join(' ');
}

function setSearchQuery(raw, options) {
    const opts = Object.assign({
        syncInput: true,
        save: false,
        refresh: false,
        updateOverlay: false,
        keepBuilderOpen: false,
    }, options || {});
    const compiled = compileSearchQuery(raw);
    currentSearch = compiled.raw;
    currentSearchMode = compiled.mode;
    currentSearchMatcher = compiled.matcher;
    currentSearchAst = compiled.ast;
    currentSearchParseError = compiled.error;
    currentSearchBuilderEditable = compiled.builderEditable;
    currentSearchBuilderReason = compiled.builderReason || '';
    if (compiled.builderRows) {
        currentSearchBuilderRows = cloneBuilderRows(compiled.builderRows);
    } else if (compiled.mode === 'simple' && compiled.raw) {
        currentSearchBuilderRows = [createDefaultBuilderRow()];
        currentSearchBuilderRows[0].value = compiled.raw;
    } else {
        currentSearchBuilderRows = [createDefaultBuilderRow()];
    }

    const input = document.getElementById('searchInput');
    if (opts.syncInput && input && input.value !== compiled.raw) input.value = compiled.raw;
    if (!opts.keepBuilderOpen) searchBuilderForceOpen = false;
    syncSearchBuilderUI();
    if (opts.save && typeof saveUIState === 'function') saveUIState();
    if (opts.refresh && typeof refreshMapLayers === 'function') refreshMapLayers();
    if (opts.updateOverlay && typeof updateMapOverlay === 'function') updateMapOverlay();
}

function searchBuilderStatusText() {
    if (currentSearchParseError) {
        return 'Запрос применён как обычный текст: ' + currentSearchParseError;
    }
    if (!currentSearchBuilderEditable && currentSearchBuilderReason) {
        return currentSearchBuilderReason;
    }
    if (currentSearchMode === 'advanced') {
        return 'Расширенный запрос активен. Поддерживаются AND, OR, NOT, скобки и поля.';
    }
    return 'Подсказка: country:Россия AND device:fw1, NOT rule:block, (A OR B)';
}

let searchBuilderActiveTab = 'builder';
let searchTemplatesMineCache = [];
let searchTemplatesAllCache = [];
let searchTemplateEditingId = '';

function searchTemplatesFetch(url, options) {
    const opts = Object.assign({ credentials: 'same-origin' }, options || {});
    if (!opts.headers) opts.headers = {};
    if (typeof nmAuthHeaders === 'function') {
        opts.headers = nmAuthHeaders(opts.headers);
    } else if (window.NMAuth && typeof NMAuth.nmAuthHeaders === 'function') {
        opts.headers = NMAuth.nmAuthHeaders(opts.headers);
    }
    return fetch(url, opts);
}

async function loadSearchTemplatesMine() {
    const host = document.getElementById('searchTemplatesMine');
    if (!host) return;
    host.innerHTML = '<div class="search-templates-empty">Загрузка…</div>';
    try {
        const res = await searchTemplatesFetch('/api/me/search-templates');
        if (!res.ok) throw new Error('HTTP ' + res.status);
        const data = await res.json();
        searchTemplatesMineCache = Array.isArray(data.templates) ? data.templates : [];
        renderSearchTemplatesMine();
    } catch (e) {
        const detail = e && e.message ? ' (' + e.message + ')' : '';
        host.innerHTML = '<div class="search-templates-empty">Не удалось загрузить шаблоны' + detail + '</div>';
    }
}

async function loadSearchTemplatesAll() {
    const host = document.getElementById('searchTemplatesAll');
    if (!host) return;
    host.innerHTML = '<div class="search-templates-empty">Загрузка…</div>';
    try {
        const res = await searchTemplatesFetch('/api/search-templates');
        if (!res.ok) throw new Error('HTTP ' + res.status);
        const data = await res.json();
        searchTemplatesAllCache = Array.isArray(data.templates) ? data.templates : [];
        renderSearchTemplatesAll();
    } catch (e) {
        host.innerHTML = '<div class="search-templates-empty">Не удалось загрузить шаблоны</div>';
    }
}

function applySearchTemplateQuery(query) {
    searchBuilderForceOpen = true;
    setSearchQuery(query || '', {
        syncInput: true,
        save: true,
        refresh: true,
        updateOverlay: true,
        keepBuilderOpen: true,
    });
}

function renderSearchTemplatesMine() {
    const host = document.getElementById('searchTemplatesMine');
    if (!host) return;
    host.innerHTML = '';
    if (!searchTemplatesMineCache.length) {
        host.innerHTML = '<div class="search-templates-empty">Пока нет сохранённых запросов. Сохраните текущий поиск кнопкой выше.</div>';
        return;
    }
    searchTemplatesMineCache.forEach(function (tpl) {
        const card = document.createElement('div');
        card.className = 'search-template-card';
        if (searchTemplateEditingId === tpl.id) {
            const edit = document.createElement('div');
            edit.className = 'search-template-edit';
            const nameInput = document.createElement('input');
            nameInput.type = 'text';
            nameInput.value = tpl.name || '';
            nameInput.placeholder = 'Название';
            const queryInput = document.createElement('textarea');
            queryInput.value = tpl.query || '';
            queryInput.placeholder = 'Запрос';
            const actions = document.createElement('div');
            actions.className = 'search-template-actions';
            appendBuilderButton(actions, 'search-builder-remove', 'Сохранить', false, async function () {
                try {
                    const res = await searchTemplatesFetch('/api/me/search-templates/' + encodeURIComponent(tpl.id), {
                        method: 'PUT',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ name: nameInput.value, query: queryInput.value }),
                    });
                    if (!res.ok) {
                        const err = await res.json().catch(function () { return {}; });
                        toast(err.error || 'Не удалось сохранить', 'error');
                        return;
                    }
                    searchTemplateEditingId = '';
                    await loadSearchTemplatesMine();
                    toast('Шаблон обновлён', 'success', 2000);
                } catch (e) {
                    toast('Не удалось сохранить', 'error');
                }
            });
            appendBuilderButton(actions, 'search-builder-remove', 'Отмена', false, function () {
                searchTemplateEditingId = '';
                renderSearchTemplatesMine();
            });
            edit.appendChild(nameInput);
            edit.appendChild(queryInput);
            edit.appendChild(actions);
            card.appendChild(edit);
            host.appendChild(card);
            return;
        }

        const head = document.createElement('div');
        head.className = 'search-template-card-head';
        const name = document.createElement('div');
        name.className = 'search-template-name';
        name.textContent = tpl.name || 'Без названия';
        head.appendChild(name);
        card.appendChild(head);

        const query = document.createElement('div');
        query.className = 'search-template-query';
        query.textContent = tpl.query || '';
        card.appendChild(query);

        const actions = document.createElement('div');
        actions.className = 'search-template-actions';
        appendBuilderButton(actions, 'search-builder-remove', 'Применить', false, function () {
            applySearchTemplateQuery(tpl.query);
        });
        appendBuilderButton(actions, 'search-builder-remove', 'Изменить', false, function () {
            searchTemplateEditingId = tpl.id;
            renderSearchTemplatesMine();
        });
        appendBuilderButton(actions, 'search-builder-remove', 'Удалить', false, async function () {
            if (!window.confirm('Удалить шаблон «' + (tpl.name || '') + '»?')) return;
            try {
                const res = await searchTemplatesFetch('/api/me/search-templates/' + encodeURIComponent(tpl.id), {
                    method: 'DELETE',
                });
                if (!res.ok) {
                    toast('Не удалось удалить', 'error');
                    return;
                }
                await loadSearchTemplatesMine();
                toast('Шаблон удалён', 'success', 2000);
            } catch (e) {
                toast('Не удалось удалить', 'error');
            }
        });
        card.appendChild(actions);
        host.appendChild(card);
    });
}

function groupSearchTemplatesByAuthor(list) {
    const groups = {};
    (list || []).forEach(function (tpl) {
        const user = tpl.username || '—';
        if (!groups[user]) groups[user] = [];
        groups[user].push(tpl);
    });
    return Object.keys(groups).sort(function (a, b) {
        return a.toLowerCase().localeCompare(b.toLowerCase(), 'ru');
    }).map(function (username) {
        return { username: username, templates: groups[username] };
    });
}

function renderSearchTemplatesAll() {
    const host = document.getElementById('searchTemplatesAll');
    if (!host) return;
    host.innerHTML = '';
    if (!searchTemplatesAllCache.length) {
        host.innerHTML = '<div class="search-templates-empty">Шаблонов пока нет.</div>';
        return;
    }
    const currentUser = (nmCurrentUser && nmCurrentUser.username) || '';
    groupSearchTemplatesByAuthor(searchTemplatesAllCache).forEach(function (group) {
        const details = document.createElement('details');
        details.className = 'search-template-group';
        if (currentUser && group.username === currentUser) details.open = true;

        const summary = document.createElement('summary');
        summary.className = 'search-template-group-summary';
        const title = document.createElement('span');
        title.className = 'search-template-group-title';
        title.textContent = group.username;
        const count = document.createElement('span');
        count.className = 'search-template-group-count';
        count.textContent = String(group.templates.length);
        summary.appendChild(title);
        summary.appendChild(count);
        details.appendChild(summary);

        const body = document.createElement('div');
        body.className = 'search-template-group-body';
        group.templates.forEach(function (tpl) {
            const card = document.createElement('div');
            card.className = 'search-template-card';
            const head = document.createElement('div');
            head.className = 'search-template-card-head';
            const name = document.createElement('div');
            name.className = 'search-template-name';
            name.textContent = tpl.name || 'Без названия';
            head.appendChild(name);
            card.appendChild(head);
            const query = document.createElement('div');
            query.className = 'search-template-query';
            query.textContent = tpl.query || '';
            card.appendChild(query);
            const actions = document.createElement('div');
            actions.className = 'search-template-actions';
            appendBuilderButton(actions, 'search-builder-remove', 'Применить', false, function () {
                applySearchTemplateQuery(tpl.query);
            });
            card.appendChild(actions);
            body.appendChild(card);
        });
        details.appendChild(body);
        host.appendChild(details);
    });
}

function setSearchBuilderTab(tab) {
    searchBuilderActiveTab = tab === 'mine' || tab === 'all' ? tab : 'builder';
    if (searchBuilderActiveTab === 'all' && !nmIsAdmin) searchBuilderActiveTab = 'builder';
    document.querySelectorAll('.search-builder-tab').forEach(function (btn) {
        btn.classList.toggle('active', btn.getAttribute('data-search-tab') === searchBuilderActiveTab);
    });
    document.querySelectorAll('[data-search-tab-panel]').forEach(function (panel) {
        const match = panel.getAttribute('data-search-tab-panel') === searchBuilderActiveTab;
        panel.hidden = !match;
    });
    if (searchBuilderActiveTab === 'mine') loadSearchTemplatesMine();
    if (searchBuilderActiveTab === 'all') loadSearchTemplatesAll();
}

async function saveCurrentSearchTemplate() {
    const query = (document.getElementById('searchInput')?.value || currentSearch || '').trim();
    if (!query) {
        toast('Сначала введите поисковый запрос', 'warn');
        return;
    }
    const suggested = query.length > 60 ? query.slice(0, 57) + '…' : query;
    const name = window.prompt('Название шаблона', suggested);
    if (name == null) return;
    const trimmed = String(name).trim();
    if (!trimmed) {
        toast('Нужно название', 'warn');
        return;
    }
    try {
        const res = await searchTemplatesFetch('/api/me/search-templates', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name: trimmed, query: query }),
        });
        if (!res.ok) {
            const err = await res.json().catch(function () { return {}; });
            toast(err.error || 'Не удалось сохранить', 'error');
            return;
        }
        await loadSearchTemplatesMine();
        toast('Шаблон сохранён', 'success', 2000);
    } catch (e) {
        toast('Не удалось сохранить', 'error');
    }
}

function syncSearchBuilderTabsVisibility() {
    const allTab = document.getElementById('searchTabAll');
    if (allTab) allTab.style.display = nmIsAdmin ? '' : 'none';
    if (!nmIsAdmin && searchBuilderActiveTab === 'all') {
        setSearchBuilderTab('builder');
    }
}

function searchBuilderPanelOpen() {
    return !!searchBuilderForceOpen;
}

function searchEventInsideSearchBox(event, searchBox) {
    if (!searchBox || !event) return false;
    if (typeof event.composedPath === 'function') {
        const path = event.composedPath();
        if (path && path.indexOf(searchBox) !== -1) return true;
    }
    const target = event.target;
    if (target && typeof searchBox.contains === 'function' && searchBox.contains(target)) return true;
    return false;
}

function appendBuilderJoinSelect(parent, row, idx, onChange) {
    const join = document.createElement('select');
    join.className = 'search-builder-join';
    join.disabled = idx === 0 || !currentSearchBuilderEditable;
    [['AND', 'И'], ['OR', 'ИЛИ']].forEach(function (entry) {
        const option = document.createElement('option');
        option.value = entry[0];
        option.textContent = entry[1];
        if ((row.joinWith || 'AND') === entry[0]) option.selected = true;
        join.appendChild(option);
    });
    join.addEventListener('change', onChange);
    parent.appendChild(join);
}

function appendBuilderNegate(parent, checked, onChange) {
    const negWrap = document.createElement('label');
    negWrap.className = 'search-builder-negate';
    const neg = document.createElement('input');
    neg.type = 'checkbox';
    neg.checked = !!checked;
    neg.disabled = !currentSearchBuilderEditable;
    neg.addEventListener('change', onChange);
    negWrap.appendChild(neg);
    negWrap.appendChild(document.createTextNode('НЕ'));
    parent.appendChild(negWrap);
}

function appendBuilderFieldSelect(parent, selected, onChange) {
    const field = document.createElement('select');
    field.className = 'search-builder-field';
    field.disabled = !currentSearchBuilderEditable;
    SEARCH_BUILDER_FIELDS.forEach(function (key) {
        const option = document.createElement('option');
        option.value = key;
        option.textContent = SEARCH_FIELD_DEFS[key].label;
        if ((selected || 'all') === key) option.selected = true;
        field.appendChild(option);
    });
    field.addEventListener('change', onChange);
    parent.appendChild(field);
}

function appendBuilderValueInput(parent, value, onInput) {
    const input = document.createElement('input');
    input.type = 'text';
    input.className = 'search-builder-value';
    input.placeholder = 'Значение';
    input.value = value || '';
    input.disabled = !currentSearchBuilderEditable;
    input.addEventListener('input', onInput);
    parent.appendChild(input);
}

function appendBuilderButton(parent, className, text, disabled, onClick) {
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = className;
    btn.textContent = text;
    btn.disabled = !!disabled;
    btn.addEventListener('click', function (event) {
        event.preventDefault();
        event.stopPropagation();
        searchBuilderForceOpen = true;
        onClick(event);
    });
    parent.appendChild(btn);
    return btn;
}

function ensureBuilderRowsMutable() {
    if (!Array.isArray(currentSearchBuilderRows) || !currentSearchBuilderRows.length) {
        currentSearchBuilderRows = [createDefaultBuilderRow()];
    }
}

function syncSearchExampleChips() {
    const host = document.getElementById('searchBuilderChips');
    if (!host) return;
    host.innerHTML = '';
    SEARCH_EXAMPLE_CHIPS.forEach(function (chip) {
        const btn = document.createElement('button');
        btn.type = 'button';
        btn.className = 'search-example-chip';
        btn.textContent = chip.label;
        btn.title = chip.query;
        btn.addEventListener('click', function (event) {
            event.preventDefault();
            event.stopPropagation();
            searchBuilderForceOpen = true;
            setSearchQuery(chip.query, {
                syncInput: true,
                save: true,
                refresh: true,
                updateOverlay: true,
                keepBuilderOpen: true,
            });
        });
        host.appendChild(btn);
    });
}

function syncSearchBuilderUI() {
    const panel = document.getElementById('searchBuilderPanel');
    const rowsHost = document.getElementById('searchBuilderRows');
    const hint = document.getElementById('searchBuilderHint');
    const toggle = document.getElementById('btnSearchBuilder');
    const notice = document.getElementById('searchBuilderNotice');
    const add = document.getElementById('btnSearchBuilderAdd');
    const addGroup = document.getElementById('btnSearchBuilderAddGroup');
    const apply = document.getElementById('btnSearchBuilderApply');
    const clear = document.getElementById('btnSearchBuilderClear');
    if (!panel || !rowsHost || !hint || !toggle || !notice) return;

    const open = searchBuilderPanelOpen();
    panel.classList.toggle('open', open);
    toggle.classList.toggle('active', open);
    toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
    hint.textContent = searchBuilderStatusText();
    syncSearchBuilderTabsVisibility();
    syncSearchExampleChips();
    document.querySelectorAll('.search-builder-tab').forEach(function (btn) {
        btn.classList.toggle('active', btn.getAttribute('data-search-tab') === searchBuilderActiveTab);
    });
    document.querySelectorAll('[data-search-tab-panel]').forEach(function (panelEl) {
        const match = panelEl.getAttribute('data-search-tab-panel') === searchBuilderActiveTab;
        panelEl.hidden = !match;
    });

    const rows = currentSearchBuilderState();
    if (add) add.disabled = !currentSearchBuilderEditable;
    if (addGroup) addGroup.disabled = !currentSearchBuilderEditable;
    if (apply) apply.disabled = !currentSearchBuilderEditable;
    if (clear) clear.disabled = false;
    rowsHost.innerHTML = '';
    rows.forEach(function (row, idx) {
        if (row.kind === 'group') {
            const group = document.createElement('div');
            group.className = 'search-builder-group';

            const head = document.createElement('div');
            head.className = 'search-builder-group-head';
            appendBuilderJoinSelect(head, row, idx, function () {
                ensureBuilderRowsMutable();
                currentSearchBuilderRows[idx].joinWith = this.value;
            });
            appendBuilderNegate(head, row.negate, function () {
                ensureBuilderRowsMutable();
                currentSearchBuilderRows[idx].negate = this.checked;
            });

            const opWrap = document.createElement('label');
            opWrap.className = 'search-builder-group-op';
            opWrap.appendChild(document.createTextNode('внутри '));
            const op = document.createElement('select');
            op.disabled = !currentSearchBuilderEditable;
            [['OR', 'ИЛИ'], ['AND', 'И']].forEach(function (entry) {
                const option = document.createElement('option');
                option.value = entry[0];
                option.textContent = entry[1];
                if ((row.op || 'OR') === entry[0]) option.selected = true;
                op.appendChild(option);
            });
            op.addEventListener('change', function () {
                ensureBuilderRowsMutable();
                currentSearchBuilderRows[idx].op = this.value;
            });
            opWrap.appendChild(op);
            head.appendChild(opWrap);

            appendBuilderButton(head, 'search-builder-remove', 'Удалить группу', !currentSearchBuilderEditable || rows.length === 1, function () {
                ensureBuilderRowsMutable();
                currentSearchBuilderRows.splice(idx, 1);
                if (!currentSearchBuilderRows.length) currentSearchBuilderRows.push(createDefaultBuilderRow());
                syncSearchBuilderUI();
            });
            group.appendChild(head);

            const childrenHost = document.createElement('div');
            childrenHost.className = 'search-builder-group-children';
            (row.children || []).forEach(function (child, childIdx) {
                const childRow = document.createElement('div');
                childRow.className = 'search-builder-row search-builder-row-nested';
                appendBuilderNegate(childRow, child.negate, function () {
                    ensureBuilderRowsMutable();
                    currentSearchBuilderRows[idx].children[childIdx].negate = this.checked;
                });
                appendBuilderFieldSelect(childRow, child.field, function () {
                    ensureBuilderRowsMutable();
                    currentSearchBuilderRows[idx].children[childIdx].field = this.value;
                });
                appendBuilderValueInput(childRow, child.value, function () {
                    ensureBuilderRowsMutable();
                    currentSearchBuilderRows[idx].children[childIdx].value = this.value;
                });
                appendBuilderButton(
                    childRow,
                    'search-builder-remove',
                    'Удалить',
                    !currentSearchBuilderEditable || row.children.length <= 1,
                    function () {
                        ensureBuilderRowsMutable();
                        currentSearchBuilderRows[idx].children.splice(childIdx, 1);
                        if (!currentSearchBuilderRows[idx].children.length) {
                            currentSearchBuilderRows[idx].children.push({ negate: false, field: 'all', value: '' });
                        }
                        syncSearchBuilderUI();
                    }
                );
                childrenHost.appendChild(childRow);
            });
            group.appendChild(childrenHost);

            const groupActions = document.createElement('div');
            groupActions.className = 'search-builder-group-actions';
            appendBuilderButton(groupActions, 'search-builder-remove', 'Условие в группу', !currentSearchBuilderEditable, function () {
                ensureBuilderRowsMutable();
                if (!Array.isArray(currentSearchBuilderRows[idx].children)) {
                    currentSearchBuilderRows[idx].children = [];
                }
                currentSearchBuilderRows[idx].children.push({ negate: false, field: 'all', value: '' });
                syncSearchBuilderUI();
            });
            group.appendChild(groupActions);
            rowsHost.appendChild(group);
            return;
        }

        const item = document.createElement('div');
        item.className = 'search-builder-row';
        appendBuilderJoinSelect(item, row, idx, function () {
            ensureBuilderRowsMutable();
            currentSearchBuilderRows[idx].joinWith = this.value;
        });
        appendBuilderNegate(item, row.negate, function () {
            ensureBuilderRowsMutable();
            currentSearchBuilderRows[idx].negate = this.checked;
        });
        appendBuilderFieldSelect(item, row.field, function () {
            ensureBuilderRowsMutable();
            currentSearchBuilderRows[idx].field = this.value;
        });
        appendBuilderValueInput(item, row.value, function () {
            ensureBuilderRowsMutable();
            currentSearchBuilderRows[idx].value = this.value;
        });
        appendBuilderButton(item, 'search-builder-remove', 'Удалить', !currentSearchBuilderEditable || rows.length === 1, function () {
            ensureBuilderRowsMutable();
            currentSearchBuilderRows.splice(idx, 1);
            if (!currentSearchBuilderRows.length) currentSearchBuilderRows.push(createDefaultBuilderRow());
            syncSearchBuilderUI();
        });
        rowsHost.appendChild(item);
    });

    notice.textContent = (!currentSearchBuilderEditable && currentSearch)
        ? 'Сложный запрос: конструктор показывает только подсказку. Для редактирования используйте строку поиска.'
        : '';
    notice.style.display = notice.textContent ? 'block' : 'none';
}

function bindSearchBuilderUI() {
    const toggle = document.getElementById('btnSearchBuilder');
    const add = document.getElementById('btnSearchBuilderAdd');
    const addGroup = document.getElementById('btnSearchBuilderAddGroup');
    const apply = document.getElementById('btnSearchBuilderApply');
    const clear = document.getElementById('btnSearchBuilderClear');
    const searchInput = document.getElementById('searchInput');
    const searchBox = document.querySelector('.topbar .search-box');

    toggle?.addEventListener('click', function (event) {
        event.preventDefault();
        event.stopPropagation();
        searchBuilderForceOpen = !searchBuilderPanelOpen();
        if (searchBuilderForceOpen && (!Array.isArray(currentSearchBuilderRows) || !currentSearchBuilderRows.length)) {
            currentSearchBuilderRows = [createDefaultBuilderRow()];
        }
        syncSearchBuilderUI();
    });

    add?.addEventListener('click', function (event) {
        event.preventDefault();
        event.stopPropagation();
        if (!currentSearchBuilderEditable) return;
        searchBuilderForceOpen = true;
        ensureBuilderRowsMutable();
        currentSearchBuilderRows.push(createDefaultBuilderRow());
        syncSearchBuilderUI();
    });

    addGroup?.addEventListener('click', function (event) {
        event.preventDefault();
        event.stopPropagation();
        if (!currentSearchBuilderEditable) return;
        searchBuilderForceOpen = true;
        ensureBuilderRowsMutable();
        currentSearchBuilderRows.push(createDefaultBuilderGroup());
        syncSearchBuilderUI();
    });

    apply?.addEventListener('click', function (event) {
        event.preventDefault();
        event.stopPropagation();
        const query = serializeSearchBuilderRows(currentSearchBuilderRows);
        searchBuilderForceOpen = true;
        setSearchQuery(query, {
            syncInput: true,
            save: true,
            refresh: true,
            updateOverlay: true,
            keepBuilderOpen: true,
        });
        if (searchInput) searchInput.focus();
    });

    clear?.addEventListener('click', function (event) {
        event.preventDefault();
        event.stopPropagation();
        currentSearchBuilderRows = [createDefaultBuilderRow()];
        searchBuilderForceOpen = true;
        setSearchQuery('', {
            syncInput: true,
            save: true,
            refresh: true,
            updateOverlay: true,
            keepBuilderOpen: true,
        });
    });

    document.addEventListener('click', function (event) {
        if (!searchBox || !searchBuilderPanelOpen()) return;
        if (searchEventInsideSearchBox(event, searchBox)) return;
        searchBuilderForceOpen = false;
        syncSearchBuilderUI();
    });

    document.querySelectorAll('.search-builder-tab').forEach(function (btn) {
        btn.addEventListener('click', function (event) {
            event.preventDefault();
            event.stopPropagation();
            searchBuilderForceOpen = true;
            setSearchBuilderTab(btn.getAttribute('data-search-tab'));
        });
    });

    document.getElementById('btnSearchTemplateSave')?.addEventListener('click', function (event) {
        event.preventDefault();
        event.stopPropagation();
        searchBuilderForceOpen = true;
        saveCurrentSearchTemplate();
    });
    document.getElementById('btnSearchTemplateReload')?.addEventListener('click', function (event) {
        event.preventDefault();
        event.stopPropagation();
        searchBuilderForceOpen = true;
        loadSearchTemplatesMine();
    });
    document.getElementById('btnSearchTemplateReloadAll')?.addEventListener('click', function (event) {
        event.preventDefault();
        event.stopPropagation();
        searchBuilderForceOpen = true;
        loadSearchTemplatesAll();
    });
}
