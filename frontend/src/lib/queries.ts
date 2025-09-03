/**
 * React Query hooks for API data fetching
 */

import { useQuery } from '@tanstack/react-query';
import { moviesApi, tvApi } from './api';

/**
 * Query keys for consistent caching
 */
export const queryKeys = {
  movies: {
    all: ['movies'] as const,
    library: () => [...queryKeys.movies.all, 'library'] as const,
    detail: (tmdbID: number) => [...queryKeys.movies.all, 'detail', tmdbID] as const,
  },
  tv: {
    all: ['tv'] as const,
    library: () => ['tv', 'library'] as const,
    detail: (tmdbID: number) => ['tv', 'detail', tmdbID] as const,
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

export function useLibraryShows() {
  return useQuery({
    queryKey: queryKeys.tv.library(),
    queryFn: tvApi.getLibraryShows,
    staleTime: 5 * 60 * 1000,
    gcTime: 10 * 60 * 1000,
  });
}

export function useTVDetail(tmdbID: number) {
  return useQuery({
    queryKey: queryKeys.tv.detail(tmdbID),
    queryFn: () => tvApi.getTVDetail(tmdbID),
    staleTime: 10 * 60 * 1000,
    gcTime: 30 * 60 * 1000,
    enabled: !!tmdbID,
  });
}

/**
 * Hook to fetch detailed information for a specific movie
 */
export function useMovieDetail(tmdbID: number) {
  return useQuery({
    queryKey: queryKeys.movies.detail(tmdbID),
    queryFn: () => moviesApi.getMovieDetail(tmdbID),
    staleTime: 10 * 60 * 1000, // 10 minutes (movie details change less frequently)
    gcTime: 30 * 60 * 1000, // 30 minutes
    enabled: !!tmdbID, // Only run query if tmdbID is provided
  });
}

