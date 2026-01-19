/**
 * API client for the Mediaz media management platform
 */

const API_BASE_URL =
  import.meta.env.VITE_API_HOST
    ? `${import.meta.env.VITE_API_HOST}/api/v1`
    : "/api/v1";

/**
 * Generic API response wrapper
 */
interface ApiResponse<T> {
  response: T;
  error?: string;
}

/**
 * LibraryMovie interface matching the API schema
 */
export interface LibraryMovie {
  path: string;
  tmdbID: number;
  title: string;
  poster_path: string;
  year?: number;
  state: string;
}

/**
 * LibraryShow interface matching the API schema
 */
export interface LibraryShow {
  path: string;
  tmdbID: number;
  title: string;
  poster_path: string;
  year?: number;
  state: string;
}

/**
 * MediaItem interface that matches what the MediaGrid component expects
 */
export interface MediaItem {
  id: number;
  title: string;
  poster_path: string;
  release_date?: string;
  first_air_date?: string;
  year?: number;
  media_type: "movie" | "tv";
  state?: string;
}

/**
 * MovieDetailResult interface matching the API schema
 */
export interface MovieDetailResult {
  id?: number;
  tmdbID: number;
  imdbID?: string;
  title: string;
  originalTitle?: string;
  overview?: string;
  posterPath: string;
  backdropPath?: string;
  releaseDate?: string;
  year?: number;
  runtime?: number;
  adult?: boolean;
  voteAverage?: number;
  voteCount?: number;
  popularity?: number;
  genres?: string;
  studio?: string;
  website?: string;
  collectionTmdbID?: number;
  collectionTitle?: string;
  libraryStatus: string;
  path?: string;
  qualityProfileID?: number;
  monitored?: boolean;
}

/**
 * Transformed movie detail data for the MovieDetail component
 */
export interface MovieDetail {
  id?: number;
  tmdbID: number;
  imdbID?: string;
  title: string;
  originalTitle?: string;
  overview?: string;
  posterPath: string;
  backdropPath?: string;
  releaseDate?: string;
  year?: number;
  runtime?: number;
  adult?: boolean;
  voteAverage?: number;
  voteCount?: number;
  popularity?: number;
  genres: string[];
  studio?: string;
  website?: string;
  collectionTmdbID?: number;
  collectionTitle?: string;
  libraryStatus: boolean;
  path?: string;
  qualityProfileID?: number;
  monitored: boolean;
}

export interface NetworkInfo {
  name: string;
  logoPath?: string;
}

export interface WatchProvider {
  providerId: number;
  name: string;
  logoPath?: string;
}

export interface ExternalIDs {
  imdbId?: string;
  tvdbId?: number;
}

export interface TVDetailResult {
  id?: number;
  tmdbID: number;
  title: string;
  originalTitle?: string;
  overview?: string;
  posterPath: string;
  backdropPath?: string;
  firstAirDate?: string;
  lastAirDate?: string;
  status?: string;
  nextAirDate?: string;
  originalLanguage?: string;
  productionCountries?: string[];
  networks?: NetworkInfo[];
  seasonCount: number;
  episodeCount: number;
  adult?: boolean;
  voteAverage?: number;
  voteCount?: number;
  popularity?: number;
  genres?: string[];
  libraryStatus: string;
  path?: string;
  qualityProfileID?: number;
  monitored?: boolean;
  monitorNewSeasons?: boolean;
  seasons?: SeasonResult[];
  externalIds?: ExternalIDs;
  watchProviders?: WatchProvider[];
}

export interface TVDetail {
  id?: number;
  tmdbID: number;
  title: string;
  originalTitle?: string;
  overview?: string;
  posterPath: string;
  backdropPath?: string;
  firstAirDate?: string;
  lastAirDate?: string;
  status?: string;
  nextAirDate?: string;
  originalLanguage?: string;
  productionCountries: string[];
  networks: NetworkInfo[];
  seasonCount: number;
  episodeCount: number;
  adult?: boolean;
  voteAverage?: number;
  voteCount?: number;
  popularity?: number;
  genres: string[];
  libraryStatus: boolean;
  path?: string;
  qualityProfileID?: number;
  monitored: boolean;
  monitorNewSeasons: boolean;
  seasons: SeasonResult[];
  externalIds?: ExternalIDs;
  watchProviders: WatchProvider[];
}

