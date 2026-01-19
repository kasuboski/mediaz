import { useState, useEffect, useRef } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { LoadingSpinner } from "@/components/ui/LoadingSpinner";
import { Checkbox } from "@/components/ui/checkbox";
import { Badge } from "@/components/ui/badge";
import { ChevronDown } from "lucide-react";
import { useToast } from "@/hooks/use-toast";
import { useMovieQualityProfiles, useSeriesQualityProfiles, useAddMovie, useAddSeries, useUpdateSeriesMonitoring, useUpdateMovieQualityProfile } from "@/lib/queries";
import { ApiError, tvApi, type SeasonResult } from "@/lib/api";

interface RequestModalProps {
  isOpen: boolean;
  onClose: () => void;
  mediaType: "movie" | "tv";
  mediaTitle: string;
  tmdbID: number;
  mode?: "request" | "edit";
  currentMonitoring?: {
    seriesID?: number;
    seasons: SeasonResult[];
    qualityProfileID?: number;
    monitorNewSeasons?: boolean;
  };
  currentMovieMonitoring?: {
    movieID: number;
    qualityProfileID?: number;
  };
}

export function RequestModal({
  isOpen,
  onClose,
  mediaType,
  mediaTitle,
  tmdbID,
  mode = "request",
  currentMonitoring,
  currentMovieMonitoring
}: RequestModalProps) {
  const [selectedProfileId, setSelectedProfileId] = useState<string>("");
  const [selectedEpisodes, setSelectedEpisodes] = useState<Set<number>>(new Set());
  const [expandedSeasons, setExpandedSeasons] = useState<Set<number>>(new Set());
  const [seriesSeasons, setSeriesSeasons] = useState<SeasonResult[]>([]);
  const [isLoadingSeasons, setIsLoadingSeasons] = useState(false);
  const [monitorNewSeasons, setMonitorNewSeasons] = useState(false);
  const { toast } = useToast();

  const { data: qualityProfiles = [], isLoading: isLoadingProfiles } =
    mediaType === 'movie'
      ? useMovieQualityProfiles()
      : useSeriesQualityProfiles();

  // Mutations for adding media
  const addMovie = useAddMovie();
  const addSeries = useAddSeries();
  const updateSeriesMonitoring = useUpdateSeriesMonitoring();
  const updateMovieQualityProfile = useUpdateMovieQualityProfile();

  // Determine which mutation is running
  const isSubmitting = addMovie.isPending || addSeries.isPending || updateSeriesMonitoring.isPending || updateMovieQualityProfile.isPending;

  const isSeasonFullySelected = (season: SeasonResult) => {
    if (!season.episodes || season.episodes.length === 0) return false;
    return season.episodes.every(ep => selectedEpisodes.has(ep.tmdbID));
  };

  const isSeasonPartiallySelected = (season: SeasonResult) => {
    if (!season.episodes || season.episodes.length === 0) return false;
    const selectedCount = season.episodes.filter(ep => selectedEpisodes.has(ep.tmdbID)).length;
    return selectedCount > 0 && selectedCount < season.episodes.length;
  };

  const toggleSeason = (season: SeasonResult) => {
    if (!season.episodes) return;

    const newSelected = new Set(selectedEpisodes);
    const isFullySelected = isSeasonFullySelected(season);

    if (isFullySelected) {
      season.episodes.forEach(ep => newSelected.delete(ep.tmdbID));
      setSelectedEpisodes(newSelected);
      return;
    }

    season.episodes.forEach(ep => newSelected.add(ep.tmdbID));
    setSelectedEpisodes(newSelected);
  };

  const toggleEpisode = (episodeTMDBID: number) => {
    const newSelected = new Set(selectedEpisodes);

    if (newSelected.has(episodeTMDBID)) {
      newSelected.delete(episodeTMDBID);
      setSelectedEpisodes(newSelected);
      return;
    }

    newSelected.add(episodeTMDBID);
    setSelectedEpisodes(newSelected);
  };

  useEffect(() => {
    if (!isOpen) {
      setSelectedProfileId("");
      setSelectedEpisodes(new Set());
      setExpandedSeasons(new Set());
      setSeriesSeasons([]);
      setMonitorNewSeasons(false);
      return;
    }

    if (mode === "edit" && mediaType === "movie" && currentMovieMonitoring) {
      if (currentMovieMonitoring.qualityProfileID) {
        setSelectedProfileId(currentMovieMonitoring.qualityProfileID.toString());
      }
      return;
    }

    if (mode === "edit" && currentMonitoring) {
      setSeriesSeasons(currentMonitoring.seasons);
      const episodeTMDBIDs = new Set<number>();
      currentMonitoring.seasons.forEach(season => {
        if (season.episodes) {
          season.episodes
            .filter(ep => ep.monitored)
            .forEach(ep => episodeTMDBIDs.add(ep.tmdbID));
        }
      });
      setSelectedEpisodes(episodeTMDBIDs);
      if (currentMonitoring.qualityProfileID) {
        setSelectedProfileId(currentMonitoring.qualityProfileID.toString());
      }
      setMonitorNewSeasons(currentMonitoring.monitorNewSeasons ?? false);
      return;
    }

    if (mediaType === 'tv') {
      setIsLoadingSeasons(true);
      tvApi.getTVDetail(tmdbID)
        .then(details => {
          setSeriesSeasons(details.seasons || []);
          const episodeTMDBIDs = new Set<number>();
          details.seasons
            .filter(s => s.seasonNumber !== 0)
            .forEach(season => {
              if (season.episodes) {
                season.episodes.forEach(ep => episodeTMDBIDs.add(ep.tmdbID));
              }
            });
          setSelectedEpisodes(episodeTMDBIDs);
        })
        .catch(() => {
          toast({
            title: "Error",
            description: "Failed to load season information",
            variant: "destructive",
          });
        })
        .finally(() => {
          setIsLoadingSeasons(false);
        });
    }
  }, [isOpen, mediaType, tmdbID, mode, currentMonitoring, currentMovieMonitoring, toast]);

  const handleSubmit = async () => {
    if (!selectedProfileId) {
      toast({
        title: "Error",
        description: "Please select a quality profile",
        variant: "destructive",
      });
      return;
    }

    try {
      if (mediaType === "movie") {
        if (mode === "edit" && currentMovieMonitoring?.movieID) {
          await updateMovieQualityProfile.mutateAsync({
            movieID: currentMovieMonitoring.movieID,
            qualityProfileID: parseInt(selectedProfileId),
          });
          toast({
            title: "Quality Profile Updated",
            description: `${mediaTitle} quality profile has been updated successfully!`,
          });
          onClose();
          return;
        }

        const request = {
          tmdbID: tmdbID,
          qualityProfileID: parseInt(selectedProfileId),
        };
        await addMovie.mutateAsync(request);
        toast({
          title: "Request Submitted",
          description: `${mediaTitle} has been requested successfully!`,
        });
        onClose();
        return;
      }

      if (mode === "edit" && currentMonitoring?.seriesID) {
        const monitoredEpisodes = Array.from(selectedEpisodes);
        const qualityProfileID = selectedProfileId ? parseInt(selectedProfileId) : undefined;
        await updateSeriesMonitoring.mutateAsync({
          seriesID: currentMonitoring.seriesID,
          request: { monitoredEpisodes, qualityProfileID, monitorNewSeasons }
        });
        toast({
          title: "Monitoring Updated",
          description: `${mediaTitle} monitoring has been updated successfully!`,
        });
        onClose();
        return;
      }

      const request = {
        tmdbID: tmdbID,
        qualityProfileID: parseInt(selectedProfileId),
        monitoredEpisodes: Array.from(selectedEpisodes),
        monitorNewSeasons,
      };
      await addSeries.mutateAsync(request);
      toast({
        title: "Request Submitted",
        description: `${mediaTitle} has been requested successfully!`,
      });
      onClose();
    } catch (error) {
      let errorMessage = "An unexpected error occurred";

      if (error instanceof ApiError) {
        errorMessage = error.message;
      }
      if (error instanceof Error) {
        errorMessage = error.message;
      }

      toast({
        title: "Request Failed",
        description: errorMessage,
        variant: "destructive",
      });
    }
  };

  const handleClose = () => {
    onClose();
    setSelectedProfileId("");
  };

  const dialogTitle = mode === "edit"
    ? `Monitoring - ${mediaTitle}`
    : `Request ${mediaType === "movie" ? "Movie" : "Series"}`;

  const dialogDescription = mode === "edit"
    ? "Update quality profile and episode monitoring settings."
    : `Add ${mediaTitle} to your library.`;

  return (
    <Dialog open={isOpen} onOpenChange={handleClose}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{dialogTitle}</DialogTitle>
          <DialogDescription>{dialogDescription}</DialogDescription>
        </DialogHeader>
        <div className="py-4">
          <label htmlFor="quality-profile" className="text-sm font-medium mb-2 block">
            Quality Profile
          </label>

          {isLoadingProfiles ? (
            <div className="flex justify-center py-4">
              <LoadingSpinner />
            </div>
          ) : qualityProfiles.length === 0 ? (
            <div className="text-center py-4 text-sm text-muted-foreground">
              No quality profiles available. Please configure quality profiles first.
            </div>
          ) : (
            <Select value={selectedProfileId} onValueChange={setSelectedProfileId}>
              <SelectTrigger>
                <SelectValue placeholder="Select a quality profile">
                  {selectedProfileId && qualityProfiles.find(p => p.id.toString() === selectedProfileId)?.name}
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                {qualityProfiles.map((profile) => {
                  const qualityNames = profile.qualities.map(q => q.name).join(", ");

                  return (
                    <SelectItem
                      key={profile.id}
                      value={profile.id.toString()}
                      className="py-3"
                    >
                      <div className="flex flex-col gap-1">
                        <span className="font-semibold">{profile.name}</span>
                        <span className="text-xs opacity-70 whitespace-normal break-words">
                          {qualityNames || "No qualities defined"}
                        </span>
                      </div>
                    </SelectItem>
                  );
                })}
              </SelectContent>
            </Select>
          )}
        </div>

        {mediaType === 'tv' && (
          <div className="flex items-center space-x-2">
            <Checkbox
              id="monitorNewSeasons"
              checked={monitorNewSeasons}
              onCheckedChange={(checked) => setMonitorNewSeasons(checked === true)}
            />
            <label
              htmlFor="monitorNewSeasons"
              className="text-sm font-medium leading-none cursor-pointer"
            >
              Monitor future seasons automatically
            </label>
          </div>
        )}

        {mediaType === 'tv' && (
          <div className="py-4">
            <label className="text-sm font-medium mb-3 block">Monitor Episodes</label>

            {isLoadingSeasons ? (
              <div className="flex justify-center py-4">
                <LoadingSpinner />
              </div>
            ) : (
              <div className="border rounded-md overflow-hidden">
                <div className="max-h-96 overflow-y-auto">
                  {seriesSeasons.map(season => {
                    const isExpanded = expandedSeasons.has(season.seasonNumber);
                    const isFullySelected = isSeasonFullySelected(season);
                    const isPartiallySelected = isSeasonPartiallySelected(season);
                    const selectedCount = season.episodes?.filter(ep => selectedEpisodes.has(ep.tmdbID)).length || 0;

                    return (
                      <div key={season.seasonNumber} className="border-b last:border-b-0">
                        <div className="flex items-center p-3 hover:bg-muted/30 transition-colors">
                          <Checkbox
                            checked={isPartiallySelected ? "indeterminate" : isFullySelected}
                            onCheckedChange={() => toggleSeason(season)}
                          />
                          <button
                            onClick={() => {
                              const newExpanded = new Set(expandedSeasons);
                              if (isExpanded) {
                                newExpanded.delete(season.seasonNumber);
                              } else {
                                newExpanded.add(season.seasonNumber);
                              }
                              setExpandedSeasons(newExpanded);
                            }}
                            className="flex-1 flex items-center justify-between ml-3"
                          >
                            <span className="font-medium">{season.title}</span>
                            <div className="flex items-center gap-2">
                              <Badge variant="secondary">
                                {selectedCount}/{season.episodeCount}
                              </Badge>
                              <ChevronDown className={`h-4 w-4 transition-transform ${isExpanded ? 'rotate-180' : ''}`} />
                            </div>
                          </button>
                        </div>

                        {isExpanded && season.episodes && (
                          <div className="bg-muted/10 p-3 space-y-2">
                            {season.episodes.map(episode => (
                              <div key={episode.tmdbID} className="flex items-center p-2 hover:bg-background/50 rounded">
                                <Checkbox
                                  checked={selectedEpisodes.has(episode.tmdbID)}
                                  onCheckedChange={() => toggleEpisode(episode.tmdbID)}
                                />
                                <label className="ml-3 flex-1 cursor-pointer text-sm" onClick={() => toggleEpisode(episode.tmdbID)}>
                                  <span className="font-medium">E{episode.episodeNumber}</span>
                                  {" - "}
                                  {episode.title}
                                </label>
                                {episode.downloaded && (
                                  <Badge variant="default" className="text-xs">Downloaded</Badge>
                                )}
                              </div>
                            ))}
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>
              </div>
            )}
          </div>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={handleClose} disabled={isSubmitting}>
            Cancel
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={!selectedProfileId || isSubmitting || qualityProfiles.length === 0}
            className="bg-gradient-primary hover:opacity-90"
          >
            {isSubmitting ? (
              <>
                <LoadingSpinner size="sm" className="mr-2" />
                {mode === "edit" ? "Updating..." : "Requesting..."}
              </>
            ) : (
              mode === "edit" ? "Update Monitoring" : `Request ${mediaType === "movie" ? "Movie" : "Series"}`
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}