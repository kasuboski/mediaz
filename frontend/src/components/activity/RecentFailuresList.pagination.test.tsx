import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { RecentFailuresList } from './RecentFailuresList';
import { queryKeys } from '@/lib/queries';

afterEach(() => cleanup());

const failure = (id: number) => ({ id, type: 'movie' as const, title: `Failure ${id}`, subtitle: 'Movie', state: 'error', error: 'failed', failedAt: new Date().toISOString(), retryable: true });

describe('RecentFailuresList pagination', () => {
  it('renders an enveloped page against its server total', () => {
    const onPageChange = vi.fn();
    const client = new QueryClient();
    render(<QueryClientProvider client={client}><RecentFailuresList data={{ failures: Array.from({ length: 50 }, (_, i) => failure(i + 51)), pagination: { page: 2, pageSize: 50, totalItems: 125, totalPages: 3 } }} hours={24} page={2} pageSize={50} onPageChange={onPageChange} onPageSizeChange={vi.fn()} /></QueryClientProvider>);
    expect(screen.getByText('Showing 51–100 of 125')).toBeInTheDocument();
    expect(screen.getByText('Failure 51')).toBeInTheDocument();
    expect(screen.getByText('Failure 100')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Go to next page' }));
    expect(onPageChange).toHaveBeenCalledWith(3);
  });

  it('retry invalidates every failures page and hours window, not only the first 24-hour page', async () => {
    const client = new QueryClient();
    const invalidate = vi.spyOn(client, 'invalidateQueries').mockResolvedValue();
    render(<QueryClientProvider client={client}><RecentFailuresList data={{ failures: [failure(51)], pagination: { page: 2, pageSize: 50, totalItems: 125, totalPages: 3 } }} hours={168} page={2} pageSize={50} onPageChange={vi.fn()} onPageSizeChange={vi.fn()} /></QueryClientProvider>);
    fireEvent.click(screen.getAllByRole('button').find(button => button.querySelector('svg.lucide-refresh-cw'))!);
    await waitFor(() => expect(invalidate).toHaveBeenCalled());
    expect(invalidate.mock.calls).toContainEqual([{ queryKey: queryKeys.activity.all }]);
    expect(invalidate.mock.calls).not.toContainEqual([{ queryKey: queryKeys.activity.failures(24, { page: 1, pageSize: 50 }) }]);
  });
});
