import { useJobs, useTriggerJob, useCancelJob, useConfig } from "@/lib/queries";
import { type Job, type JobType, type JobState } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
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
  RefreshCw,
} from "lucide-react";
import { toast } from "sonner";
import { formatDistanceToNow } from "date-fns";
import { usePagination } from "@/hooks/use-pagination";
import { PageSizeSelector } from "@/components/ui/page-size-selector";
import { PaginationControls } from "@/components/ui/pagination-controls";

/**
 * Get badge variant and icon for job state
 */
function getJobStateBadge(state: JobState) {
  switch (state) {
    case "pending":
      return {
        variant: "outline" as const,
        icon: <Clock className="h-3 w-3" />,
        className: "bg-yellow-500/10 text-yellow-500 border-yellow-500/20",
      };
    case "running":
      return {
        variant: "secondary" as const,
        icon: <Loader2 className="h-3 w-3 animate-spin" />,
        className: "bg-blue-500/10 text-blue-500 border-blue-500/20",
      };
    case "done":
      return {
        variant: "outline" as const,
        icon: <CheckCircle className="h-3 w-3" />,
        className: "bg-green-500/10 text-green-500 border-green-500/20",
      };
    case "error":
      return {
        variant: "outline" as const,
        icon: <AlertCircle className="h-3 w-3" />,
        className: "bg-red-500/10 text-red-500 border-red-500/20",
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
    <Badge variant={variant} className={`flex items-center gap-1.5 pointer-events-none ${className}`}>
      {icon}
      <span className="capitalize">{state || "new"}</span>
    </Badge>
  );
}

/**
 * Jobs page component for managing background jobs
 */
export default function Jobs() {
  const pagination = usePagination({
    defaultPageSize: 10,
    defaultPage: 1,
    storageKey: 'jobs-pagination',
  });

  const { data, isLoading, error, refetch } = useJobs(
    pagination.pageSize > 0
      ? { page: pagination.page, pageSize: pagination.pageSize }
      : undefined
  );
  const { data: config } = useConfig();
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

  const jobs = data?.jobs || [];

  return (
    <div className="container mx-auto px-6 py-8">
      {/* Page Header */}
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-foreground mb-2">Jobs</h1>
        <p className="text-muted-foreground">
          Monitor and manage background jobs for library indexing and reconciliation
        </p>
      </div>

      {/* Job Schedules */}
      {config?.jobs && (
        <Card className="mb-6">
          <CardHeader>
            <CardTitle>Schedule</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Job Type</TableHead>
                  <TableHead>Schedule</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow>
                  <TableCell className="font-medium">Movie Index</TableCell>
                  <TableCell className="font-semibold text-blue-400">
                    {config.jobs.movieIndex}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button
                      size="sm"
                      onClick={() => handleTriggerJob("MovieIndex")}
                      disabled={triggerJob.isPending}
                      className="bg-gradient-primary hover:opacity-90"
                    >
                      {triggerJob.isPending ? (
                        <Loader2 className="h-4 w-4 animate-spin" />
                      ) : (
                        "Run"
                      )}
                    </Button>
                  </TableCell>
                </TableRow>
                <TableRow>
                  <TableCell className="font-medium">Movie Reconcile</TableCell>
                  <TableCell className="font-semibold text-blue-400">
                    {config.jobs.movieReconcile}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button
                      size="sm"
                      onClick={() => handleTriggerJob("MovieReconcile")}
                      disabled={triggerJob.isPending}
                      className="bg-gradient-primary hover:opacity-90"
                    >
                      {triggerJob.isPending ? (
                        <Loader2 className="h-4 w-4 animate-spin" />
                      ) : (
                        "Run"
                      )}
                    </Button>
                  </TableCell>
                </TableRow>
                <TableRow>
                  <TableCell className="font-medium">Series Index</TableCell>
                  <TableCell className="font-semibold text-orange-400">
                    {config.jobs.seriesIndex}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button
                      size="sm"
                      onClick={() => handleTriggerJob("SeriesIndex")}
                      disabled={triggerJob.isPending}
                      className="bg-gradient-primary hover:opacity-90"
                    >
                      {triggerJob.isPending ? (
                        <Loader2 className="h-4 w-4 animate-spin" />
                      ) : (
                        "Run"
                      )}
                    </Button>
                  </TableCell>
                </TableRow>
                <TableRow>
                  <TableCell className="font-medium">Series Reconcile</TableCell>
                  <TableCell className="font-semibold text-orange-400">
                    {config.jobs.seriesReconcile}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button
                      size="sm"
                      onClick={() => handleTriggerJob("SeriesReconcile")}
                      disabled={triggerJob.isPending}
                      className="bg-gradient-primary hover:opacity-90"
                    >
                      {triggerJob.isPending ? (
                        <Loader2 className="h-4 w-4 animate-spin" />
                      ) : (
                        "Run"
                      )}
                    </Button>
                  </TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}

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
      {!isLoading && !error && jobs.length === 0 && (
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
      {!isLoading && !error && jobs.length > 0 && (
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle>History</CardTitle>
            <PageSizeSelector
              value={pagination.pageSize}
              onChange={pagination.setPageSize}
              options={[10, 25, 50, 100]}
            />
          </CardHeader>
          <CardContent>
            {data?.pagination && (
              <div className="mb-4 text-sm text-muted-foreground">
                Showing {((data.pagination.page - 1) * data.pagination.pageSize) + 1}-{Math.min(data.pagination.page * data.pagination.pageSize, data.pagination.totalItems)} of {data.pagination.totalItems} jobs
              </div>
            )}
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Job Type</TableHead>
                    <TableHead>State</TableHead>
                    <TableHead>Created</TableHead>
                    <TableHead>Updated</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {jobs.map((job) => (
                    <TableRow key={job.id}>
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
                            variant="destructive"
                            size="sm"
                            onClick={() => handleCancelJob(job)}
                            disabled={cancelJob.isPending}
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

            {data?.pagination && data.pagination.totalPages > 1 && (
              <div className="mt-4 flex justify-center">
                <PaginationControls
                  currentPage={data.pagination.page}
                  totalPages={data.pagination.totalPages}
                  onPageChange={pagination.setPage}
                />
              </div>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
