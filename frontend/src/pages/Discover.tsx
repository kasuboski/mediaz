import { useMemo } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { PaginationControls } from "@/components/ui/pagination-controls";
import { Film, Tv, Search as SearchIcon, Sparkles } from "lucide-react";
import { MediaGrid } from "@/components/media/MediaGrid";
import { LoadingSpinner } from "@/components/ui/LoadingSpinner";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useSearchMoviesPage, useSearchTVPage } from "@/lib/queries";
import type { MediaItem, MediaSearchPage } from "@/lib/api";

type FilterType = "all" | "movies" | "tv";

export default function Discover() {
  const [searchParams, setSearchParams] = useSearchParams();
  const query = searchParams.get("query") || "";
  const requestedPage = Number(searchParams.get("page"));
  const page = Number.isSafeInteger(requestedPage) && requestedPage > 0 ? requestedPage : 1;
  const rawFilter = searchParams.get("type");
  const filter: FilterType = rawFilter === "movies" || rawFilter === "tv" ? rawFilter : "all";

  const queryClient = useQueryClient();
  const movieFirstPage = queryClient.getQueryData<MediaSearchPage>(["search", "movies", query.trim(), 1]);
  const tvFirstPage = queryClient.getQueryData<MediaSearchPage>(["search", "tv", query.trim(), 1]);
  const movieMetadataQuery = useSearchMoviesPage(query, 1, page > 1);
  const tvMetadataQuery = useSearchTVPage(query, 1, page > 1);
  const moviePageEnabled = page === 1 || (!!movieFirstPage && page <= movieFirstPage.pagination.totalPages);
  const tvPageEnabled = page === 1 || (!!tvFirstPage && page <= tvFirstPage.pagination.totalPages);
  const moviesQuery = useSearchMoviesPage(query, page, moviePageEnabled);
  const tvQuery = useSearchTVPage(query, page, tvPageEnabled);
  const movies = moviesQuery.data?.items ?? [];
  const tvShows = tvQuery.data?.items ?? [];
  const isLoading = (filter !== "tv" && (moviePageEnabled ? moviesQuery.isLoading : page > 1 && !movieFirstPage && movieMetadataQuery.isLoading)) || (filter !== "movies" && (tvPageEnabled ? tvQuery.isLoading : page > 1 && !tvFirstPage && tvMetadataQuery.isLoading));
  const movieError = filter !== "tv" && !!(moviesQuery.error || (page > 1 && movieMetadataQuery.error));
  const tvError = filter !== "movies" && !!(tvQuery.error || (page > 1 && tvMetadataQuery.error));
  const hasError = movieError || tvError;
  const partialError = filter === "all" && movieError !== tvError;
  const retryFailed = () => {
    if (movieError) void queryClient.invalidateQueries({ queryKey: ["search", "movies", query.trim()] });
    if (tvError) void queryClient.invalidateQueries({ queryKey: ["search", "tv", query.trim()] });
  };

  const updateParams = (updates: Record<string, string | null>) => {
    const next = new URLSearchParams(searchParams);
    for (const [key, value] of Object.entries(updates)) {
      if (value === null) next.delete(key);
      else next.set(key, value);
    }
    setSearchParams(next);
  };

  // Combine and filter results based on selected tab
  const filteredResults = useMemo(() => {
    let results: MediaItem[] = [];

    if (filter === "all") {
      results = [...movies, ...tvShows];
    } else if (filter === "movies") {
      results = movies;
    } else if (filter === "tv") {
      results = tvShows;
    }

    // Sort by popularity/relevance (you could enhance this with actual sorting)
    return results;
  }, [movies, tvShows, filter]);

  const moviePagination = moviesQuery.data?.pagination ?? movieFirstPage?.pagination;
  const tvPagination = tvQuery.data?.pagination ?? tvFirstPage?.pagination;
  const moviesCount = moviePagination?.totalItems ?? 0;
  const tvCount = tvPagination?.totalItems ?? 0;
  const knownTotalResults = (movieError ? 0 : moviesCount) + (tvError ? 0 : tvCount);
  const totalResults = moviesCount + tvCount;
  const pagination = filter === "movies" ? moviePagination : filter === "tv" ? tvPagination : undefined;
  const totalPages = filter === "movies" ? (pagination?.totalPages ?? 0) : filter === "tv" ? (pagination?.totalPages ?? 0) : Math.max(moviePagination?.totalPages ?? movieFirstPage?.pagination.totalPages ?? 0, tvPagination?.totalPages ?? tvFirstPage?.pagination.totalPages ?? 0);
  const filteredTotal = filter === "movies" ? moviesCount : filter === "tv" ? tvCount : totalResults;
  const displayedCount = filteredResults.length;
  const rangeStart = (page - 1) * 20 + 1;
  const rangeEnd = rangeStart + displayedCount - 1;

  // Empty state - no query
  if (!query) {
    return (
      <div className="container mx-auto px-6 py-12">
        <div className="text-center py-20">
          <div className="inline-flex items-center justify-center w-20 h-20 rounded-full bg-gradient-primary/10 mb-6">
            <Sparkles className="h-10 w-10 text-primary" />
          </div>
          <h1 className="text-4xl font-bold text-foreground mb-4">
            Discover Movies & TV Shows
          </h1>
          <p className="text-muted-foreground text-lg max-w-md mx-auto mb-8">
            Search for your favorite content using the search bar above. Find
            movies and TV shows from TMDB's extensive catalog.
          </p>
          <div className="flex items-center justify-center gap-6 text-sm text-muted-foreground">
            <div className="flex items-center gap-2">
              <Film className="h-4 w-4" />
              <span>Movies</span>
            </div>
            <div className="flex items-center gap-2">
              <Tv className="h-4 w-4" />
              <span>TV Shows</span>
            </div>
          </div>
        </div>
      </div>
    );
  }

  // Error state
  if (hasError && !isLoading && !partialError) {
    return (
      <div className="container mx-auto px-6 py-8">
        <div className="text-center py-16">
          <div className="inline-flex items-center justify-center w-16 h-16 rounded-full bg-destructive/10 mb-4">
            <SearchIcon className="h-8 w-8 text-destructive" />
          </div>
          <h2 className="text-2xl font-bold text-foreground mb-2">
            Search Error
          </h2>
          <p className="text-muted-foreground">
            Something went wrong while searching. Please try again.
          </p>
          <button className="mt-4 rounded-md border px-4 py-2" onClick={retryFailed}>Retry</button>
        </div>
      </div>
    );
  }

  return (
    <div className="container mx-auto px-6 py-8">
      <div className="mb-8">
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-6">
          <div>
            <h1 className="text-3xl font-bold text-foreground mb-2">
              Search Results
            </h1>
            <p className="text-muted-foreground">
              {isLoading
                ? "Searching..."
                : partialError
                  ? `At least ${knownTotalResults} results for "${query}"`
                  : totalResults === 0
                    ? "No results found"
                    : `Found ${totalResults} result${totalResults !== 1 ? "s" : ""} for "${query}"`}
            </p>
          </div>

          {!isLoading && (totalResults > 0 || partialError) && (
            <Tabs value={filter} onValueChange={(v) => updateParams({ type: v === "all" ? null : v, page: null })}>
              <TabsList className="bg-muted/50 border border-border/50">
                <TabsTrigger
                  value="all"
                  className="data-[state=active]:bg-gradient-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm"
                >
                  All ({partialError ? `at least ${knownTotalResults}` : totalResults})
                </TabsTrigger>
                <TabsTrigger
                  value="movies"
                  className="data-[state=active]:bg-gradient-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm"
                >
                  <Film className="h-4 w-4 mr-2" />
                  Movies ({moviesCount})
                </TabsTrigger>
                <TabsTrigger
                  value="tv"
                  className="data-[state=active]:bg-gradient-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm"
                >
                  <Tv className="h-4 w-4 mr-2" />
                  TV ({tvCount})
                </TabsTrigger>
              </TabsList>
            </Tabs>
          )}
        </div>
      </div>

      {partialError && (
        <div role="status" className="mb-6 rounded-md border border-destructive/30 bg-destructive/5 p-4 text-sm">
          {movieError ? "Movie" : "TV"} results could not be loaded. Showing the available {movieError ? "TV" : "movie"} results; the total is unknown.
          <button className="ml-3 underline" onClick={retryFailed}>Retry failed search</button>
        </div>
      )}

      {isLoading ? (
        <div className="flex flex-col items-center justify-center py-20">
          <LoadingSpinner size="lg" />
          <p className="text-muted-foreground mt-4">Searching TMDB...</p>
        </div>
      ) : totalResults === 0 ? (
        <div className="text-center py-20">
          <div className="inline-flex items-center justify-center w-16 h-16 rounded-full bg-muted mb-4">
            <SearchIcon className="h-8 w-8 text-muted-foreground" />
          </div>
          <h2 className="text-2xl font-bold text-foreground mb-2">
            No results found
          </h2>
          <p className="text-muted-foreground max-w-md mx-auto">
            We couldn't find any movies or TV shows matching "{query}". Try
            adjusting your search terms.
          </p>
        </div>
      ) : (
        <div>
          <div className="mb-6">
            <p className="text-sm text-muted-foreground">
              {filter === "all"
                ? `Page ${page}: showing ${displayedCount} results across movie and TV search pages`
                : `Showing ${displayedCount ? `${rangeStart}–${Math.min(rangeEnd, filteredTotal)}` : 0} of ${filteredTotal} ${filter === "movies" ? "movies" : "TV shows"}`}
            </p>
          </div>
          <MediaGrid items={filteredResults} />
          {totalPages > 1 && (
            <PaginationControls
              currentPage={page}
              totalPages={totalPages}
              onPageChange={(nextPage) => updateParams({ page: nextPage === 1 ? null : String(nextPage) })}
            />
          )}
        </div>
      )}
    </div>
  );
}