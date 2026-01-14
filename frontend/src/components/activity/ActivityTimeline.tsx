import { useMemo, useState, useEffect, useRef } from "react";
import { format as formatDateFn } from "date-fns";
import { Film, Tv, Briefcase, Activity } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from "recharts";
import type { TimelineResponse, TransitionItem } from "@/lib/api";

interface ActivityTimelineProps {
  data: TimelineResponse | null;
  isLoading?: boolean;
  page: number;
  onPageChange: (page: number) => void;
}

type ChartView = "overview" | "movies" | "series";

export function ActivityTimeline({ data, isLoading, page, onPageChange }: ActivityTimelineProps) {
  const savedViewRef = useRef<ChartView>("overview");
  const [chartView, setChartView] = useState<ChartView>(() => savedViewRef.current);
  const PAGE_SIZE = 20;

  useEffect(() => {
    savedViewRef.current = chartView;
  }, [chartView]);

  const chartData = useMemo(() => {
    if (!data?.timeline) return [];
    return data.timeline.map((entry) => ({
      date: formatDate(entry.date),
      fullDate: formatDetailedDate(entry.date),
      downloaded: entry.movies.downloaded || 0,
      downloading: entry.movies.downloading || 0,
      completed: entry.series.completed || 0,
      downloadingSeries: entry.series.downloading || 0,
      done: entry.jobs.done || 0,
      error: entry.jobs.error || 0,
    }));
  }, [data]);

  const transitionsByDate = useMemo(() => {
    if (!data?.transitions) return {};
    return data.transitions.reduce((acc, transition) => {
      const date = formatDetailedDate(transition.createdAt);
      if (!acc[date]) {
        acc[date] = [];
      }
      acc[date].push(transition);
      return acc;
    }, {} as Record<string, TransitionItem[]>);
  }, [data]);

  const sortedDates = useMemo(() => {
    return Object.keys(transitionsByDate).sort((a, b) => new Date(b).getTime() - new Date(a).getTime());
  }, [transitionsByDate]);

  if (isLoading || !data) {
    return (
      <Card className="border-border/50 shadow-card">
        <CardHeader>
          <CardTitle className="text-xl font-semibold">Activity Timeline</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-center h-64">
            <Activity className="h-8 w-8 animate-pulse text-muted-foreground" />
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="border-border/50 shadow-card">
      <CardHeader>
        <CardTitle className="text-xl font-semibold">Activity Timeline</CardTitle>
      </CardHeader>
      <CardContent className="space-y-6">
        <Tabs value={chartView} onValueChange={(v) => setChartView(v as ChartView)}>
          <TabsList className="grid w-full grid-cols-3">
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="movies">Movies</TabsTrigger>
            <TabsTrigger value="series">Series</TabsTrigger>
          </TabsList>

          <TabsContent value="overview" className="mt-4">
            <ResponsiveContainer width="100%" height={300}>
              <LineChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
                <XAxis dataKey="date" stroke="hsl(var(--muted-foreground))" />
                <YAxis stroke="hsl(var(--muted-foreground))" />
                <Tooltip content={<CustomTooltip />} />
                <Legend />
                <Line type="monotone" dataKey="downloaded" stroke="#22c55e" strokeWidth={2} name="Movies Downloaded" />
                <Line type="monotone" dataKey="completed" stroke="#3b82f6" strokeWidth={2} name="Series Completed" />
                <Line type="monotone" dataKey="done" stroke="#a855f7" strokeWidth={2} name="Jobs Done" />
              </LineChart>
            </ResponsiveContainer>
          </TabsContent>

          <TabsContent value="movies" className="mt-4">
            <ResponsiveContainer width="100%" height={300}>
              <LineChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
                <XAxis dataKey="date" stroke="hsl(var(--muted-foreground))" />
                <YAxis stroke="hsl(var(--muted-foreground))" />
                <Tooltip content={<CustomTooltip />} />
                <Legend />
                <Line type="monotone" dataKey="downloaded" stroke="#22c55e" strokeWidth={2} name="Downloaded" />
                <Line type="monotone" dataKey="downloading" stroke="#3b82f6" strokeWidth={2} name="Downloading" />
              </LineChart>
            </ResponsiveContainer>
          </TabsContent>

          <TabsContent value="series" className="mt-4">
            <ResponsiveContainer width="100%" height={300}>
              <LineChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
                <XAxis dataKey="date" stroke="hsl(var(--muted-foreground))" />
                <YAxis stroke="hsl(var(--muted-foreground))" />
                <Tooltip content={<CustomTooltip />} />
                <Legend />
                <Line type="monotone" dataKey="completed" stroke="#22c55e" strokeWidth={2} name="Completed" />
                <Line type="monotone" dataKey="downloadingSeries" stroke="#3b82f6" strokeWidth={2} name="Downloading" />
              </LineChart>
            </ResponsiveContainer>
          </TabsContent>
        </Tabs>

        <div className="border-t border-border/50 pt-6">
          <h3 className="text-lg font-semibold mb-4">Detailed Transitions</h3>
          {sortedDates.length === 0 ? (
            <EmptyState />
          ) : (
            sortedDates.map((date) => (
              <TransitionGroup key={date} date={date} transitions={transitionsByDate[date]} />
            ))
          )}
        </div>

        {data?.count && data.count > PAGE_SIZE && (
          <div className="flex items-center justify-between border-t border-border/50 pt-4">
            <span className="text-sm text-muted-foreground">
              Showing {((page - 1) * PAGE_SIZE) + (data.items?.length ?? Math.min(PAGE_SIZE, Math.max(0, data.count - (page - 1) * PAGE_SIZE)))} of {data.count}
            </span>
            <div className="flex gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => onPageChange(page - 1)}
                disabled={page === 1}
              >
                Previous
              </Button>
              <span className="flex items-center px-3 text-sm">
                Page {page}
              </span>
              <Button
                variant="outline"
                size="sm"
                onClick={() => onPageChange(page + 1)}
                disabled={page * PAGE_SIZE >= data.count}
              >
                Next
              </Button>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function CustomTooltip({ active, payload, label }: any) {
  if (!active || !payload) return null;
  return (
    <div className="rounded-lg border border-border/50 bg-background px-2.5 py-1.5 text-xs shadow-xl">
      <div className="font-medium mb-1">{label}</div>
      {payload.map((entry: any, index: number) => (
        <div key={index} className="flex items-center gap-2">
          <div
            className="h-2 w-2 rounded-[2px]"
            style={{ backgroundColor: entry.color }}
          />
          <span className="text-muted-foreground">{entry.name}:</span>
          <span className="font-mono font-medium">{entry.value}</span>
        </div>
      ))}
    </div>
  );
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <Activity className="h-12 w-12 text-muted-foreground mb-3" />
      <p className="text-muted-foreground text-sm">No activity recorded in the selected time range.</p>
    </div>
  );
}

function TransitionGroup({ date, transitions }: { date: string; transitions: TransitionItem[] }) {
  return (
    <div className="mb-6">
      <h4 className="text-sm font-semibold text-muted-foreground mb-3">{date}</h4>
      <div className="space-y-3">
        {transitions.map((transition) => (
          <TransitionItemCard key={`${transition.entityType}-${transition.id}`} transition={transition} />
        ))}
      </div>
    </div>
  );
}

function TransitionItemCard({ transition }: { transition: TransitionItem }) {
  const icon = getEntityIcon(transition.entityType);
  const stateColor = getStateColor(transition.toState);

  return (
    <div className="flex items-start gap-3 p-3 rounded-lg bg-muted/20 border border-border/30">
      <div className="mt-0.5 text-muted-foreground">
        {icon}
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 mb-1">
          <Badge variant="outline" className="text-xs">
            {transition.entityType}
          </Badge>
          <span className="text-sm font-medium truncate">{transition.entityTitle}</span>
        </div>
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <span className={transition.fromState ? "line-through opacity-60" : ""}>
            {transition.fromState || "initial"}
          </span>
          <span>→</span>
          <span className={`font-medium ${stateColor}`}>
            {transition.toState}
          </span>
        </div>
      </div>
      <div className="text-xs text-muted-foreground whitespace-nowrap">
        {formatDateTime(transition.createdAt)}
      </div>
    </div>
  );
}

function getEntityIcon(entityType: string) {
  switch (entityType) {
    case "movie":
      return <Film className="h-4 w-4" />;
    case "series":
      return <Tv className="h-4 w-4" />;
    default:
      return <Briefcase className="h-4 w-4" />;
  }
}

function getStateColor(state: string): string {
  if (state === "downloaded" || state === "completed" || state === "done") {
    return "text-green-500";
  }
  if (state === "downloading" || state === "running" || state === "pending") {
    return "text-blue-500";
  }
  if (state === "error" || state === "failed") {
    return "text-red-500";
  }
  return "text-muted-foreground";
}

function formatDate(dateStr: string): string {
  return formatDateFn(new Date(dateStr), "MMM d");
}

function formatDetailedDate(dateStr: string): string {
  return formatDateFn(new Date(dateStr), "MMM d, yyyy");
}

function formatDateTime(dateStr: string): string {
  return formatDateFn(new Date(dateStr), "MMM d, yyyy h:mm a");
}
