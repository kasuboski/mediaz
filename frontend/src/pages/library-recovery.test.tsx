import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, useLocation } from 'react-router-dom';
import Movies from './Movies';

const byState = { missing: 60, discovered: 30, downloaded: 20, completed: 0, unreleased: 10, downloading: 5, continuing: 0 };

function LocationProbe() {
  const location = useLocation();
  return <output data-testid="location">{location.pathname}{location.search}</output>;
}

function response(totalItems: number, page: number, pageSize: number, counts: Record<string, number> = byState) {
  return {
    response: {
      items: [],
      pagination: { page, pageSize, totalItems, totalPages: Math.ceil(totalItems / pageSize) },
      byState: counts,
    },
  };
}

function setup(initialEntry: string, handler: (url: URL) => Response | Promise<Response>) {
  const fetch = vi.fn(async (input: RequestInfo | URL) => handler(new URL(String(input), 'http://localhost')));
  vi.stubGlobal('fetch', fetch);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[initialEntry]}><Movies /><LocationProbe /></MemoryRouter></QueryClientProvider>);
  return { fetch };
}

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

describe('movie library recovery and empty states', () => {
  it('recovers an out-of-range URL page to the last page and replaces the URL', async () => {
    const { fetch } = setup('/library/movies?page=99', url => new Response(
      JSON.stringify(response(125, Number(url.searchParams.get('page')), 50)),
      { status: 200, headers: { 'Content-Type': 'application/json' } },
    ));

    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('page=3'));
    expect(screen.getByTestId('location')).not.toHaveTextContent('page=99');
    await waitFor(() => expect(fetch.mock.calls.some(([input]) => new URL(String(input), 'http://localhost').searchParams.get('page') === '3')).toBe(true));
  });

  it('distinguishes an empty library from a filter with no matches', async () => {
    setup('/library/movies', url => new Response(JSON.stringify(response(0, 1, 50, {})), { status: 200, headers: { 'Content-Type': 'application/json' } }));
    expect(await screen.findByText('Your movie library is empty.')).toBeInTheDocument();

    cleanup();
    const { fetch } = setup('/library/movies?state=missing', url => new Response(JSON.stringify(response(0, 1, 50)), { status: 200, headers: { 'Content-Type': 'application/json' } }));
    expect(await screen.findByText('No movies match this filter.')).toBeInTheDocument();
    expect(fetch).toHaveBeenCalled();
  });

  it('shows a request error and retries successfully', async () => {
    let attempts = 0;
    const { fetch } = setup('/library/movies', () => {
      attempts++;
      if (attempts === 1) return new Response('failure', { status: 500, statusText: 'Internal Server Error' });
      return new Response(JSON.stringify(response(0, 1, 50, {})), { status: 200, headers: { 'Content-Type': 'application/json' } });
    });

    expect(await screen.findByText('Failed to load movies')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    expect(await screen.findByText('Your movie library is empty.')).toBeInTheDocument();
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
  });
});
