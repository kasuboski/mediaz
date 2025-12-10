import { ReactNode } from "react";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { getMediaStateColor, type MediaType } from "@/lib/media-state";
import type { MovieStateFilter } from "@/hooks/use-media-state-filter";
import type { SeriesStateFilter } from "@/hooks/use-series-state-filter";

interface StateCounts {
  all: number;
  missing: number;
  available: number;
  unreleased: number;
  downloading: number;
  continuing?: number;
  completed?: number;
}

interface MediaStateTabsProps {
  filter: MovieStateFilter | SeriesStateFilter;
  onFilterChange: (filter: MovieStateFilter | SeriesStateFilter) => void;
  counts: StateCounts;
  children: ReactNode;
  mediaType: MediaType;
}

export function MediaStateTabs({
  filter,
  onFilterChange,
  counts,
  children,
  mediaType,
}: MediaStateTabsProps) {
  return (
    <Tabs
      value={filter}
      onValueChange={(v) => onFilterChange(v as MovieStateFilter | SeriesStateFilter)}
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
              className={`h-1 w-full ${getMediaStateColor("discovered", mediaType)}`}
            />
          </div>
        </TabsTrigger>
        {mediaType === 'tv' && (
          <TabsTrigger value="continuing">
            <div className="flex flex-col items-center gap-1">
              <span>
                Continuing{" "}
                <span className="ml-1.5 text-xs opacity-70">
                  ({counts.continuing})
                </span>
              </span>
              <div
                className={`h-1 w-full ${getMediaStateColor("continuing", mediaType)}`}
              />
            </div>
          </TabsTrigger>
        )}
        <TabsTrigger value="downloading">
          <div className="flex flex-col items-center gap-1">
            <span>
              Downloading{" "}
              <span className="ml-1.5 text-xs opacity-70">
                ({counts.downloading})
              </span>
            </span>
            <div
              className={`h-1 w-full ${getMediaStateColor("downloading", mediaType)}`}
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
              className={`h-1 w-full ${getMediaStateColor("missing", mediaType)}`}
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
              className={`h-1 w-full ${getMediaStateColor("unreleased", mediaType)}`}
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
