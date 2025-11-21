import { useState, useMemo } from "react";
import { useSearchParams } from "react-router-dom";
import { Film, Tv, Search as SearchIcon, Sparkles } from "lucide-react";
import { MediaGrid } from "@/components/media/MediaGrid";
import { LoadingSpinner } from "@/components/ui/LoadingSpinner";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useSearchMovies, useSearchTV } from "@/lib/queries";
import type { MediaItem } from "@/lib/api";

type FilterType = "all" | "movies" | "tv";

export default function Discover() {
  const [searchParams] = useSearchParams();
  const [filter, setFilter] = useState<FilterType>("all");
  const query = searchParams.get("query") || "";

  // Fetch both movies and TV shows
  const {
    data: movies = [],
    isLoading: isLoadingMovies,
    error: moviesError,
  } = useSearchMovies(query);
  const {
    data: tvShows = [],
    isLoading: isLoadingTV,
    error: tvError,
  } = useSearchTV(query);

  const isLoading = isLoadingMovies || isLoadingTV;
  const hasError = moviesError || tvError;

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

  const totalResults = movies.length + tvShows.length;
  const moviesCount = movies.length;
  const tvCount = tvShows.length;

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
  if (hasError && !isLoading) {
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
                : totalResults === 0
                  ? "No results found"
                  : `Found ${totalResults} result${totalResults !== 1 ? "s" : ""} for "${query}"`}
            </p>
          </div>

          {!isLoading && totalResults > 0 && (
            <Tabs value={filter} onValueChange={(v) => setFilter(v as FilterType)}>
              <TabsList className="bg-muted/50 border border-border/50">
                <TabsTrigger
                  value="all"
                  className="data-[state=active]:bg-gradient-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm"
                >
                  All ({totalResults})
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
          {filter !== "all" && (
            <div className="mb-6">
              <p className="text-sm text-muted-foreground">
                Showing {filteredResults.length}{" "}
                {filter === "movies" ? "movie" : "TV show"}
                {filteredResults.length !== 1 ? "s" : ""}
              </p>
            </div>
          )}
          <MediaGrid items={filteredResults} />
        </div>
      )}
    </div>
  );
}