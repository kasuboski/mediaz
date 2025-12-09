import { useState, useMemo } from "react";
import { MediaGrid } from "@/components/media/MediaGrid";
import { LoadingSpinner } from "@/components/ui/LoadingSpinner";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { useLibraryMovies } from "@/lib/queries";
import { Film } from "lucide-react";

type MovieFilter = "all" | "missing" | "discovered" | "unreleased" | "downloading" | "downloaded";

export default function Movies() {
  const { data: movies = [], isLoading, error } = useLibraryMovies();
  const [filter, setFilter] = useState<MovieFilter>("all");

  const counts = useMemo(() => {
    const stateCounts = {
      all: movies.length,
      missing: 0,
      discovered: 0,
      unreleased: 0,
      downloading: 0,
      downloaded: 0,
    };

    movies.forEach((movie) => {
      const state = movie.state as MovieFilter;
      if (state && state in stateCounts) {
        stateCounts[state]++;
      }
    });

    return stateCounts;
  }, [movies]);

  const filteredMovies = useMemo(() => {
    if (filter === "all") {
      return movies;
    }
    return movies.filter((movie) => movie.state === filter);
  }, [movies, filter]);

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
      <div className="mb-6">
        <div className="flex items-center gap-3 mb-2">
          <Film className="h-8 w-8 text-primary" />
          <h1 className="text-3xl font-bold text-foreground">Movie Library</h1>
        </div>
        <p className="text-muted-foreground">
          {isLoading ? "Loading..." : `${counts.all} movies in your library`}
        </p>
      </div>

      {isLoading ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner size="lg" />
        </div>
      ) : (
        <>
          <Tabs value={filter} onValueChange={(v) => setFilter(v as MovieFilter)} className="mb-6">
            <TabsList>
              <TabsTrigger value="all">
                All <span className="ml-1.5 text-xs opacity-70">({counts.all})</span>
              </TabsTrigger>
              <TabsTrigger value="downloaded">
                Downloaded <span className="ml-1.5 text-xs opacity-70">({counts.downloaded})</span>
              </TabsTrigger>
              <TabsTrigger value="downloading">
                Downloading <span className="ml-1.5 text-xs opacity-70">({counts.downloading})</span>
              </TabsTrigger>
              <TabsTrigger value="discovered">
                Discovered <span className="ml-1.5 text-xs opacity-70">({counts.discovered})</span>
              </TabsTrigger>
              <TabsTrigger value="missing">
                Missing <span className="ml-1.5 text-xs opacity-70">({counts.missing})</span>
              </TabsTrigger>
              <TabsTrigger value="unreleased">
                Unreleased <span className="ml-1.5 text-xs opacity-70">({counts.unreleased})</span>
              </TabsTrigger>
            </TabsList>

            <TabsContent value={filter} className="mt-6">
              <MediaGrid items={filteredMovies} />
            </TabsContent>
          </Tabs>
        </>
      )}
    </div>
  );
}