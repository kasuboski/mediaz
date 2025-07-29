/**
 * API client for the Mediaz media management platform
 */

const API_BASE_URL = 'http://localhost:8080/api/v1';

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
 * MediaItem interface that matches what the MediaGrid component expects
 */
export interface MediaItem {
  id: number;
  title: string;
  poster_path: string;
  release_date?: string;
  media_type: "movie" | "tv";
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
    
    // Network or other errors
    throw new ApiError(0, error instanceof Error ? error.message : 'Unknown error occurred');
  }
}

/**
 * Transform LibraryMovie to MediaItem format expected by MediaGrid
 */
function transformLibraryMovieToMediaItem(movie: LibraryMovie): MediaItem {
  return {
    id: movie.tmdbID,
    title: movie.title,
    poster_path: movie.poster_path,
    release_date: movie.year ? `${movie.year}-01-01` : undefined,
    media_type: "movie" as const,
  };
}

/**
 * Movies API endpoints
 */
export const moviesApi = {
  /**
   * Get all movies in the library
   */
  async getLibraryMovies(): Promise<MediaItem[]> {
    const movies = await apiRequest<LibraryMovie[]>('/library/movies');
    return movies.map(transformLibraryMovieToMediaItem);
  },
};