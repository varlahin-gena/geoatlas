import { createHunt, type HuntMapState } from '@/api/hunts';

const DEFAULT_SCHEDULE = {
  enabled: false,
  interval_min: 60,
  edge_threshold: 0,
  edge_ratio: 3,
};

export async function promptSaveHuntFromMap(
  mapState: HuntMapState,
  toast: (msg: string, kind?: 'success' | 'error' | 'warn') => void,
) {
  const name = window.prompt('Название saved hunt');
  if (!name?.trim()) return;
  try {
    await createHunt({
      name: name.trim(),
      map: mapState,
      schedule: DEFAULT_SCHEDULE,
    });
    toast('Hunt сохранён', 'success');
  } catch (e) {
    toast(e instanceof Error ? e.message : 'Ошибка сохранения hunt', 'error');
  }
}
