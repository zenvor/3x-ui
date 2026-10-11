import type { ReactNode } from 'react';
import { act, renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { useSubconverterTemplate, useSubconverterTemplates } from '@/api/queries/useSubconverter';
import { keys } from '@/api/queryKeys';
import { HttpUtil, Msg } from '@/utils';

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

const status = {
  variant: 'general',
  url: 'https://templates.example/current.yaml',
  cached: true,
  updatedAt: 1000,
  checkedAt: 2000,
  checkResult: 'unchanged',
};
const records = [{ ...status, id: 'general', name: 'General', source: 'builtin', builtin: true }];

afterEach(() => vi.restoreAllMocks());

describe('subconverter template query recovery', () => {
  it.each([
    ['active refresh', 'network'],
    ['active refresh', 'business'],
    ['catalog save', 'network'],
    ['catalog save', 'business'],
    ['named refresh', 'network'],
    ['named refresh', 'business'],
  ] as const)('recovers cancelled initial queries after %s %s failure', async (action, failure) => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const staleStatus = deferred<Msg<unknown>>();
    const staleList = deferred<Msg<unknown>>();
    const postResponse = deferred<Msg<unknown>>();
    let statusRequests = 0;
    let listRequests = 0;
    vi.spyOn(HttpUtil, 'get').mockImplementation(async (url) => {
      if (url.endsWith('/template')) {
        statusRequests += 1;
        return statusRequests === 1 ? staleStatus.promise : new Msg(true, '', status);
      }
      if (url.endsWith('/templates')) {
        listRequests += 1;
        return listRequests === 1 ? staleList.promise : new Msg(true, '', records);
      }
      throw new Error(`Unexpected GET ${url}`);
    });
    const post = vi.spyOn(HttpUtil, 'post').mockImplementation(() => postResponse.promise);
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    );
    const hook = renderHook(
      () => ({ active: useSubconverterTemplate(true), catalog: useSubconverterTemplates(true) }),
      { wrapper },
    );
    try {
      await waitFor(() => {
        expect(statusRequests).toBe(1);
        expect(listRequests).toBe(1);
      });
      let mutation!: Promise<unknown>;
      act(() => {
        const request =
          action === 'active refresh'
            ? hook.result.current.active.refresh()
            : action === 'catalog save'
              ? hook.result.current.catalog.save({
                  input: {
                    name: 'Custom',
                    url: 'https://templates.example/custom.yaml',
                    source: 'remote',
                  },
                })
              : hook.result.current.catalog.refresh('general');
        // Attach the rejection handler before settling the network request.
        mutation = request.catch((error: unknown) => error);
      });
      await waitFor(() => expect(post).toHaveBeenCalledOnce());
      expect(client.getQueryState(keys.subconverter.template())?.fetchStatus).toBe('idle');
      expect(hook.result.current.active.statusQuery.isPending).toBe(true);
      await act(async () => {
        if (failure === 'network') postResponse.reject(new Error('offline'));
        else postResponse.resolve(new Msg(false, 'download failed'));
        await mutation;
      });
      await waitFor(() => {
        expect(statusRequests).toBe(2);
        expect(hook.result.current.active.statusQuery.isSuccess).toBe(true);
        expect(hook.result.current.active.statusQuery.data).toEqual(status);
        expect(hook.result.current.active.refreshPending).toBe(false);
        expect(hook.result.current.catalog.pending).toBe(false);
        if (action !== 'active refresh') {
          expect(listRequests).toBe(2);
          expect(hook.result.current.catalog.listQuery.data).toEqual(records);
        }
      });
      // An ignored, cancelled request must not overwrite recovered state later.
      await act(async () => {
        staleStatus.resolve(new Msg(true, '', { ...status, checkedAt: 1 }));
        staleList.resolve(new Msg(true, '', []));
      });
      expect(hook.result.current.active.statusQuery.data?.checkedAt).toBe(2000);
      if (action !== 'active refresh')
        expect(hook.result.current.catalog.listQuery.data).toEqual(records);
    } finally {
      hook.unmount();
      client.clear();
    }
  });

  it('leaves loading and exposes an error when recovery GET also fails', async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const initial = deferred<Msg<unknown>>();
    vi.spyOn(HttpUtil, 'get')
      .mockImplementationOnce(() => initial.promise)
      .mockRejectedValue(new Error('status unavailable'));
    vi.spyOn(HttpUtil, 'post').mockResolvedValue(new Msg(false, 'refresh failed'));
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    );
    const hook = renderHook(() => useSubconverterTemplate(true), { wrapper });
    try {
      await waitFor(() => expect(hook.result.current.statusQuery.isFetching).toBe(true));
      await act(async () => hook.result.current.refresh());
      await waitFor(() => expect(hook.result.current.statusQuery.isError).toBe(true));
      expect(hook.result.current.statusQuery.isPending).toBe(false);
      expect(hook.result.current.statusQuery.isFetching).toBe(false);
    } finally {
      hook.unmount();
      client.clear();
    }
  });
});
