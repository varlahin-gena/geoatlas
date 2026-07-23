'use strict';

/** Активные фильтры репутации (категории и/или list_name). */
let repFilterCategories = new Set();
let repFilterLists = new Set();
let repFilterSide = 'any'; // any|src|dst|both
let repMenuOpen = false;

function lineHasReputationHits(line) {
    return (line.src_reputation && line.src_reputation.length) ||
        (line.dst_reputation && line.dst_reputation.length);
}

function hitsMatchFilters(hits) {
    if (!hits || !hits.length) return false;
    if (!repFilterCategories.size && !repFilterLists.size) return true;
    return hits.some(function (h) {
        if (repFilterLists.size && repFilterLists.has(h.list)) return true;
        if (repFilterCategories.size && repFilterCategories.has(h.category)) return true;
        return false;
    });
}

function lineMatchesReputation(line) {
    if (!repFilterCategories.size && !repFilterLists.size) return true;
    const srcOk = hitsMatchFilters(line.src_reputation);
    const dstOk = hitsMatchFilters(line.dst_reputation);
    switch (repFilterSide) {
        case 'src': return srcOk;
        case 'dst': return dstOk;
        case 'both': return srcOk && dstOk;
        default: return srcOk || dstOk;
    }
}

function reputationFilterActiveCount() {
    return repFilterCategories.size + repFilterLists.size;
}

function collectReputationMenuTree(lines) {
    // category -> Set(list)
    const tree = {};
    (lines || []).forEach(function (line) {
        [].concat(line.src_reputation || [], line.dst_reputation || []).forEach(function (h) {
            if (!h || !h.category) return;
            if (!tree[h.category]) tree[h.category] = new Set();
            if (h.list) tree[h.category].add(h.list);
        });
    });
    return tree;
}

function formatReputationHits(hits) {
    if (!hits || !hits.length) return '';
    return hits.map(function (h) {
        return (h.category || '?') + '/' + (h.list || '?');
    }).join(', ');
}

function updateReputationMenuUI() {
    const btn = document.getElementById('btnReputationFilter');
    const panel = document.getElementById('reputationMenu');
    const badge = document.getElementById('repFilterBadge');
    const body = document.getElementById('reputationMenuBody');
    if (!btn || !panel || !body) return;

    const gb = typeof currentGroupBy === 'function' ? currentGroupBy() : 'city';
    const ipMode = gb === 'ip';
    btn.disabled = !ipMode;
    btn.title = ipMode
        ? 'Фильтр по репутационным спискам'
        : 'Доступно в режиме Группа: IP';

    const n = reputationFilterActiveCount();
    if (badge) {
        if (n > 0 && ipMode) {
            badge.style.display = 'inline-flex';
            badge.textContent = String(n);
        } else {
            badge.style.display = 'none';
        }
    }
    btn.classList.toggle('active', n > 0 && ipMode);

    if (!ipMode) {
        body.innerHTML = '<div class="rep-menu-empty">Переключите «Группа» на IP</div>';
        return;
    }

    const tree = collectReputationMenuTree(typeof allLines !== 'undefined' ? allLines : []);
    const cats = Object.keys(tree).sort();
    if (!cats.length) {
        body.innerHTML = '<div class="rep-menu-empty">Нет совпадений на карте</div>';
        return;
    }

    let html = '';
    cats.forEach(function (cat) {
        const lists = Array.from(tree[cat]).sort();
        const catChecked = repFilterCategories.has(cat);
        html += '<label class="rep-cat"><input type="checkbox" data-rep-cat="' +
            escapeHTML(cat) + '"' + (catChecked ? ' checked' : '') + ' /> <strong>' +
            escapeHTML(cat) + '</strong></label>';
        lists.forEach(function (list) {
            const listChecked = repFilterLists.has(list);
            html += '<label class="rep-list"><input type="checkbox" data-rep-list="' +
                escapeHTML(list) + '"' + (listChecked ? ' checked' : '') + ' /> ' +
                escapeHTML(list) + '</label>';
        });
    });
    body.innerHTML = html;
}

function applyReputationFilterChange() {
    saveUIState();
    syncViewToURL();
    updateReputationMenuUI();
    if (typeof viewMode !== 'undefined' && viewMode === 'map') updateDeck();
    else if (typeof updateGlobe === 'function') updateGlobe();
    if (typeof updateMapOverlay === 'function') updateMapOverlay();
}

function clearReputationFilters() {
    repFilterCategories.clear();
    repFilterLists.clear();
    repFilterSide = 'any';
    const sideEl = document.getElementById('repFilterSide');
    if (sideEl) sideEl.value = 'any';
    applyReputationFilterChange();
}

function bindReputationMenu() {
    const btn = document.getElementById('btnReputationFilter');
    const panel = document.getElementById('reputationMenu');
    const body = document.getElementById('reputationMenuBody');
    const clearBtn = document.getElementById('btnRepFilterClear');
    const sideEl = document.getElementById('repFilterSide');
    if (!btn || !panel) return;

    btn.addEventListener('click', function (e) {
        e.stopPropagation();
        if (btn.disabled) return;
        repMenuOpen = !repMenuOpen;
        panel.classList.toggle('open', repMenuOpen);
        if (repMenuOpen) updateReputationMenuUI();
    });
    document.addEventListener('click', function (e) {
        if (!repMenuOpen) return;
        if (panel.contains(e.target) || btn.contains(e.target)) return;
        repMenuOpen = false;
        panel.classList.remove('open');
    });
    clearBtn?.addEventListener('click', function (e) {
        e.preventDefault();
        clearReputationFilters();
    });
    sideEl?.addEventListener('change', function () {
        repFilterSide = this.value || 'any';
        applyReputationFilterChange();
    });
    body?.addEventListener('change', function (e) {
        const t = e.target;
        if (!t || t.tagName !== 'INPUT') return;
        if (t.dataset.repCat) {
            if (t.checked) repFilterCategories.add(t.dataset.repCat);
            else repFilterCategories.delete(t.dataset.repCat);
        }
        if (t.dataset.repList) {
            if (t.checked) repFilterLists.add(t.dataset.repList);
            else repFilterLists.delete(t.dataset.repList);
        }
        applyReputationFilterChange();
    });

    document.getElementById('groupBy')?.addEventListener('change', function () {
        updateReputationMenuUI();
        if (currentGroupBy() !== 'ip' && reputationFilterActiveCount()) {
            // фильтр остаётся в state, но не применяется (getVisibleLines)
            if (viewMode === 'map') updateDeck();
            else updateGlobe();
            updateMapOverlay();
        }
    });
}

function escapeHTML(v) {
    return String(v ?? '').replace(/[&<>"']/g, function (ch) {
        return ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[ch];
    });
}
