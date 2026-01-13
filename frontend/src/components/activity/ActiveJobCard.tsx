import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Clock, PlayCircle } from "lucide-react";
import type { ActiveJob } from "@/lib/api";

interface ActiveJobCardProps {
  job: ActiveJob;
}

export function ActiveJobCard({ job }: ActiveJobCardProps) {
  const getJobStateColor = (state: string): string => {
    switch (state) {
      case "running":
        return "bg-blue-500/10 text-blue-500 border-blue-500/20";
      case "pending":
        return "bg-yellow-500/10 text-yellow-500 border-yellow-500/20";
      case "error":
        return "bg-red-500/10 text-red-500 border-red-500/20";
      case "done":
        return "bg-green-500/10 text-green-500 border-green-500/20";
      default:
        return "text-muted-foreground";
    }
  };

  const getJobStateLabel = (state: string): string => {
    switch (state) {
      case "running":
        return "Running";
      case "pending":
        return "Pending";
      case "error":
        return "Error";
      case "done":
        return "Done";
      case "cancelled":
        return "Cancelled";
      default:
        return state;
    }
  };

  const stateColor = getJobStateColor(job.state);
  const stateLabel = getJobStateLabel(job.state);

  return (
    <Card className="bg-gradient-card border-border/50 shadow-card hover:shadow-card-hover transition-all duration-300">
      <CardContent className="p-4 space-y-4">
        <div className="flex items-start justify-between gap-3">
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 mb-1">
              <PlayCircle className="h-4 w-4 text-primary flex-shrink-0" />
              <h3 className="font-medium text-card-foreground text-sm truncate">
                {job.type}
              </h3>
            </div>
            <p className="text-xs text-muted-foreground">Job ID: {job.id}</p>
          </div>

          <Badge variant="outline" className={stateColor}>
            {stateLabel}
          </Badge>
        </div>

        <div className="space-y-2 pt-2 border-t border-border/50">
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <Clock className="h-3 w-3" />
            <span>{job.duration}</span>
          </div>

          <div className="grid grid-cols-2 gap-2 text-xs text-muted-foreground">
            <div>
              <span className="block text-[10px] uppercase tracking-wider opacity-70">Started</span>
              <span className="font-mono">
                {new Date(job.createdAt).toLocaleTimeString([], {
                  hour: "2-digit",
                  minute: "2-digit",
                })}
              </span>
            </div>
            <div>
              <span className="block text-[10px] uppercase tracking-wider opacity-70">Updated</span>
              <span className="font-mono">
                {new Date(job.updatedAt).toLocaleTimeString([], {
                  hour: "2-digit",
                  minute: "2-digit",
                })}
              </span>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
