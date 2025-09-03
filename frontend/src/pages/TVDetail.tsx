import { useParams } from "react-router-dom";
import { Calendar, Tv, Users, Film, Play } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { LoadingSpinner } from "@/components/ui/LoadingSpinner";
import { RequestModal } from "@/components/media/RequestModal";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useState } from "react";
import { useTVDetail, useSeasons, useEpisodes } from "@/lib/queries";
import { type SeasonResult, type EpisodeResult } from "@/lib/api";

// Component for rendering individual episodes

const EpisodeItem = ({ episode }: { episode: EpisodeResult }) => (
  <div className="bg-background/50 dark:bg-gray-800/50 rounded-lg p-4 border border-border/50 hover:border-border transition-colors">
    <div className="flex items-start justify-between mb-3">
      <div className="flex-1">
        <h4 className="font-bold text-lg text-foreground mb-1">
          {episode.episodeNumber} - {episode.title}
        </h4>
        <div className="flex items-center gap-2 mb-2">
          {episode.airDate && (
            <Badge variant="secondary" className="text-xs px-2 py-1 bg-muted/80 text-muted-foreground">
              {new Date(episode.airDate).toLocaleDateString('en-US', { 
                month: 'short', 
                day: 'numeric', 
                year: 'numeric' 
              })}
            </Badge>
          )}
          {episode.runtime && (
            <Badge variant="secondary" className="text-xs px-2 py-1 bg-muted/80 text-muted-foreground">
              {episode.runtime}m
            </Badge>
          )}
          {episode.downloaded && (
            <Badge variant="default" className="text-xs px-2 py-1">
              Downloaded
            </Badge>
          )}
        </div>
      </div>
    </div>
    <p className="text-sm text-muted-foreground leading-relaxed">
      {episode.overview || 'No episode overview available.'}
    </p>
  </div>
);

// Component for rendering season content with episodes
const SeasonContent = ({ tmdbID, season }: { tmdbID: number, season: SeasonResult }) => {
  const { data: episodes, isLoading: episodesLoading, error: episodesError } = useEpisodes(tmdbID, season.seasonNumber);

  if (episodesLoading) {
    return (
      <div className="pl-15 space-y-6">
        <p className="text-sm text-muted-foreground">
          {season.overview || "Season overview is not available."}
        </p>
        <div className="flex justify-center py-8">
          <LoadingSpinner size="md" />
        </div>
      </div>
    );
  }

  if (episodesError) {
    return (
      <div className="pl-15 space-y-6">
        <p className="text-sm text-muted-foreground">
          {season.overview || "Season overview is not available."}
        </p>
        <div className="text-center py-4">
          <p className="text-sm text-red-500">Failed to load episodes</p>
        </div>
      </div>
    );
  }

  const episodesToShow = episodes?.slice(0, 8) || [];
  const remainingCount = (episodes?.length || 0) - 8;

  return (
    <div className="pl-15 space-y-6">
      <p className="text-sm text-muted-foreground">
        {season.overview || "Season overview is not available."}
      </p>
      <div className="space-y-4">
        {episodesToShow.map((episode) => (
          <EpisodeItem key={episode.episodeNumber} episode={episode} />
        ))}
        {remainingCount > 0 && (
          <div className="text-center py-4">
            <p className="text-sm text-muted-foreground">
              ... and {remainingCount} more episodes
            </p>
          </div>
        )}
      </div>
    </div>
  );
};

