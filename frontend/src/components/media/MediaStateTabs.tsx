import { ReactNode } from "react";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { getMovieStateColor } from "@/lib/movie-state";
import type { MediaStateFilter } from "@/hooks/use-media-state-filter";

interface StateCounts {
  all: number;
  missing: number;
  available: number;
  unreleased: number;
  downloading: number;
}

interface MediaStateTabsProps {
  filter: MediaStateFilter;
  onFilterChange: (filter: MediaStateFilter) => void;
  counts: StateCounts;
  children: ReactNode;
}

/**
 * Shared component for filtering media items by state.
 * Displays tabs for all, available, downloading, missing, and unreleased states.
 */
export function MediaStateTabs({
  filter,
  onFilterChange,
  counts,
  children,
}: MediaStateTabsProps) {
  return (
    <Tabs
      value={filter}
      onValueChange={(v) => onFilterChange(v as MediaStateFilter)}
      className="mb-6"
    >
      <TabsList>
        <TabsTrigger value="all">
          All <span className="ml-1.5 text-xs opacity-70">({counts.all})</span>
        </TabsTrigger>
        <TabsTrigger value="available">
          <div className="flex flex-col items-center gap-1">
            <span>
              Available{" "}
              <span className="ml-1.5 text-xs opacity-70">
                ({counts.available})
              </span>
            </span>
            <div
              className={`h-1 w-full ${getMovieStateColor("discovered")}`}
            />
          </div>
        </TabsTrigger>
        <TabsTrigger value="downloading">
          <div className="flex flex-col items-center gap-1">
            <span>
              Downloading{" "}
              <span className="ml-1.5 text-xs opacity-70">
                ({counts.downloading})
              </span>
            </span>
            <div
              className={`h-1 w-full ${getMovieStateColor("downloading")}`}
            />
          </div>
        </TabsTrigger>
        <TabsTrigger value="missing">
          <div className="flex flex-col items-center gap-1">
            <span>
              Missing{" "}
              <span className="ml-1.5 text-xs opacity-70">
                ({counts.missing})
              </span>
            </span>
            <div
              className={`h-1 w-full ${getMovieStateColor("missing")}`}
            />
          </div>
        </TabsTrigger>
        <TabsTrigger value="unreleased">
          <div className="flex flex-col items-center gap-1">
            <span>
              Unreleased{" "}
              <span className="ml-1.5 text-xs opacity-70">
                ({counts.unreleased})
              </span>
            </span>
            <div
              className={`h-1 w-full ${getMovieStateColor("unreleased")}`}
            />
          </div>
        </TabsTrigger>
      </TabsList>

      <TabsContent value={filter} className="mt-6">
        {children}
      </TabsContent>
    </Tabs>
  );
}
