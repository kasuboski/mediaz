import { useState, useMemo } from "react";

/**
 * Defines which backend states map to a filter option
 */
export interface StateMapping {
  states: string[];
}

/**
 * Defines how to count items for a specific count field
 */
export interface CountRule {
  states: string[];
}

/**
 * Configuration for media state filtering behavior
 */
export interface MediaStateFilterConfig<
  TFilter extends string,
  TCounts extends Record<string, number>
> {
  filters: readonly TFilter[];
  defaultFilter: TFilter;
  filterMappings: Record<TFilter, StateMapping>;
  countRules: Record<keyof TCounts, CountRule>;
}

/**
 * Return type for the media state filter hook
 */
export interface UseMediaStateFilterReturn<
  TItem,
  TFilter extends string,
  TCounts extends Record<string, number>
> {
  filter: TFilter;
  setFilter: (filter: TFilter) => void;
  counts: TCounts;
  filteredItems: TItem[];
}

/**
 * Generic media state filter hook that works with any media type.
 * Behavior is driven by the provided configuration object.
 *
 * @param items - Array of media items to filter
 * @param config - Configuration defining filter behavior
 * @returns Object containing filter state, setter, counts, and filtered items
 */
export function useConfigurableMediaStateFilter<
  TItem extends { state?: string },
  TFilter extends string,
  TCounts extends Record<string, number>
>(
  items: TItem[],
  config: MediaStateFilterConfig<TFilter, TCounts>
): UseMediaStateFilterReturn<TItem, TFilter, TCounts> {
  const [filter, setFilter] = useState<TFilter>(config.defaultFilter);

  // Calculate counts based on configuration rules
  const counts = useMemo(() => {
    const result = {} as TCounts;

    // Initialize all count fields to 0
    for (const key in config.countRules) {
      result[key] = 0 as TCounts[typeof key];
    }

    // Count items based on their state
    items.forEach((item) => {
      const itemState = item.state;

      for (const countKey in config.countRules) {
        const rule = config.countRules[countKey];

        // Count item if: states array is empty (count all) OR item state matches rule
        const shouldCount =
          rule.states.length === 0 ||
          (itemState !== undefined && rule.states.includes(itemState));

        if (shouldCount) {
          (result[countKey] as number)++;
        }
      }
    });

    return result;
  }, [items, config.countRules]);

  // Filter items based on current filter selection
  const filteredItems = useMemo(() => {
    const mapping = config.filterMappings[filter];

    // If states array is empty, return all items (for "all" filter)
    if (!mapping || mapping.states.length === 0) {
      return items;
    }

    // Filter items whose state matches the filter mapping
    return items.filter((item) => {
      return item.state && mapping.states.includes(item.state);
    });
  }, [items, filter, config.filterMappings]);

  return { filter, setFilter, counts, filteredItems };
}
