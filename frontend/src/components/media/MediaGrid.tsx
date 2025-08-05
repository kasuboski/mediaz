import { MediaCard } from "./MediaCard";

interface MediaItem {
  id: number;
  title: string;
  poster_path: string;
  release_date?: string;
  first_air_date?: string;
  media_type: "movie" | "tv";
}

interface MediaGridProps {
  items: MediaItem[];
}

export function MediaGrid({ items }: MediaGridProps) {
  if (items.length === 0) {
    return (
      <div className="text-center py-16">
        <p className="text-muted-foreground text-lg">No results found</p>
      </div>
    );
  }

  return (
    <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 2xl:grid-cols-7 gap-4">
      {items.map((item, idx) => (
        <MediaCard key={`${item.media_type}-${item.id || item.poster_path || idx}`} item={item} />
      ))}
    </div>
  );
}