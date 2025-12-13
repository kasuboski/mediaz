import { MediaGrid } from "@/components/media/MediaGrid";
import { LoadingSpinner } from "@/components/ui/LoadingSpinner";
import { MediaStateTabs } from "@/components/media/MediaStateTabs";
import { useLibraryShows } from "@/lib/queries";
import { useConfigurableMediaStateFilter } from "@/hooks/use-configurable-media-state-filter";
import { seriesFilterConfig } from "@/config/media-filter-configs";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";
import { MoreVertical, RefreshCw } from "lucide-react";
import { metadataApi } from "@/lib/api";
import { toast } from "sonner";
import { useState } from "react";

export default function Series() {
  const { data: shows = [], isLoading, error } = useLibraryShows();
  const { filter, setFilter, counts, filteredItems: filteredShows } =
    useConfigurableMediaStateFilter(shows, seriesFilterConfig);
  const [isRefreshing, setIsRefreshing] = useState(false);

  const handleRefreshMetadata = async () => {
    setIsRefreshing(true);
    try {
      await metadataApi.refreshSeriesMetadata();
      toast.success("Series metadata refresh started");
    } catch (error) {
      toast.error("Failed to refresh metadata");
      console.error(error);
    } finally {
      setIsRefreshing(false);
    }
  };

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
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="h-8 w-8">
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
          availableFilters={seriesFilterConfig.filters}
          mediaType="tv"
        >
          <MediaGrid items={filteredShows} />
        </MediaStateTabs>
      )}
    </div>
  );
}