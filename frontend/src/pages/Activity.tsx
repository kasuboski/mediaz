import { useState, useCallback } from "react";
import { useActiveActivity, useRecentFailures, useActivityTimeline } from "@/lib/queries";
import { usePagination } from "@/hooks/use-pagination";
import { type ActiveActivityResponse, type FailureItem } from "@/lib/api";
import { Button } from "@/components/ui/button";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ChevronDown, ChevronUp, RefreshCw, Activity as ActivityIcon, Loader2 } from "lucide-react";
import { ActiveProcessesPanel } from "@/components/activity/ActiveProcessesPanel";
import { RecentFailuresList } from "@/components/activity/RecentFailuresList";
import { ActivityTimeline } from "@/components/activity/ActivityTimeline";
import { TimeRangeSelector } from "@/components/activity/TimeRangeSelector";

function formatRelativeTime(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSec = Math.floor(diffMs / 1000);
  const diffMin = Math.floor(diffSec / 60);
  const diffHour = Math.floor(diffMin / 60);
  const diffDay = Math.floor(diffHour / 24);

  if (diffSec < 60) return `${diffSec}s ago`;
  if (diffMin < 60) return `${diffMin}m ago`;
  if (diffHour < 24) return `${diffHour}h ago`;
  return `${diffDay}d ago`;
}

interface ActiveProcessesSectionProps {
  activeData: ActiveActivityResponse | null;
  isLoading: boolean;
  pages: { movies: number; series: number; jobs: number };
  pageSize: number;
  onPageChange: (group: "movies" | "series" | "jobs", page: number) => void;
  onPageSizeChange: (size: number) => void;
}

function ActiveProcessesSection({ activeData, isLoading, pages, pageSize, onPageChange, onPageSizeChange }: ActiveProcessesSectionProps) {
  const [isOpen, setIsOpen] = useState(true);

  return (
    <Collapsible open={isOpen} onOpenChange={setIsOpen}>
      <Card>
        <CardHeader>
          <CollapsibleTrigger className="flex items-center justify-between w-full">
            <div className="flex items-center gap-3">
              <ActivityIcon className="h-5 w-5 text-blue-400" />
              <CardTitle className="text-xl">Active Processes</CardTitle>
            </div>
            <div className="flex items-center gap-4">
              {isLoading ? (
                <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
              ) : (
                <span className="text-sm text-muted-foreground">
                  {activeData?.counts.total ?? 0} active
                </span>
              )}
              {isOpen ? <ChevronUp className="h-5 w-5 text-muted-foreground" /> : <ChevronDown className="h-5 w-5 text-muted-foreground" />}
            </div>
          </CollapsibleTrigger>
        </CardHeader>
        <CollapsibleContent>
          <CardContent>
            {isLoading ? (
              <div className="flex items-center justify-center py-8">
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
              </div>
            ) : (
              <ActiveProcessesPanel data={activeData} pages={pages} pageSize={pageSize} onPageChange={onPageChange} onPageSizeChange={onPageSizeChange} />
            )}
          </CardContent>
        </CollapsibleContent>
      </Card>
    </Collapsible>
  );
}

interface RecentFailuresSectionProps {
  failuresData: { failures: FailureItem[]; pagination: { page: number; pageSize: number; totalItems: number; totalPages: number } } | null;
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  page: number;
  pageSize: number;
  onPageChange: (page: number) => void;
  onPageSizeChange: (size: number) => void;
}

function RecentFailuresSection({ failuresData, isLoading, isError, onRetry, page, pageSize, onPageChange, onPageSizeChange }: RecentFailuresSectionProps) {
  const [isOpen, setIsOpen] = useState(false);

  if (!isError && !failuresData?.pagination.totalItems && !isLoading) return null;

  return (
    <Collapsible open={isOpen} onOpenChange={setIsOpen}>
      <Card>
        <CardHeader>
          <CollapsibleTrigger className="flex items-center justify-between w-full">
            <div className="flex items-center gap-3">
              <span className="text-2xl">⚠️</span>
              <CardTitle className="text-xl">Recent Failures</CardTitle>
            </div>
            <div className="flex items-center gap-4">
              {isLoading ? (
                <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
              ) : (
                <span className="text-sm text-muted-foreground">
                  {failuresData?.pagination.totalItems ?? 0} failures
                </span>
              )}
              {isOpen ? <ChevronUp className="h-5 w-5 text-muted-foreground" /> : <ChevronDown className="h-5 w-5 text-muted-foreground" />}
            </div>
          </CollapsibleTrigger>
        </CardHeader>
        <CollapsibleContent>
          <CardContent>
            {isLoading ? (
              <div className="flex items-center justify-center py-8">
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
              </div>
            ) : isError ? (
              <div className="flex items-center justify-between gap-4">
                <p className="text-sm text-destructive">Could not load recent failures.</p>
                <Button variant="outline" size="sm" onClick={onRetry}>Retry</Button>
              </div>
            ) : failuresData ? (
              <RecentFailuresList data={failuresData} hours={24} page={page} pageSize={pageSize} onPageChange={onPageChange} onPageSizeChange={onPageSizeChange} />
            ) : null}
          </CardContent>
        </CollapsibleContent>
      </Card>
    </Collapsible>
  );
}

interface ActivityTimelineSectionProps {
  timelineData: any;
  isLoading: boolean;
  selectedDays: number;
  onDaysChange: (days: number) => void;
  page: number;
  onPageChange: (page: number) => void;
}

