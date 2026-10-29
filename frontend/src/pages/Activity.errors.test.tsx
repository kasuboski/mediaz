import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import Activity from './Activity';

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

function setup() {
  const failureRequests: URL[] = [];
  let failed = false;
  const fetch = vi.fn(async (input: RequestInfo | URL) => {
    const url = new URL(String(input), 'http://localhost');
    const json = (value: unknown) => new Response(JSON.stringify(value), { status: 200, headers: { 'Content-Type': 'application/json' } });
    if (url.pathname === '/api/v1/activity/active') return json({ response: { movies: [], series: [], jobs: [], counts: { movies: 0, series: 0, jobs: 0, total: 0 }, pagination: {} } });
    if (url.pathname === '/api/v1/activity/timeline') return json({ response: { timeline: [], transitions: [], count: 0 } });
    if (url.pathname === '/api/v1/activity/failures') {
      failureRequests.push(url);
      const page = Number(url.searchParams.get('page'));
      if (page === 2 && !failed) { failed = true; return new Response('server error', { status: 503 }); }
      return json({ response: { failures: [], pagination: { page, pageSize: 50, totalItems: 51, totalPages: 2 } } });
    }
    throw new Error(`Unexpected request: ${url}`);
  });
  vi.stubGlobal('fetch', fetch);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}><MemoryRouter><Activity /></MemoryRouter></QueryClientProvider>);
  return { failureRequests };
}

describe('Activity failure-page recovery', () => {
  it('shows page-two errors and retries the selected page', async () => {
    const { failureRequests } = setup();
    fireEvent.click(await screen.findByRole('button', { name: /Recent Failures/i }));
    fireEvent.click(await screen.findByRole('button', { name: 'Go to next page' }));
    await waitFor(() => expect(failureRequests.some(url => url.searchParams.get('page') === '2')).toBe(true));
    expect(await screen.findByText('Could not load recent failures.')).toBeInTheDocument();
    expect(screen.getByText('0 active')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    await waitFor(() => expect(failureRequests.filter(url => url.searchParams.get('page') === '2')).toHaveLength(2));
    await waitFor(() => expect(screen.getByText('51 failures')).toBeInTheDocument());
    expect(screen.getByRole('button', { name: 'Go to previous page' })).toBeInTheDocument();
  });
});
