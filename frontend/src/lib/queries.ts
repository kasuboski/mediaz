/**
 * React Query hooks for API data fetching
 */

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  moviesApi,
  tvApi,
  searchApi,
  jobsApi,
  libraryApi,
  downloadClientsApi,
  indexersApi,
  indexerSourcesApi,
  qualityProfilesApi,
  qualityDefinitionsApi,
  type JobType,
  type CreateDownloadClientRequest,
  type UpdateDownloadClientRequest,
  type IndexerRequest,
  type AddIndexerSourceRequest,
  type UpdateIndexerSourceRequest,
  type AddMovieRequest,
  type AddSeriesRequest,
  type CreateQualityProfileRequest,
  type UpdateQualityProfileRequest,
  type CreateQualityDefinitionRequest,
  type UpdateQualityDefinitionRequest,
  type PaginationParams,
} from './api';

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
  indexerSources: {
    all: ['indexerSources'] as const,
    list: () => [...queryKeys.indexerSources.all, 'list'] as const,
    detail: (id: number) => [...queryKeys.indexerSources.all, 'detail', id] as const,
  },
  qualityProfiles: {
    all: ['qualityProfiles'] as const,
    lists: () => [...queryKeys.qualityProfiles.all, 'list'] as const,
    list: (type?: 'movie' | 'series') => [...queryKeys.qualityProfiles.lists(), type] as const,
    detail: (id: number) => [...queryKeys.qualityProfiles.all, 'detail', id] as const,
  },
  qualityDefinitions: {
    all: ['qualityDefinitions'] as const,
    list: () => [...queryKeys.qualityDefinitions.all, 'list'] as const,
    detail: (id: number) => [...queryKeys.qualityDefinitions.all, 'detail', id] as const,
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
export function useJobs(params?: PaginationParams) {
  return useQuery({
    queryKey: [...queryKeys.jobs.list(), params?.page ?? 1, params?.pageSize ?? 0],
    queryFn: () => jobsApi.listJobs(params),
    refetchInterval: (query) => {
      const data = query.state.data;
      if (!data?.jobs || !Array.isArray(data.jobs)) {
        return false;
      }
      const hasActiveJobs = data.jobs.some(j =>
        ['pending', 'running'].includes(j.state)
      );
      return hasActiveJobs ? 3000 : false;
    },
    staleTime: 1000,
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

export function useIndexerSources() {
  return useQuery({
    queryKey: queryKeys.indexerSources.list(),
    queryFn: indexerSourcesApi.list,
    staleTime: 5 * 60 * 1000,
  });
}

export function useIndexerSource(id: number) {
  return useQuery({
    queryKey: queryKeys.indexerSources.detail(id),
    queryFn: () => indexerSourcesApi.get(id),
    enabled: !!id,
  });
}

export function useCreateIndexerSource() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (request: AddIndexerSourceRequest) => indexerSourcesApi.create(request),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.indexerSources.list() });
      queryClient.invalidateQueries({ queryKey: queryKeys.indexers.list() });
    },
  });
}

export function useUpdateIndexerSource() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, request }: { id: number; request: UpdateIndexerSourceRequest }) =>
      indexerSourcesApi.update(id, request),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.indexerSources.list() });
      queryClient.invalidateQueries({ queryKey: queryKeys.indexerSources.detail(variables.id) });
      queryClient.invalidateQueries({ queryKey: queryKeys.indexers.list() });
    },
  });
}

export function useDeleteIndexerSource() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: number) => indexerSourcesApi.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.indexerSources.list() });
      queryClient.invalidateQueries({ queryKey: queryKeys.indexers.list() });
    },
  });
}

export function useTestIndexerSource() {
  return useMutation({
    mutationFn: (request: AddIndexerSourceRequest) => indexerSourcesApi.test(request),
  });
}

export function useRefreshIndexerSource() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: number) => indexerSourcesApi.refresh(id),
    onSuccess: (_, id) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.indexerSources.detail(id) });
      queryClient.invalidateQueries({ queryKey: queryKeys.indexers.list() });
    },
  });
}

export function useQualityProfiles(type?: 'movie' | 'series') {
  return useQuery({
    queryKey: queryKeys.qualityProfiles.list(type),
    queryFn: () => qualityProfilesApi.listProfiles(type),
    staleTime: 10 * 60 * 1000,
    gcTime: 30 * 60 * 1000,
  });
}

export function useMovieQualityProfiles() {
  return useQualityProfiles('movie');
}

export function useSeriesQualityProfiles() {
  return useQualityProfiles('series');
}

export function useQualityProfile(id: number) {
  return useQuery({
    queryKey: queryKeys.qualityProfiles.detail(id),
    queryFn: () => qualityProfilesApi.getProfile(id),
    staleTime: 10 * 60 * 1000,
    gcTime: 30 * 60 * 1000,
    enabled: !!id,
  });
}

export function useQualityDefinitions() {
  return useQuery({
    queryKey: queryKeys.qualityDefinitions.list(),
    queryFn: qualityDefinitionsApi.listDefinitions,
    staleTime: 10 * 60 * 1000,
    gcTime: 30 * 60 * 1000,
  });
}

export function useCreateQualityProfile() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (request: CreateQualityProfileRequest) =>
      qualityProfilesApi.createProfile(request),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.qualityProfiles.lists() });
    },
  });
}

export function useUpdateQualityProfile() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, request }: { id: number; request: UpdateQualityProfileRequest }) =>
      qualityProfilesApi.updateProfile(id, request),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.qualityProfiles.lists() });
      queryClient.invalidateQueries({ queryKey: queryKeys.qualityProfiles.detail(variables.id) });
    },
  });
}

export function useDeleteQualityProfile() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: number) => qualityProfilesApi.deleteProfile(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.qualityProfiles.lists() });
    },
  });
}

export function useCreateQualityDefinition() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (request: CreateQualityDefinitionRequest) =>
      qualityDefinitionsApi.createDefinition(request),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.qualityDefinitions.list() });
      queryClient.invalidateQueries({ queryKey: queryKeys.qualityProfiles.lists() });
    },
  });
}

export function useUpdateQualityDefinition() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, request }: { id: number; request: UpdateQualityDefinitionRequest }) =>
      qualityDefinitionsApi.updateDefinition(id, request),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.qualityDefinitions.list() });
      queryClient.invalidateQueries({ queryKey: queryKeys.qualityProfiles.lists() });
    },
  });
}

export function useDeleteQualityDefinition() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: number) => qualityDefinitionsApi.deleteDefinition(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.qualityDefinitions.list() });
      queryClient.invalidateQueries({ queryKey: queryKeys.qualityProfiles.lists() });
    },
  });
}

/**
 * Mutation hook to add a movie to the library
 */
export function useAddMovie() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (request: AddMovieRequest) => moviesApi.addMovie(request),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.movies.library() });
      queryClient.invalidateQueries({ queryKey: queryKeys.movies.detail(data.tmdbID) });
    },
  });
}

/**
 * Mutation hook to add a TV series to the library
 */
export function useAddSeries() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (request: AddSeriesRequest) => tvApi.addSeries(request),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.tv.library() });
      queryClient.invalidateQueries({ queryKey: queryKeys.tv.detail(data.tmdbID) });
    },
  });
}

