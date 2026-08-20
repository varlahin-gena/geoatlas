import { useState } from 'react';
import { fetchGeoMissing } from '@/api/geo';
import { listParseErrors } from '@/api/parseErrors';
import { isAbortError } from '@/api/client';
import { useAuth } from '@/auth/AuthContext';
import { usePolling } from '@/lib/usePolling';
import { buildPeriodQuery } from '@/pages/Map/mapConstants';
import { formatNavBadge } from './nav';

export type NavBadges = Partial<Record<string, string>>;

const POLL_MS = 60_000;
const PARSE_LIMIT = 100;
const GEO_LIMIT = 100;

/**
 * Lightweight queue badges for admin nav (parse errors + geo-missing).
 * Counts are capped by fetch limit and shown as "99+" when saturated.
 */
export function useNavBadges(): NavBadges {
  const { isAdmin } = useAuth();
  const [badges, setBadges] = useState<NavBadges>({});

  usePolling(
    async (signal) => {
      if (!isAdmin) {
        setBadges({});
        return;
      }
      try {
        const periodQs = buildPeriodQuery('12h', '', '').replace(/^&/, '');
        const [parseRes, geoRes] = await Promise.all([
          listParseErrors({ limit: PARSE_LIMIT }),
          fetchGeoMissing(`${periodQs}&limit=${GEO_LIMIT}`),
        ]);
        if (signal.aborted) return;

        const parseCount = (parseRes.errors || []).length;
        const geoCount = Number(geoRes.summary?.unique_ips ?? geoRes.items?.length ?? 0);

        const next: NavBadges = {};
        const parseBadge = formatNavBadge(parseCount, PARSE_LIMIT - 1);
        if (parseBadge) next['/parse-errors'] = parseBadge;
        const geoBadge = formatNavBadge(geoCount, GEO_LIMIT - 1);
        if (geoBadge) next['/geo-missing'] = geoBadge;
        setBadges(next);
      } catch (e) {
        if (isAbortError(e) || signal.aborted) return;
        /* keep last known badges on transient errors */
      }
    },
    POLL_MS,
    isAdmin,
  );

  return badges;
}
