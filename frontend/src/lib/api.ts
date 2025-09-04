/**
 * API client for the Mediaz media management platform
 */

const API_BASE_URL = '/api/v1';

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
  media_type: "movie" | "tv";
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

export interface TVDetailResult {
  tmdbID: number;
  title: string;
  originalTitle?: string;
  overview?: string;
  posterPath: string;
  backdropPath?: string;
  firstAirDate?: string;
  lastAirDate?: string;
  networks?: string[];
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
  networks: string[];
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
async function apiRequest<T>(endpoint: string): Promise<T> {
  const url = `${API_BASE_URL}${endpoint}`;
  try {
    const response = await fetch(url);
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
    media_type: "movie" as const,
  };
}

function transformLibraryShowToMediaItem(show: LibraryShow): MediaItem {
  return {
    id: show.tmdbID,
    title: show.title,
    poster_path: show.poster_path,
    first_air_date: show.year ? `${show.year}-01-01` : undefined,
    media_type: "tv" as const,
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
    networks: result.networks ?? [],
    libraryStatus: result.libraryStatus === 'InLibrary',
    monitored: result.monitored ?? false,
    seasons: result.seasons ?? [],
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
};