export default function TVDetail() {
  const { id } = useParams<{ id: string }>();
  const [showRequestModal, setShowRequestModal] = useState(false);

  const tmdbID = parseInt(id || '0');
  const { data: show, isLoading, error } = useTVDetail(tmdbID);
  const { data: seasons, isLoading: seasonsLoading, error: seasonsError } = useSeasons(tmdbID);

  if (isLoading || seasonsLoading) {
    return (
      <div className="flex justify-center items-center min-h-screen">
        <LoadingSpinner size="lg" />
      </div>
    );
  }

  if (error || !show) {
    return (
      <div className="flex justify-center items-center min-h-screen">
        <div className="text-center">
          <p className="text-muted-foreground mb-2">TV show not found</p>
          {error && (
            <p className="text-sm text-red-500">
              {error instanceof Error ? error.message : 'An error occurred while loading the TV show'}
            </p>
          )}
        </div>
      </div>
    );
  }

  const backdropUrl = (show.backdropPath && show.backdropPath.trim() !== '')
    ? `https://image.tmdb.org/t/p/original${show.backdropPath}`
    : null;

  const posterUrl = (show.posterPath && show.posterPath.trim() !== '')
    ? `https://image.tmdb.org/t/p/w500${show.posterPath}`
    : "/placeholder.svg";

  return (
    <div className="min-h-screen">
      <div 
        className="relative h-96 bg-cover bg-center"
        style={{ 
          backgroundImage: backdropUrl ? `url(${backdropUrl})` : 'none',
          backgroundColor: backdropUrl ? 'transparent' : 'hsl(var(--muted))'
        }}
      >
        <div className="absolute inset-0 bg-gradient-hero" />
        <div className="absolute bottom-0 left-0 right-0 p-8">
          <div className="container mx-auto">
            <div className="flex items-end gap-6">
              <img
                src={posterUrl}
                alt={show.title}
                className="w-48 h-72 object-cover rounded-lg shadow-modal border border-border/20"
                onError={(e) => {
                  const target = e.target as HTMLImageElement;
                  if (target.src !== '/placeholder.svg') {
                    target.src = '/placeholder.svg';
                  }
                }}
              />
              <div className="flex-1 text-white">
                <h1 className="text-4xl font-bold mb-2">{show.title}</h1>
                <div className="flex items-center gap-4 text-sm opacity-90 mb-4">
                  {show.firstAirDate && (
                    <div className="flex items-center gap-1">
                      <Calendar className="h-4 w-4" />
                      <span>{new Date(show.firstAirDate).getFullYear()}</span>
                      {show.lastAirDate && (
                        <span>- {new Date(show.lastAirDate).getFullYear()}</span>
                      )}
                    </div>
                  )}
                  <div className="flex items-center gap-1">
                    <Tv className="h-4 w-4" />
                    <span>{show.seasonCount} Seasons</span>
                  </div>
                  <div className="flex items-center gap-1">
                    <Users className="h-4 w-4" />
                    <span>{show.episodeCount} Episodes</span>
                  </div>
                </div>
                <div className="flex items-center gap-2 mb-4">
                  {show.genres && show.genres.length > 0 && show.genres.map((genre) => (
                    <Badge key={genre} variant="secondary" className="bg-white/20 text-white border-white/30">
                      {genre}
                    </Badge>
                  ))}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className="container mx-auto px-8 py-8">
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
          <div className="lg:col-span-2">
            <h2 className="text-2xl font-bold mb-4">Overview</h2>
            <p className="text-muted-foreground leading-relaxed mb-6">
              {show.overview && show.overview.trim() 
                ? show.overview 
                : "No overview available for this series."}
            </p>

            {show.networks && show.networks.length > 0 && (
              <div className="mb-6">
                <h3 className="font-semibold mb-2">Networks</h3>
                <div className="flex gap-2">
                  {show.networks.map((network) => (
                    <Badge key={network} variant="outline">{network}</Badge>
                  ))}
                </div>
              </div>
            )}


            {/* Seasons Section */}
            <Card>
              <CardHeader>
                <CardTitle className="text-lg">Seasons</CardTitle>
              </CardHeader>
              <CardContent>
                {seasonsError ? (
                  <div className="text-center py-8">
                    <p className="text-sm text-red-500">Failed to load seasons data</p>
                  </div>
                ) : !seasons || seasons.length === 0 ? (
                  <div className="text-center py-8">
                    <p className="text-sm text-muted-foreground">No seasons data available</p>
                  </div>
                ) : (
                  <Accordion type="single" collapsible className="w-full">
                    {seasons.map((season) => (
                      <AccordionItem key={season.seasonNumber} value={`season-${season.seasonNumber}`}>
                        <AccordionTrigger className="hover:no-underline">
                          <div className="flex items-center justify-between w-full mr-4">
                            <div className="flex items-center gap-3">
                              <div className="w-12 h-16 bg-muted rounded flex items-center justify-center">
                                {season.posterPath ? (
                                  <img
                                    src={`https://image.tmdb.org/t/p/w92${season.posterPath}`}
                                    alt={season.title}
                                    className="w-full h-full object-cover rounded"
                                    onError={(e) => {
                                      const target = e.target as HTMLImageElement;
                                      target.style.display = 'none';
                                      const parentDiv = target.parentElement;
                                      if (parentDiv) {
                                        parentDiv.innerHTML = '<svg class="h-4 w-4 text-muted-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.828 14.828a4 4 0 01-5.656 0M9 10h1m4 0h1m-6 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>';
                                      }
                                    }}
                                  />
                                ) : (
                                  <Play className="h-4 w-4 text-muted-foreground" />
                                )}
                              </div>
                              <div className="text-left">
                                <h4 className="font-medium">{season.title}</h4>
                                <p className="text-sm text-muted-foreground">
                                  {season.episodeCount} episodes
                                  {season.airDate && (
                                    <span> • {new Date(season.airDate).getFullYear()}</span>
                                  )}
                                </p>
                              </div>
                            </div>
                          </div>
                        </AccordionTrigger>
                        <AccordionContent>
                          <SeasonContent tmdbID={tmdbID} season={season} />
                        </AccordionContent>
                      </AccordionItem>
                    ))}
                  </Accordion>
                )}
              </CardContent>
            </Card>
          </div>

          <div className="lg:col-span-1">
            <div className="sticky top-8">
              {show.libraryStatus ? (
                <Button disabled className="w-full mb-4" size="lg">
                  In Library
                </Button>
              ) : (
                <Button 
                  onClick={() => setShowRequestModal(true)}
                  className="w-full mb-4 bg-gradient-primary hover:opacity-90" 
                  size="lg"
                >
                  Request Series
                </Button>
              )}

              <Card>
                <CardHeader>
                  <CardTitle className="text-lg">Details</CardTitle>
                </CardHeader>
                <CardContent className="pt-0">
                  <div className="space-y-3 text-sm">
                    {show.firstAirDate && (
                      <div className="flex justify-between items-center py-2 border-b border-border/50">
                        <span className="text-muted-foreground">First Air Date</span>
                        <span className="font-medium">{new Date(show.firstAirDate).toLocaleDateString()}</span>
                      </div>
                    )}
                    {show.lastAirDate && (
                      <div className="flex justify-between items-center py-2 border-b border-border/50">
                        <span className="text-muted-foreground">Last Air Date</span>
                        <span className="font-medium">{new Date(show.lastAirDate).toLocaleDateString()}</span>
                      </div>
                    )}
                    <div className="flex justify-between items-center py-2 border-b border-border/50">
                      <span className="text-muted-foreground">Seasons</span>
                      <span className="font-medium">{show.seasonCount}</span>
                    </div>
                    <div className="flex justify-between items-center py-2 border-b border-border/50">
                      <span className="text-muted-foreground">Episodes</span>
                      <span className="font-medium">{show.episodeCount}</span>
                    </div>
                    {show.voteAverage && (
                      <div className="flex justify-between items-center py-2 border-b border-border/50">
                        <span className="text-muted-foreground">Rating</span>
                        <span className="font-medium">{show.voteAverage.toFixed(1)}/10</span>
                      </div>
                    )}
                    {show.popularity && (
                      <div className="flex justify-between items-center py-2 border-b border-border/50">
                        <span className="text-muted-foreground">Popularity</span>
                        <span className="font-medium">{Math.round(show.popularity)}</span>
                      </div>
                    )}
                    <div className="flex justify-between items-center py-2">
                      <span className="text-muted-foreground">TMDB ID</span>
                      <span className="font-medium">{show.tmdbID}</span>
                    </div>
                  </div>
                </CardContent>
              </Card>

              {/* External Links */}
              <div className="flex gap-2 mt-4">
                <Button
                  variant="outline"
                  size="icon"
                  onClick={() => window.open(`https://www.themoviedb.org/tv/${show.tmdbID}`, '_blank')}
                  title="View on TMDB"
                >
                  <Film className="h-4 w-4" />
                </Button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <RequestModal
        isOpen={showRequestModal}
        onClose={() => setShowRequestModal(false)}
        mediaType="tv"
        mediaTitle={show.title}
        tmdbID={show.tmdbID}
      />
    </div>
  );
}