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
  seasons?: SeasonResult[];
  externalIds?: ExternalIDs;
  watchProviders?: WatchProvider[];
}

export interface TVDetail {
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
  seasons: SeasonResult[];
  externalIds?: ExternalIDs;
  watchProviders: WatchProvider[];
}

/**
 * Quality profile request/response types
 */
export interface QualityDefinition {
  name: string;
  type: string; // "movie" or "episode"
  preferredSize: number;
  minSize: number;
  maxSize: number;
}

export interface QualityProfile {
  id: number;
  name: string;
  cutoff_quality_id: number;
  upgradeAllowed: boolean;
  qualities: QualityDefinition[];
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
      throw new ApiError(response.status, `HTTP ${response.status}: ${response.statusText}`);
    }
    const data: ApiResponse<T> = await response.json();
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
    libraryStatus: result.libraryStatus === 'InLibrary',
    monitored: result.monitored ?? false,
  };
}

function transformTVDetailResult(result: TVDetailResult): TVDetail {
  return {
    ...result,
    genres: result.genres ?? [],
    productionCountries: result.productionCountries ?? [],
    networks: result.networks ?? [],
    libraryStatus: result.libraryStatus === 'InLibrary',
    monitored: result.monitored ?? false,
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
};

/**
 * SeasonResult interface matching the backend SeasonResult struct
 */
export interface SeasonResult {
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
export interface JobListResponse {
  jobs: Job[];
  count: number;
}

/**
 * Jobs API for managing background jobs
 */
export const jobsApi = {
  async listJobs(): Promise<JobListResponse> {
    return apiRequest<JobListResponse>('/jobs');
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
