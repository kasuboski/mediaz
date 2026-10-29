import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import Activity from './Activity';

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

function setup() {
  Element.prototype.scrollIntoView = vi.fn();
  const requests: URL[] = [];
  const fetch = vi.fn(async (input: RequestInfo | URL) => {
    const url = new URL(String(input), 'http://localhost');
    requests.push(url);
    const json = (value: unknown) => new Response(JSON.stringify(value), { status: 200, headers: { 'Content-Type': 'application/json' } });
    if (url.pathname === '/api/v1/activity/active') {
      const pageSize = Number(url.searchParams.get('pageSize') || 50);
      const pages = { movies: Number(url.searchParams.get('moviesPage') || 1), series: Number(url.searchParams.get('seriesPage') || 1), jobs: Number(url.searchParams.get('jobsPage') || 1) };
      const page = (kind: string, total: number, current: number) => ({ items: Array.from({ length: Math.max(0, Math.min(pageSize, total - (current - 1) * pageSize)) }, (_, i) => ({ id: `${kind}-${(current - 1) * pageSize + i + 1}`, title: `${kind} item ${(current - 1) * pageSize + i + 1}`, name: `${kind} item ${(current - 1) * pageSize + i + 1}` })), pagination: { page: current, pageSize, totalItems: total, totalPages: Math.ceil(total / pageSize) } });
      const movies = page('movie', 125, pages.movies);
      const series = page('series', 65, pages.series);
      const jobs = page('job', 12, pages.jobs);
      return json({ response: { movies: movies.items, series: series.items, jobs: jobs.items, counts: { movies: 125, series: 65, jobs: 12, total: 202 }, pagination: { movies: movies.pagination, series: series.pagination, jobs: jobs.pagination } } });
    }
    if (url.pathname === '/api/v1/activity/failures') return json({ response: { failures: [], pagination: { page: 1, pageSize: 50, totalItems: 0, totalPages: 0 } } });
    if (url.pathname === '/api/v1/activity/timeline') return json({ response: { timeline: [], transitions: [], count: 0 } });
    throw new Error(`Unexpected request: ${url}`);
  });
  vi.stubGlobal('fetch', fetch);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}><MemoryRouter><Activity /></MemoryRouter></QueryClientProvider>);
  return { requests };
}

describe('Activity pagination integration', () => {
  it('pages active groups independently, preserves global counts, and resets all pages on size change', async () => {
    const { requests } = setup();
    const movieHeading = await screen.findByRole('heading', { name: 'Downloading Movies' });
    const movieSection = movieHeading.closest('section');
    expect(movieSection).not.toBeNull();
    fireEvent.click(within(movieSection!).getByRole('button', { name: 'Go to next page' }));
    await waitFor(() => expect(requests.some(url => url.searchParams.get('moviesPage') === '2' && url.searchParams.get('seriesPage') === '1' && url.searchParams.get('jobsPage') === '1')).toBe(true));
    await waitFor(() => expect(screen.getByText('202 active')).toBeInTheDocument(), { onTimeout: error => new Error(`${error.message}\nFetch calls: ${requests.map(url => `${url.pathname}${url.search}`).join('\n')}`) });
    expect(screen.getByText('(125)')).toBeInTheDocument();
    expect(screen.getByText('(65)')).toBeInTheDocument();
    expect(screen.getByText('(12)')).toBeInTheDocument();
    expect(await screen.findByText('movie item 51')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('combobox'));
    fireEvent.click(await screen.findByRole('option', { name: '25' }));
    await waitFor(() => expect(requests.some(url => url.searchParams.get('pageSize') === '25' && url.searchParams.get('moviesPage') === '1' && url.searchParams.get('seriesPage') === '1' && url.searchParams.get('jobsPage') === '1')).toBe(true));
    await waitFor(() => expect(screen.getByText('202 active')).toBeInTheDocument());
  });
});
