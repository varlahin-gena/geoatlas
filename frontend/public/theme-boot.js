/*!
 * Early theme attribute before CSS paints (no inline script → CSP script-src 'self').
 */
(function () {
  try {
    var t = localStorage.getItem('nm.theme');
    if (t === 'light' || t === 'dark') {
      document.documentElement.setAttribute('data-theme', t);
    }
  } catch (e) {
    /* ignore */
  }
})();
