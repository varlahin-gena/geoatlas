const DENSITY_KEY = 'nm.uiDensity';

export type UiDensity = 'comfortable' | 'compact';

export function getDensity(): UiDensity {
  try {
    return localStorage.getItem(DENSITY_KEY) === 'compact' ? 'compact' : 'comfortable';
  } catch {
    return 'comfortable';
  }
}

export function densityLabel(d: UiDensity): string {
  return d === 'compact' ? 'Компакт' : 'Комфорт';
}

function applyDensity(d: UiDensity): UiDensity {
  const next = d === 'compact' ? 'compact' : 'comfortable';
  document.documentElement.setAttribute('data-density', next);
  try {
    localStorage.setItem(DENSITY_KEY, next);
  } catch {
    /* ignore */
  }
  document.dispatchEvent(new CustomEvent('ga-density-change', { detail: { density: next } }));
  return next;
}

export function toggleDensity(): UiDensity {
  return applyDensity(getDensity() === 'compact' ? 'comfortable' : 'compact');
}

applyDensity(getDensity());
