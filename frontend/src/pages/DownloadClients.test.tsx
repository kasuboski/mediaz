import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import DownloadClients from './DownloadClients';

function page(totalItems: number, requestPage: number, pageSize: number) {
  const totalPages = Math.ceil(totalItems / pageSize);
  const items = requestPage <= totalPages ? Array.from({ length: Math.min(pageSize, totalItems - (requestPage - 1) * pageSize) }, (_, i) => ({
    ID: (requestPage - 1) * pageSize + i + 1,
    Implementation: 'transmission', Scheme: 'http', Host: `host-${(requestPage - 1) * pageSize + i + 1}`, Port: 9091, Type: 'torrent',
  })) : [];
  return { response: { items, pagination: { page: requestPage, pageSize, totalItems, totalPages } } };
}

function setup(handler: (url: URL) => Response | Promise<Response>) {
  const fetch = vi.fn(async (input: RequestInfo | URL) => handler(new URL(String(input), 'http://localhost')));
  vi.stubGlobal('fetch', fetch);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}><MemoryRouter><DownloadClients /></MemoryRouter></QueryClientProvider>);
  return { fetch };
}

afterEach(() => { cleanup(); vi.unstubAllGlobals(); localStorage.clear(); });

describe('DownloadClients pagination', () => {
  it('requests the next page with positive pagination params and shows the accurate range', async () => {
    const { fetch } = setup(url => new Response(JSON.stringify(page(75, Number(url.searchParams.get('page')), Number(url.searchParams.get('pageSize')))), { status: 200, headers: { 'Content-Type': 'application/json' } }));
    expect(await screen.findByText('Showing 1–50 of 75')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /next/i }));
    expect(await screen.findByText('Showing 51–75 of 75')).toBeInTheDocument();
    await waitFor(() => expect(fetch.mock.calls.some(([input]) => {
      const url = new URL(String(input), 'http://localhost');
      return url.searchParams.get('page') === '2' && url.searchParams.get('pageSize') === '50';
    })).toBe(true));
  });

  it('recovers a persisted out-of-range page and retries request errors', async () => {
    localStorage.setItem('download-clients-page', '3');
    let attempts = 0;
    const { fetch } = setup(url => {
      attempts++;
      if (attempts === 1) return new Response('failure', { status: 500, statusText: 'Internal Server Error' });
      return new Response(JSON.stringify(page(75, Number(url.searchParams.get('page')), Number(url.searchParams.get('pageSize')))), { status: 200, headers: { 'Content-Type': 'application/json' } });
    });
    expect(await screen.findByText('Failed to load download clients')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Try Again' }));
    expect(await screen.findByText('Showing 51–75 of 75')).toBeInTheDocument();
    await waitFor(() => expect(fetch.mock.calls.some(([input]) => new URL(String(input), 'http://localhost').searchParams.get('page') === '2')).toBe(true));
  });
});
