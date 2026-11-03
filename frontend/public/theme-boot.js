/*!
 * Early theme attribute before CSS paints (no inline script → CSP script-src 'self').
 * Only light | dark; legacy `system` resolves once from OS.
 */
(function () {
  try {
    var t = localStorage.getItem('nm.theme');
    if (t === 'system') {
      t =
        window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches
          ? 'dark'
          : 'light';
      try {
        localStorage.setItem('nm.theme', t);
      } catch (e) {
        /* ignore */
      }
    }
    if (t !== 'light' && t !== 'dark') {
      t = 'dark';
    }
    document.documentElement.setAttribute('data-theme', t);
  } catch (e) {
    /* ignore */
  }
})();
