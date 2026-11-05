import {
  CheckCircle,
  Download,
  FolderSearch,
  AlertCircle,
  Clock,
  type LucideIcon
} from "lucide-react";

export type MovieState =
  | ""
  | "missing"
  | "discovered"
  | "unreleased"
  | "downloading"
  | "downloaded";

interface StateBadgeConfig {
  variant: "outline" | "secondary" | "default" | "destructive";
  icon: LucideIcon;
  className: string;
  label: string;
}

export function getMovieStateBadge(state?: string): StateBadgeConfig {
  switch (state) {
    case "downloaded":
      return {
        variant: "outline",
        icon: CheckCircle,
        className: "bg-success/10 text-success border-success/20",
        label: "Downloaded",
      };
    case "downloading":
      return {
        variant: "secondary",
        icon: Download,
        className: "bg-info/10 text-info border-info/20",
        label: "Downloading",
      };
    case "discovered":
      return {
        variant: "outline",
        icon: FolderSearch,
        className: "bg-info/10 text-info border-info/20",
        label: "Discovered",
      };
    case "missing":
      return {
        variant: "outline",
        icon: AlertCircle,
        className: "bg-warning/10 text-warning border-warning/20",
        label: "Missing",
      };
    case "unreleased":
      return {
        variant: "outline",
        icon: Clock,
        className: "bg-primary/10 text-primary border-primary/20",
        label: "Unreleased",
      };
    default:
      return {
        variant: "outline",
        icon: Clock,
        className: "text-muted-foreground",
        label: "New",
      };
  }
}

export function getMovieStateColor(state?: string): string {
  switch (state) {
    case "downloaded":
      return "bg-success";
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