/**
 * Quality profile request/response types
 */
export interface QualityDefinition {
  ID: number;
  Name: string;
  MediaType: string;
  PreferredSize: number;
  MinSize: number;
  MaxSize: number;
}

export interface ProfileQuality {
  id: number;
  name: string;
  type: string;
  preferredSize: number;
  minSize: number;
  maxSize: number;
}

export interface QualityProfile {
  id: number;
  name: string;
  cutoff_quality_id: number | null;
  upgradeAllowed: boolean;
  qualities: ProfileQuality[];
}

export interface CreateQualityProfileRequest {
  name: string;
  cutoffQualityId: number | null;
  upgradeAllowed: boolean;
  qualityIds: number[];
}

export interface UpdateQualityProfileRequest {
  name: string;
  cutoffQualityId: number | null;
  upgradeAllowed: boolean;
  qualityIds: number[];
}

export interface CreateQualityDefinitionRequest {
  name: string;
  type: 'movie' | 'episode';
  preferredSize: number;
  minSize: number;
  maxSize: number;
}

export interface UpdateQualityDefinitionRequest {
  name: string;
  type: 'movie' | 'episode';
  preferredSize: number;
  minSize: number;
  maxSize: number;
}

export interface PendingQualityDefinition {
  tempId: string;
  name: string;
  type: 'movie' | 'episode';
  preferredSize: number;
  minSize: number;
  maxSize: number;
}

/**
 * Request types for adding media to library
 */
export interface AddMovieRequest {
  tmdbID: number;
  qualityProfileID: number;
}

export interface AddSeriesRequest {
  tmdbID: number;
  qualityProfileID: number;
  monitoredEpisodes?: number[];
  monitorNewSeasons?: boolean;
}

export interface UpdateSeriesMonitoringRequest {
  monitoredEpisodes: number[];
  qualityProfileID?: number;
  monitorNewSeasons?: boolean;
}

/**
 * Response types for added media
 */
export interface AddMovieResponse {
  path: string;
  tmdbID: number;
  title: string;
  poster_path: string;
  year?: number;
  state: string;
  qualityProfileID?: number;
}

export interface AddSeriesResponse {
  path: string;
  tmdbID: number;
  title: string;
  poster_path: string;
  year?: number;
  state: string;
  qualityProfileID?: number;
}

/**
 * Generic API error class
 */
export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
    this.name = 'ApiError';
  }
}

/**
 * Generic fetch wrapper with error handling
 */
async function apiRequest<T>(
  endpoint: string,
  options?: RequestInit
): Promise<T> {
  const url = `${API_BASE_URL}${endpoint}`;
  try {
    const response = await fetch(url, {
      headers: { 'Content-Type': 'application/json' },
      ...options,
    });

    if (!response.ok) {
      const text = await response.text();
      let errorMessage = `HTTP ${response.status}: ${response.statusText}`;

      if (text && text.trim()) {
        try {
          const data: ApiResponse<T> = JSON.parse(text);
          if (data.error && typeof data.error === 'string' && data.error.trim()) {
            errorMessage = data.error;
          }
        } catch {
        }
      }

      throw new ApiError(response.status, errorMessage);
    }

    const text = await response.text();
    if (!text || text.trim() === '') {
      return undefined as T;
    }

    const data: ApiResponse<T> = JSON.parse(text);
    return data.response;
  } catch (error) {
    if (error instanceof ApiError) {
      throw error;
    }
    throw new ApiError(0, error instanceof Error ? error.message : 'Unknown error occurred');
  }
}

function transformLibraryMovieToMediaItem(movie: LibraryMovie): MediaItem {
  return {
    id: movie.tmdbID,
    title: movie.title,
    poster_path: movie.poster_path,
    release_date: movie.year ? `${movie.year}-01-01` : undefined,
    year: movie.year,
    media_type: "movie" as const,
    state: movie.state,
  };
}

function transformLibraryShowToMediaItem(show: LibraryShow): MediaItem {
  return {
    id: show.tmdbID,
    title: show.title,
    poster_path: show.poster_path,
    first_air_date: show.year ? `${show.year}-01-01` : undefined,
    year: show.year,
    media_type: "tv" as const,
    state: show.state,
  };
}

function transformMovieDetailResult(result: MovieDetailResult): MovieDetail {
  return {
    ...result,
    genres: result.genres ? result.genres.split(',').map(g => g.trim()) : [],
    libraryStatus: result.libraryStatus !== 'Not In Library',
    monitored: result.monitored ?? false,
  };
}

