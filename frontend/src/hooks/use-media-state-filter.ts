import { useState, useMemo } from "react";

export type MediaStateFilter = "all" | "missing" | "available" | "unreleased" | "downloading";

interface MediaItem {
  state?: string;
}

interface StateCounts {
  all: number;
  missing: number;
  available: number;
  unreleased: number;
  downloading: number;
}

interface UseMediaStateFilterReturn<T extends MediaItem> {
  filter: MediaStateFilter;
  setFilter: (filter: MediaStateFilter) => void;
  counts: StateCounts;
  filteredItems: T[];
}

/**
 * Custom hook for filtering media items by state.
 * Works with any media type (movies, TV shows) that has a state property.
 *
 * @param items - Array of media items to filter
 * @returns Object containing filter state, setter, counts, and filtered items
 */
export function useMediaStateFilter<T extends MediaItem>(
  items: T[]
): UseMediaStateFilterReturn<T> {
  const [filter, setFilter] = useState<MediaStateFilter>("all");

  const counts = useMemo(() => {
    const stateCounts: StateCounts = {
      all: items.length,
      missing: 0,
      available: 0,
      unreleased: 0,
      downloading: 0,
    };

    items.forEach((item) => {
      const state = item.state;
      if (state === "discovered" || state === "downloaded") {
        stateCounts.available++;
      } else if (state && state in stateCounts) {
        stateCounts[state as keyof StateCounts]++;
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
        (item) => item.state === "discovered" || item.state === "downloaded"
      );
    }
    return items.filter((item) => item.state === filter);
  }, [items, filter]);

  return { filter, setFilter, counts, filteredItems };
}
