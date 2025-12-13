import { MediaStateFilterConfig } from "@/hooks/use-configurable-media-state-filter";

// Movie types
export type MovieStateFilter =
  | "all"
  | "missing"
  | "available"
  | "unreleased"
  | "downloading";

export interface MovieStateCounts {
  all: number;
  missing: number;
  available: number;
  unreleased: number;
  downloading: number;
}

// Series types
export type SeriesStateFilter =
  | "all"
  | "missing"
  | "available"
  | "unreleased"
  | "downloading"
  | "continuing";

export interface SeriesStateCounts {
  all: number;
  missing: number;
  available: number;
  unreleased: number;
  downloading: number;
  continuing: number;
  completed: number;
}

/**
 * Configuration for movie state filtering.
 * - "available" includes items with "discovered" or "downloaded" states
 */
export const movieFilterConfig: MediaStateFilterConfig<
  MovieStateFilter,
  MovieStateCounts
> = {
  filters: ["all", "missing", "available", "unreleased", "downloading"],
  defaultFilter: "all",
  filterMappings: {
    all: { states: [] },
    missing: { states: ["missing"] },
    available: { states: ["discovered", "downloaded"] },
    unreleased: { states: ["unreleased"] },
    downloading: { states: ["downloading"] },
  },
  countRules: {
    all: { states: [] },
    missing: { states: ["missing"] },
    available: { states: ["discovered", "downloaded"] },
    unreleased: { states: ["unreleased"] },
    downloading: { states: ["downloading"] },
  },
};

/**
 * Configuration for series state filtering.
 * - "available" includes items with "discovered" or "completed" states
 * - "completed" is counted separately AND as part of "available"
 */
export const seriesFilterConfig: MediaStateFilterConfig<
  SeriesStateFilter,
  SeriesStateCounts
> = {
  filters: [
    "all",
    "missing",
    "available",
    "unreleased",
    "downloading",
    "continuing",
  ],
  defaultFilter: "all",
  filterMappings: {
    all: { states: [] },
    missing: { states: ["missing"] },
    available: { states: ["discovered", "completed"] },
    unreleased: { states: ["unreleased"] },
    downloading: { states: ["downloading"] },
    continuing: { states: ["continuing"] },
  },
  countRules: {
    all: { states: [] },
    missing: { states: ["missing"] },
    available: { states: ["discovered", "completed"] },
    unreleased: { states: ["unreleased"] },
    downloading: { states: ["downloading"] },
    continuing: { states: ["continuing"] },
    completed: { states: ["completed"] },
  },
};
