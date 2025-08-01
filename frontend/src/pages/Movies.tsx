import { MediaGrid } from "@/components/media/MediaGrid";
import { LoadingSpinner } from "@/components/ui/LoadingSpinner";
import { useLibraryMovies } from "@/lib/queries";

export default function Movies() {
  const { data: movies = [], isLoading, error } = useLibraryMovies();

  if (error) {
    return (
      <div className="container mx-auto px-6 py-8">
        <div className="mb-8">
          <h1 className="text-3xl font-bold text-foreground mb-2">Movie Library</h1>
        </div>
        <div className="text-center py-16">
          <p className="text-destructive text-lg mb-2">Failed to load movies</p>
          <p className="text-muted-foreground">
            {error instanceof Error ? error.message : 'An unknown error occurred'}
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="container mx-auto px-6 py-8">
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-foreground mb-2">Movie Library</h1>
        <p className="text-muted-foreground">
          {isLoading ? "Loading..." : `${movies.length} movies in your library`}
        </p>
      </div>

      {isLoading ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner size="lg" />
        </div>
      ) : (
        <MediaGrid items={movies} />
      )}
    </div>
  );
}