import { useState, useEffect } from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Loader2 } from 'lucide-react';
import { toast } from 'sonner';
import type { QualityDefinition, PendingQualityDefinition } from '@/lib/api';
import { useCreateQualityDefinition, useUpdateQualityDefinition } from '@/lib/queries';

interface QualityDefinitionDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  definition?: QualityDefinition | null;
  defaultMediaType?: 'movie' | 'episode';
  onCreated?: (definition: QualityDefinition) => void;
  onPendingCreated?: (definition: PendingQualityDefinition) => void;
  lockMediaType?: boolean;
}

export function QualityDefinitionDialog({
  open,
  onOpenChange,
  definition,
  defaultMediaType = 'movie',
  onCreated,
  onPendingCreated,
  lockMediaType = false
}: QualityDefinitionDialogProps) {
  const [name, setName] = useState('');
  const [mediaType, setMediaType] = useState<'movie' | 'episode'>(defaultMediaType);
  const [preferredSize, setPreferredSize] = useState('');
  const [minSize, setMinSize] = useState('');
  const [maxSize, setMaxSize] = useState('');

  const createDefinition = useCreateQualityDefinition();
  const updateDefinition = useUpdateQualityDefinition();

  useEffect(() => {
    if (open) {
      if (definition) {
        setName(definition.Name);
        setMediaType(definition.MediaType as 'movie' | 'episode');
        setPreferredSize(definition.PreferredSize.toString());
        setMinSize(definition.MinSize.toString());
        setMaxSize(definition.MaxSize.toString());
      } else {
        setName('');
        setMediaType(defaultMediaType);
        setPreferredSize('');
        setMinSize('');
        setMaxSize('');
      }
    }
  }, [open, definition, defaultMediaType]);

  const handleSubmit = async () => {
    if (!name) {
      toast.error('Name is required');
      return;
    }

    const parsedPreferredSize = parseFloat(preferredSize) || 0;
    const parsedMinSize = parseFloat(minSize) || 0;
    const parsedMaxSize = parseFloat(maxSize) || 0;

    if (parsedMinSize >= parsedMaxSize) {
      toast.error('Min size must be less than max size');
      return;
    }

    const request = {
      name,
      type: mediaType,
      preferredSize: parsedPreferredSize,
      minSize: parsedMinSize,
      maxSize: parsedMaxSize,
    };

    try {
      if (definition) {
        await updateDefinition.mutateAsync({
          id: definition.ID,
          request
        });
        toast.success('Quality definition updated');
        onOpenChange(false);
        return;
      }

      if (onPendingCreated) {
        const pendingDef: PendingQualityDefinition = {
          tempId: crypto.randomUUID(),
          ...request
        };
        onPendingCreated(pendingDef);
        toast.success('Definition will be created when profile is saved');
        onOpenChange(false);
        return;
      }

      const newDefinition = await createDefinition.mutateAsync(request);
      toast.success('Quality definition created');
      if (onCreated && newDefinition) {
        onCreated(newDefinition);
      }
      onOpenChange(false);
    } catch (error) {
      toast.error(`Failed to save: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  };

  const isLoading = createDefinition.isPending || updateDefinition.isPending;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>
            {definition ? 'Edit Quality Definition' : 'Add Quality Definition'}
          </DialogTitle>
        </DialogHeader>

        <div className="grid gap-4 py-4 px-1">
          <div className="grid gap-2">
            <Label htmlFor="name">Name</Label>
            <Input
              id="name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g., 1080p BluRay"
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="mediaType">Media Type</Label>
            <Select value={mediaType} defaultValue={defaultMediaType} onValueChange={(v) => setMediaType(v as 'movie' | 'episode')} disabled={lockMediaType}>
              <SelectTrigger id="mediaType" className={lockMediaType ? 'text-muted-foreground' : ''}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="movie">Movie</SelectItem>
                <SelectItem value="episode">Episode</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="minSize">Min Size (MB/min)</Label>
            <Input
              id="minSize"
              type="number"
              step="0.1"
              value={minSize}
              onChange={(e) => setMinSize(e.target.value)}
              placeholder="e.g., 2.0"
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="preferredSize">Preferred Size (MB/min)</Label>
            <Input
              id="preferredSize"
              type="number"
              step="0.1"
              value={preferredSize}
              onChange={(e) => setPreferredSize(e.target.value)}
              placeholder="e.g., 5.0"
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="maxSize">Max Size (MB/min)</Label>
            <Input
              id="maxSize"
              type="number"
              step="0.1"
              value={maxSize}
              onChange={(e) => setMaxSize(e.target.value)}
              placeholder="e.g., 10.0"
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={isLoading}>
            Cancel
          </Button>
          <Button onClick={handleSubmit} disabled={isLoading}>
            {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {definition ? 'Update' : 'Create'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
