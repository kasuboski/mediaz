import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import { Calendar, Tv, Users, Globe, ExternalLink } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { LoadingSpinner } from "@/components/ui/LoadingSpinner";
import { RequestModal } from "@/components/media/RequestModal";

// Mock TV show detail data
const mockTVDetails = {
  1399: {
    tmdbID: 1399,
    title: "Game of Thrones",
    overview: "Seven noble families fight for control of the mythical land of Westeros. Friction between the houses leads to full-scale war. All while a very ancient evil awakens in the farthest north. Amidst the war, a neglected military order of misfits, the Night's Watch, is all that stands between the realms of men and icy horrors beyond.",
    posterPath: "/1XS1oqL89opfnbLl8WnZY1O1uJx.jpg",
    backdropPath: "/mUkuc2wyV9dHLG0D0Loaw5pO2s8.jpg",
    firstAirDate: "2011-04-17",
    lastAirDate: "2019-05-19",
    networks: ["HBO"],
    seasonCount: 8,
    episodeCount: 73,
    genres: ["Sci-Fi & Fantasy", "Drama", "Action & Adventure"],
    libraryStatus: false,
    path: "",
    qualityProfileID: null,
    monitored: false,
  },
  1396: {
    tmdbID: 1396,
    title: "Breaking Bad",
    overview: "A high school chemistry teacher diagnosed with inoperable lung cancer turns to manufacturing and selling methamphetamine in order to secure his family's future.",
    posterPath: "/ztkUQFLlC19CCMYHW9o1zWhJRNq.jpg",
    backdropPath: "/eSzpy96DwBujGFj0xMbXBcGcfxX.jpg",
    firstAirDate: "2008-01-20",
    lastAirDate: "2013-09-29",
    networks: ["AMC"],
    seasonCount: 5,
    episodeCount: 62,
    genres: ["Drama", "Crime"],
    libraryStatus: false,
    path: "",
    qualityProfileID: null,
    monitored: false,
  },
  60735: {
    tmdbID: 60735,
    title: "The Flash",
    overview: "After a particle accelerator causes a freak storm, CSI Investigator Barry Allen is struck by lightning and falls into a coma. Months later he awakens with the power of super speed, granting him the ability to move through Central City like an unseen guardian angel.",
    posterPath: "/lJA2RCMfsWoskqlQhXPSLFQGXEJ.jpg",
    backdropPath: "/mmkPUKXbHkOaZgqyOGZ4S8Adela.jpg",
    firstAirDate: "2014-10-07",
    lastAirDate: "2023-05-24",
    networks: ["The CW"],
    seasonCount: 9,
    episodeCount: 184,
    genres: ["Drama", "Sci-Fi & Fantasy"],
    libraryStatus: false,
    path: "",
    qualityProfileID: null,
    monitored: false,
  },
};

export default function TVDetail() {
  const { id } = useParams<{ id: string }>();
  const [show, setShow] = useState<any>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [showRequestModal, setShowRequestModal] = useState(false);

  useEffect(() => {
    // Simulate API call
    setTimeout(() => {
      const showId = parseInt(id || '0');
      const showDetail = mockTVDetails[showId as keyof typeof mockTVDetails];
      setShow(showDetail || null);
      setIsLoading(false);
    }, 800);
  }, [id]);

  if (isLoading) {
    return (
      <div className="flex justify-center items-center min-h-screen">
        <LoadingSpinner size="lg" />
      </div>
    );
  }

  if (!show) {
    return (
      <div className="flex justify-center items-center min-h-screen">
        <p className="text-muted-foreground">TV show not found</p>
      </div>
    );
  }

  const backdropUrl = show.backdropPath
    ? `https://image.tmdb.org/t/p/original${show.backdropPath}`
    : null;

  const posterUrl = show.posterPath
    ? `https://image.tmdb.org/t/p/w500${show.posterPath}`
    : "/placeholder.svg";

  return (
    <div className="min-h-screen">
      {/* Hero Section with Backdrop */}
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
              />
              <div className="flex-1 text-white">
                <h1 className="text-4xl font-bold mb-2">{show.title}</h1>
                <div className="flex items-center gap-4 text-sm opacity-90 mb-4">
                  <div className="flex items-center gap-1">
                    <Calendar className="h-4 w-4" />
                    <span>{new Date(show.firstAirDate).getFullYear()}</span>
                    {show.lastAirDate && (
                      <span>- {new Date(show.lastAirDate).getFullYear()}</span>
                    )}
                  </div>
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
                  {show.genres?.map((genre: string) => (
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

      {/* Content Section */}
      <div className="container mx-auto px-8 py-8">
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
          <div className="lg:col-span-2">
            <h2 className="text-2xl font-bold mb-4">Overview</h2>
            <p className="text-muted-foreground leading-relaxed mb-6">
              {show.overview}
            </p>

            {show.networks && show.networks.length > 0 && (
              <div className="mb-4">
                <h3 className="font-semibold mb-2">Networks</h3>
                <div className="flex gap-2">
                  {show.networks.map((network: string) => (
                    <Badge key={network} variant="outline">{network}</Badge>
                  ))}
                </div>
              </div>
            )}
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

              <div className="bg-card border border-border rounded-lg p-4">
                <h3 className="font-semibold mb-3">Details</h3>
                <div className="space-y-2 text-sm">
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">First Air Date</span>
                    <span>{new Date(show.firstAirDate).toLocaleDateString()}</span>
                  </div>
                  {show.lastAirDate && (
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Last Air Date</span>
                      <span>{new Date(show.lastAirDate).toLocaleDateString()}</span>
                    </div>
                  )}
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Seasons</span>
                    <span>{show.seasonCount}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Episodes</span>
                    <span>{show.episodeCount}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">TMDB ID</span>
                    <span>{show.tmdbID}</span>
                  </div>
                </div>
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