function transformTVDetailResult(result: TVDetailResult): TVDetail {
  return {
    ...result,
    genres: result.genres ?? [],
    productionCountries: result.productionCountries ?? [],
    networks: result.networks ?? [],
    libraryStatus: result.libraryStatus !== 'Not In Library',
    monitored: result.monitored ?? false,
    monitorNewSeasons: result.monitorNewSeasons ?? false,
    seasons: result.seasons ?? [],
    watchProviders: result.watchProviders ?? [],
  };
}

export const moviesApi = {
  async getLibraryMovies(): Promise<MediaItem[]> {
    const movies = await apiRequest<LibraryMovie[] | null>('/library/movies');
    return (movies ?? []).map(transformLibraryMovieToMediaItem);
  },
  async getMovieDetail(tmdbID: number): Promise<MovieDetail> {
    const result = await apiRequest<MovieDetailResult>(`/movie/${tmdbID}`);
    return transformMovieDetailResult(result);
  },
  async addMovie(request: AddMovieRequest): Promise<AddMovieResponse> {
    return apiRequest<AddMovieResponse>('/library/movies', {
      method: 'POST',
      body: JSON.stringify(request),
    });
  },
  async deleteMovie(movieID: number, deleteFiles: boolean = false): Promise<void> {
    const endpoint = `/library/movies/${movieID}${deleteFiles ? '?deleteFiles=true' : ''}`;
    return apiRequest<void>(endpoint, { method: 'DELETE' });
  },
  async updateMovieMonitored(movieID: number, monitored: boolean): Promise<MovieDetail> {
    const result = await apiRequest<MovieDetailResult>(`/library/movies/${movieID}/monitored`, {
      method: 'PATCH',
      body: JSON.stringify({ monitored }),
    });
    return transformMovieDetailResult(result);
  },
  async searchForMovie(movieID: number): Promise<void> {
    return apiRequest<void>(`/library/movies/${movieID}/search`, {
      method: 'POST',
    });
  },
  async updateMovieQualityProfile(movieID: number, qualityProfileID: number): Promise<MovieDetail> {
    const result = await apiRequest<MovieDetailResult>(`/library/movies/${movieID}/quality`, {
      method: 'PATCH',
      body: JSON.stringify({ qualityProfileId: qualityProfileID }),
    });
    return transformMovieDetailResult(result);
  },
};

/**
 * SeasonResult interface matching the backend SeasonResult struct
 */
export interface SeasonResult {
  id: number;
  tmdbID: number;
  seriesID: number;
  seasonNumber: number;
  title: string;
  overview?: string;
  airDate?: string;
  posterPath?: string;
  episodeCount: number;
  monitored: boolean;
  episodes?: EpisodeResult[];
}

/**
 * EpisodeResult interface matching the backend EpisodeResult struct
 */
export interface EpisodeResult {
  id: number;
  tmdbID: number;
  seriesID: number;
  seasonNumber: number;
  episodeNumber: number;
  title: string;
  overview?: string;
  airDate?: string;
  stillPath?: string;
  runtime?: number;
  voteAverage?: number;
  monitored: boolean;
  downloaded: boolean;
}

export const tvApi = {
  async getLibraryShows(): Promise<MediaItem[]> {
    const shows = await apiRequest<LibraryShow[] | null>('/library/tv');
    return (shows ?? []).map(transformLibraryShowToMediaItem);
  },
  async getTVDetail(tmdbID: number): Promise<TVDetail> {
    const result = await apiRequest<TVDetailResult>(`/tv/${tmdbID}`);
    return transformTVDetailResult(result);
  },
  async addSeries(request: AddSeriesRequest): Promise<AddSeriesResponse> {
    return apiRequest<AddSeriesResponse>('/library/tv', {
      method: 'POST',
      body: JSON.stringify(request),
    });
  },
  async deleteSeries(seriesID: number, deleteDirectory: boolean = false): Promise<void> {
    const endpoint = `/library/tv/${seriesID}${deleteDirectory ? '?deleteDirectory=true' : ''}`;
    return apiRequest<void>(endpoint, { method: 'DELETE' });
  },
  async updateSeriesMonitoring(seriesID: number, request: UpdateSeriesMonitoringRequest): Promise<TVDetail> {
    const result = await apiRequest<TVDetailResult>(`/library/tv/${seriesID}/monitoring`, {
      method: 'PATCH',
      body: JSON.stringify(request),
    });
    return transformTVDetailResult(result);
  },
  async searchForSeries(seriesID: number): Promise<void> {
    return apiRequest<void>(`/library/tv/${seriesID}/search`, {
      method: 'POST',
    });
  },
  async searchForSeason(seasonID: number): Promise<void> {
    return apiRequest<void>(`/season/${seasonID}/search`, {
      method: 'POST',
    });
  },
  async searchForEpisode(episodeID: number): Promise<void> {
    return apiRequest<void>(`/episode/${episodeID}/search`, {
      method: 'POST',
    });
  },
  async updateSeasonMonitored(seasonID: number, monitored: boolean): Promise<SeasonResult> {
    return apiRequest<SeasonResult>(`/season/${seasonID}/monitored`, {
      method: 'PATCH',
      body: JSON.stringify({ monitored }),
    });
  },
};

