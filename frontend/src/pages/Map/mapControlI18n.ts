/** MapLibre ships English control titles; map UI is Russian-first. */
const CONTROL_LABELS: Record<string, string> = {
  'Zoom in': 'Приблизить',
  'Zoom out': 'Отдалить',
  'Reset bearing to north': 'Сбросить ориентацию на север',
  'Drag to rotate map, click to reset north': 'Тяните для поворота, клик — на север',
  'Toggle attribution': 'Показать источник карты',
  'Map compass': 'Компас карты',
};

/** Replace known English titles/aria-labels on MapLibre control buttons. */
export function localizeMapLibreControls(root: ParentNode): number {
  let updated = 0;
  root.querySelectorAll('button').forEach((btn) => {
    const title = btn.getAttribute('title') || '';
    const aria = btn.getAttribute('aria-label') || '';
    const nextTitle = CONTROL_LABELS[title];
    const nextAria = CONTROL_LABELS[aria];
    if (nextTitle) {
      btn.setAttribute('title', nextTitle);
      updated += 1;
    }
    if (nextAria) {
      btn.setAttribute('aria-label', nextAria);
      if (!nextTitle && !title) btn.setAttribute('title', nextAria);
      updated += 1;
    } else if (nextTitle && aria === title) {
      btn.setAttribute('aria-label', nextTitle);
    }
  });
  return updated;
}
