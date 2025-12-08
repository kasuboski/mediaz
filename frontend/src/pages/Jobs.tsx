import { useJobs, useTriggerJob, useCancelJob } from "@/lib/queries";
import { type Job, type JobType, type JobState } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Clock,
  Loader2,
  CheckCircle,
  AlertCircle,
  X,
  Plus,
  RefreshCw,
} from "lucide-react";
import { toast } from "sonner";
import { formatDistanceToNow } from "date-fns";

/**
 * Get badge variant and icon for job state
 */
function getJobStateBadge(state: JobState) {
  switch (state) {
    case "pending":
      return {
        variant: "outline" as const,
        icon: <Clock className="h-3 w-3" />,
        className: "border-yellow-500 text-yellow-500",
      };
    case "running":
      return {
        variant: "secondary" as const,
        icon: <Loader2 className="h-3 w-3 animate-spin" />,
        className: "bg-blue-500/10 text-blue-500 border-blue-500/20",
      };
    case "done":
      return {
        variant: "default" as const,
        icon: <CheckCircle className="h-3 w-3" />,
        className: "bg-green-500/10 text-green-500 border-green-500/20",
      };
    case "error":
      return {
        variant: "destructive" as const,
        icon: <AlertCircle className="h-3 w-3" />,
        className: "",
      };
    case "cancelled":
      return {
        variant: "outline" as const,
        icon: <X className="h-3 w-3" />,
        className: "text-muted-foreground",
      };
    default:
      return {
        variant: "outline" as const,
        icon: null,
        className: "",
      };
  }
}

/**
 * Format job type to display name
 */
function formatJobType(type: JobType): string {
  switch (type) {
    case "MovieIndex":
      return "Movie Index";
    case "MovieReconcile":
      return "Movie Reconcile";
    case "SeriesIndex":
      return "Series Index";
    case "SeriesReconcile":
      return "Series Reconcile";
    default:
      return type;
  }
}

/**
 * Format timestamp to relative time
 */
function formatTimestamp(timestamp: string): string {
  try {
    return formatDistanceToNow(new Date(timestamp), { addSuffix: true });
  } catch {
    return "Invalid date";
  }
}

/**
 * Job state badge component
 */
function JobStateBadge({ state }: { state: JobState }) {
  const { variant, icon, className } = getJobStateBadge(state);

  return (
    <Badge variant={variant} className={`flex items-center gap-1.5 ${className}`}>
      {icon}
      <span className="capitalize">{state || "new"}</span>
    </Badge>
  );
}

/**
 * Jobs page component for managing background jobs
 */
export default function Jobs() {
  const { data, isLoading, error, refetch } = useJobs();
  const triggerJob = useTriggerJob();
  const cancelJob = useCancelJob();

  // Handle job trigger
  const handleTriggerJob = (type: JobType) => {
    triggerJob.mutate(type, {
      onSuccess: () => {
        toast.success(`${formatJobType(type)} job triggered successfully`);
      },
      onError: (error) => {
        toast.error(`Failed to trigger job: ${error.message}`);
      },
    });
  };

  // Handle job cancellation
  const handleCancelJob = (job: Job) => {
    cancelJob.mutate(job.id, {
      onSuccess: () => {
        toast.success(`${formatJobType(job.type)} job cancelled`);
      },
      onError: (error) => {
        toast.error(`Failed to cancel job: ${error.message}`);
      },
    });
  };

  // Check if job can be cancelled
  const canCancelJob = (state: JobState): boolean => {
    return state === "pending" || state === "running";
  };

  // Sort jobs by created date (newest first)
  const sortedJobs = data?.jobs
    ? [...data.jobs].sort(
        (a, b) =>
          new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
      )
    : [];

  return (
    <div className="container mx-auto px-6 py-8">
      {/* Page Header */}
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-foreground mb-2">Jobs</h1>
        <p className="text-muted-foreground">
          Monitor and manage background jobs for library indexing and reconciliation
        </p>
      </div>

      {/* Actions Bar */}
      <div className="mb-6 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                disabled={triggerJob.isPending}
                className="flex items-center gap-2"
              >
                {triggerJob.isPending ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <Plus className="h-4 w-4" />
                )}
                Trigger Job
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start">
              <DropdownMenuItem onClick={() => handleTriggerJob("MovieIndex")}>
                Movie Index
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => handleTriggerJob("MovieReconcile")}>
                Movie Reconcile
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => handleTriggerJob("SeriesIndex")}>
                Series Index
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => handleTriggerJob("SeriesReconcile")}>
                Series Reconcile
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>

        {data && data.jobs.length > 0 && (
          <div className="text-sm text-muted-foreground">
            {data.count} {data.count === 1 ? "job" : "jobs"} total
          </div>
        )}
      </div>

      {/* Loading State */}
      {isLoading && (
        <Card>
          <CardContent className="flex items-center justify-center py-12">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </CardContent>
        </Card>
      )}

      {/* Error State */}
      {error && (
        <Card className="border-destructive">
          <CardContent className="flex flex-col items-center justify-center py-12 gap-4">
            <AlertCircle className="h-12 w-12 text-destructive" />
            <div className="text-center">
              <p className="font-semibold text-destructive mb-1">
                Failed to load jobs
              </p>
              <p className="text-sm text-muted-foreground mb-4">
                {error.message}
              </p>
              <Button onClick={() => refetch()} variant="outline" size="sm">
                <RefreshCw className="h-4 w-4 mr-2" />
                Try Again
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Empty State */}
      {!isLoading && !error && sortedJobs.length === 0 && (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <Clock className="h-12 w-12 text-muted-foreground mb-4" />
            <p className="text-lg font-semibold text-foreground mb-2">
              No jobs yet
            </p>
            <p className="text-sm text-muted-foreground">
              Trigger a job to see it appear here
            </p>
          </CardContent>
        </Card>
      )}

      {/* Jobs Table */}
      {!isLoading && !error && sortedJobs.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Job History</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-[80px]">ID</TableHead>
                    <TableHead>Type</TableHead>
                    <TableHead>State</TableHead>
                    <TableHead>Created</TableHead>
                    <TableHead>Updated</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {sortedJobs.map((job) => (
                    <TableRow key={job.id}>
                      <TableCell className="font-mono text-sm">
                        {job.id}
                      </TableCell>
                      <TableCell className="font-medium">
                        {formatJobType(job.type)}
                      </TableCell>
                      <TableCell>
                        <JobStateBadge state={job.state} />
                        {job.error && (
                          <div className="mt-1 text-xs text-destructive">
                            {job.error}
                          </div>
                        )}
                      </TableCell>
                      <TableCell className="text-sm text-muted-foreground">
                        <span title={new Date(job.createdAt).toLocaleString()}>
                          {formatTimestamp(job.createdAt)}
                        </span>
                      </TableCell>
                      <TableCell className="text-sm text-muted-foreground">
                        <span title={new Date(job.updatedAt).toLocaleString()}>
                          {formatTimestamp(job.updatedAt)}
                        </span>
                      </TableCell>
                      <TableCell className="text-right">
                        {canCancelJob(job.state) && (
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => handleCancelJob(job)}
                            disabled={cancelJob.isPending}
                            className="text-destructive hover:text-destructive"
                          >
                            {cancelJob.isPending ? (
                              <Loader2 className="h-4 w-4 animate-spin" />
                            ) : (
                              "Cancel"
                            )}
                          </Button>
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