// Library Configuration & Stats Types
export interface LibraryConfig {
  movieDir: string;
  tvDir: string;
  downloadMountDir: string;
}

export interface ServerConfig {
  port: number;
}

export interface JobsConfig {
  movieReconcile: string;
  movieIndex: string;
  seriesReconcile: string;
  seriesIndex: string;
}

export interface ConfigSummary {
  library: LibraryConfig;
  server: ServerConfig;
  jobs: JobsConfig;
}

export interface MovieStats {
  total: number;
  byState: Record<string, number>;
}

export interface TVStats {
  total: number;
  byState: Record<string, number>;
}

export interface LibraryStats {
  movies: MovieStats;
  tv: TVStats;
}

// Library API
export const libraryApi = {
  async getConfig(): Promise<ConfigSummary> {
    return await apiRequest<ConfigSummary>('/config');
  },
  async getLibraryStats(): Promise<LibraryStats> {
    return await apiRequest<LibraryStats>('/library/stats');
  },
};

/**
 * SearchMediaResult interface matching the backend SearchMediaResult struct
 * TV shows use "name" and "first_air_date" while movies use "title" and "release_date"
 */
export interface SearchMediaResult {
  adult?: boolean;
  backdrop_path?: string;
  genre_ids?: number[];
  id?: number;
  original_language?: string;
  original_title?: string;
  original_name?: string; // TV shows use original_name
  overview?: string;
  popularity?: number;
  poster_path?: string;
  release_date?: string; // Movies use release_date
  first_air_date?: string; // TV shows use first_air_date
  title?: string; // Movies use title
  name?: string; // TV shows use name
  video?: boolean;
  vote_average?: number;
  vote_count?: number;
}

/**
 * SearchMediaResponse interface matching the backend SearchMediaResponse struct
 */
export interface SearchMediaResponse {
  page?: number;
  total_pages?: number;
  total_results?: number;
  results?: SearchMediaResult[];
}

function transformSearchResultToMediaItem(
  result: SearchMediaResult,
  mediaType: "movie" | "tv"
): MediaItem | null {
  // Early return if no valid ID
  if (!result.id) {
    return null;
  }

  // TV shows use "name" and "first_air_date", movies use "title" and "release_date"
  // Fallback: if name/title is missing, try to use overview or generic label
  const title = mediaType === "tv"
    ? (result.name || result.title || result.overview?.substring(0, 50) || "TV Show")
    : (result.title || result.overview?.substring(0, 50) || "Movie");
  const date = mediaType === "tv" ? result.first_air_date : result.release_date;

  return {
    id: result.id,
    title: title,
    poster_path: result.poster_path || "",
    release_date: mediaType === "movie" ? date : undefined,
    first_air_date: mediaType === "tv" ? date : undefined,
    media_type: mediaType,
  };
}

// Search API
export const searchApi = {
  async searchMovies(query: string): Promise<MediaItem[]> {
    if (!query.trim()) {
      return [];
    }
    const response = await apiRequest<SearchMediaResponse>(
      `/discover/movie?query=${encodeURIComponent(query.trim())}`
    );
    if (!response.results) {
      return [];
    }
    return response.results
      .map((result) => transformSearchResultToMediaItem(result, "movie"))
      .filter((item): item is MediaItem => item !== null);
  },
  async searchTV(query: string): Promise<MediaItem[]> {
    if (!query.trim()) {
      return [];
    }
    const response = await apiRequest<SearchMediaResponse>(
      `/discover/tv?query=${encodeURIComponent(query.trim())}`
    );
    if (!response.results) {
      return [];
    }
    return response.results
      .map((result) => transformSearchResultToMediaItem(result, "tv"))
      .filter((item): item is MediaItem => item !== null);
  },
};

