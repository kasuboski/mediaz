import { MovieState, getMovieStateColor } from './movie-state';
import { SeriesState, getSeriesStateColor } from './series-state';

export type MediaType = 'movie' | 'tv';
export type MediaState = MovieState | SeriesState;

/**
 * Get the color for a media state indicator based on media type.
 * Dispatches to the appropriate state handler (movie or series).
 */
export function getMediaStateColor(state: string | undefined, mediaType: MediaType) {
  return mediaType === 'movie'
    ? getMovieStateColor(state)
    : getSeriesStateColor(state);
}

// Re-export for convenience
export type { MovieState, SeriesState };
