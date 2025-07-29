import { useEffect, useState } from "react";
import { MediaGrid } from "@/components/media/MediaGrid";
import { LoadingSpinner } from "@/components/ui/LoadingSpinner";

// Mock library TV shows data
const mockLibrarySeries = [
  {
    id: 1399,
    title: "Game of Thrones",
    poster_path: "/1XS1oqL89opfnbLl8WnZY1O1uJx.jpg",
    first_air_date: "2011-04-17",
    media_type: "tv" as const,
  },
  {
    id: 1396,
    title: "Breaking Bad",
    poster_path: "/ztkUQFLlC19CCMYHW9o1zWhJRNq.jpg",
    first_air_date: "2008-01-20",
    media_type: "tv" as const,
  },
  {
    id: 60735,
    title: "The Flash",
    poster_path: "/lJA2RCMfsWoskqlQhXPSLFQGXEJ.jpg",
    first_air_date: "2014-10-07",
    media_type: "tv" as const,
  },
];

export default function Series() {
  const [series, setSeries] = useState<any[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    // Simulate API call
    setTimeout(() => {
      setSeries(mockLibrarySeries);
      setIsLoading(false);
    }, 500);
  }, []);

  return (
    <div className="container mx-auto px-6 py-8">
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-foreground mb-2">TV Show Library</h1>
        <p className="text-muted-foreground">
          {isLoading ? "Loading..." : `${series.length} TV shows in your library`}
        </p>
      </div>

      {isLoading ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner size="lg" />
        </div>
      ) : (
        <MediaGrid items={series} />
      )}
    </div>
  );
}