import { MediaGrid } from "@/components/media/MediaGrid";
import { LoadingSpinner } from "@/components/ui/LoadingSpinner";
import { MediaStateTabs } from "@/components/media/MediaStateTabs";
import { useLibraryShows } from "@/lib/queries";
import { useMediaStateFilter } from "@/hooks/use-media-state-filter";

export default function Series() {
  const { data: shows = [], isLoading, error } = useLibraryShows();
  const { filter, setFilter, counts, filteredItems: filteredShows } =
    useMediaStateFilter(shows);

  if (error) {
    return (
      <div className="container mx-auto px-6 py-8">
        <div className="mb-8">
          <div className="flex items-center gap-3 mb-2">
            <h1 className="text-3xl font-bold text-foreground">TV Show Library</h1>
          </div>
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
      <div className="mb-6">
        <div className="flex items-center gap-3 mb-2">
          <h1 className="text-3xl font-bold text-foreground">TV Show Library</h1>
        </div>
        <p className="text-muted-foreground">
          {isLoading ? "Loading..." : `${counts.all} TV shows in your library`}
        </p>
      </div>

      {isLoading ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner size="lg" />
        </div>
      ) : (
        <MediaStateTabs
          filter={filter}
          onFilterChange={setFilter}
          counts={counts}
        >
          <MediaGrid items={filteredShows} />
        </MediaStateTabs>
      )}
    </div>
  );
}