import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, useLocation } from 'react-router-dom';
import Movies from './Movies';
import Series from './Series';

const stateTotals = { missing: 60, discovered: 30, downloaded: 20, completed: 0, unreleased: 10, downloading: 5, continuing: 0 };
const pageResponse = (url: URL, kind: 'movie' | 'tv') => {
  const states = url.searchParams.getAll('state');
  const totalItems = states.length ? (states.includes('missing') ? stateTotals.missing : 50) : 125;
  const page = Number(url.searchParams.get('page') || 1);
  const pageSize = Number(url.searchParams.get('pageSize') || 50);
  const totalPages = Math.ceil(totalItems / pageSize);
  const start = (page - 1) * pageSize;
  const items = Array.from({ length: Math.max(0, Math.min(pageSize, totalItems - start)) }, (_, index) => ({
    path: `/library/item-${start + index + 1}`, tmdbID: start + index + 1,
    title: `${kind} item ${start + index + 1}`, poster_path: '', year: 2020, state: 'missing',
  }));
  return { response: { items, pagination: { page, pageSize, totalItems, totalPages }, byState: stateTotals } };
};

function LocationProbe() {
  const location = useLocation();
  return <output data-testid="location">{location.pathname}{location.search}</output>;
}

const pages = [
  { name: 'movies', Component: Movies, path: '/library/movies', heading: 'Movie Library', count: '125 movies in your library' },
  { name: 'series', Component: Series, path: '/library/tv', heading: 'TV Show Library', count: '125 TV shows in your library' },
] as const;

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

describe.each(pages)('$name library pagination integration', ({ Component, path, heading, count }) => {
  function setup(initialEntry: string = path) {
    const fetch = vi.fn(async (input: RequestInfo | URL) => {
      const url = new URL(String(input), 'http://localhost');
      const kind = url.pathname.endsWith('/library/movies') ? 'movie' : 'tv';
      return new Response(JSON.stringify(pageResponse(url, kind)), { status: 200, headers: { 'Content-Type': 'application/json' } });
    });
    vi.stubGlobal('fetch', fetch);
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[initialEntry]}><Component /><LocationProbe /></MemoryRouter></QueryClientProvider>);
    return { fetch };
  }

  it('uses aggregate counts and paginates the missing filter, resetting to page one', async () => {
    const { fetch } = setup(`${path}?page=2`);
    expect(await screen.findByText(count)).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /Missing/ })).toHaveTextContent('60');
    expect(screen.getByRole('tab', { name: /Available/ })).toHaveTextContent(/Available \((50|30)\)/);

    await screen.findByRole('button', { name: 'Go to next page' });
    fireEvent.mouseDown(screen.getByRole('tab', { name: /Missing/ }), { button: 0, ctrlKey: false });
    await waitFor(() => {
      expect(screen.getByTestId('location')).toHaveTextContent('state=missing');
      expect(screen.getByTestId('location')).not.toHaveTextContent('page=');
    });
    await waitFor(() => {
      const missing = fetch.mock.calls.map(([input]) => new URL(String(input), 'http://localhost')).find(url => url.searchParams.get('state') === 'missing');
      expect(missing?.searchParams.get('page')).toBe('1');
      expect(missing?.searchParams.get('pageSize')).toBe('50');
    });

    fireEvent.click(await screen.findByRole('button', { name: 'Go to next page' }));
    await waitFor(() => {
      expect(screen.getByTestId('location')).toHaveTextContent('state=missing');
      expect(screen.getByTestId('location')).toHaveTextContent('page=2');
    });
    await waitFor(() => {
      const missing = fetch.mock.calls.map(([input]) => new URL(String(input), 'http://localhost')).find(url => url.searchParams.get('state') === 'missing' && url.searchParams.get('page') === '2');
      expect(missing?.searchParams.get('pageSize')).toBe('50');
    });
  });
});