/**
 * Job types matching the backend JobType
 */
export type JobType = 'MovieIndex' | 'MovieReconcile' | 'MovieMetadata' | 'SeriesIndex' | 'SeriesReconcile' | 'SeriesMetadata';

/**
 * Job states matching the backend JobState
 */
export type JobState = '' | 'pending' | 'running' | 'error' | 'done' | 'cancelled';

/**
 * Job interface matching the backend JobResponse
 */
export interface Job {
  id: number;
  type: JobType;
  state: JobState;
  createdAt: string;
  updatedAt: string;
  error?: string;
}

/**
 * JobListResponse interface matching the backend
 */
export interface PaginationParams {
  page?: number;
  pageSize?: number;
}

export interface PaginationMeta {
  page: number;
  pageSize: number;
  totalItems: number;
  totalPages: number;
}

export interface JobListResponse {
  jobs: Job[];
  count: number;
  pagination?: PaginationMeta;
}

/**
 * Jobs API for managing background jobs
 */
export const jobsApi = {
  async listJobs(params?: PaginationParams): Promise<JobListResponse> {
    const queryParams = new URLSearchParams();

    if (params?.page !== undefined && params.page > 0) {
      queryParams.set('page', params.page.toString());
    }

    if (params?.pageSize !== undefined && params.pageSize >= 0) {
      queryParams.set('pageSize', params.pageSize.toString());
    }

    const endpoint = queryParams.toString()
      ? `/jobs?${queryParams.toString()}`
      : '/jobs';

    return apiRequest<JobListResponse>(endpoint);
  },

  async getJob(id: number): Promise<Job> {
    return apiRequest<Job>(`/jobs/${id}`);
  },

  async triggerJob(type: JobType): Promise<Job> {
    return apiRequest<Job>('/jobs', {
      method: 'POST',
      body: JSON.stringify({ type }),
    });
  },

  async cancelJob(id: number): Promise<Job> {
    return apiRequest<Job>(`/jobs/${id}/cancel`, {
      method: 'POST',
    });
  },
};

export interface DownloadClient {
  ID: number;
  Type: string;
  Implementation: string;
  Scheme: string;
  Host: string;
  Port: number;
  APIKey?: string | null;
}

export interface CreateDownloadClientRequest {
  type: string;
  implementation: string;
  scheme: string;
  host: string;
  port: number;
  apiKey?: string | null;
}

export interface UpdateDownloadClientRequest extends CreateDownloadClientRequest {
  id: number;
}

export const downloadClientsApi = {
  async listClients(): Promise<DownloadClient[]> {
    return apiRequest<DownloadClient[]>('/download/clients');
  },

  async getClient(id: number): Promise<DownloadClient> {
    return apiRequest<DownloadClient>(`/download/clients/${id}`);
  },

  async createClient(request: CreateDownloadClientRequest): Promise<DownloadClient> {
    return apiRequest<DownloadClient>('/download/clients', {
      method: 'POST',
      body: JSON.stringify(request),
    });
  },

  async updateClient(id: number, request: UpdateDownloadClientRequest): Promise<DownloadClient> {
    return apiRequest<DownloadClient>(`/download/clients/${id}`, {
      method: 'PUT',
      body: JSON.stringify(request),
    });
  },

  async deleteClient(id: number): Promise<void> {
    return apiRequest<void>(`/download/clients/${id}`, {
      method: 'DELETE',
    });
  },

  async testConnection(request: CreateDownloadClientRequest): Promise<void> {
    return apiRequest<void>('/download/clients/test', {
      method: 'POST',
      body: JSON.stringify(request),
    });
  },
};

export interface Indexer {
  id: number;
  name: string;
  source: string;
  priority: number;
  uri: string;
}

export interface IndexerRequest {
  name: string;
  priority: number;
  uri: string;
  api_key?: string;
}

