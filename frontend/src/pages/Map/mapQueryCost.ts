/** Оценка «тяжести» map/events запросов — зеркало backend/internal/mapsearch/cost.go */

export type MapQueryCostTier = 'light' | 'medium' | 'heavy';

export interface MapQueryCost {
  tier: MapQueryCostTier;
  reasons: string[];
  limitCap: number;
}

const CAP_HEAVY = 3000;
const CAP_HEAVY_FILTER = 8000;
const CAP_MEDIUM = 10000;

function isIPGroup(groupBy: string): boolean {
  const g = (groupBy || '').trim().toLowerCase();
  return g === 'ip' || g === 'subnet';
}

function hasMapFilter(country: string | null, search: string): boolean {
  return Boolean((country || '').trim() || (search || '').trim());
}

function parsePeriodSpanDays(period: string, periodFrom: string, periodTo: string): number | null {
  if (period === 'custom') {
    const from = periodFrom ? new Date(periodFrom) : null;
    const to = periodTo ? new Date(periodTo) : new Date();
    if (!from || Number.isNaN(from.getTime()) || Number.isNaN(to.getTime())) return null;
    return Math.max(0, (to.getTime() - from.getTime()) / (24 * 3600 * 1000));
  }
  const d = period.match(/^(\d+)d$/);
  if (d) return parseInt(d[1], 10);
  return null;
}

export function assessMapQueryCost(opts: {
  period: string;
  periodFrom: string;
  periodTo: string;
  groupBy: string;
  search: string;
  focusedCountry: string | null;
  repActive: boolean;
}): MapQueryCost {
  const groupBy = (opts.groupBy || 'city').trim().toLowerCase();
  const filtered = hasMapFilter(opts.focusedCountry, opts.search);
  const ipGroup = isIPGroup(groupBy);
  const reasons: string[] = [];
  let heavy = false;
  let medium = false;

  const add = (reason: string, h: boolean, m: boolean) => {
    reasons.push(reason);
    if (h) heavy = true;
    if (m) medium = true;
  };

  const m = opts.period.match(/^(\d+)m$/);
  const h = opts.period.match(/^(\d+)h$/);
  const spanDays = parsePeriodSpanDays(opts.period, opts.periodFrom, opts.periodTo);

  if (m) {
    const mins = parseInt(m[1], 10);
    if (ipGroup) add('группировка по IP на коротком периоде (минуты)', true, false);
    else if (mins <= 60) add('короткий период (минуты)', false, true);
  } else if (h) {
    const hours = parseInt(h[1], 10);
    if (ipGroup && hours <= 12) add('группировка по IP на периоде до 12 часов', true, false);
    else if (ipGroup) add('группировка по IP на часовом периоде', false, true);
    else if (hours >= 6) add('период от 6 часов без daily geo-agg', false, true);
  } else if (spanDays != null) {
    if (ipGroup && spanDays >= 7) add('группировка по IP на периоде 7+ дней', true, false);
    else if (ipGroup && spanDays >= 3 && !filtered) add('группировка по IP без фильтра на периоде 3+ дней', true, false);
    else if (ipGroup) add('группировка по IP', false, true);
    else if (spanDays >= 14 && !filtered) add('длинный период без фильтра', false, true);
    else if (spanDays >= 7 && !filtered) add('период 7+ дней без фильтра', false, true);
  }

  if (opts.repActive && ipGroup) {
    add('фильтр репутации при группировке по IP', true, false);
  }

  let tier: MapQueryCostTier = 'light';
  let limitCap = 50000;
  if (heavy) {
    tier = 'heavy';
    limitCap = filtered ? CAP_HEAVY_FILTER : CAP_HEAVY;
  } else if (medium) {
    tier = 'medium';
    limitCap = CAP_MEDIUM;
  }

  return { tier, reasons, limitCap };
}

export function effectiveMapLimit(requested: number, cost: MapQueryCost): { applied: number; capped: boolean } {
  let applied = requested > 0 ? requested : 10000;
  let capped = false;
  if (cost.limitCap > 0 && applied > cost.limitCap) {
    applied = cost.limitCap;
    capped = true;
  }
  return { applied: Math.max(1, applied), capped };
}

/** Сообщение для баннера на карте; null — не показывать. */
export function mapQueryWarning(opts: {
  cost: MapQueryCost;
  requestedLimit: number;
  effectiveLimit: number;
  limitCapped: boolean;
  source?: string;
}): string | null {
  const parts: string[] = [];
  const liveSource = isLiveTrafficLogsSource(opts.source);

  if (opts.cost.tier === 'heavy') {
    parts.push('Тяжёлый запрос к ClickHouse');
    if (opts.limitCapped) {
      parts.push(
        `лимит снижен с ${opts.requestedLimit.toLocaleString('ru-RU')} до ${opts.effectiveLimit.toLocaleString('ru-RU')}`,
      );
    }
    parts.push('сузьте период, добавьте фильтр в поиске или переключите группировку на «город»/«страна»');
    if (liveSource) {
      parts.push('источник: traffic_logs');
    }
  } else if (opts.cost.tier === 'medium') {
    if (opts.limitCapped) {
      parts.push(
        `Лимит дуг снижен до ${opts.effectiveLimit.toLocaleString('ru-RU')} для ускорения запроса`,
      );
    } else if (liveSource) {
      parts.push('Запрос читает сырые логи — возможна задержка');
    }
  }

  return parts.length ? parts.join('. ') + '.' : null;
}

function isLiveTrafficLogsSource(source: string | undefined): boolean {
  const s = (source || '').toLowerCase();
  return s.includes('ip_live') || s.includes('traffic_logs');
}
