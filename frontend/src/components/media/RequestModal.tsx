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

// Mock quality profiles
const mockQualityProfiles = [
  { id: 1, name: "HD - 720p/1080p" },
  { id: 2, name: "4K - 2160p" },
  { id: 3, name: "SD - 480p" },
];

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
  const [qualityProfiles, setQualityProfiles] = useState<any[]>([]);
  const [selectedProfileId, setSelectedProfileId] = useState<string>("");
  const [isLoadingProfiles, setIsLoadingProfiles] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const { toast } = useToast();

  useEffect(() => {
    if (isOpen) {
      setIsLoadingProfiles(true);
      // Simulate API call to fetch quality profiles
      setTimeout(() => {
        setQualityProfiles(mockQualityProfiles);
        setIsLoadingProfiles(false);
      }, 500);
    }
  }, [isOpen]);

  const handleSubmit = async () => {
    if (!selectedProfileId) return;

    setIsSubmitting(true);
    
    // Simulate API call to request media
    setTimeout(() => {
      toast({
        title: "Request Submitted",
        description: `${mediaTitle} has been requested successfully!`,
      });
      setIsSubmitting(false);
      onClose();
      setSelectedProfileId("");
    }, 1000);
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
          ) : (
            <Select value={selectedProfileId} onValueChange={setSelectedProfileId}>
              <SelectTrigger>
                <SelectValue placeholder="Select a quality profile" />
              </SelectTrigger>
              <SelectContent>
                {qualityProfiles.map((profile) => (
                  <SelectItem key={profile.id} value={profile.id.toString()}>
                    {profile.name}
                  </SelectItem>
                ))}
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
            disabled={!selectedProfileId || isSubmitting}
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