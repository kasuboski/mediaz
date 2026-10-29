import { MediaGrid } from "@/components/media/MediaGrid";
import { LoadingSpinner } from "@/components/ui/LoadingSpinner";
import { MediaStateTabs } from "@/components/media/MediaStateTabs";
import { useLibraryShows } from "@/lib/queries";
import { useLibraryUrlState } from "@/hooks/use-library-url-state";
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
import { useState, useEffect } from "react";
import { PaginationControls } from "@/components/ui/pagination-controls";
import { PageSizeSelector } from "@/components/ui/page-size-selector";

export default function Series() {
  const { page, pageSize, state: filter, setPage, setState: setFilter, setPageSize: setSize } = useLibraryUrlState(seriesFilterConfig.filters);
  const states = seriesFilterConfig.filterMappings[filter as keyof typeof seriesFilterConfig.filterMappings].states;
  const { data, isLoading, error, refetch } = useLibraryShows({ page, pageSize, states });
  useEffect(() => {
    const lastPage = Math.max(1, data?.pagination.totalPages ?? 1);
    if (data && page > lastPage) setPage(lastPage, true);
  }, [data, page, setPage]);
  const shows = data?.items ?? [];
  const counts = { all: Object.values(data?.byState ?? {}).reduce((a,b) => a+b,0), missing: data?.byState.missing ?? 0, available: (data?.byState.discovered ?? 0)+(data?.byState.completed ?? 0), unreleased: data?.byState.unreleased ?? 0, downloading: data?.byState.downloading ?? 0, continuing: data?.byState.continuing ?? 0 };
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
          <Button className="mt-4" onClick={() => refetch()}>Retry</Button>
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
          {shows.length === 0 ? <p className="py-12 text-center text-muted-foreground">{filter === 'all' ? 'Your TV library is empty.' : 'No TV shows match this filter.'}</p> : <MediaGrid items={shows} />}
          <div className="mt-6 flex flex-wrap items-center justify-between gap-4">
            <span className="text-sm text-muted-foreground">{data?.pagination.totalItems ? `Showing ${(page - 1) * pageSize + 1}–${Math.min(page * pageSize, data.pagination.totalItems)} of ${data.pagination.totalItems} matching TV shows` : '0 matching TV shows'}</span>
            <PageSizeSelector value={pageSize} onChange={setSize} />
            <PaginationControls currentPage={page} totalPages={data?.pagination.totalPages ?? 0} onPageChange={setPage} />
          </div>
        </MediaStateTabs>
      )}
    </div>
  );
}