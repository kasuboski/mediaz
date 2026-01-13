import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Progress } from "@/components/ui/progress";
import { Clock, Download, Server, Tv } from "lucide-react";
import type { ActiveSeries } from "@/lib/api";

interface ActiveSeriesCardProps {
  series: ActiveSeries;
}

export function ActiveSeriesCard({ series }: ActiveSeriesCardProps) {
  const imageUrl = series.poster_path
    ? `https://image.tmdb.org/t/p/w500${series.poster_path}`
    : "/placeholder.svg";

  return (
    <Card className="bg-gradient-card border-border/50 shadow-card hover:shadow-card-hover transition-all duration-300">
      <CardContent className="p-0">
        <div className="relative aspect-[2/3] overflow-hidden rounded-t-lg">
          <img
            src={imageUrl}
            alt={series.title}
            className="h-full w-full object-cover"
            loading="lazy"
          />

          <Badge
            variant="secondary"
            className="absolute top-2 right-2 bg-background/80 backdrop-blur-sm"
          >
            <Download className="h-3 w-3 mr-1" />
            Downloading
          </Badge>

          <Badge
            variant="secondary"
            className="absolute top-2 left-2 bg-background/80 backdrop-blur-sm"
          >
            <Tv className="h-3 w-3 mr-1" />
            S{series.currentEpisode.seasonNumber}E{series.currentEpisode.episodeNumber}
          </Badge>

          <div className="absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black/80 to-transparent p-3">
            <Progress value={45} className="h-1.5" />
          </div>
        </div>

        <div className="p-4 space-y-3">
          <div>
            <h3 className="font-medium text-card-foreground text-sm line-clamp-2">
              {series.title}
            </h3>
            <p className="text-xs text-muted-foreground">{series.year}</p>
          </div>

          <div className="space-y-2">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <Clock className="h-3 w-3" />
              <span>{series.duration}</span>
            </div>

            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <Server className="h-3 w-3" />
              <span>
                {series.downloadClient.host}:{series.downloadClient.port}
              </span>
            </div>
          </div>

          <Progress value={45} className="h-2" />
        </div>
      </CardContent>
    </Card>
  );
}
