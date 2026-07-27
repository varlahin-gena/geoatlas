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
const SEARCH_COMPLEXITY_MESSAGE = 'Этот запрос использует группировку или смешанные AND/OR. Его можно выполнять, но удобнее редактировать прямо в строке.';

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
        joinWith: joinWith || 'AND',
        negate: negate,
        field: term.field || 'all',
        value: term.value || '',
    };
}

function searchBuilderRowsFromAst(ast) {
    if (!ast) return { editable: true, rows: [] };
    const single = searchBuilderTermToRow(ast, 'AND');
    if (single) return { editable: true, rows: [single] };

    function walk(node, currentOp, acc) {
        if (!node) return false;
        const row = searchBuilderTermToRow(node, currentOp || 'AND');
        if (row) {
            acc.push(row);
            return true;
        }
        if (node.type !== 'AND' && node.type !== 'OR') return false;
        const op = node.type;
        if (currentOp && currentOp !== op) return false;
        return walk(node.left, op, acc) && walk(node.right, op, acc);
    }

    const rows = [];
    if (!walk(ast, null, rows) || !rows.length) {
        return { editable: false, rows: [], reason: SEARCH_COMPLEXITY_MESSAGE };
    }
    rows[0].joinWith = 'AND';
    return { editable: true, rows: rows };
}

function createDefaultBuilderRow() {
    return { joinWith: 'AND', negate: false, field: 'all', value: '' };
}

function currentSearchBuilderState() {
    if (!currentSearchBuilderEditable && currentSearch) return [];
    if (currentSearchBuilderEditable && Array.isArray(currentSearchBuilderRows) && currentSearchBuilderRows.length) {
        return currentSearchBuilderRows.map(function (row, idx) {
            return {
                joinWith: idx === 0 ? 'AND' : (row.joinWith === 'OR' ? 'OR' : 'AND'),
                negate: !!row.negate,
                field: SEARCH_FIELD_DEFS[row.field] ? row.field : 'all',
                value: row.value || '',
            };
        });
    }
    if (currentSearchMode === 'simple' && currentSearch) {
        return [{ joinWith: 'AND', negate: false, field: 'all', value: currentSearch }];
    }
    return [createDefaultBuilderRow()];
}

function serializeSearchBuilderRows(rows) {
    const prepared = (rows || [])
        .map(function (row, idx) {
            const value = String(row && row.value || '').trim();
            if (!value) return null;
            const field = SEARCH_FIELD_DEFS[row.field] ? row.field : 'all';
            const prefix = field === 'all' ? '' : field + ':';
            const expr = prefix + quoteSearchValue(value);
            return {
                joinWith: idx === 0 ? '' : ((row.joinWith === 'OR') ? 'OR ' : 'AND '),
                negate: row.negate ? 'NOT ' : '',
                expr: expr,
            };
        })
        .filter(Boolean);
    return prepared.map(function (row) {
        return row.joinWith + row.negate + row.expr;
    }).join(' ');
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
        currentSearchBuilderRows = compiled.builderRows.map(function (row) { return Object.assign({}, row); });
    } else if (compiled.mode === 'simple' && compiled.raw) {
        currentSearchBuilderRows = [{ joinWith: 'AND', negate: false, field: 'all', value: compiled.raw }];
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
    return 'Подсказка: country:Россия AND device:fw1, NOT rule:block, ip:1.2.3.4';
}

function searchBuilderPanelOpen() {
    return !!searchBuilderForceOpen;
}

