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
      return "bg-success";
    case "continuing":
      return "bg-info";
    case "downloading":
      return "bg-primary";
    case "discovered":
      return "bg-success";
    case "missing":
      return "bg-destructive";
    case "unreleased":
      return "bg-info";
    default:
      return "bg-muted";
  }
}
