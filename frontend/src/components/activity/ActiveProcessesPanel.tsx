import { ActiveMovieCard } from "./ActiveMovieCard";
import { ActiveSeriesCard } from "./ActiveSeriesCard";
import { ActiveJobCard } from "./ActiveJobCard";
import type { ActiveActivityResponse } from "@/lib/api";
import { Film, Tv, Briefcase } from "lucide-react";
import { PageSizeSelector } from "@/components/ui/page-size-selector";
import { PaginationControls } from "@/components/ui/pagination-controls";

interface ActiveProcessesPanelProps {
  data: ActiveActivityResponse | null;
  isLoading?: boolean;
  pages: { movies: number; series: number; jobs: number };
  pageSize: number;
  onPageChange: (group: "movies" | "series" | "jobs", page: number) => void;
  onPageSizeChange: (size: number) => void;
}

export function ActiveProcessesPanel({ data, isLoading, pages, pageSize, onPageChange, onPageSizeChange }: ActiveProcessesPanelProps) {
  const totalActive = data?.counts.total ?? 0;
  const groups = [
    { key: "movies" as const, title: "Downloading Movies", icon: Film, items: data?.movies ?? [], count: data?.counts.movies ?? 0, meta: data?.pagination.movies },
    { key: "series" as const, title: "Downloading Series", icon: Tv, items: data?.series ?? [], count: data?.counts.series ?? 0, meta: data?.pagination.series },
    { key: "jobs" as const, title: "Active Jobs", icon: Briefcase, items: data?.jobs ?? [], count: data?.counts.jobs ?? 0, meta: data?.pagination.jobs },
  ];

  if (isLoading) return <div className="space-y-6">{[1, 2, 3].map(i => <div key={i} className="h-24 rounded bg-muted/30 animate-pulse" />)}</div>;
  if (totalActive === 0) return <div className="py-16 text-center"><h3 className="text-lg font-medium">No Active Processes</h3><p className="mt-2 text-sm text-muted-foreground">There are currently no downloads or jobs running. Check back later for activity.</p></div>;

  return <div className="space-y-6">
    <div className="flex justify-end"><PageSizeSelector value={pageSize} onChange={onPageSizeChange} options={[25, 50, 100]} /></div>
    {groups.filter(group => group.count > 0).map(group => {
      const Icon = group.icon;
      const cards = group.key === "movies"
        ? group.items.map(item => <ActiveMovieCard key={item.id} movie={item} />)
        : group.key === "series"
          ? group.items.map(item => <ActiveSeriesCard key={item.id} series={item} />)
          : group.items.map(item => <ActiveJobCard key={item.id} job={item} />);
      return <section key={group.key}>
        <div className="mb-4 flex items-center gap-3"><Icon className="h-5 w-5 text-primary" /><h2 className="text-lg font-semibold text-card-foreground">{group.title}</h2><span className="text-sm text-muted-foreground">({group.count})</span></div>
        {cards.length ? <div className={group.key === "jobs" ? "grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4" : "grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4"}>{cards}</div> : <p className="py-6 text-sm text-muted-foreground">No items on this page.</p>}
        {group.meta.totalPages > 1 && <div className="mt-4 flex justify-end"><PaginationControls currentPage={pages[group.key]} totalPages={group.meta.totalPages} onPageChange={page => onPageChange(group.key, page)} /></div>}
      </section>;
    })}
  </div>;
}
