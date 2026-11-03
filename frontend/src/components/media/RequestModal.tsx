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
import { useToast } from "@/hooks/use-toast";
import { useMovieQualityProfiles, useSeriesQualityProfiles, useAddMovie, useAddSeries } from "@/lib/queries";
import { ApiError } from "@/lib/api";
import { useTVDetail } from "@/lib/queries";
import { TVSelectionEditor, makeTVSelection, hasIncompleteSeasonMetadata, type TVSelection } from "@/components/media/TVSelectionEditor";
import { Checkbox } from "@/components/ui/checkbox";

interface RequestModalProps {
  isOpen: boolean;
  onClose: () => void;
  mediaType: "movie" | "tv";
  mediaTitle: string;
  tmdbID: number;
}

export function RequestModal({
  isOpen,
  onClose,
  mediaType,
  mediaTitle,
  tmdbID
}: RequestModalProps) {
  const [selectedProfileId, setSelectedProfileId] = useState<string>("");
  const { data: tvPreview, isLoading: isLoadingTV, error: tvPreviewError } = useTVDetail(mediaType === 'tv' && isOpen ? tmdbID : 0);
  const [selection, setSelection] = useState<TVSelection>({});
  const [monitorNewSeasons, setMonitorNewSeasons] = useState(false);
  const { toast } = useToast();

  const { data: qualityProfiles = [], isLoading: isLoadingProfiles } =
    mediaType === 'movie'
      ? useMovieQualityProfiles()
      : useSeriesQualityProfiles();

  // Mutations for adding media
  const addMovie = useAddMovie();
  const addSeries = useAddSeries();

  // Determine which mutation is running
  const isSubmitting = addMovie.isPending || addSeries.isPending;

  // Reset selected profile when modal closes
  //
  // Draft policy: the selection draft is reinitialized ONLY when the underlying
  // metadata scope genuinely changes (different show, or season/episode
  // structure changes shape). A refetch of the same preview returns a NEW
  // object with identical structure; ordinary object identity must never wipe
  // the user's monitoring and quality drafts.
  const previewScopeRef = useRef<string | null>(null);
  const previewScope = mediaType === 'tv' && tvPreview?.seasons
    ? `${tmdbID}|${tvPreview.seasons.map(season => `${season.seasonNumber}:${season.episodes?.length ?? -1}`).join(',')}`
    : null;
  useEffect(() => {
    if (!isOpen) {
      setSelectedProfileId("");
      setSelection({});
      previewScopeRef.current = null;
    } else if (mediaType === 'tv' && tvPreview?.seasons && previewScopeRef.current !== previewScope) {
      previewScopeRef.current = previewScope;
      setSelection(makeTVSelection(tvPreview.seasons, 'all'));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen, tvPreview, mediaType, previewScope]);

  const handleSubmit = async () => {
    if (!selectedProfileId) {
      toast({
        title: "Error",
        description: "Please select a quality profile",
        variant: "destructive",
      });
      return;
    }

    const request = {
      tmdbID: tmdbID,
      qualityProfileID: parseInt(selectedProfileId),
    };

    try {
      if (mediaType === "movie") {
        await addMovie.mutateAsync(request);
      } else {
        if (!tvPreview || tvPreviewError || isLoadingTV || Object.keys(selection).length !== tvPreview.seasons.length) throw new Error("Full TV metadata is required before requesting this series.");
        // Incomplete metadata must never silently become an empty complete selection.
        if (tvPreview.seasons.some(season => !Array.isArray(season.episodes) || (season.episodes.length === 0 && season.episodeCount > 0))) throw new Error("Episode metadata is incomplete for one or more seasons; selection is unavailable.");
        const seasons = tvPreview.seasons.map(season => ({
          seasonNumber: season.seasonNumber,
          monitored: selection[season.seasonNumber]?.monitored ?? false,
          qualityProfileID: selection[season.seasonNumber]?.qualityProfileID ?? null,
          episodes: (season.episodes ?? []).map(episode => ({
            episodeNumber: episode.episodeNumber,
            monitored: selection[season.seasonNumber]?.episodes[episode.episodeNumber]?.monitored ?? false,
            qualityProfileID: selection[season.seasonNumber]?.episodes[episode.episodeNumber]?.qualityProfileID ?? null,
          })),
        }));
        await addSeries.mutateAsync({ ...request, monitorNewSeasons, selection: { mode: 'custom', seasons } });
      }

      toast({
        title: "Request Submitted",
        description: `${mediaTitle} has been requested successfully!`,
      });

      onClose();
    } catch (error) {
      // Error handling
      let errorMessage = "An unexpected error occurred";

      if (error instanceof ApiError) {
        errorMessage = error.message;
      } else if (error instanceof Error) {
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

  return (
    <Dialog open={isOpen} onOpenChange={handleClose}>
      {/* Viewport-constrained dialog: header and actions stay reachable while the
          season/episode selection scrolls. Heights use viewport tokens (dvh/vh)
          rather than arbitrary pixel sizes. */}
      <DialogContent className="flex max-h-[100dvh] flex-col gap-4 p-4 sm:max-h-[85vh] sm:max-w-md sm:p-6">
        <DialogHeader className="shrink-0">
          <DialogTitle>Request {mediaType === "movie" ? "Movie" : "TV Show"}</DialogTitle>
          <DialogDescription>
            You are requesting <span className="font-medium">{mediaTitle}</span>
            {mediaType === "tv" && ". Choose what automation may acquire. Metadata is retained regardless of selection."}
          </DialogDescription>
        </DialogHeader>

        {/* Scrollable middle region; header above and footer below remain visible. */}
        <div className="min-h-0 flex-1 overflow-y-auto">
        <div className="py-4">
          {mediaType === 'tv' && <div className="mb-4 space-y-3">
            {isLoadingTV ? <LoadingSpinner /> : tvPreviewError || !tvPreview ? <p role="alert" className="text-sm text-destructive">Could not load complete season and episode metadata; selection is unavailable.</p> : <>
              <label className="flex items-center gap-2 text-sm"><Checkbox checked={monitorNewSeasons} onCheckedChange={v => setMonitorNewSeasons(v === true)} />Monitor future seasons</label>
              <TVSelectionEditor compact seasons={tvPreview.seasons} profiles={qualityProfiles} selection={selection} onChange={setSelection} disabled={isSubmitting} previewProfileID={selectedProfileId ? Number(selectedProfileId) : undefined} />
            </>}
          </div>}
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
        </div>

        <DialogFooter className="shrink-0">
          <Button variant="outline" onClick={handleClose} disabled={isSubmitting}>
            Cancel
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={!selectedProfileId || isSubmitting || qualityProfiles.length === 0 || (mediaType === 'tv' && (!tvPreview || !!tvPreviewError || isLoadingTV || hasIncompleteSeasonMetadata(tvPreview.seasons)))}
            className="bg-gradient-primary hover:opacity-90"
          >
            {isSubmitting ? (
              <>
                <LoadingSpinner size="sm" className="mr-2" />
                Requesting...
              </>
            ) : (
              `Request ${mediaType === "movie" ? "Movie" : "Series"}`
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}