import { useNavigate } from "react-router-dom";
import { Film, Tv, Calendar } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

interface MediaItem {
  id: number;
  title: string;
  poster_path: string;
  release_date?: string;
  first_air_date?: string;
  media_type: "movie" | "tv";
}

interface MediaCardProps {
  item: MediaItem;
}

export function MediaCard({ item }: MediaCardProps) {
  const navigate = useNavigate();

  const handleClick = () => {
    navigate(`/${item.media_type}/${item.id}`);
  };

  const getYear = () => {
    const date = item.release_date || item.first_air_date;
    return date ? new Date(date).getFullYear() : null;
  };

  const imageUrl = item.poster_path
    ? `https://image.tmdb.org/t/p/w500${item.poster_path}`
    : "/placeholder.svg";

  return (
    <Card 
      className="group cursor-pointer bg-gradient-card border-border/50 shadow-card hover:shadow-card-hover transition-all duration-300 hover:-translate-y-1"
      onClick={handleClick}
    >
      <CardContent className="p-0">
        <div className="relative aspect-[2/3] overflow-hidden rounded-t-lg">
          <img
            src={imageUrl}
            alt={item.title}
            className="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
            loading="lazy"
          />
          
          {/* Media type badge */}
          <Badge 
            variant="secondary"
            className="absolute top-2 right-2 bg-background/80 backdrop-blur-sm text-xs"
          >
            {item.media_type === "movie" ? (
              <><Film className="h-3 w-3 mr-1" /> Movie</>
            ) : (
              <><Tv className="h-3 w-3 mr-1" /> TV</>
            )}
          </Badge>

          {/* Hover overlay */}
          <div className="absolute inset-0 bg-gradient-hero opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
        </div>

        <div className="p-3">
          <h3 className="font-medium text-card-foreground text-sm line-clamp-2 mb-1">
            {item.title}
          </h3>
          
          {getYear() && (
            <div className="flex items-center gap-1 text-xs text-muted-foreground">
              <Calendar className="h-3 w-3" />
              <span>{getYear()}</span>
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  );
}