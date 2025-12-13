export type SeriesState =
  | ""
  | "missing"
  | "discovered"
  | "unreleased"
  | "continuing"
  | "downloading"
  | "completed";

export function getSeriesStateColor(state?: string): string {
  switch (state) {
    case "completed":
      return "bg-emerald-500";
    case "continuing":
      return "bg-blue-500";
    case "downloading":
      return "bg-purple-500";
    case "discovered":
      return "bg-green-500";
    case "missing":
      return "bg-red-500";
    case "unreleased":
      return "bg-cyan-500";
    default:
      return "bg-muted";
  }
}
