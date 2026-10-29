import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, useLocation } from 'react-router-dom';
import Discover from './Discover';

function LocationProbe() {
  const location = useLocation();
  return <output data-testid="location">{location.search}</output>;
}

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

describe('Discover pagination', () => {
  function setup(entry = '/discover?query=alien&type=movies') {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = new URL(String(input), 'http://localhost');
      const page = Number(url.searchParams.get('page') || 1);
      const isMovie = url.pathname.includes('movie');
      const item = { id: page, title: `${isMovie ? 'Movie' : 'TV'} page ${page}`, poster_path: '', release_date: '', media_type: isMovie ? 'movie' : 'tv' };
      return new Response(JSON.stringify({ response: { results: [{ ...item, title: item.title }], page, total_results: isMovie ? 45 : 20, total_pages: isMovie ? 3 : 1 } }), { status: 200, headers: { 'Content-Type': 'application/json' } });
    });
    vi.stubGlobal('fetch', fetchMock);
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[entry]}><Discover /><LocationProbe /></MemoryRouter></QueryClientProvider>);
    return fetchMock;
  }

  it('shows loading and waits for movie metadata before requesting cold page two', async () => {
    let resolveMetadata!: (response: Response) => void;
    const metadataResponse = new Promise<Response>((resolve) => { resolveMetadata = resolve; });
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = new URL(String(input), 'http://localhost');
      if (url.pathname.includes('movie') && url.searchParams.get('page') === '1') return metadataResponse;
      const page = Number(url.searchParams.get('page') || 1);
      return Promise.resolve(new Response(JSON.stringify({ response: { results: [{ id: page, title: `Movie page ${page}`, poster_path: '', release_date: '', media_type: 'movie' }], page, total_results: 45, total_pages: 3 } }), { status: 200, headers: { 'Content-Type': 'application/json' } }));
    });
    vi.stubGlobal('fetch', fetchMock);
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={client}><MemoryRouter initialEntries={['/discover?query=alien&type=movies&page=2']}><Discover /></MemoryRouter></QueryClientProvider>);

    expect(await screen.findByText('Searching...')).toBeInTheDocument();
    expect(screen.queryByText('No results found')).not.toBeInTheDocument();
    expect(fetchMock.mock.calls.filter(([input]) => new URL(String(input), 'http://localhost').searchParams.get('page') === '2')).toHaveLength(0);

    resolveMetadata(new Response(JSON.stringify({ response: { results: [{ id: 1, title: 'Movie page 1', poster_path: '', release_date: '', media_type: 'movie' }], page: 1, total_results: 45, total_pages: 3 } }), { status: 200, headers: { 'Content-Type': 'application/json' } }));
    expect(await screen.findByText('Movie page 2')).toBeInTheDocument();
    await waitFor(() => expect(fetchMock.mock.calls.some(([input]) => new URL(String(input), 'http://localhost').searchParams.get('page') === '2')).toBe(true));
  });

  it('forwards page two and displays upstream totals', async () => {
    const fetchMock = setup();
    await screen.findByText('Movie page 1');
    fireEvent.click(screen.getByRole('button', { name: 'Go to next page' }));
    expect(await screen.findByText('Movie page 2')).toBeInTheDocument();
    expect(screen.getByTestId('location')).toHaveTextContent('page=2');
    await waitFor(() => expect(fetchMock.mock.calls.some(([input]) => new URL(String(input), 'http://localhost').searchParams.get('page') === '2')).toBe(true));
    expect(screen.getByText(/Showing 21–21 of 45 movies/)).toBeInTheDocument();
  });

  it('paginates All across both streams and stops requesting the shorter stream', async () => {
    const fetchMock = setup('/discover?query=alien');
    await screen.findByText('Movie page 1');
    await waitFor(() => expect(screen.getByRole('button', { name: 'Go to next page' })).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: 'Go to next page' }));
    expect(await screen.findByText('Movie page 2')).toBeInTheDocument();
    expect(screen.queryByText('TV page 2')).not.toBeInTheDocument();
    expect(screen.getByTestId('location')).toHaveTextContent('page=2');
    expect(screen.getByText(/Page 2: showing 1 results across movie and TV search pages/)).toBeInTheDocument();
    expect(fetchMock.mock.calls.filter(([input]) => new URL(String(input), 'http://localhost').searchParams.get('page') === '2')).toHaveLength(1);
  });

  it('resets the page when switching type and restores page/type from the URL', async () => {
    setup('/discover?query=alien&type=movies&page=2');
    await screen.findByText('Movie page 2');
    fireEvent.mouseDown(screen.getByRole('tab', { name: /TV/ }), { button: 0, ctrlKey: false });
    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('type=tv'));
    expect(screen.getByTestId('location')).not.toHaveTextContent('page=');
    expect(await screen.findByText('TV page 1')).toBeInTheDocument();
  });

  it('keeps successful movie results when TV search fails and retries only TV', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = new URL(String(input), 'http://localhost');
      if (url.pathname.includes('tv')) return new Response('failed', { status: 500 });
      return new Response(JSON.stringify({ response: { results: [{ id: 1, title: 'Movie hit', poster_path: '', release_date: '', media_type: 'movie' }], page: 1, total_results: 1, total_pages: 1 } }), { status: 200, headers: { 'Content-Type': 'application/json' } });
    });
    vi.stubGlobal('fetch', fetchMock);
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={client}><MemoryRouter initialEntries={['/discover?query=alien']}><Discover /></MemoryRouter></QueryClientProvider>);
    expect(await screen.findByText('Movie hit')).toBeInTheDocument();
    expect(screen.getByText(/TV results could not be loaded/)).toBeInTheDocument();
    expect(screen.getByText(/total is unknown/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Retry failed search' }));
    await waitFor(() => expect(fetchMock.mock.calls.filter(([input]) => new URL(String(input), 'http://localhost').pathname.includes('tv'))).toHaveLength(2));
  });

  it('keeps successful TV results when movie search fails', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = new URL(String(input), 'http://localhost');
      if (url.pathname.includes('movie')) return new Response('failed', { status: 500 });
      return new Response(JSON.stringify({ response: { results: [{ id: 1, name: 'TV hit', poster_path: '', first_air_date: '', media_type: 'tv' }], page: 1, total_results: 1, total_pages: 1 } }), { status: 200, headers: { 'Content-Type': 'application/json' } });
    });
    vi.stubGlobal('fetch', fetchMock);
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={client}><MemoryRouter initialEntries={['/discover?query=alien']}><Discover /></MemoryRouter></QueryClientProvider>);
    expect(await screen.findByText('TV hit')).toBeInTheDocument();
    expect(screen.getByText(/Movie results could not be loaded/)).toBeInTheDocument();
  });

  it('shows full error when both All streams fail and retries both', async () => {
    const fetchMock = vi.fn(async () => new Response('failed', { status: 500 }));
    vi.stubGlobal('fetch', fetchMock);
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={client}><MemoryRouter initialEntries={['/discover?query=alien']}><Discover /></MemoryRouter></QueryClientProvider>);
    expect(await screen.findByText('Search Error')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(4));
  });

  it('keeps movie pagination usable on direct All page two when TV metadata fails', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = new URL(String(input), 'http://localhost');
      const isMovie = url.pathname.includes('movie');
      if (!isMovie) return new Response('failed', { status: 500 });
      const page = Number(url.searchParams.get('page') || 1);
      return new Response(JSON.stringify({ response: { results: [{ id: page, title: `Movie ${page}`, poster_path: '', release_date: '', media_type: 'movie' }], page, total_results: 45, total_pages: 3 } }), { status: 200, headers: { 'Content-Type': 'application/json' } });
    });
    vi.stubGlobal('fetch', fetchMock);
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={client}><MemoryRouter initialEntries={['/discover?query=alien&page=2']}><Discover /></MemoryRouter></QueryClientProvider>);
    expect(await screen.findByText('Movie 2')).toBeInTheDocument();
    expect(screen.getByText(/TV results could not be loaded/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Go to next page' })).toBeInTheDocument();
  });

  it('renders empty results without inventing pagination', async () => {
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({ response: { results: [], page: 1, total_results: 0, total_pages: 0 } }), { status: 200, headers: { 'Content-Type': 'application/json' } }));
    vi.stubGlobal('fetch', fetchMock);
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={client}><MemoryRouter initialEntries={['/discover?query=nothing&type=movies']}><Discover /></MemoryRouter></QueryClientProvider>);
    expect((await screen.findAllByText('No results found')).length).toBeGreaterThan(0);
    expect(screen.queryByRole('button', { name: 'Go to next page' })).not.toBeInTheDocument();
  });
});
