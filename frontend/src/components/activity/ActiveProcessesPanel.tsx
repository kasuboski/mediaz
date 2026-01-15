import { ActiveMovieCard } from "./ActiveMovieCard";
import { ActiveSeriesCard } from "./ActiveSeriesCard";
import { ActiveJobCard } from "./ActiveJobCard";
import type { ActiveActivityResponse } from "@/lib/api";
import { Film, Tv, Briefcase } from "lucide-react";

interface ActiveProcessesPanelProps {
  data: ActiveActivityResponse | null;
  isLoading?: boolean;
}

export function ActiveProcessesPanel({ data, isLoading }: ActiveProcessesPanelProps) {
  const totalActive =
    (data?.movies.length ?? 0) +
    (data?.series.length ?? 0) +
    (data?.jobs.length ?? 0);

  if (isLoading) {
    return (
      <div className="space-y-6">
        {[1, 2, 3].map((i) => (
          <div key={i} className="space-y-3">
            <div className="h-6 w-32 bg-muted/30 rounded animate-pulse" />
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {[1, 2].map((j) => (
                <div key={j} className="aspect-[2/3] bg-muted/20 rounded-lg animate-pulse" />
              ))}
            </div>
          </div>
        ))}
      </div>
    );
  }

  if (totalActive === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-16 px-4 text-center">
        <div className="w-16 h-16 rounded-full bg-muted/20 flex items-center justify-center mb-4">
          <div className="w-8 h-8 border-2 border-muted-foreground/30 rounded-full animate-pulse" />
        </div>
        <h3 className="text-lg font-medium text-card-foreground mb-2">No Active Processes</h3>
        <p className="text-sm text-muted-foreground max-w-md">
          There are currently no downloads or jobs running. Check back later for activity.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {data?.movies.length ? (
        <section>
          <div className="flex items-center gap-3 mb-4">
            <Film className="h-5 w-5 text-primary" />
            <h2 className="text-lg font-semibold text-card-foreground">
              Downloading Movies
            </h2>
            <span className="text-sm text-muted-foreground">
              ({data.movies.length})
            </span>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {data.movies.map((movie) => (
              <ActiveMovieCard key={movie.id} movie={movie} />
            ))}
          </div>
        </section>
      ) : null}

      {data?.series.length ? (
        <section>
          <div className="flex items-center gap-3 mb-4">
            <Tv className="h-5 w-5 text-primary" />
            <h2 className="text-lg font-semibold text-card-foreground">
              Downloading Series
            </h2>
            <span className="text-sm text-muted-foreground">
              ({data.series.length})
            </span>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {data.series.map((series) => (
              <ActiveSeriesCard key={series.id} series={series} />
            ))}
          </div>
        </section>
      ) : null}

      {data?.jobs.length ? (
        <section>
          <div className="flex items-center gap-3 mb-4">
            <Briefcase className="h-5 w-5 text-primary" />
            <h2 className="text-lg font-semibold text-card-foreground">
              Active Jobs
            </h2>
            <span className="text-sm text-muted-foreground">
              ({data.jobs.length})
            </span>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
            {data.jobs.map((job) => (
              <ActiveJobCard key={job.id} job={job} />
            ))}
          </div>
        </section>
      ) : null}
    </div>
  );
}