export const indexersApi = {
  async list(): Promise<Indexer[]> {
    return apiRequest<Indexer[]>('/indexers');
  },

  async create(request: IndexerRequest): Promise<Indexer> {
    return apiRequest<Indexer>('/indexers', {
      method: 'POST',
      body: JSON.stringify(request),
    });
  },

  async update(id: number, request: IndexerRequest): Promise<Indexer> {
    return apiRequest<Indexer>(`/indexers/${id}`, {
      method: 'PUT',
      body: JSON.stringify(request),
    });
  },

  async delete(id: number): Promise<void> {
    return apiRequest<void>('/indexers', {
      method: 'DELETE',
      body: JSON.stringify({ id }),
    });
  },
};

export interface IndexerSource {
  id: number;
  name: string;
  implementation: string;
  scheme: string;
  host: string;
  port?: number;
  enabled: boolean;
}

export interface AddIndexerSourceRequest {
  name: string;
  implementation: string;
  scheme: string;
  host: string;
  port?: number;
  apiKey?: string;
  enabled: boolean;
}

export interface UpdateIndexerSourceRequest {
  name: string;
  implementation: string;
  scheme: string;
  host: string;
  port?: number;
  apiKey?: string;
  enabled: boolean;
}

export const indexerSourcesApi = {
  async list(): Promise<IndexerSource[]> {
    return apiRequest<IndexerSource[]>('/indexer-sources');
  },

  async create(request: AddIndexerSourceRequest): Promise<IndexerSource> {
    return apiRequest<IndexerSource>('/indexer-sources', {
      method: 'POST',
      body: JSON.stringify(request),
    });
  },

  async get(id: number): Promise<IndexerSource> {
    return apiRequest<IndexerSource>(`/indexer-sources/${id}`);
  },

  async update(id: number, request: UpdateIndexerSourceRequest): Promise<IndexerSource> {
    return apiRequest<IndexerSource>(`/indexer-sources/${id}`, {
      method: 'PUT',
      body: JSON.stringify(request),
    });
  },

  async delete(id: number): Promise<void> {
    return apiRequest<void>(`/indexer-sources/${id}`, {
      method: 'DELETE',
    });
  },

  async test(request: AddIndexerSourceRequest): Promise<void> {
    return apiRequest<void>('/indexer-sources/test', {
      method: 'POST',
      body: JSON.stringify(request),
    });
  },

  async refresh(id: number): Promise<void> {
    return apiRequest<void>(`/indexer-sources/${id}/refresh`, {
      method: 'POST',
    });
  },
};

/**
 * Quality Profiles API
 */
export const qualityProfilesApi = {
  async listProfiles(mediaType?: 'movie' | 'series'): Promise<QualityProfile[]> {
    const endpoint = mediaType
      ? `/quality/profiles?type=${mediaType}`
      : '/quality/profiles';
    return apiRequest<QualityProfile[]>(endpoint);
  },

  async getProfile(id: number): Promise<QualityProfile> {
    return apiRequest<QualityProfile>(`/quality/profiles/${id}`);
  },

  async createProfile(request: CreateQualityProfileRequest): Promise<QualityProfile> {
    return apiRequest<QualityProfile>('/quality/profiles', {
      method: 'POST',
      body: JSON.stringify(request),
    });
  },

  async updateProfile(id: number, request: UpdateQualityProfileRequest): Promise<QualityProfile> {
    return apiRequest<QualityProfile>(`/quality/profiles/${id}`, {
      method: 'PUT',
      body: JSON.stringify(request),
    });
  },

  async deleteProfile(id: number): Promise<void> {
    return apiRequest<void>(`/quality/profiles/${id}`, {
      method: 'DELETE',
    });
  },
};

/**
 * Quality Definitions API
 */
export const qualityDefinitionsApi = {
  async listDefinitions(): Promise<QualityDefinition[]> {
    return apiRequest<QualityDefinition[]>('/quality/definitions');
  },

  async getDefinition(id: number): Promise<QualityDefinition> {
    return apiRequest<QualityDefinition>(`/quality/definitions/${id}`);
  },

  async createDefinition(request: CreateQualityDefinitionRequest): Promise<QualityDefinition> {
    return apiRequest<QualityDefinition>('/quality/definitions', {
      method: 'POST',
      body: JSON.stringify(request),
    });
  },

  async updateDefinition(id: number, request: UpdateQualityDefinitionRequest): Promise<QualityDefinition> {
    return apiRequest<QualityDefinition>(`/quality/definitions/${id}`, {
      method: 'PUT',
      body: JSON.stringify(request),
    });
  },

  async deleteDefinition(id: number): Promise<void> {
    return apiRequest<void>('/quality/definitions', {
      method: 'DELETE',
      body: JSON.stringify({ id }),
    });
  },
};

