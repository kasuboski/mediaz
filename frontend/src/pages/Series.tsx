import { MediaGrid } from "@/components/media/MediaGrid";
import { LoadingSpinner } from "@/components/ui/LoadingSpinner";
import { useLibraryShows } from "@/lib/queries";

export default function Series() {
  const { data: shows = [], isLoading, error } = useLibraryShows();

  if (error) {
    return (
      <div className="container mx-auto px-6 py-8">
        <div className="mb-8">
          <h1 className="text-3xl font-bold text-foreground mb-2">TV Show Library</h1>
        </div>
        <div className="text-center py-16">
          <p className="text-destructive text-lg mb-2">Failed to load TV shows</p>
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
        <h1 className="text-3xl font-bold text-foreground mb-2">TV Show Library</h1>
        <p className="text-muted-foreground">
          {isLoading ? "Loading..." : `${shows.length} TV shows in your library`}
        </p>
      </div>

      {isLoading ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner size="lg" />
        </div>
      ) : (
        <MediaGrid items={shows} />
      )}
    </div>
  );
}