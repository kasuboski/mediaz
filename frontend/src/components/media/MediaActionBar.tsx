import { LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

export interface MediaAction {
  icon: LucideIcon;
  label: string;
  onClick: () => void;
  disabled?: boolean;
  loading?: boolean;
  variant?: "default" | "destructive";
}

interface MediaActionBarProps {
  actions: MediaAction[];
}

export function MediaActionBar({ actions }: MediaActionBarProps) {
  if (actions.length === 0) {
    return null;
  }

  return (
    <div className="inline-flex items-center gap-1 bg-background/95 backdrop-blur rounded-lg p-2 shadow-lg border border-border">
      {actions.map((action) => {
        const Icon = action.icon;
        return (
          <button
            key={action.label}
            onClick={action.onClick}
            disabled={action.disabled || action.loading}
            className={cn(
              "flex flex-col items-center gap-1 px-4 py-2 rounded-md transition-colors min-w-16",
              "disabled:opacity-50 disabled:cursor-not-allowed",
              action.variant === "destructive"
                ? "text-destructive hover:bg-destructive hover:text-destructive-foreground"
                : "hover:bg-accent"
            )}
          >
            <Icon className={cn("h-5 w-5", action.loading && "animate-spin")} />
            <span className="text-xs font-medium">{action.label}</span>
          </button>
        );
      })}
    </div>
  );
}
