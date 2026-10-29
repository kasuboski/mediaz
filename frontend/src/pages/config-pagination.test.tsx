import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import Indexers from './Indexers';
import QualityProfiles from './QualityProfiles';

function response(items: unknown[], page: number, pageSize: number, totalItems: number) {
  return new Response(JSON.stringify({ response: { items, pagination: { page, pageSize, totalItems, totalPages: Math.ceil(totalItems / pageSize) } } }), { status: 200, headers: { 'Content-Type': 'application/json' } });
}

function setup(handler: (url: URL) => Response | Promise<Response>) {
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => handler(new URL(String(input), 'http://localhost')));
  vi.stubGlobal('fetch', fetchMock);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return { fetchMock, client };
}

function renderPage(client: QueryClient, page: React.ReactNode) {
  render(<QueryClientProvider client={client}><MemoryRouter>{page}</MemoryRouter></QueryClientProvider>);
}

afterEach(() => { cleanup(); vi.unstubAllGlobals(); localStorage.clear(); });

describe('configuration pagination', () => {
  it('pages indexers by 50 and loads the 125-source catalog in two pages of 100', async () => {
    const { fetchMock, client } = setup(url => {
      const page = Number(url.searchParams.get('page'));
      const pageSize = Number(url.searchParams.get('pageSize'));
      if (url.pathname.endsWith('/indexer-sources')) {
        const items = Array.from({ length: Math.max(0, Math.min(pageSize, 125 - (page - 1) * pageSize)) }, (_, i) => ({ id: (page - 1) * pageSize + i + 1, name: `Source ${(page - 1) * pageSize + i + 1}`, implementation: 'prowlarr', scheme: 'http', host: 'localhost', enabled: true }));
        return response(items, page, pageSize, 125);
      }
      if (url.pathname.endsWith('/indexers')) {
        const items = Array.from({ length: Math.max(0, Math.min(pageSize, 125 - (page - 1) * pageSize)) }, (_, i) => ({ id: (page - 1) * pageSize + i + 1, name: `Indexer ${(page - 1) * pageSize + i + 1}`, source: 'Source', uri: 'http://indexer', priority: 1 }));
        return response(items, page, pageSize, 125);
      }
      throw new Error(`Unexpected request: ${url}`);
    });

    renderPage(client, <Indexers />);
    expect(await screen.findByText('Showing 1–50 of 125 indexers')).toBeInTheDocument();
    expect(screen.getByText('Indexer 1')).toBeInTheDocument();
    expect(screen.queryByText('Indexer 51')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /next/i }));
    expect(await screen.findByText('Showing 51–100 of 125 indexers')).toBeInTheDocument();
    expect(screen.getByText('Indexer 51')).toBeInTheDocument();
    expect(await screen.findByText('Source 125')).toBeInTheDocument();

    await waitFor(() => {
      const requests = fetchMock.mock.calls.map(([input]) => new URL(String(input), 'http://localhost'));
      expect(requests.some(url => url.pathname.endsWith('/indexers') && url.searchParams.get('page') === '2' && url.searchParams.get('pageSize') === '50')).toBe(true);
      const catalogPages = requests.filter(url => url.pathname.endsWith('/indexer-sources')).map(url => [url.searchParams.get('page'), url.searchParams.get('pageSize')]);
      expect(catalogPages).toEqual(expect.arrayContaining([['1', '100'], ['2', '100']]));
    });
  });

  it('filters quality profiles by tab and resets pagination when switching to series', async () => {
    const { fetchMock, client } = setup(url => {
      const page = Number(url.searchParams.get('page'));
      const pageSize = Number(url.searchParams.get('pageSize'));
      const type = url.searchParams.get('type');
      const totalItems = type === 'series' ? 63 : 125;
      if (url.pathname.endsWith('/quality/profiles')) {
        return response([{ id: page, name: `${type} profile page ${page}`, upgradeAllowed: false, qualities: [] }], page, pageSize, totalItems);
      }
      if (url.pathname.endsWith('/quality/definitions')) {
        return response([], page, pageSize, totalItems);
      }
      throw new Error(`Unexpected request: ${url}`);
    });

    renderPage(client, <QualityProfiles />);
    expect(await screen.findByText('Showing 1–50 of 125 profiles')).toBeInTheDocument();
    fireEvent.mouseDown(screen.getByRole('tab', { name: 'TV Series' }));
    expect(await screen.findByText('Showing 1–50 of 63 profiles')).toBeInTheDocument();
    expect(screen.getByText('series profile page 1')).toBeInTheDocument();
    fireEvent.click(screen.getAllByRole('button', { name: /next/i })[0]);
    expect(await screen.findByText('Showing 51–63 of 63 profiles')).toBeInTheDocument();
    expect(screen.getByText('series profile page 2')).toBeInTheDocument();

    await waitFor(() => {
      const requests = fetchMock.mock.calls.map(([input]) => new URL(String(input), 'http://localhost'));
      expect(requests.some(url => url.pathname.endsWith('/quality/profiles') && url.searchParams.get('type') === 'movie')).toBe(true);
      expect(requests.some(url => url.pathname.endsWith('/quality/profiles') && url.searchParams.get('type') === 'series' && url.searchParams.get('page') === '1')).toBe(true);
      expect(requests.some(url => url.pathname.endsWith('/quality/profiles') && url.searchParams.get('type') === 'series' && url.searchParams.get('page') === '2')).toBe(true);
    });
  });
});
