import {
  CheckCircle,
  Download,
  FolderSearch,
  AlertCircle,
  Clock,
  Radio,
  CheckCheck,
  type LucideIcon
} from "lucide-react";

export type SeriesState =
  | ""
  | "missing"
  | "discovered"
  | "unreleased"
  | "continuing"
  | "downloading"
  | "completed";

interface StateBadgeConfig {
  variant: "outline" | "secondary" | "default" | "destructive";
  icon: LucideIcon;
  className: string;
  label: string;
}

export function getSeriesStateBadge(state?: string): StateBadgeConfig {
  switch (state) {
    case "completed":
      return {
        variant: "outline",
        icon: CheckCheck,
        className: "bg-emerald-500/10 text-emerald-500 border-emerald-500/20",
        label: "Completed",
      };
    case "continuing":
      return {
        variant: "outline",
        icon: Radio,
        className: "bg-blue-500/10 text-blue-500 border-blue-500/20",
        label: "Continuing",
      };
    case "downloading":
      return {
        variant: "secondary",
        icon: Download,
        className: "bg-blue-500/10 text-blue-500 border-blue-500/20",
        label: "Downloading",
      };
    case "discovered":
      return {
        variant: "outline",
        icon: FolderSearch,
        className: "bg-cyan-500/10 text-cyan-500 border-cyan-500/20",
        label: "Discovered",
      };
    case "missing":
      return {
        variant: "outline",
        icon: AlertCircle,
        className: "bg-yellow-500/10 text-yellow-500 border-yellow-500/20",
        label: "Missing",
      };
    case "unreleased":
      return {
        variant: "outline",
        icon: Clock,
        className: "bg-purple-500/10 text-purple-500 border-purple-500/20",
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
