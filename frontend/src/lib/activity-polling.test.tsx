import { renderHook, waitFor, cleanup } from '@testing-library/react';
import { focusManager, onlineManager, QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { useActiveActivity, queryKeys } from './queries';

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  focusManager.setFocused(undefined);
  onlineManager.setOnline(undefined);
});

function envelope(total: number) {
  return { response: { movies: [], series: [], jobs: [], counts: { movies: 0, series: 0, jobs: total, total }, pagination: { movies: { page: 1, pageSize: 50, totalItems: 0, totalPages: 0 }, series: { page: 1, pageSize: 50, totalItems: 0, totalPages: 0 }, jobs: { page: 1, pageSize: 50, totalItems: total, totalPages: Math.ceil(total / 50) } } } };
}
function setup(responses: number[]) {
  let call = 0;
  const fetch = vi.fn(async () => {
    const total = responses[Math.min(call++, responses.length - 1)];
    return new Response(JSON.stringify(envelope(total)), { status: 200, headers: { 'Content-Type': 'application/json' } });
  });
  vi.stubGlobal('fetch', fetch);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: 0, refetchOnWindowFocus: false } } });
  const wrapper = ({ children }: { children: React.ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  return { fetch, client, wrapper };
}

function enableQueryEnvironment() {
  focusManager.setFocused(true);
  onlineManager.setOnline(true);
}

describe('active activity polling', () => {
  it('polls with empty arrays while total is positive, then stops when total reaches zero', async () => {
    enableQueryEnvironment();
    const { fetch, client, wrapper } = setup([5, 0]);
    const { result, unmount } = renderHook(() => useActiveActivity(), { wrapper });
    try {
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(result.current.data).toMatchObject({ movies: [], series: [], jobs: [], counts: { total: 5 } });
      expect(fetch).toHaveBeenCalledTimes(1);

      await waitFor(() => expect(result.current.data?.counts.total).toBe(0), { timeout: 7500 });
      expect(fetch).toHaveBeenCalledTimes(2);

      await new Promise((resolve) => setTimeout(resolve, 5200));
      expect(fetch).toHaveBeenCalledTimes(2);
    } finally {
      unmount();
      client.clear();
    }
  }, 20000);

  it('uses separate query keys for every collection page and page size', () => {
    const variants = [
      queryKeys.activity.active({ moviesPage: 1, seriesPage: 1, jobsPage: 1, pageSize: 50 }),
      queryKeys.activity.active({ moviesPage: 2, seriesPage: 1, jobsPage: 1, pageSize: 50 }),
      queryKeys.activity.active({ moviesPage: 1, seriesPage: 2, jobsPage: 1, pageSize: 50 }),
      queryKeys.activity.active({ moviesPage: 1, seriesPage: 1, jobsPage: 2, pageSize: 50 }),
      queryKeys.activity.active({ moviesPage: 1, seriesPage: 1, jobsPage: 1, pageSize: 25 }),
    ];
    expect(new Set(variants.map((variant) => JSON.stringify(variant))).size).toBe(variants.length);
  });
});
