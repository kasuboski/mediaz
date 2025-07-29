import { useEffect, useState } from "react";
import { MediaGrid } from "@/components/media/MediaGrid";
import { LoadingSpinner } from "@/components/ui/LoadingSpinner";

// Mock library movies data
const mockLibraryMovies = [
  {
    id: 550,
    title: "Fight Club",
    poster_path: "/pB8BM7pdSp6B6Ih7QZ4DrQ3PmJK.jpg",
    release_date: "1999-10-15",
    media_type: "movie" as const,
  },
  {
    id: 13,
    title: "Forrest Gump",
    poster_path: "/arw2vcBveWOVZr6pxd9XTd1TdQa.jpg",
    release_date: "1994-06-23", 
    media_type: "movie" as const,
  },
  {
    id: 155,
    title: "The Dark Knight",
    poster_path: "/qJ2tW6WMUDux911r6m7haRef0WH.jpg",
    release_date: "2008-07-16",
    media_type: "movie" as const,
  },
];

export default function Movies() {
  const [movies, setMovies] = useState<any[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    // Simulate API call
    setTimeout(() => {
      setMovies(mockLibraryMovies);
      setIsLoading(false);
    }, 500);
  }, []);

  return (
    <div className="container mx-auto px-6 py-8">
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-foreground mb-2">Movie Library</h1>
        <p className="text-muted-foreground">
          {isLoading ? "Loading..." : `${movies.length} movies in your library`}
        </p>
      </div>

      {isLoading ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner size="lg" />
        </div>
      ) : (
        <MediaGrid items={movies} />
      )}
    </div>
  );
}