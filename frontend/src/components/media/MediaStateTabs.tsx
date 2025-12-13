import { ReactNode } from "react";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { getMediaStateColor, type MediaType } from "@/lib/media-state";

/**
 * Mapping of filter values to their display configuration
 */
interface TabConfig {
  label: string;
  colorState?: string; // State to use for color (if undefined, no color bar shown)
}

interface MediaStateTabsProps<
  TFilter extends string,
  TCounts extends Record<string, number>
> {
  filter: TFilter;
  onFilterChange: (filter: TFilter) => void;
  counts: TCounts;
  availableFilters: readonly TFilter[];
  children: ReactNode;
  mediaType: MediaType;
}

/**
 * Tab configurations for each filter type
 */
const TAB_CONFIGS: Record<string, TabConfig> = {
  all: { label: "All" },
  available: { label: "Available", colorState: "discovered" },
  continuing: { label: "Continuing", colorState: "continuing" },
  downloading: { label: "Downloading", colorState: "downloading" },
  missing: { label: "Missing", colorState: "missing" },
  unreleased: { label: "Unreleased", colorState: "unreleased" },
};

export function MediaStateTabs<
  TFilter extends string,
  TCounts extends Record<string, number>
>({
  filter,
  onFilterChange,
  counts,
  availableFilters,
  children,
  mediaType,
}: MediaStateTabsProps<TFilter, TCounts>) {
  return (
    <Tabs
      value={filter}
      onValueChange={(v) => onFilterChange(v as TFilter)}
      className="mb-6"
    >
      <TabsList>
        {availableFilters.map((filterValue) => {
          const config = TAB_CONFIGS[filterValue] || {
            label: filterValue,
          };
          const count = counts[filterValue as keyof TCounts] ?? 0;

          return (
            <TabsTrigger key={filterValue} value={filterValue}>
              {config.colorState ? (
                <div className="flex flex-col items-center gap-1">
                  <span>
                    {config.label}{" "}
                    <span className="ml-1.5 text-xs opacity-70">({count})</span>
                  </span>
                  <div
                    className={`h-1 w-full ${getMediaStateColor(
                      config.colorState,
                      mediaType
                    )}`}
                  />
                </div>
              ) : (
                <>
                  {config.label}{" "}
                  <span className="ml-1.5 text-xs opacity-70">({count})</span>
                </>
              )}
            </TabsTrigger>
          );
        })}
      </TabsList>

      <TabsContent value={filter} className="mt-6">
        {children}
      </TabsContent>
    </Tabs>
  );
}
