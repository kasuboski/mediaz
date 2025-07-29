/**
 * React Query hooks for API data fetching
 */

import { useQuery } from '@tanstack/react-query';
import { moviesApi, type MediaItem } from './api';

/**
 * Query keys for consistent caching
 */
export const queryKeys = {
  movies: {
    all: ['movies'] as const,
    library: () => [...queryKeys.movies.all, 'library'] as const,
  },
} as const;

/**
 * Hook to fetch library movies
 */
export function useLibraryMovies() {
  return useQuery({
    queryKey: queryKeys.movies.library(),
    queryFn: moviesApi.getLibraryMovies,
    staleTime: 5 * 60 * 1000, // 5 minutes
    gcTime: 10 * 60 * 1000, // 10 minutes (formerly cacheTime)
  });
}