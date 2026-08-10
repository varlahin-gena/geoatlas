/**
 * Общие хелперы локальной авторизации UI.
 * Подключать после /config.js на защищённых страницах.
 * Тема: /theme.css + /common.css + data-theme на <html>.
 * UI-хелперы: /common.js (escapeHTML, toast, mountAdminTopbar).
 */
(function (global) {
  'use strict';

  const ROLE_ADMIN = 'administrator';
  const ROLE_OPERATOR = 'operator';
  const THEME_KEY = 'nm.theme';

  function getTheme() {
    try {
      return localStorage.getItem(THEME_KEY) === 'light' ? 'light' : 'dark';
    } catch (e) {
      return 'dark';
    }
  }

  function themeLabel(theme) {
    return theme === 'light' ? 'Светлая' : 'Тёмная';
  }

  function applyTheme(theme) {
    const t = theme === 'light' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', t);
    try {
      localStorage.setItem(THEME_KEY, t);
    } catch (e) { /* ignore */ }
    document.dispatchEvent(new CustomEvent('nm-theme-change', { detail: { theme: t } }));
    return t;
  }

  function toggleTheme() {
    return applyTheme(getTheme() === 'light' ? 'dark' : 'light');
  }

  // Применить сохранённую тему как можно раньше (если auth.js в <head> — без FOUC).
  applyTheme(getTheme());

  async function fetchMe() {
    const res = await fetch('/api/auth/me', { credentials: 'same-origin' });
    if (res.status === 401) return null;
    if (!res.ok) throw new Error('auth me: HTTP ' + res.status);
    return res.json();
  }

  function loginUrl(next) {
    const q = next ? '?next=' + encodeURIComponent(next) : '';
    return '/login.html' + q;
  }

  function changePasswordUrl(next) {
    const q = next ? '?next=' + encodeURIComponent(next) : '';
    return '/change-password.html' + q;
  }

  function safeNext(next) {
    if (!next || typeof next !== 'string') return '/index.html';
    return next.startsWith('/') && !next.startsWith('//') ? next : '/index.html';
  }

  /**
   * Редирект на login / смену пароля при необходимости.
   * opts.admin — только administrator
   * opts.allowMustReset — не редиректить на change-password (для самой страницы смены)
   */
  async function requireLogin(opts) {
    const options = opts || {};
    const needAdmin = !!options.admin;
    const allowMustReset = !!options.allowMustReset;
    let user;
    try {
      user = await fetchMe();
    } catch (e) {
      user = null;
    }
    if (!user) {
      location.replace(loginUrl(location.pathname + location.search));
      return null;
    }
    if (user.must_reset_password && !user.authDisabled && !allowMustReset) {
      location.replace(changePasswordUrl(location.pathname + location.search));
      return null;
    }
    if (needAdmin && user.role !== ROLE_ADMIN && !user.authDisabled) {
      location.replace('/index.html');
      return null;
    }
    global.__nmReputationEnabled = user.reputationEnabled !== false;
    global.__nmUIAuthEnabled = !user.authDisabled;
    applyAdminVisibility(user);
    return user;
  }

  async function logout() {
    try {
      await fetch('/api/auth/logout', {
        method: 'POST',
        credentials: 'same-origin',
        headers: nmAuthHeaders(),
      });
    } catch (e) { /* ignore */ }
    location.replace(loginUrl());
  }

  function applyAdminVisibility(user) {
    const isAdmin = !user || user.authDisabled || user.role === ROLE_ADMIN;
    document.querySelectorAll('[data-admin-only]').forEach((el) => {
      if (isAdmin) el.style.removeProperty('display');
      else el.style.display = 'none';
    });
    const reputationEnabled = !user || user.reputationEnabled !== false;
    const uiAuthEnabled = !user || !user.authDisabled;
    document.querySelectorAll('[data-reputation-only]').forEach((el) => {
      if (reputationEnabled) el.style.removeProperty('display');
      else el.style.display = 'none';
    });
    document.querySelectorAll('[data-auth-only]').forEach((el) => {
      if (uiAuthEnabled) el.style.removeProperty('display');
      else el.style.display = 'none';
    });
    const navOpts = {
      isAdmin: isAdmin,
      reputationEnabled: reputationEnabled,
      uiAuthEnabled: uiAuthEnabled,
    };
    const sidebar = document.getElementById('adminSidebar');
    if (
      sidebar &&
      global.NMUI &&
      typeof global.NMUI.mountAdminSidebar === 'function' &&
      (sidebar.getAttribute('data-nm-dynamic-nav') === '1' ||
        sidebar.children.length === 0)
    ) {
      global.NMUI.mountAdminSidebar(undefined, navOpts);
    }
    const adminNav = document.getElementById('adminNavSection');
    if (
      adminNav &&
      global.NMUI &&
      typeof global.NMUI.mountPageNav === 'function'
    ) {
      global.NMUI.mountPageNav(adminNav, Object.assign({ adminLinksOnly: true }, navOpts));
    }
    return isAdmin;
  }

  function applyReputationVisibility(user) {
    applyAdminVisibility(user);
    return !user || user.reputationEnabled !== false;
  }

  function csrfToken() {
    const m = document.cookie.match(/(?:^|;\s*)nm_csrf=([^;]*)/);
    return m ? decodeURIComponent(m[1]) : '';
  }

  function nmAuthHeaders(extra) {
    const h = Object.assign({}, extra || {});
    const token = (global.NM_CONFIG && global.NM_CONFIG.apiAuthToken) || '';
    if (token) h['Authorization'] = 'Bearer ' + token;
    const csrf = csrfToken();
    if (csrf) h['X-CSRF-Token'] = csrf;
    return h;
  }

  function closeAllUserMenus() {
    document.querySelectorAll('.nm-user-menu.open').forEach((el) => {
      el.classList.remove('open');
      const btn = el.querySelector('.nm-user-menu-trigger');
      if (btn) btn.setAttribute('aria-expanded', 'false');
    });
  }

  function renderUserBar(user, host) {
    if (!host || !user || user.authDisabled) return;
    host.innerHTML = '';

    const fio = (user.full_name || '').trim();
    const displayName = fio || user.username;
    const roleRu = user.role === ROLE_ADMIN ? 'Администратор' : 'Оператор';

    const menu = document.createElement('div');
    menu.className = 'nm-user-menu';

    const trigger = document.createElement('button');
    trigger.type = 'button';
    trigger.className = 'nm-user-menu-trigger';
    trigger.setAttribute('aria-haspopup', 'menu');
    trigger.setAttribute('aria-expanded', 'false');
    trigger.title = fio
      ? ('ФИО: ' + fio + '\nЛогин: ' + user.username + '\nРоль: ' + roleRu)
      : ('Логин: ' + user.username + '\nРоль: ' + roleRu);

    const nameEl = document.createElement('span');
    nameEl.className = 'nm-user-name';
    nameEl.textContent = displayName;

    const caret = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    caret.setAttribute('class', 'nm-user-caret');
    caret.setAttribute('viewBox', '0 0 24 24');
    caret.setAttribute('fill', 'none');
    caret.setAttribute('stroke', 'currentColor');
    caret.setAttribute('stroke-width', '2');
    const caretPath = document.createElementNS('http://www.w3.org/2000/svg', 'path');
    caretPath.setAttribute('d', 'M6 9l6 6 6-6');
    caret.appendChild(caretPath);

    trigger.appendChild(nameEl);
    trigger.appendChild(caret);

    const dropdown = document.createElement('div');
    dropdown.className = 'nm-user-menu-dropdown';
    dropdown.setAttribute('role', 'menu');

    const meta = document.createElement('div');
    meta.className = 'nm-user-menu-meta';
    meta.innerHTML =
      '<div class="nm-meta-name"></div><div class="nm-meta-role"></div><div class="nm-meta-version" hidden></div>';
    meta.querySelector('.nm-meta-name').textContent = displayName;
    meta.querySelector('.nm-meta-role').textContent =
      (fio ? '@' + user.username + ' · ' : '') + roleRu;

    const versionEl = meta.querySelector('.nm-meta-version');
    fetch('/api/system/version', { credentials: 'same-origin' })
      .then(function (res) {
        if (!res.ok) return null;
        return res.json();
      })
      .then(function (data) {
        if (!data || !versionEl) return;
        const label = (data.display || data.ref || data.version || '').trim();
        if (!label) return;
        let text = 'Версия: ' + label;
        if (data.source === 'main' && data.commit) {
          text = 'Версия: main · ' + data.commit;
        } else if (data.source === 'release' && data.version && label.indexOf(data.version) === -1) {
          text = 'Версия: ' + label + ' (' + data.version + ')';
        }
        versionEl.textContent = text;
        versionEl.hidden = false;
        versionEl.title =
          (data.ref ? 'ref: ' + data.ref : '') +
          (data.commit ? '\ncommit: ' + data.commit : '') +
          (data.version ? '\nproduct: ' + data.version : '');
      })
      .catch(function () { /* ignore */ });

    const themeBtn = document.createElement('button');
    themeBtn.type = 'button';
    themeBtn.className = 'nm-user-menu-item';
    themeBtn.setAttribute('role', 'menuitem');
    const themeLabelEl = document.createElement('span');
    themeLabelEl.textContent = 'Тема';
    const themeValueEl = document.createElement('span');
    themeValueEl.className = 'nm-theme-value';
    themeValueEl.textContent = themeLabel(getTheme());
    themeBtn.appendChild(themeLabelEl);
    themeBtn.appendChild(themeValueEl);
    themeBtn.addEventListener('click', function (e) {
      e.stopPropagation();
      const next = toggleTheme();
      themeValueEl.textContent = themeLabel(next);
    });

    const sep = document.createElement('div');
    sep.className = 'nm-user-menu-sep';

    const logoutBtn = document.createElement('button');
    logoutBtn.type = 'button';
    logoutBtn.className = 'nm-user-menu-item danger';
    logoutBtn.setAttribute('role', 'menuitem');
    logoutBtn.textContent = 'Выйти';
    logoutBtn.addEventListener('click', function (e) {
      e.stopPropagation();
      closeAllUserMenus();
      logout();
    });

    dropdown.appendChild(meta);
    dropdown.appendChild(themeBtn);
    dropdown.appendChild(sep);
    dropdown.appendChild(logoutBtn);

    trigger.addEventListener('click', function (e) {
      e.stopPropagation();
      const open = !menu.classList.contains('open');
      closeAllUserMenus();
      if (open) {
        menu.classList.add('open');
        trigger.setAttribute('aria-expanded', 'true');
      }
    });

    menu.appendChild(trigger);
    menu.appendChild(dropdown);
    host.appendChild(menu);

    if (!global.__nmUserMenuDocBound) {
      global.__nmUserMenuDocBound = true;
      document.addEventListener('click', closeAllUserMenus);
      document.addEventListener('keydown', function (e) {
        if (e.key === 'Escape') closeAllUserMenus();
      });
    }
  }

  global.NMAuth = {
    ROLE_ADMIN,
    ROLE_OPERATOR,
    fetchMe,
    requireLogin,
    logout,
    applyAdminVisibility,
    applyReputationVisibility,
    nmAuthHeaders,
    renderUserBar,
    loginUrl,
    changePasswordUrl,
    safeNext,
    getTheme,
    applyTheme,
    toggleTheme,
    themeLabel,
  };
})(window);