export const metadataApi = {
  async refreshMoviesMetadata(tmdbIds?: number[]): Promise<string> {
    return apiRequest<string>('/movies/refresh', {
      method: 'POST',
      body: JSON.stringify({ tmdbIds: tmdbIds || [] }),
    });
  },
  async refreshSeriesMetadata(tmdbIds?: number[]): Promise<string> {
    return apiRequest<string>('/tv/refresh', {
      method: 'POST',
      body: JSON.stringify({ tmdbIds: tmdbIds || [] }),
    });
  },
};

/**
 * Activity API types
 */
export interface DownloadClientInfo {
  id: number;
  host: string;
  port: number;
}

export interface EpisodeInfo {
  seasonNumber: number;
  episodeNumber: number;
}

export interface ActiveMovie {
  id: number;
  tmdbID: number;
  title: string;
  year: number;
  poster_path: string;
  state: string;
  stateSince: string;
  duration: string;
  downloadClient: DownloadClientInfo;
  downloadID: string;
}

export interface ActiveSeries {
  id: number;
  tmdbID: number;
  title: string;
  year: number;
  poster_path: string;
  state: string;
  stateSince: string;
  duration: string;
  downloadClient: DownloadClientInfo;
  downloadID: string;
  currentEpisode: EpisodeInfo;
}

export interface ActiveJob {
  id: number;
  type: string;
  state: string;
  createdAt: string;
  updatedAt: string;
  duration: string;
}

export interface ActiveActivityResponse {
  movies: ActiveMovie[];
  series: ActiveSeries[];
  jobs: ActiveJob[];
}

export interface FailureItem {
  type: string;
  id: number;
  title: string;
  subtitle: string;
  state: string;
  failedAt: string;
  error: string;
  retryable: boolean;
}

export interface FailuresResponse {
  failures: FailureItem[];
}

export interface MovieCounts {
  downloaded?: number;
  downloading?: number;
}

export interface SeriesCounts {
  completed?: number;
  downloading?: number;
}

export interface JobCounts {
  done?: number;
  error?: number;
}

export interface TimelineEntry {
  date: string;
  movies: MovieCounts;
  series: SeriesCounts;
  jobs: JobCounts;
}

export interface TransitionItem {
  id: number;
  entityType: string;
  entityId: number;
  entityTitle: string;
  toState: string;
  fromState: string | null;
  createdAt: string;
}

export interface TimelineResponse {
  timeline: TimelineEntry[];
  transitions: TransitionItem[];
  count: number;
}

export interface EntityInfo {
  type: string;
  id: number;
  title: string;
  poster_path: string;
}

export interface TransitionMetadata {
  downloadClient?: DownloadClientInfo;
  downloadID?: string;
}

export interface HistoryEntry {
  sortKey: number;
  toState: string;
  fromState: string | null;
  createdAt: string;
  duration: string;
  metadata: TransitionMetadata | null;
}

export interface HistoryResponse {
  entity: EntityInfo;
  history: HistoryEntry[];
}

/**
 * Activity API for monitoring downloads, jobs, and system activity
 */
export const activityApi = {
  async getActiveActivity(): Promise<ActiveActivityResponse> {
    return apiRequest<ActiveActivityResponse>('/activity/active');
  },

  async getRecentFailures(hours: number = 24): Promise<FailureItem[]> {
    return apiRequest<FailureItem[]>(`/activity/failures?hours=${hours}`);
  },

  async getActivityTimeline(days: number = 1, page: number = 1, pageSize: number = 20): Promise<TimelineResponse> {
    const params = new URLSearchParams({ days: days.toString() });
    if (page > 1) params.append('page', page.toString());
    if (pageSize > 0) params.append('pageSize', pageSize.toString());
    return apiRequest<TimelineResponse>(`/activity/timeline?${params.toString()}`);
  },

  async getEntityHistory(entityType: string, entityId: number): Promise<HistoryResponse> {
    return apiRequest<HistoryResponse>(`/activity/history/${entityType}/${entityId}`);
  },
};
