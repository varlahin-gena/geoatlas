import type { AuditEvent, DREvent } from '@/api/systemTypes';

/** Short RU labels for common audit action codes (title keeps the raw code). */
const AUDIT_ACTION_LABELS: Record<string, string> = {
  'auth.login.success': 'Вход',
  'auth.login.failed': 'Неверный пароль',
  'auth.logout': 'Выход',
  'auth.session.revoke': 'Завершение входа',
  'auth.session.revoke_all': 'Завершение всех входов',
  'auth.password.change': 'Смена пароля',
  'auth.password.reset': 'Сброс пароля',
  'auth.login_policy.update': 'Защита от подбора пароля',
  'auth.user.create': 'Новый пользователь',
  'auth.user.update': 'Изменение пользователя',
  'auth.user.delete': 'Удаление пользователя',
  'auth.user.role_change': 'Смена роли',
  'auth.token.create': 'Новый ключ API',
  'auth.token.revoke': 'Отзыв ключа API',
  'auth.token.rotate': 'Замена ключа API',
  'security.csrf.reject': 'Поддельный запрос',
  'security.proxy_gate.reject': 'Доступ через прокси',
  'security.api_rate_limit': 'Слишком частые запросы',
  'users.create': 'Создание пользователя',
  'users.update': 'Изменение пользователя',
  'users.delete': 'Удаление пользователя',
  'users.password.reset': 'Сброс пароля',
  'tokens.create': 'Создание токена',
  'tokens.revoke': 'Отзыв токена',
  'tokens.rotate': 'Ротация токена',
  'backup.create': 'Создание бэкапа',
  'backup.attach': 'Подключение бэкапа',
  'backup.detach': 'Отключение бэкапа',
  'backup.delete': 'Удаление бэкапа',
  'backup.schedule.update': 'Расписание бэкапа',
  'system.retention.update': 'TTL хранения',
};

export function auditActionLabel(action?: string): string {
  if (!action) return '—';
  return AUDIT_ACTION_LABELS[action] || action;
}

function pad2(n: number): string {
  return String(n).padStart(2, '0');
}

function formatBool(v: unknown, on = 'вкл', off = 'выкл'): string {
  if (v === true) return on;
  if (v === false) return off;
  return String(v ?? '—');
}

function formatScheduleChangeDetails(d: Record<string, unknown>): string {
  const parts: string[] = [];
  const prev = d.prev_enabled;
  const next = d.enabled;
  if (typeof prev === 'boolean' && typeof next === 'boolean' && prev !== next) {
    parts.push(`авто: ${formatBool(prev)} → ${formatBool(next)}`);
  } else if (typeof next === 'boolean') {
    parts.push(`авто: ${formatBool(next)}`);
  }
  if (typeof d.hour === 'number' && typeof d.minute === 'number') {
    parts.push(`время: ${pad2(d.hour)}:${pad2(d.minute)}`);
  }
  if (typeof d.timezone === 'string' && d.timezone) {
    parts.push(`TZ: ${d.timezone}`);
  }
  if (typeof d.keep === 'number') {
    parts.push(`keep: ${d.keep}`);
  }
  if (typeof d.include_edges === 'boolean') {
    parts.push(`edges: ${formatBool(d.include_edges)}`);
  }
  if (typeof d.include_auth === 'boolean') {
    parts.push(`auth: ${formatBool(d.include_auth)}`);
  }
  return parts.length ? parts.join('; ') : '—';
}

const DETAIL_KEY_LABELS: Record<string, string> = {
  role: 'роль',
  sid: 'сессия',
  reason: 'причина',
  username: 'учётная запись',
  user: 'пользователь',
  ip: 'адрес',
  count: 'попыток',
  token_id: 'ключ',
  error: 'ошибка',
};

const DETAIL_VALUE_LABELS: Record<string, string> = {
  invalid_credentials: 'неверный пароль',
  locked: 'учётная запись заблокирована',
  lockout: 'слишком много попыток',
  administrator: 'администратор',
  admin: 'администратор',
  operator: 'оператор',
  viewer: 'просмотр',
  readonly: 'только чтение',
};

function formatDetailValue(v: unknown): string {
  if (typeof v === 'boolean') return formatBool(v);
  const s = String(v ?? '—');
  return DETAIL_VALUE_LABELS[s] || s;
}

function formatAuditDetails(action?: string, details?: Record<string, unknown>): string {
  if (!details || !Object.keys(details).length) return '—';
  if (details.error) return String(details.error);

  if (action === 'backup.schedule.update') {
    return formatScheduleChangeDetails(details);
  }

  return Object.entries(details)
    .map(([k, v]) => {
      const key = DETAIL_KEY_LABELS[k] || k;
      // Long opaque ids: keep short for scanning
      if ((k === 'sid' || k === 'token_id') && typeof v === 'string' && v.length > 12) {
        return `${key}: ${v.slice(0, 8)}…`;
      }
      return `${key}: ${formatDetailValue(v)}`;
    })
    .join('; ');
}

export function formatDRMessage(item: DREvent): string {
  if (item.action === 'backup.schedule.update' && item.meta) {
    const details = formatScheduleChangeDetails(item.meta);
    if (details !== '—') return details;
  }
  return item.message || '—';
}

export function formatAuditRow(item: AuditEvent): string {
  return formatAuditDetails(item.action, item.details);
}
