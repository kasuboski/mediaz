import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import { Calendar, Clock, Star, Globe, ExternalLink } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { LoadingSpinner } from "@/components/ui/LoadingSpinner";
import { RequestModal } from "@/components/media/RequestModal";

// Mock movie detail data
const mockMovieDetails = {
  550: {
    tmdbID: 550,
    title: "Fight Club",
    overview: "A ticking-time-bomb insomniac and a slippery soap salesman channel primal male aggression into a shocking new form of therapy. Their concept catches on, with underground \"fight clubs\" forming in every town, until an eccentric gets in the way and ignites an out-of-control spiral toward oblivion.",
    posterPath: "/pB8BM7pdSp6B6Ih7QZ4DrQ3PmJK.jpg",
    backdropPath: "/hZkgoQYus5vegHoetLkCJzb17zJ.jpg",
    releaseDate: "1999-10-15",
    year: 1999,
    runtime: 139,
    genres: ["Drama", "Thriller"],
    studio: "20th Century Fox",
    website: "",
    imdbID: "tt0137523",
    libraryStatus: false,
    path: "",
    qualityProfileID: null,
    monitored: false,
  },
  13: {
    tmdbID: 13,
    title: "Forrest Gump",
    overview: "A man with a low IQ has accomplished great things in his life and been present during significant historic events—in each case, far exceeding what anyone imagined he could do. But despite all he has achieved, his one true love eludes him.",
    posterPath: "/arw2vcBveWOVZr6pxd9XTd1TdQa.jpg",
    backdropPath: "/saHP97rTPS5eLmrLQEcANmKrsFl.jpg",
    releaseDate: "1994-06-23",
    year: 1994,
    runtime: 142,
    genres: ["Comedy", "Drama", "Romance"],
    studio: "Paramount Pictures",
    website: "",
    imdbID: "tt0109830",
    libraryStatus: false,
    path: "",
    qualityProfileID: null,
    monitored: false,
  },
  155: {
    tmdbID: 155,
    title: "The Dark Knight",
    overview: "Batman raises the stakes in his war on crime. With the help of Lt. Jim Gordon and District Attorney Harvey Dent, Batman sets out to dismantle the remaining criminal organizations that plague the streets. The partnership proves to be effective, but they soon find themselves prey to a reign of chaos unleashed by a rising criminal mastermind known to the terrified citizens of Gotham as the Joker.",
    posterPath: "/qJ2tW6WMUDux911r6m7haRef0WH.jpg",
    backdropPath: "/dqK9Hag1054tghRQSqLSfrkvQnA.jpg",
    releaseDate: "2008-07-16",
    year: 2008,
    runtime: 152,
    genres: ["Drama", "Action", "Crime", "Thriller"],
    studio: "Warner Bros. Pictures",
    website: "",
    imdbID: "tt0468569",
    libraryStatus: false,
    path: "",
    qualityProfileID: null,
    monitored: false,
  },
};

export default function MovieDetail() {
  const { id } = useParams<{ id: string }>();
  const [movie, setMovie] = useState<any>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [showRequestModal, setShowRequestModal] = useState(false);

  useEffect(() => {
    // Simulate API call
    setTimeout(() => {
      const movieId = parseInt(id || '0');
      const movieDetail = mockMovieDetails[movieId as keyof typeof mockMovieDetails];
      setMovie(movieDetail || null);
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

  if (!movie) {
    return (
      <div className="flex justify-center items-center min-h-screen">
        <p className="text-muted-foreground">Movie not found</p>
      </div>
    );
  }

  const backdropUrl = movie.backdropPath
    ? `https://image.tmdb.org/t/p/original${movie.backdropPath}`
    : null;

  const posterUrl = movie.posterPath
    ? `https://image.tmdb.org/t/p/w500${movie.posterPath}`
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
                alt={movie.title}
                className="w-48 h-72 object-cover rounded-lg shadow-modal border border-border/20"
              />
              <div className="flex-1 text-white">
                <h1 className="text-4xl font-bold mb-2">{movie.title}</h1>
                <div className="flex items-center gap-4 text-sm opacity-90 mb-4">
                  {movie.year && (
                    <div className="flex items-center gap-1">
                      <Calendar className="h-4 w-4" />
                      <span>{movie.year}</span>
                    </div>
                  )}
                  {movie.runtime && (
                    <div className="flex items-center gap-1">
                      <Clock className="h-4 w-4" />
                      <span>{movie.runtime} min</span>
                    </div>
                  )}
                </div>
                <div className="flex items-center gap-2 mb-4">
                  {movie.genres?.map((genre: string) => (
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
              {movie.overview}
            </p>

            {movie.studio && (
              <div className="mb-4">
                <h3 className="font-semibold mb-2">Studio</h3>
                <p className="text-muted-foreground">{movie.studio}</p>
              </div>
            )}

            {movie.imdbID && (
              <div className="flex items-center gap-2">
                <Button variant="outline" size="sm" asChild>
                  <a 
                    href={`https://www.imdb.com/title/${movie.imdbID}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex items-center gap-2"
                  >
                    <Star className="h-4 w-4" />
                    View on IMDb
                    <ExternalLink className="h-3 w-3" />
                  </a>
                </Button>
              </div>
            )}
          </div>

          <div className="lg:col-span-1">
            <div className="sticky top-8">
              {movie.libraryStatus ? (
                <Button disabled className="w-full mb-4" size="lg">
                  In Library
                </Button>
              ) : (
                <Button 
                  onClick={() => setShowRequestModal(true)}
                  className="w-full mb-4 bg-gradient-primary hover:opacity-90" 
                  size="lg"
                >
                  Request Movie
                </Button>
              )}

              <div className="bg-card border border-border rounded-lg p-4">
                <h3 className="font-semibold mb-3">Details</h3>
                <div className="space-y-2 text-sm">
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Release Date</span>
                    <span>{new Date(movie.releaseDate).toLocaleDateString()}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Runtime</span>
                    <span>{movie.runtime} minutes</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">TMDB ID</span>
                    <span>{movie.tmdbID}</span>
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
        mediaType="movie"
        mediaTitle={movie.title}
        tmdbID={movie.tmdbID}
      />
    </div>
  );
}