import { useParams, useNavigate } from "react-router-dom";
import { Calendar, Clock, Star, ExternalLink, RefreshCw, Trash2, Search, Settings } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { LoadingSpinner } from "@/components/ui/LoadingSpinner";
import { RequestModal } from "@/components/media/RequestModal";
import { MediaActionBar, MediaAction } from "@/components/media/MediaActionBar";
import { useMovieDetail, useSearchMovie } from "@/lib/queries";
import { useState } from "react";
import { Checkbox } from "@/components/ui/checkbox";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { metadataApi, moviesApi } from "@/lib/api";
import { toast } from "sonner";
import { useQueryClient } from "@tanstack/react-query";

export default function MovieDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [showRequestModal, setShowRequestModal] = useState(false);
  const [showEditMonitoringModal, setShowEditMonitoringModal] = useState(false);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [deleteFiles, setDeleteFiles] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);

  const tmdbID = parseInt(id || '0');
  const { data: movie, isLoading, error } = useMovieDetail(tmdbID);
  const searchMovie = useSearchMovie();

  const handleRefreshMetadata = async () => {
    setIsRefreshing(true);
    try {
      await metadataApi.refreshMoviesMetadata([tmdbID]);
      toast.success("Movie metadata refresh started");
    } catch (error) {
      toast.error("Failed to refresh metadata");
      console.error(error);
    } finally {
      setIsRefreshing(false);
    }
  };

  const handleDelete = async () => {
    if (!movie?.id) return;

    setIsDeleting(true);
    try {
      await moviesApi.deleteMovie(movie.id, deleteFiles);
      await queryClient.invalidateQueries({ queryKey: ['movies', 'library'] });
      toast.success(deleteFiles ? "Movie and files deleted" : "Movie deleted from library");
      navigate("/movies");
    } catch (error) {
      toast.error("Failed to delete movie");
      console.error(error);
      setIsDeleting(false);
    }
  };

  const handleSearch = () => {
    if (!movie?.id) return;
    searchMovie.mutate(movie.id, {
      onSuccess: () => {
        toast.success("Search started");
      },
      onError: (error) => {
        toast.error(error instanceof Error ? error.message : "Failed to start search");
      },
    });
  };

  if (isLoading) {
    return (
      <div className="flex justify-center items-center min-h-screen">
        <LoadingSpinner size="lg" />
      </div>
    );
  }

  if (error || !movie) {
    return (
      <div className="flex justify-center items-center min-h-screen">
        <div className="text-center">
          <p className="text-muted-foreground mb-2">Movie not found</p>
          {error && (
            <p className="text-sm text-red-500">
              {error instanceof Error ? error.message : 'An error occurred while loading the movie'}
            </p>
          )}
        </div>
      </div>
    );
  }

  const backdropUrl = movie.backdropPath
    ? `https://image.tmdb.org/t/p/original${movie.backdropPath}`
    : null;

  const posterUrl = movie.posterPath
    ? `https://image.tmdb.org/t/p/w500${movie.posterPath}`
    : "/placeholder.svg";

  const mediaActions: MediaAction[] = movie.libraryStatus
    ? [
        {
          icon: Settings,
          label: "Edit",
          onClick: () => setShowEditMonitoringModal(true),
        },
        ...(movie.monitored
          ? [
              {
                icon: Search,
                label: "Search",
                onClick: handleSearch,
                disabled: searchMovie.isPending,
                loading: searchMovie.isPending,
              },
            ]
          : []),
        {
          icon: RefreshCw,
          label: "Refresh",
          onClick: handleRefreshMetadata,
          disabled: isRefreshing,
          loading: isRefreshing,
        },
        {
          icon: Trash2,
          label: "Delete",
          onClick: () => setShowDeleteModal(true),
          disabled: isDeleting,
          variant: "destructive" as const,
        },
      ]
    : [];

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

        {mediaActions.length > 0 && (
          <div className="absolute top-4 right-8 z-[1]">
            <MediaActionBar actions={mediaActions} />
          </div>
        )}

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
                  {movie.genres.map((genre) => (
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
              {!movie.libraryStatus && (
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
                  {movie.releaseDate && (
                    <div className="flex justify-between items-center py-2 border-b border-border/50">
                      <span className="text-muted-foreground">Release Date</span>
                      <span>{new Date(movie.releaseDate).toLocaleDateString()}</span>
                    </div>
                  )}
                  {movie.runtime && (
                    <div className="flex justify-between items-center py-2 border-b border-border/50">
                      <span className="text-muted-foreground">Runtime</span>
                      <span>{movie.runtime} minutes</span>
                    </div>
                  )}
                  <div className="flex justify-between items-center py-2">
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

      <RequestModal
        isOpen={showEditMonitoringModal}
        onClose={() => setShowEditMonitoringModal(false)}
        mediaType="movie"
        mediaTitle={movie.title}
        tmdbID={movie.tmdbID}
        mode="edit"
        currentMovieMonitoring={{
          movieID: movie.id!,
          qualityProfileID: movie.qualityProfileID
        }}
      />

      <AlertDialog open={showDeleteModal} onOpenChange={setShowDeleteModal}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Movie</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete "{movie.title}" from your library?
              <div className="flex items-center space-x-2 mt-4">
                <Checkbox
                  id="deleteFiles"
                  checked={deleteFiles}
                  onCheckedChange={(checked) => setDeleteFiles(checked === true)}
                />
                <label
                  htmlFor="deleteFiles"
                  className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
                >
                  Also delete files from disk
                </label>
              </div>
              {deleteFiles && (
                <p className="text-sm text-destructive font-semibold mt-3">
                  Warning: This will permanently delete all files from disk. This action cannot be undone.
                </p>
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isDeleting}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDelete}
              disabled={isDeleting}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {isDeleting ? "Deleting..." : "Delete"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}