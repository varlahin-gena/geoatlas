/**
 * Общие UI-хелперы ГеоАтлас.
 * Подключать до page-скриптов; после /config.js и /auth.js на защищённых страницах.
 */
(function (global) {
  'use strict';

  function $(id) {
    return document.getElementById(id);
  }

  function escapeHTML(v) {
    return String(v ?? '').replace(/[&<>"']/g, function (ch) {
      return ({
        '&': '&amp;',
        '<': '&lt;',
        '>': '&gt;',
        '"': '&quot;',
        "'": '&#39;',
      })[ch];
    });
  }

  function fmtNumber(n) {
    return (n ?? 0).toLocaleString('ru-RU');
  }

  function fmtDate(iso) {
    if (!iso) return '—';
    try {
      return new Date(iso).toLocaleString('ru-RU');
    } catch (e) {
      return String(iso);
    }
  }

  function ensureToastHost() {
    let host = document.getElementById('toastHost');
    if (!host) {
      host = document.createElement('div');
      host.id = 'toastHost';
      host.className = 'toast-host';
      host.setAttribute('aria-live', 'polite');
    }
    // Держим у body (не внутри .app с overflow:hidden), поверх layout.
    if (host.parentNode !== document.body) {
      document.body.appendChild(host);
    } else if (document.body.lastElementChild !== host) {
      document.body.appendChild(host);
    }
    return host;
  }

  /**
   * Показывает уведомление без автоскрытия — закрытие только крестиком.
   * @param {string} msg
   * @param {'info'|'success'|'error'|'warn'|string} [kind]
   * @param {number} [_timeout] устарел, игнорируется (совместимость вызовов)
   */
  function toast(msg, kind, _timeout) {
    const host = ensureToastHost();
    const el = document.createElement('div');
    el.className = 'toast' + (kind ? ' ' + kind : '');
    el.setAttribute('role', kind === 'error' ? 'alert' : 'status');

    const body = document.createElement('div');
    body.className = 'toast-body';
    body.textContent = msg == null ? '' : String(msg);

    const closeBtn = document.createElement('button');
    closeBtn.type = 'button';
    closeBtn.className = 'toast-close';
    closeBtn.setAttribute('aria-label', 'Закрыть');
    closeBtn.title = 'Закрыть';
    closeBtn.textContent = '×';
    closeBtn.addEventListener('click', function () {
      el.style.opacity = '0';
      setTimeout(function () {
        el.remove();
      }, 200);
    });

    el.appendChild(closeBtn);
    el.appendChild(body);
    host.appendChild(el);
  }

  var SIDEBAR_COLLAPSE_KEY = 'nm.adminSidebarCollapsed';
  var ICON =
    '<svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">';

  /** Ссылки между страницами (как в сайдбаре карты). */
  var PAGE_NAV = [
    {
      href: '/',
      label: 'Карта',
      match: ['/', '/index.html'],
      adminOnly: false,
      icon:
        ICON +
        '<path d="M1 6v15l7-3 8 3 7-3V3l-7 3-8-3-7 3z"/><path d="M8 3v15M16 6v15"/></svg>',
    },
    {
      href: '/system.html',
      label: 'Мониторинг',
      adminOnly: true,
      icon:
        ICON +
        '<path d="M3 3v18h18"/><path d="M7 14l3-3 3 3 5-5"/></svg>',
    },
    {
      href: '/parser-test.html',
      label: 'Тест парсеров',
      adminOnly: true,
      icon:
        ICON +
        '<polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg>',
    },
    {
      href: '/parse-errors.html',
      label: 'Ошибки парсинга',
      adminOnly: true,
      icon:
        ICON +
        '<path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>' +
        '<line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>',
    },
    {
      href: '/geo-missing.html',
      label: 'IP без GeoIP',
      adminOnly: true,
      icon:
        ICON +
        '<circle cx="12" cy="12" r="10"/><path d="M12 8v4M12 16h.01"/></svg>',
    },
    {
      href: '/geo-ranges.html',
      label: 'База GeoIP',
      adminOnly: true,
      icon:
        ICON +
        '<ellipse cx="12" cy="5" rx="9" ry="3"/>' +
        '<path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/><path d="M3 12c0 1.66 4 3 9 3s9-1.34 9-3"/></svg>',
    },
    {
      href: '/reputation.html',
      label: 'Репутация IP',
      adminOnly: true,
      requiresReputation: true,
      icon:
        ICON +
        '<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>',
    },
    {
      href: '/users.html',
      label: 'Пользователи',
      adminOnly: true,
      requiresUIAuth: true,
      icon:
        ICON +
        '<path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/>' +
        '<path d="M23 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75"/></svg>',
    },
    {
      href: '/api-tokens.html',
      label: 'API-токены',
      adminOnly: true,
      icon:
        ICON +
        '<rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>' +
        '<path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>',
    },
  ];

  function normalizePath(pathname) {
    var p = pathname || '/';
    if (p.length > 1 && p.charAt(p.length - 1) === '/') p = p.slice(0, -1);
    return p || '/';
  }

  function isNavActive(item, pathname) {
    var path = normalizePath(pathname);
    var matches = item.match || [item.href];
    for (var i = 0; i < matches.length; i++) {
      if (normalizePath(matches[i]) === path) return true;
    }
    return false;
  }

  function readSidebarCollapsed() {
    try {
      return localStorage.getItem(SIDEBAR_COLLAPSE_KEY) === '1';
    } catch (e) {
      return false;
    }
  }

  function writeSidebarCollapsed(collapsed) {
    try {
      localStorage.setItem(SIDEBAR_COLLAPSE_KEY, collapsed ? '1' : '0');
    } catch (e) {}
  }

  /**
   * Собирает layout: sidebar | admin-main (topbar + content).
   * @returns {{ app: HTMLElement, sidebar: HTMLElement }}
   */
  function ensureAdminLayout() {
    var existing = document.getElementById('adminApp');
    if (existing) {
      return {
        app: existing,
        sidebar: document.getElementById('adminSidebar'),
      };
    }

    if (!document.body.classList.contains('page-admin')) {
      document.body.classList.add('page-admin');
    }

    var app = document.createElement('div');
    app.id = 'adminApp';
    app.className = 'app';

    var sidebar = document.createElement('aside');
    sidebar.id = 'adminSidebar';
    sidebar.className = 'sidebar';
    sidebar.setAttribute('aria-label', 'Навигация');

    var adminMain = document.createElement('div');
    adminMain.className = 'admin-main';

    var topbar = document.getElementById('adminTopbar');
    var main =
      document.querySelector('main.page-content') ||
      document.querySelector('main');

    if (topbar) adminMain.appendChild(topbar);
    if (main) adminMain.appendChild(main);

    app.appendChild(sidebar);
    app.appendChild(adminMain);

    var anchor = document.body.querySelector('script');
    if (anchor) {
      document.body.insertBefore(app, anchor);
    } else {
      document.body.appendChild(app);
    }

    if (readSidebarCollapsed()) {
      app.classList.add('sidebar-collapsed');
    }

    return { app: app, sidebar: sidebar };
  }

  /**
   * Нормализует флаги доступа для PAGE_NAV.
   * @param {{ isAdmin?: boolean, reputationEnabled?: boolean, uiAuthEnabled?: boolean, adminLinksOnly?: boolean }} [opts]
   * @returns {{ isAdmin: boolean, reputationEnabled: boolean, uiAuthEnabled: boolean, adminLinksOnly: boolean, path: string }}
   */
  function resolveNavOpts(pathname, opts) {
    var o = opts || {};
    var reputationEnabled;
    if (typeof o.reputationEnabled === 'boolean') {
      reputationEnabled = o.reputationEnabled;
    } else if (typeof global.__nmReputationEnabled === 'boolean') {
      reputationEnabled = global.__nmReputationEnabled;
    } else {
      reputationEnabled = true;
    }
    var uiAuthEnabled;
    if (typeof o.uiAuthEnabled === 'boolean') {
      uiAuthEnabled = o.uiAuthEnabled;
    } else if (typeof global.__nmUIAuthEnabled === 'boolean') {
      uiAuthEnabled = global.__nmUIAuthEnabled;
    } else {
      uiAuthEnabled = true;
    }
    return {
      isAdmin: o.isAdmin !== false,
      reputationEnabled: reputationEnabled,
      uiAuthEnabled: uiAuthEnabled,
      adminLinksOnly: !!o.adminLinksOnly,
      path: pathname || (typeof location !== 'undefined' ? location.pathname : '/'),
    };
  }

  /**
   * HTML ссылок из PAGE_NAV с фильтрами ролей.
   * @param {string} [pathname]
   * @param {{ isAdmin?: boolean, reputationEnabled?: boolean, uiAuthEnabled?: boolean, adminLinksOnly?: boolean }} [opts]
   * @returns {string}
   */
  function buildPageNavLinksHtml(pathname, opts) {
    var nav = resolveNavOpts(pathname, opts);
    var html = '';
    for (var i = 0; i < PAGE_NAV.length; i++) {
      var item = PAGE_NAV[i];
      if (nav.adminLinksOnly && !item.adminOnly) continue;
      if (item.adminOnly && !nav.isAdmin) continue;
      if (item.requiresReputation && !nav.reputationEnabled) continue;
      if (item.requiresUIAuth && !nav.uiAuthEnabled) continue;
      var active = isNavActive(item, nav.path);
      html +=
        '<a href="' +
        escapeHTML(item.href) +
        '" class="side-btn' +
        (active ? ' active' : '') +
        '"' +
        (active ? ' aria-current="page"' : '') +
        ' title="' +
        escapeHTML(item.label) +
        '">' +
        item.icon +
        '<span class="label">' +
        escapeHTML(item.label) +
        '</span></a>';
    }
    return html;
  }

  /**
   * HTML бокового меню навигации.
   * @param {string} [pathname]
   * @param {{ isAdmin?: boolean, reputationEnabled?: boolean, uiAuthEnabled?: boolean }} [opts]
   * @returns {string}
   */
  function buildAdminSidebarHtml(pathname, opts) {
    return (
      '<div class="sidebar-header">' +
      '<img class="logo" src="/logo.png" alt="" width="28" height="28" aria-hidden="true" />' +
      '<div class="title">ГеоАтлас</div>' +
      '</div>' +
      '<div class="sidebar-section">' +
      '<div class="sidebar-section-title">Разделы</div>' +
      buildPageNavLinksHtml(pathname, opts) +
      '</div>' +
      '<div class="sidebar-collapse-btn">' +
      '<button type="button" class="side-btn" id="btnToggleAdminSidebar" title="Развернуть / свернуть меню">' +
      ICON +
      '<path d="M15 18l-6-6 6-6"/></svg>' +
      '<span class="label">Свернуть меню</span>' +
      '</button></div>'
    );
  }

  /**
   * Монтирует боковое меню и layout админ-страницы.
   * @param {string} [pathname]
   * @param {{ isAdmin?: boolean, reputationEnabled?: boolean, uiAuthEnabled?: boolean }} [opts]
   * @returns {{ app: HTMLElement, sidebar: HTMLElement }}
   */
  function mountAdminSidebar(pathname, opts) {
    var layout = ensureAdminLayout();
    if (!layout.sidebar) return layout;

    layout.sidebar.setAttribute('data-nm-dynamic-nav', '1');

    if (readSidebarCollapsed()) {
      layout.app.classList.add('sidebar-collapsed');
    } else {
      layout.app.classList.remove('sidebar-collapsed');
    }

    layout.sidebar.innerHTML = buildAdminSidebarHtml(pathname, opts);

    var btn = document.getElementById('btnToggleAdminSidebar');
    if (btn) {
      btn.addEventListener('click', function () {
        layout.app.classList.toggle('sidebar-collapsed');
        writeSidebarCollapsed(layout.app.classList.contains('sidebar-collapsed'));
      });
    }

    return layout;
  }

  /**
   * Встраивает ссылки PAGE_NAV в контейнер (секция «Администрирование» на карте).
   * @param {string|HTMLElement} host
   * @param {{ isAdmin?: boolean, reputationEnabled?: boolean, uiAuthEnabled?: boolean, adminLinksOnly?: boolean, pathname?: string, title?: string }} [opts]
   * @returns {HTMLElement|null}
   */
  function mountPageNav(host, opts) {
    var el = typeof host === 'string' ? document.querySelector(host) : host;
    if (!el) return null;
    var o = opts || {};
    var title = o.title || 'Администрирование';
    var navOpts = {
      isAdmin: o.isAdmin,
      reputationEnabled: o.reputationEnabled,
      uiAuthEnabled: o.uiAuthEnabled,
      adminLinksOnly: o.adminLinksOnly !== false,
    };
    el.setAttribute('data-nm-dynamic-nav', '1');
    el.innerHTML =
      '<div class="sidebar-section-title">' +
      escapeHTML(title) +
      '</div>' +
      buildPageNavLinksHtml(o.pathname, navOpts);
    return el;
  }

  var STATUS_REFRESH_MS = 5000;
  var STATUS_PILL_ICON =
    '<svg class="status-pill-icon" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">' +
    '<path d="M14 3h7v7M10 14L21 3M21 14v5a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5"/></svg>';
  var _statusPollTimer = null;
  var _statusPillIsAdmin = false;

  function systemHealthPillHTML(isAdmin) {
    if (isAdmin) {
      return (
        '<a href="/system.html" id="systemHealthPill" class="status-pill" ' +
        'style="text-decoration:none;cursor:pointer;" title="Открыть мониторинг системы">' +
        '<span class="dot"></span>' +
        '<span id="systemHealthText">— загрузка —</span>' +
        STATUS_PILL_ICON +
        '</a>'
      );
    }
    return (
      '<span id="systemHealthPill" class="status-pill" title="Состояние системы">' +
      '<span class="dot"></span>' +
      '<span id="systemHealthText">— загрузка —</span>' +
      '</span>'
    );
  }

  /**
   * Кликабельность индикатора только для администраторов.
   * @param {boolean} isAdmin
   */
  function applySystemHealthPillAccess(isAdmin) {
    _statusPillIsAdmin = !!isAdmin;
    const pill = document.getElementById('systemHealthPill');
    if (!pill) return;

    const textEl = document.getElementById('systemHealthText');
    const text = textEl ? textEl.textContent : '— загрузка —';
    const cls = pill.className;
    const html = systemHealthPillHTML(_statusPillIsAdmin);
    const wrap = document.createElement('div');
    wrap.innerHTML = html;
    const next = wrap.firstElementChild;
    next.className = cls;
    const nextText = next.querySelector('#systemHealthText');
    if (nextText) nextText.textContent = text;
    pill.replaceWith(next);
  }

  async function fetchSystemHealth() {
    const pill = document.getElementById('systemHealthPill');
    const text = document.getElementById('systemHealthText');
    if (!pill || !text) return;

    try {
      const res = await fetch('/api/system/status', { credentials: 'same-origin' });
      if (!res.ok) throw new Error('HTTP ' + res.status);
      const status = await res.json();
      const alerts = status.alerts || [];
      const level = status.level || 'ok';

      pill.classList.remove('ok', 'warn', 'bad');
      if (level === 'error') {
        pill.classList.add('bad');
        text.textContent = '⚠ ' + alerts.length + ' проблем';
        pill.title = alerts.map(function (a) {
          return '[' + a.level + '] ' + a.target + ': ' + a.message;
        }).join('\n');
      } else if (level === 'warn') {
        pill.classList.add('warn');
        text.textContent = alerts.length + ' предупр.';
        pill.title = alerts.map(function (a) {
          return '[' + a.level + '] ' + a.target + ': ' + a.message;
        }).join('\n');
      } else {
        pill.classList.add('ok');
        text.textContent = 'Система ОК';
        pill.title = _statusPillIsAdmin
          ? 'Кликни, чтобы открыть мониторинг'
          : 'Состояние системы';
      }
    } catch (e) {
      pill.classList.remove('ok', 'warn');
      pill.classList.add('bad');
      text.textContent = 'API недоступен';
      pill.title = 'Не удалось получить статус системы';
    }
  }

  /**
   * @param {{ isAdmin?: boolean }} [opts]
   */
  function startSystemHealthPolling(opts) {
    const o = opts || {};
    if (typeof o.isAdmin === 'boolean') {
      applySystemHealthPillAccess(o.isAdmin);
    }
    fetchSystemHealth();
    if (_statusPollTimer) clearInterval(_statusPollTimer);
    _statusPollTimer = setInterval(function () {
      if (document.hidden) return;
      fetchSystemHealth();
    }, STATUS_REFRESH_MS);
  }

  /**
   * Единая шапка админ-страниц (+ боковое меню навигации).
   * @param {HTMLElement|string} host
   * @param {{
   *   title?: string,
   *   subtitle?: string,
   *   userBar?: boolean,
   *   sidebar?: boolean,
   *   systemHealth?: boolean,
   * }} opts
   * @returns {{ userBarHost: HTMLElement|null }}
   */
  function mountAdminTopbar(host, opts) {
    const el = typeof host === 'string' ? document.querySelector(host) : host;
    const o = opts || {};
    if (!el) return { userBarHost: null };

    if (o.sidebar !== false) {
      mountAdminSidebar();
    }

    const title = o.title || 'ГеоАтлас';
    const subtitle = o.subtitle || '';
    const withUserBar = o.userBar !== false;
    const withHealth = o.systemHealth !== false;

    if (!el.classList.contains('topbar')) el.classList.add('topbar');

    let html =
      '<div class="title">' +
      escapeHTML(title);
    if (subtitle) {
      html += ' <span class="sub">' + escapeHTML(subtitle) + '</span>';
    }
    html += '</div><div class="topbar-spacer"></div>';
    if (withUserBar) {
      html += '<div id="userBarHost"></div>';
    }
    if (withHealth) {
      html += systemHealthPillHTML(true);
      _statusPillIsAdmin = true;
    }
    el.innerHTML = html;

    return { userBarHost: withUserBar ? document.getElementById('userBarHost') : null };
  }

  global.NMUI = {
    $,
    escapeHTML,
    fmtNumber,
    fmtDate,
    toast,
    ensureToastHost,
    mountAdminSidebar,
    mountPageNav,
    mountAdminTopbar,
    applySystemHealthPillAccess,
    fetchSystemHealth,
    startSystemHealthPolling,
  };

  global.escapeHTML = escapeHTML;
  global.fmtNumber = fmtNumber;
})(window);
