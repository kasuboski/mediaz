import { useState, useEffect } from "react";
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
  useEffect(() => {
    if (!isOpen) {
      setSelectedProfileId("");
    }
  }, [isOpen]);

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
        await addSeries.mutateAsync(request);
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
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Request {mediaType === "movie" ? "Movie" : "TV Show"}</DialogTitle>
          <DialogDescription>
            You are requesting <span className="font-medium">{mediaTitle}</span>
            {mediaType === "tv" && ". This will request the entire series."}
          </DialogDescription>
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