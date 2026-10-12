// Paste into DevTools console on:
//   https://www.bestpractices.dev/en/projects/15404/passing/edit
// Then click the form Save/Submit button (or run: document.querySelector('input[type=submit],button[type=submit]')?.click()).
(async () => {
  const res = await fetch(
    'https://raw.githubusercontent.com/varlahin-gena/geoatlas/main/scripts/openssf-passing-fill.json'
  );
  const data = await res.json();
  const set = (sel, val) => {
    const el = document.querySelector(sel);
    if (!el) return false;
    el.focus();
    el.value = val;
    el.dispatchEvent(new Event('input', { bubbles: true }));
    el.dispatchEvent(new Event('change', { bubbles: true }));
    return true;
  };
  set('input[name="project[name]"]', data.name);
  set('textarea[name="project[description]"], input[name="project[description]"]', data.description);
  set('input[name="project[homepage_url]"]', data.homepage_url);
  set('input[name="project[license]"]', data.license);
  let ok = 0, miss = 0;
  for (const [k, v] of Object.entries(data.fields)) {
    if (k.endsWith('_status')) {
      const radio = document.querySelector(`input[type=radio][name="project[${k}]"][value="${v}"]`);
      if (radio) { radio.click(); ok++; } else { miss++; console.warn('missing status', k, v); }
    } else if (k.endsWith('_justification')) {
      const el = document.querySelector(`textarea[name="project[${k}]"], input[name="project[${k}]"]`);
      if (el) {
        el.value = v;
        el.dispatchEvent(new Event('input', { bubbles: true }));
        el.dispatchEvent(new Event('change', { bubbles: true }));
        ok++;
      } else { miss++; console.warn('missing justification', k); }
    }
  }
  console.log(`Filled ok=${ok} miss=${miss}. Now save the form.`);
})();
