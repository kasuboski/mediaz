import { useParams } from "react-router-dom";
import { Calendar, Tv, Users, Film, Play, MoreVertical, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { LoadingSpinner } from "@/components/ui/LoadingSpinner";
import { RequestModal } from "@/components/media/RequestModal";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useState } from "react";
import { useTVDetail } from "@/lib/queries";
import { type SeasonResult, type EpisodeResult } from "@/lib/api";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { metadataApi } from "@/lib/api";
import { toast } from "sonner";

// Component for rendering individual episodes

const EpisodeItem = ({ episode }: { episode: EpisodeResult }) => (
  <div className="bg-background/50 dark:bg-gray-800/50 rounded-lg p-4 border border-border/50 hover:border-border transition-colors">
    <div className="flex items-start gap-4">
      <div className="aspect-video w-32 bg-muted rounded-lg overflow-hidden flex-shrink-0">
        {episode.stillPath ? (
          <img
            src={`https://image.tmdb.org/t/p/w300${episode.stillPath}`}
            alt=""
            className="w-full h-full object-cover"
            loading="lazy"
          />
        ) : (
          <div className="w-full h-full grid place-items-center">
            <Play className="h-5 w-5 text-muted-foreground" />
          </div>
        )}
      </div>

      <div className="flex-1 min-w-0">
        <div className="flex items-start justify-between mb-3">
          <div className="flex-1 min-w-0">
            <h4 className="font-bold text-lg text-foreground mb-1 line-clamp-1">
              {episode.episodeNumber} - {episode.title}
            </h4>
            <div className="flex items-center gap-2 mb-2 text-xs">
              {episode.airDate && (
                <Badge variant="secondary" className="px-2 py-1 bg-muted/80 text-muted-foreground">
                  {new Date(episode.airDate).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })}
                </Badge>
              )}
              {episode.runtime && (
                <Badge variant="secondary" className="px-2 py-1 bg-muted/80 text-muted-foreground">
                  {episode.runtime}m
                </Badge>
              )}
              {episode.downloaded && (
                <Badge variant="default" className="px-2 py-1">
                  Downloaded
                </Badge>
              )}
            </div>
          </div>

          {episode.voteAverage && episode.voteAverage > 0 && (
            <span
              className="text-xs rounded border px-2 py-0.5"
              aria-label={`TMDB episode rating ${episode.voteAverage.toFixed(1)} of 10`}
              title={`TMDB rating ${episode.voteAverage.toFixed(1)}/10`}
            >
              ⭐ {episode.voteAverage.toFixed(1)}
            </span>
          )}
        </div>

        <p className="text-sm text-muted-foreground leading-relaxed line-clamp-3">
          {episode.overview || 'No episode overview available.'}
        </p>
      </div>
    </div>
  </div>
);

// Component for rendering season content with episodes
const SeasonContent = ({ season }: { season: SeasonResult }) => {
  const episodes = season.episodes || [];
  const sortedEpisodes = [...episodes].sort((a, b) => a.episodeNumber - b.episodeNumber);

  return (
    <div className="pl-15 space-y-6">
      <p className="text-sm text-muted-foreground">
        {season.overview || "Season overview is not available."}
      </p>
      {sortedEpisodes.length === 0 ? (
        <div className="text-center py-4">
          <p className="text-sm text-muted-foreground">No episodes available</p>
        </div>
      ) : (
        <div className="space-y-4">
          {sortedEpisodes.map((episode) => (
            <EpisodeItem key={episode.episodeNumber} episode={episode} />
          ))}
        </div>
      )}
    </div>
  );
};

