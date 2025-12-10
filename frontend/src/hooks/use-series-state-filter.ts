import { useState, useMemo } from "react";

export type SeriesStateFilter = "all" | "missing" | "available" | "unreleased" | "downloading" | "continuing";

interface SeriesItem {
  state?: string;
}

interface SeriesStateCounts {
  all: number;
  missing: number;
  available: number;
  unreleased: number;
  downloading: number;
  continuing: number;
  completed: number;
}

interface UseSeriesStateFilterReturn<T extends SeriesItem> {
  filter: SeriesStateFilter;
  setFilter: (filter: SeriesStateFilter) => void;
  counts: SeriesStateCounts;
  filteredItems: T[];
}

export function useSeriesStateFilter<T extends SeriesItem>(
  items: T[]
): UseSeriesStateFilterReturn<T> {
  const [filter, setFilter] = useState<SeriesStateFilter>("all");

  const counts = useMemo(() => {
    const stateCounts: SeriesStateCounts = {
      all: items.length,
      missing: 0,
      available: 0,
      unreleased: 0,
      downloading: 0,
      continuing: 0,
      completed: 0,
    };

    items.forEach((item) => {
      const state = item.state;

      if (state === "discovered" || state === "completed") {
        stateCounts.available++;
      }

      if (state === "missing") {
        stateCounts.missing++;
      }
      if (state === "unreleased") {
        stateCounts.unreleased++;
      }
      if (state === "downloading") {
        stateCounts.downloading++;
      }
      if (state === "continuing") {
        stateCounts.continuing++;
      }
      if (state === "completed") {
        stateCounts.completed++;
      }
    });

    return stateCounts;
  }, [items]);

  const filteredItems = useMemo(() => {
    if (filter === "all") {
      return items;
    }

    if (filter === "available") {
      return items.filter(
        (item) => item.state === "discovered" || item.state === "completed"
      );
    }

    return items.filter((item) => item.state === filter);
  }, [items, filter]);

  return { filter, setFilter, counts, filteredItems };
}