function ActivityTimelineSection({ timelineData, isLoading, selectedDays, onDaysChange, page, onPageChange }: ActivityTimelineSectionProps) {
  const [isOpen, setIsOpen] = useState(false);

  return (
    <Collapsible open={isOpen} onOpenChange={setIsOpen}>
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CollapsibleTrigger className="flex items-center gap-3">
              <span className="text-2xl">📊</span>
              <CardTitle className="text-xl">Activity Timeline</CardTitle>
              {isOpen ? <ChevronUp className="h-5 w-5 text-muted-foreground" /> : <ChevronDown className="h-5 w-5 text-muted-foreground" />}
            </CollapsibleTrigger>
            {isOpen && (
              <TimeRangeSelector
                days={selectedDays}
                onChange={onDaysChange}
              />
            )}
          </div>
        </CardHeader>
        <CollapsibleContent>
          <CardContent>
            {isLoading ? (
              <div className="flex items-center justify-center py-8">
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
              </div>
            ) : (
              <ActivityTimeline data={timelineData} page={page} onPageChange={onPageChange} />
            )}
          </CardContent>
        </CollapsibleContent>
      </Card>
    </Collapsible>
  );
}

function ActivityPageSkeleton() {
  return (
    <div className="container mx-auto px-6 py-8 space-y-6">
      <div className="space-y-2">
        <div className="h-8 w-48 bg-muted animate-pulse rounded" />
        <div className="h-4 w-96 bg-muted animate-pulse rounded" />
      </div>
      <Card>
        <CardHeader>
          <div className="h-6 w-48 bg-muted animate-pulse rounded" />
        </CardHeader>
        <CardContent>
          <div className="h-32 bg-muted animate-pulse rounded" />
        </CardContent>
      </Card>
    </div>
  );
}

export default function Activity() {
  const [lastUpdated, setLastUpdated] = useState<Date>(new Date());
  const [selectedDays, setSelectedDays] = useState<number>(1);
  const [page, setPage] = useState<number>(1);
  const [activeMoviesPage, setActiveMoviesPage] = useState(1);
  const [activeSeriesPage, setActiveSeriesPage] = useState(1);
  const [activeJobsPage, setActiveJobsPage] = useState(1);
  const [activePageSize, setActivePageSize] = useState(50);
  const failuresPagination = usePagination({ defaultPageSize: 50 });

  const {
    data: activeData,
    isLoading: activeLoading,
    refetch: refetchActive,
  } = useActiveActivity({ moviesPage: activeMoviesPage, seriesPage: activeSeriesPage, jobsPage: activeJobsPage, pageSize: activePageSize });

  const {
    data: failuresData,
    isLoading: failuresLoading,
    isError: failuresError,
    refetch: refetchFailures,
  } = useRecentFailures(24, { page: failuresPagination.page, pageSize: failuresPagination.pageSize });

  const {
    data: timelineData,
    isLoading: timelineLoading,
    refetch: refetchTimeline,
  } = useActivityTimeline(selectedDays, page, 20);

  const handleRefresh = useCallback(() => {
    refetchActive();
    refetchFailures();
    refetchTimeline();
    setLastUpdated(new Date());
  }, [refetchActive, refetchFailures, refetchTimeline]);

  const handleDaysChange = useCallback((days: number) => {
    setSelectedDays(days);
    setPage(1);
  }, []);

  const isLoading = activeLoading || failuresLoading || timelineLoading;

  // Only show skeleton if we have no data at all
  const showSkeleton = isLoading && !activeData && !failuresData && !timelineData;

  return (
    <div className="container mx-auto px-6 py-8">
      <div className="mb-8 flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-foreground mb-2">Activity</h1>
          <p className="text-muted-foreground">
            Monitor downloads, jobs, and system activity
          </p>
        </div>
        <div className="flex items-center gap-4">
          <span className="text-sm text-muted-foreground">
            Updated {formatRelativeTime(lastUpdated.toISOString())}
          </span>
          <Button
            variant="outline"
            size="sm"
            onClick={handleRefresh}
            disabled={isLoading}
          >
            <RefreshCw className={`h-4 w-4 ${isLoading ? "animate-spin" : ""}`} />
          </Button>
        </div>
      </div>

      {showSkeleton ? (
        <ActivityPageSkeleton />
      ) : (
        <div className="space-y-6">
          <ActiveProcessesSection
            activeData={activeData}
            isLoading={activeLoading}
            pages={{ movies: activeMoviesPage, series: activeSeriesPage, jobs: activeJobsPage }}
            pageSize={activePageSize}
            onPageChange={(group, nextPage) => ({ movies: setActiveMoviesPage, series: setActiveSeriesPage, jobs: setActiveJobsPage })[group](nextPage)}
            onPageSizeChange={(size) => { setActiveMoviesPage(1); setActiveSeriesPage(1); setActiveJobsPage(1); setActivePageSize(size); }}
          />
          <RecentFailuresSection
            failuresData={failuresData}
            isLoading={failuresLoading}
            isError={failuresError}
            onRetry={() => { void refetchFailures(); }}
            page={failuresPagination.page}
            pageSize={failuresPagination.pageSize}
            onPageChange={failuresPagination.setPage}
            onPageSizeChange={failuresPagination.setPageSize}
          />
          <ActivityTimelineSection
            timelineData={timelineData}
            isLoading={timelineLoading}
            selectedDays={selectedDays}
            onDaysChange={handleDaysChange}
            page={page}
            onPageChange={setPage}
          />
        </div>
      )}
    </div>
  );
}