export default function TVDetail() {
  const { id } = useParams<{ id: string }>();
  const [showRequestModal, setShowRequestModal] = useState(false);
  const [isRefreshing, setIsRefreshing] = useState(false);

  const tmdbID = parseInt(id || '0');
  const { data: show, isLoading, error } = useTVDetail(tmdbID);

  const handleRefreshMetadata = async () => {
    setIsRefreshing(true);
    try {
      await metadataApi.refreshSeriesMetadata([tmdbID]);
      toast.success("Series metadata refresh started");
    } catch (error) {
      toast.error("Failed to refresh metadata");
      console.error(error);
    } finally {
      setIsRefreshing(false);
    }
  };

  if (isLoading) {
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
        role="banner"
        aria-label={`${show.title} hero backdrop`}
        className="relative h-96 bg-cover bg-center"
        style={{ 
          backgroundImage: backdropUrl ? `url(${backdropUrl})` : 'none',
          backgroundColor: backdropUrl ? 'transparent' : 'hsl(var(--muted))'
        }}
      >
        <div className="absolute inset-0 bg-gradient-hero" />
        <div className="absolute top-6 right-6 flex gap-2">
          {show.voteAverage != null ? (
            <span
              className="rating-badge bg-white/15 text-white border-white/20"
              aria-label={`TMDB rating ${show.voteAverage.toFixed(1)} of 10`}
              title={`TMDB rating ${show.voteAverage.toFixed(1)}/10`}
            >
              {Math.round(show.voteAverage * 10)}%
            </span>
          ) : (
            <span
              className="rating-badge bg-white/10 text-white/90 border-white/15"
              aria-label="Not rated"
              title="Not rated"
            >
              NR
            </span>
          )}
        </div>
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
                <div className="flex items-center gap-3 mb-2">
                  <h1 className="hero-title">{show.title}</h1>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="ghost" size="icon" className="h-8 w-8 text-white hover:bg-white/20">
                        <MoreVertical className="h-4 w-4" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="start" side="right">
                      <DropdownMenuItem onClick={handleRefreshMetadata} disabled={isRefreshing}>
                        <RefreshCw className={`mr-2 h-4 w-4 ${isRefreshing ? 'animate-spin' : ''}`} />
                        Refresh Metadata
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>
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
                <div className="flex gap-3 flex-wrap items-center">
                  {show.networks.map((network) => (
                    <div key={network.name} className="flex items-center gap-2">
                      {network.logoPath ? (
                        <img src={`https://image.tmdb.org/t/p/w45${network.logoPath}`} alt="" className="h-6 w-6 rounded" loading="lazy" />
                      ) : null}
                      <Badge variant="outline">{network.name}</Badge>
                    </div>
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
                {!show.seasons || show.seasons.length === 0 ? (
                  <div className="text-center py-8">
                    <p className="text-sm text-muted-foreground">No seasons data available</p>
                  </div>
                ) : (
                  <Accordion type="single" collapsible className="w-full">
                    {show.seasons.map((season) => (
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
                          <SeasonContent season={season} />
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
                    {show.status && (
                      <div className="flex justify-between items-center py-2 border-b border-border/50">
                        <span className="text-muted-foreground">Status</span>
                        <span className="font-medium">{show.status}</span>
                      </div>
                    )}
                    {show.firstAirDate && (
                      <div className="flex justify-between items-center py-2 border-b border-border/50">
                        <span className="text-muted-foreground">First Air Date</span>
                        <span className="font-medium">{new Date(show.firstAirDate).toLocaleDateString()}</span>
                      </div>
                    )}
                    {show.nextAirDate && (
                      <div className="flex justify-between items-center py-2 border-b border-border/50">
                        <span className="text-muted-foreground">Next Air Date</span>
                        <span className="font-medium">{new Date(show.nextAirDate).toLocaleDateString()}</span>
                      </div>
                    )}
                    {show.lastAirDate && (
                      <div className="flex justify-between items-center py-2 border-b border-border/50">
                        <span className="text-muted-foreground">Last Air Date</span>
                        <span className="font-medium">{new Date(show.lastAirDate).toLocaleDateString()}</span>
                      </div>
                    )}
                    {show.originalLanguage && (
                      <div className="flex justify-between items-center py-2 border-b border-border/50">
                        <span className="text-muted-foreground">Original Language</span>
                        <span className="font-medium">{show.originalLanguage.toUpperCase()}</span>
                      </div>
                    )}
                    {show.productionCountries && show.productionCountries.length > 0 && (
                      <div className="flex justify-between items-center py-2 border-b border-border/50">
                        <span className="text-muted-foreground">Production Countries</span>
                        <span className="font-medium">{show.productionCountries.join(', ')}</span>
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

              {show.watchProviders && show.watchProviders.length > 0 && (
                <Card className="mt-4">
                  <CardHeader>
                    <CardTitle className="text-lg">Currently Streaming On</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className="flex flex-wrap gap-3">
                      {show.watchProviders.map((p) => (
                        <div key={p.providerId} className="platform-card flex items-center gap-2">
                          {p.logoPath ? (
                            <img src={`https://image.tmdb.org/t/p/w45${p.logoPath}`} alt="" className="h-8 w-8 rounded" loading="lazy" />
                          ) : null}
                          <span className="text-sm">{p.name}</span>
                        </div>
                      ))}
                    </div>
                  </CardContent>
                </Card>
              )}

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
                {show.externalIds?.imdbId && (
                  <Button
                    variant="outline"
                    size="icon"
                    onClick={() => window.open(`https://www.imdb.com/title/${show.externalIds!.imdbId}`, '_blank')}
                    title="View on IMDb"
                  >
                    IMDB
                  </Button>
                )}
                {show.externalIds?.tvdbId && (
                  <Button
                    variant="outline"
                    size="icon"
                    onClick={() => window.open(`https://thetvdb.com/?id=${show.externalIds!.tvdbId}&tab=series`, '_blank')}
                    title="View on TVDB"
                  >
                    TVDB
                  </Button>
                )}
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