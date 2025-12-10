import { MovieState, getMovieStateBadge, getMovieStateColor } from './movie-state';
import { SeriesState, getSeriesStateBadge, getSeriesStateColor } from './series-state';

export type MediaType = 'movie' | 'tv';
export type MediaState = MovieState | SeriesState;

/**
 * Get the badge configuration for a media state based on media type.
 * Dispatches to the appropriate state handler (movie or series).
 */
export function getMediaStateBadge(state: string | undefined, mediaType: MediaType) {
  return mediaType === 'movie'
    ? getMovieStateBadge(state)
    : getSeriesStateBadge(state);
}

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