function syncSearchBuilderUI() {
    const panel = document.getElementById('searchBuilderPanel');
    const rowsHost = document.getElementById('searchBuilderRows');
    const hint = document.getElementById('searchBuilderHint');
    const toggle = document.getElementById('btnSearchBuilder');
    const notice = document.getElementById('searchBuilderNotice');
    const add = document.getElementById('btnSearchBuilderAdd');
    const apply = document.getElementById('btnSearchBuilderApply');
    const clear = document.getElementById('btnSearchBuilderClear');
    if (!panel || !rowsHost || !hint || !toggle || !notice) return;

    const open = searchBuilderPanelOpen();
    panel.classList.toggle('open', open);
    toggle.classList.toggle('active', open);
    toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
    hint.textContent = searchBuilderStatusText();

    const rows = currentSearchBuilderState();
    if (add) add.disabled = !currentSearchBuilderEditable;
    if (apply) apply.disabled = !currentSearchBuilderEditable;
    if (clear) clear.disabled = false;
    rowsHost.innerHTML = '';
    rows.forEach(function (row, idx) {
        const item = document.createElement('div');
        item.className = 'search-builder-row';

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
        join.addEventListener('change', function () {
            currentSearchBuilderRows[idx].joinWith = this.value;
        });
        item.appendChild(join);

        const negWrap = document.createElement('label');
        negWrap.className = 'search-builder-negate';
        const neg = document.createElement('input');
        neg.type = 'checkbox';
        neg.checked = !!row.negate;
        neg.disabled = !currentSearchBuilderEditable;
        neg.addEventListener('change', function () {
            currentSearchBuilderRows[idx].negate = this.checked;
        });
        negWrap.appendChild(neg);
        negWrap.appendChild(document.createTextNode('НЕ'));
        item.appendChild(negWrap);

        const field = document.createElement('select');
        field.className = 'search-builder-field';
        field.disabled = !currentSearchBuilderEditable;
        SEARCH_BUILDER_FIELDS.forEach(function (key) {
            const option = document.createElement('option');
            option.value = key;
            option.textContent = SEARCH_FIELD_DEFS[key].label;
            if ((row.field || 'all') === key) option.selected = true;
            field.appendChild(option);
        });
        field.addEventListener('change', function () {
            currentSearchBuilderRows[idx].field = this.value;
        });
        item.appendChild(field);

        const value = document.createElement('input');
        value.type = 'text';
        value.className = 'search-builder-value';
        value.placeholder = 'Значение';
        value.value = row.value || '';
        value.disabled = !currentSearchBuilderEditable;
        value.addEventListener('input', function () {
            currentSearchBuilderRows[idx].value = this.value;
        });
        item.appendChild(value);

        const remove = document.createElement('button');
        remove.type = 'button';
        remove.className = 'search-builder-remove';
        remove.textContent = 'Удалить';
        remove.disabled = !currentSearchBuilderEditable || rows.length === 1;
        remove.addEventListener('click', function () {
            currentSearchBuilderRows.splice(idx, 1);
            if (!currentSearchBuilderRows.length) currentSearchBuilderRows.push(createDefaultBuilderRow());
            syncSearchBuilderUI();
        });
        item.appendChild(remove);

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
    const apply = document.getElementById('btnSearchBuilderApply');
    const clear = document.getElementById('btnSearchBuilderClear');
    const searchInput = document.getElementById('searchInput');
    const searchBox = document.querySelector('.topbar .search-box');

    toggle?.addEventListener('click', function () {
        searchBuilderForceOpen = !searchBuilderPanelOpen();
        if (searchBuilderForceOpen && (!Array.isArray(currentSearchBuilderRows) || !currentSearchBuilderRows.length)) {
            currentSearchBuilderRows = [createDefaultBuilderRow()];
        }
        syncSearchBuilderUI();
    });

    add?.addEventListener('click', function () {
        if (!currentSearchBuilderEditable) return;
        currentSearchBuilderRows.push({
            joinWith: currentSearchBuilderRows.length ? 'AND' : 'AND',
            negate: false,
            field: 'all',
            value: '',
        });
        syncSearchBuilderUI();
    });

    apply?.addEventListener('click', function () {
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

    clear?.addEventListener('click', function () {
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
        if (searchBox.contains(event.target)) return;
        searchBuilderForceOpen = false;
        syncSearchBuilderUI();
    });
}
