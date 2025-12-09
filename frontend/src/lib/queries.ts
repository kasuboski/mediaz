/**
 * React Query hooks for API data fetching
 */

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { moviesApi, tvApi, searchApi, jobsApi, libraryApi, downloadClientsApi, indexersApi, type JobType, type CreateDownloadClientRequest, type UpdateDownloadClientRequest, type IndexerRequest } from './api';

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
  search: {
    all: ['search'] as const,
    movies: (query: string) => [...queryKeys.search.all, 'movies', query] as const,
    tv: (query: string) => [...queryKeys.search.all, 'tv', query] as const,
  },
  jobs: {
    all: ['jobs'] as const,
    list: () => [...queryKeys.jobs.all, 'list'] as const,
    detail: (id: number) => [...queryKeys.jobs.all, 'detail', id] as const,
  },
  config: {
    all: ['config'] as const,
  },
  downloadClients: {
    all: ['downloadClients'] as const,
    list: () => [...queryKeys.downloadClients.all, 'list'] as const,
    detail: (id: number) => [...queryKeys.downloadClients.all, 'detail', id] as const,
  },
  indexers: {
    all: ['indexers'] as const,
    list: () => [...queryKeys.indexers.all, 'list'] as const,
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

/**
 * Hook to search for movies
 */
export function useSearchMovies(query: string) {
  const normalizedQuery = query.trim();
  return useQuery({
    queryKey: queryKeys.search.movies(normalizedQuery),
    queryFn: () => searchApi.searchMovies(normalizedQuery),
    staleTime: 5 * 60 * 1000, // 5 minutes
    gcTime: 10 * 60 * 1000, // 10 minutes
    enabled: !!normalizedQuery, // Only run query if query is provided
  });
}

/**
 * Hook to search for TV shows
 */
export function useSearchTV(query: string) {
  const normalizedQuery = query.trim();
  return useQuery({
    queryKey: queryKeys.search.tv(normalizedQuery),
    queryFn: () => searchApi.searchTV(normalizedQuery),
    staleTime: 5 * 60 * 1000, // 5 minutes
    gcTime: 10 * 60 * 1000, // 10 minutes
    enabled: !!normalizedQuery, // Only run query if query is provided
  });
}

/**
 * Hook to fetch jobs list with auto-refresh for active jobs
 */
export function useJobs() {
  return useQuery({
    queryKey: queryKeys.jobs.list(),
    queryFn: jobsApi.listJobs,
    refetchInterval: (data) => {
      // Poll every 3 seconds if there are active jobs
      if (!data?.jobs || !Array.isArray(data.jobs)) {
        return false;
      }
      const hasActiveJobs = data.jobs.some(j =>
        ['pending', 'running'].includes(j.state)
      );
      return hasActiveJobs ? 3000 : false;
    },
    staleTime: 1000, // Very short stale time since jobs change rapidly
    gcTime: 5 * 60 * 1000,
  });
}

/**
 * Mutation hook to trigger new jobs
 */
export function useTriggerJob() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (type: JobType) => jobsApi.triggerJob(type),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.jobs.list() });
    },
  });
}

/**
 * Mutation hook to cancel jobs
 */
export function useCancelJob() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: number) => jobsApi.cancelJob(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.jobs.list() });
    },
  });
}

/**
 * Hook to fetch configuration including job schedules
 */
export function useConfig() {
  return useQuery({
    queryKey: queryKeys.config.all,
    queryFn: libraryApi.getConfig,
    staleTime: 10 * 60 * 1000,
    gcTime: 30 * 60 * 1000,
  });
}

export function useDownloadClients() {
  return useQuery({
    queryKey: queryKeys.downloadClients.list(),
    queryFn: downloadClientsApi.listClients,
    staleTime: 5 * 60 * 1000,
    gcTime: 10 * 60 * 1000,
  });
}

export function useDownloadClient(id: number) {
  return useQuery({
    queryKey: queryKeys.downloadClients.detail(id),
    queryFn: () => downloadClientsApi.getClient(id),
    staleTime: 5 * 60 * 1000,
    gcTime: 10 * 60 * 1000,
    enabled: !!id,
  });
}

export function useCreateDownloadClient() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (request: CreateDownloadClientRequest) =>
      downloadClientsApi.createClient(request),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.downloadClients.list() });
    },
  });
}

export function useUpdateDownloadClient() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, request }: { id: number; request: UpdateDownloadClientRequest }) =>
      downloadClientsApi.updateClient(id, request),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.downloadClients.list() });
    },
  });
}

export function useDeleteDownloadClient() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: number) => downloadClientsApi.deleteClient(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.downloadClients.list() });
    },
  });
}

export function useTestDownloadClient() {
  return useMutation({
    mutationFn: (request: CreateDownloadClientRequest) => downloadClientsApi.testConnection(request),
  });
}

export function useIndexers() {
  return useQuery({
    queryKey: queryKeys.indexers.list(),
    queryFn: indexersApi.list,
    staleTime: 5 * 60 * 1000,
  });
}

export function useCreateIndexer() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (request: IndexerRequest) => indexersApi.create(request),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.indexers.list() });
    },
  });
}

export function useUpdateIndexer() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, request }: { id: number; request: IndexerRequest }) =>
      indexersApi.update(id, request),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.indexers.list() });
    },
  });
}

export function useDeleteIndexer() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: number) => indexersApi.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.indexers.list() });
    },
  });
}

