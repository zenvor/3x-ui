import type { ReactNode } from 'react';
import { act, renderHook } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { useSponsorsQuery } from '@/api/queries/useSponsorsQuery';
import { keys } from '@/api/queryKeys';
import type { Sponsor, SponsorList } from '@/generated/types';
import { HttpUtil, Msg } from '@/utils';

const START = Date.parse('2026-10-11T00:00:00Z');

function sponsor(id: string, from: number, until: number): Sponsor {
  return {
    id,
    name: id,
    from: new Date(from).toISOString(),
    until: new Date(until).toISOString(),
    slots: ['dashboard'],
    title: {},
    text: {},
    link: 'https://example.test/',
  };
}

function mount(cached: SponsorList) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  client.setQueryData(keys.sponsors(), cached);
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  const hook = renderHook(() => useSponsorsQuery(), { wrapper });
  return { ...hook, client };
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(START);
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.useRealTimers();
});

describe('sponsor query time windows', () => {
  it('discovers a newly started booking while the page stays open', async () => {
    const active = sponsor('new', START + 5_000, START + 120_000);
    const get = vi
      .spyOn(HttpUtil, 'get')
      .mockResolvedValue(new Msg(true, '', { sponsors: [active] }));
    const hook = mount({ sponsors: [] });
    try {
      expect(hook.result.current.data.sponsors).toEqual([]);
      await act(async () => {
        await vi.advanceTimersByTimeAsync(60_001);
      });
      expect(get).toHaveBeenCalledOnce();
      expect(hook.result.current.data.sponsors.map((s) => s.id)).toEqual(['new']);
      hook.unmount();
      await act(async () => {
        await vi.advanceTimersByTimeAsync(120_000);
      });
      expect(get).toHaveBeenCalledOnce();
    } finally {
      hook.unmount();
      hook.client.clear();
    }
  });

  it('removes an expired booking at its boundary even when refresh fails', async () => {
    const active = sponsor('ending', START - 1_000, START + 5_000);
    const get = vi.spyOn(HttpUtil, 'get').mockRejectedValue(new Error('offline'));
    const hook = mount({ contact: 'https://example.test/contact', sponsors: [active] });
    try {
      expect(hook.result.current.data.sponsors).toHaveLength(1);
      await act(async () => {
        await vi.advanceTimersByTimeAsync(4_999);
      });
      expect(hook.result.current.data.sponsors).toHaveLength(1);
      await act(async () => {
        await vi.advanceTimersByTimeAsync(1);
      });
      expect(hook.result.current.data.sponsors).toEqual([]);
      expect(hook.result.current.data.contact).toBe('https://example.test/contact');
      expect(get).not.toHaveBeenCalled();
      await act(async () => {
        await vi.advanceTimersByTimeAsync(55_001);
      });
      expect(get).toHaveBeenCalledOnce();
      expect(hook.result.current.data.sponsors).toEqual([]);
      expect(hook.client.getQueryData<SponsorList>(keys.sponsors())?.sponsors).toEqual([active]);
    } finally {
      hook.unmount();
      hook.client.clear();
    }
  });

  it('enforces start boundaries and disabled bookings in cached data', async () => {
    const next = sponsor('next', START + 5_000, START + 10_000);
    const disabled = { ...sponsor('disabled', START - 1, START + 10_000), enable: false };
    const hook = mount({ sponsors: [next, disabled] });
    try {
      expect(hook.result.current.data.sponsors).toEqual([]);
      await act(async () => {
        await vi.advanceTimersByTimeAsync(5_000);
      });
      expect(hook.result.current.data.sponsors.map((s) => s.id)).toEqual(['next']);
      await act(async () => {
        await vi.advanceTimersByTimeAsync(5_000);
      });
      expect(hook.result.current.data.sponsors).toEqual([]);
    } finally {
      hook.unmount();
      hook.client.clear();
    }
  });
});
