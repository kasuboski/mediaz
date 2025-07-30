import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { MediaGrid } from "@/components/media/MediaGrid";
import { LoadingSpinner } from "@/components/ui/LoadingSpinner";

// Mock data for development - will be replaced with API calls
const mockSearchResults = [
  {
    id: 550,
    title: "Fight Club",
    poster_path: "/pB8BM7pdSp6B6Ih7QZ4DrQ3PmJK.jpg",
    release_date: "1999-10-15",
    media_type: "movie",
  },
  {
    id: 13,
    title: "Forrest Gump", 
    poster_path: "/arw2vcBveWOVZr6pxd9XTd1TdQa.jpg",
    release_date: "1994-06-23",
    media_type: "movie",
  },
  {
    id: 155,
    title: "The Dark Knight",
    poster_path: "/qJ2tW6WMUDux911r6m7haRef0WH.jpg", 
    release_date: "2008-07-16",
    media_type: "movie",
  },
  {
    id: 1399,
    title: "Game of Thrones",
    poster_path: "/1XS1oqL89opfnbLl8WnZY1O1uJx.jpg",
    first_air_date: "2011-04-17",
    media_type: "tv",
  },
];

export default function Discover() {
  const [searchParams] = useSearchParams();
  const [searchResults, setSearchResults] = useState<any[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const query = searchParams.get("query");

  useEffect(() => {
    if (query) {
      setIsLoading(true);
      // Simulate API call delay
      setTimeout(() => {
        setSearchResults(mockSearchResults);
        setIsLoading(false);
      }, 800);
    } else {
      setSearchResults([]);
    }
  }, [query]);

  if (!query) {
    return (
      <div className="container mx-auto px-6 py-8">
        <div className="text-center py-16">
          <h1 className="text-3xl font-bold text-foreground mb-4">
            Discover Movies & TV Shows
          </h1>
          <p className="text-muted-foreground text-lg">
            Search for your favorite content using the search bar above
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="container mx-auto px-6 py-8">
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-foreground mb-2">
          Search Results for "{query}"
        </h1>
        <p className="text-muted-foreground">
          {isLoading ? "Searching..." : `Found ${searchResults.length} results`}
        </p>
      </div>

      {isLoading ? (
        <div className="flex justify-center py-16">
          <LoadingSpinner size="lg" />
        </div>
      ) : (
        <MediaGrid items={searchResults} />
      )}
    </div>
  );
}