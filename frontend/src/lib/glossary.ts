/** Shared RU product language (P3). API field names stay English. */

export const TERM = {
  hunts: 'Охоты',
  hunt: 'Охота',
  episode: 'Эпизод',
  episodes: 'Эпизоды',
  peer: 'Пир',
  peers: 'Пиры',
  peerHint: 'связанный узел / IP',
  scope: 'Область',
} as const;

export const COPY = {
  episodesLoadFailed: 'Не удалось загрузить эпизоды',
  huntsLoadFailed: 'Не удалось загрузить охоты',
  huntsEmptyTitle: 'Нет охот',
  huntsEmptyDesc:
    'Сохраните текущий вид карты кнопкой «Охота» — период, группировка и фильтры останутся в одном месте.',
  huntDeleteConfirm: 'Удалить охоту?',
  huntsLoading: 'Загрузка охот…',
  peersHead: 'Пиры источника',
  peersEmpty: 'Нет связей за выбранное окно (часто для private IP без гео).',
  peersLoading: 'Загрузка пиров…',
  loginLead: 'Карта связей по syslog',
  loginLocal: 'Локальный вход',
} as const;

export function scopeLabel(scope: string): string {
  switch (scope) {
    case 'read':
      return 'чтение';
    case 'ops':
      return 'ops';
    case 'admin':
      return 'admin';
    default:
      return scope;
  }
}

export function scopeOptionLabel(scope: 'read' | 'ops' | 'admin'): string {
  switch (scope) {
    case 'read':
      return 'чтение — карта';
    case 'ops':
      return 'ops — ingest/upload';
    case 'admin':
      return 'admin — полный API';
  }
}
