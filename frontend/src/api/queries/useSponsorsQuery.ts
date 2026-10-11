import { useQuery } from '@tanstack/react-query';
import { useEffect, useState } from 'react';

import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';
import type { SponsorList } from '@/generated/types';

const EMPTY: SponsorList = { sponsors: [] };
const REFRESH_INTERVAL_MS = 60_000;

async function fetchSponsors(): Promise<SponsorList> {
  const msg = await HttpUtil.get<SponsorList>('/sponsors', undefined, { silent: true });
  if (!msg?.success || !msg.obj) return EMPTY;
  return { contact: msg.obj.contact, sponsors: msg.obj.sponsors ?? [] };
}

export function useSponsorsQuery() {
  const [clock, setClock] = useState(Date.now);
  const query = useQuery({
    queryKey: keys.sponsors(),
    queryFn: fetchSponsors,
    staleTime: REFRESH_INTERVAL_MS,
    refetchInterval: REFRESH_INTERVAL_MS,
    refetchIntervalInBackground: false,
    retry: false,
  });
  const data = query.data ?? EMPTY;
  const now = Math.max(clock, query.dataUpdatedAt);

  // The API omits future bookings; polling discovers them. Known expiry times
  // are enforced locally even when the next request is delayed or fails.
  useEffect(() => {
    const boundaries = data.sponsors
      .flatMap((sponsor) => [
        sponsor.from ? Date.parse(sponsor.from) : NaN,
        Date.parse(sponsor.until),
      ])
      .filter((at) => at > now);
    if (!boundaries.length) return;
    const delay = Math.min(Math.max(Math.min(...boundaries) - Date.now(), 1), 2_147_483_647);
    const timer = setTimeout(() => setClock(Date.now()), delay);
    return () => clearTimeout(timer);
  }, [data.sponsors, now]);

  return {
    data: {
      ...data,
      sponsors: data.sponsors.filter(
        (sponsor) =>
          sponsor.enable !== false &&
          (!sponsor.from || Date.parse(sponsor.from) <= now) &&
          Date.parse(sponsor.until) > now,
      ),
    },
    fetched: query.isFetched,
  };
}
