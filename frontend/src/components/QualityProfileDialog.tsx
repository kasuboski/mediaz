import { useState, useEffect } from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Badge } from '@/components/ui/badge';
import { Loader2, Plus, X } from 'lucide-react';
import { toast } from 'sonner';
import type { QualityProfile, PendingQualityDefinition } from '@/lib/api';
import { useCreateQualityProfile, useUpdateQualityProfile, useQualityDefinitions, useCreateQualityDefinition } from '@/lib/queries';
import { QualityDefinitionDialog } from './QualityDefinitionDialog';

interface QualityProfileDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  profile?: QualityProfile | null;
  mediaType?: 'movie' | 'series';
}

export function QualityProfileDialog({
  open,
  onOpenChange,
  profile,
  mediaType = 'movie'
}: QualityProfileDialogProps) {
  const [name, setName] = useState('');
  const [upgradeAllowed, setUpgradeAllowed] = useState(true);
  const [cutoffQualityId, setCutoffQualityId] = useState<number | null>(null);
  const [selectedQualityIds, setSelectedQualityIds] = useState<Set<number>>(new Set());
  const [definitionDialogOpen, setDefinitionDialogOpen] = useState(false);
  const [pendingDefinitions, setPendingDefinitions] = useState<PendingQualityDefinition[]>([]);

  const { data: allDefinitions = [] } = useQualityDefinitions();
  const createProfile = useCreateQualityProfile();
  const updateProfile = useUpdateQualityProfile();
  const createDefinition = useCreateQualityDefinition();

  const definitions = allDefinitions.filter(d =>
    d.MediaType === (mediaType === 'series' ? 'episode' : 'movie')
  );

  useEffect(() => {
    if (open) {
      if (profile) {
        setName(profile.name);
        setUpgradeAllowed(profile.upgradeAllowed);
        setCutoffQualityId(profile.cutoff_quality_id);
        setSelectedQualityIds(new Set(profile.qualities.map(q => q.id)));
        setPendingDefinitions([]);
      } else {
        setName('');
        setUpgradeAllowed(true);
        setCutoffQualityId(null);
        setSelectedQualityIds(new Set());
        setPendingDefinitions([]);
      }
    }
  }, [open, profile]);

  useEffect(() => {
    if (!upgradeAllowed) {
      setCutoffQualityId(null);
    }
  }, [upgradeAllowed]);

  const toggleQuality = (qualityId: number) => {
    setSelectedQualityIds(prev => {
      const newSet = new Set(prev);
      if (newSet.has(qualityId)) {
        newSet.delete(qualityId);
        if (cutoffQualityId === qualityId) {
          setCutoffQualityId(null);
        }
      } else {
        newSet.add(qualityId);
      }
      return newSet;
    });
  };

  const handleSubmit = async () => {
    if (!name) {
      toast.error('Name is required');
      return;
    }

    if (selectedQualityIds.size === 0 && pendingDefinitions.length === 0) {
      toast.error('At least one quality must be selected or created');
      return;
    }

    if (upgradeAllowed && !cutoffQualityId) {
      toast.error('Cutoff quality must be selected when upgrades are allowed');
      return;
    }

    if (cutoffQualityId !== null && !selectedQualityIds.has(cutoffQualityId)) {
      toast.error('Cutoff quality must be one of the selected qualities');
      return;
    }

    try {
      const newlyCreatedIds: number[] = [];

      if (pendingDefinitions.length > 0) {
        toast.info(`Creating ${pendingDefinitions.length} new definition(s)...`);

        for (const pendingDef of pendingDefinitions) {
          const created = await createDefinition.mutateAsync({
            name: pendingDef.name,
            type: pendingDef.type,
            preferredSize: pendingDef.preferredSize,
            minSize: pendingDef.minSize,
            maxSize: pendingDef.maxSize,
          });
          newlyCreatedIds.push(created.ID);
        }
      }

      const allQualityIds = [
        ...Array.from(selectedQualityIds),
        ...newlyCreatedIds
      ];

      const request = {
        name,
        cutoffQualityId,
        upgradeAllowed,
        qualityIds: allQualityIds,
      };

      if (profile) {
        await updateProfile.mutateAsync({ id: profile.id, request });
        toast.success('Quality profile updated');
        onOpenChange(false);
        return;
      }

      await createProfile.mutateAsync(request);
      toast.success('Quality profile created');
      onOpenChange(false);
    } catch (error) {
      toast.error(`Failed to save: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  };

  const isLoading = createProfile.isPending || updateProfile.isPending || createDefinition.isPending;

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="sm:max-w-[600px]">
          <DialogHeader>
            <DialogTitle>
              {profile ? 'Edit Quality Profile' : 'Add Quality Profile'}
            </DialogTitle>
          </DialogHeader>

          <div className="grid gap-4 py-4 max-h-[60vh] overflow-y-auto pl-1 pr-3">
            <div className="grid gap-2">
              <Label htmlFor="name">Profile Name</Label>
              <Input
                id="name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g., HD Bluray"
              />
            </div>

            <div className="flex items-center space-x-2">
              <Switch
                id="upgradeAllowed"
                checked={upgradeAllowed}
                onCheckedChange={setUpgradeAllowed}
              />
              <Label htmlFor="upgradeAllowed">Allow Upgrades</Label>
            </div>
            {!upgradeAllowed && (
              <p className="text-sm text-muted-foreground mt-1">
                Cutoff quality is only applicable when upgrades are allowed
              </p>
            )}

            <div className="grid gap-2">
              <div className="flex items-center justify-between">
                <Label>Quality Definitions</Label>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => setDefinitionDialogOpen(true)}
                >
                  <Plus className="h-4 w-4 mr-2" />
                  New Definition
                </Button>
              </div>

              {definitions.length === 0 ? (
                <div className="text-sm text-muted-foreground py-4 text-center">
                  No quality definitions available. Create one first.
                </div>
              ) : (
                <div className="space-y-4">
                  {(definitions.filter(def => selectedQualityIds.has(def.ID)).length > 0 || pendingDefinitions.length > 0) && (
                    <div className="border rounded-md p-3 bg-accent/10">
                      <h4 className="text-sm font-semibold mb-2 text-foreground/80">Currently in Profile</h4>
                      <div className="space-y-2">
                        {pendingDefinitions.map((pendingDef) => (
                          <div
                            key={pendingDef.tempId}
                            className="flex items-center justify-between py-2 px-2 bg-background rounded"
                          >
                            <div className="flex items-center space-x-3">
                              <Button
                                type="button"
                                variant="ghost"
                                size="sm"
                                className="h-8 w-8 p-0"
                                onClick={() => {
                                  setPendingDefinitions(prev =>
                                    prev.filter(d => d.tempId !== pendingDef.tempId)
                                  );
                                }}
                              >
                                <X className="h-4 w-4" />
                              </Button>
                              <div>
                                <div className="font-medium flex items-center gap-2">
                                  {pendingDef.name}
                                  <Badge variant="outline" className="text-xs">New</Badge>
                                </div>
                                <div className="text-sm text-muted-foreground">
                                  {pendingDef.minSize} - {pendingDef.maxSize} MB/min
                                </div>
                              </div>
                            </div>
                            <Button
                              type="button"
                              variant="outline"
                              size="sm"
                              disabled={true}
                            >
                              Set Cutoff
                            </Button>
                          </div>
                        ))}
                        {definitions
                          .filter(def => selectedQualityIds.has(def.ID))
                          .map((def) => (
                            <div
                              key={def.ID}
                              className="flex items-center justify-between py-2 px-2 bg-background rounded"
                            >
                              <div className="flex items-center space-x-3">
                                <Button
                                  type="button"
                                  variant="ghost"
                                  size="sm"
                                  className="h-8 w-8 p-0"
                                  onClick={() => toggleQuality(def.ID)}
                                >
                                  <X className="h-4 w-4" />
                                </Button>
                                <div>
                                  <div className="font-medium">{def.Name}</div>
                                  <div className="text-sm text-muted-foreground">
                                    {def.MinSize} - {def.MaxSize} MB/min
                                  </div>
                                </div>
                              </div>
                              {upgradeAllowed && (
                                <Button
                                  type="button"
                                  variant={cutoffQualityId === def.ID ? 'default' : 'outline'}
                                  size="sm"
                                  disabled={!selectedQualityIds.has(def.ID)}
                                  onClick={() => setCutoffQualityId(def.ID)}
                                >
                                  {cutoffQualityId === def.ID ? 'Cutoff' : 'Set Cutoff'}
                                </Button>
                              )}
                            </div>
                          ))}
                      </div>
                    </div>
                  )}

                  {definitions.filter(def => !selectedQualityIds.has(def.ID)).length > 0 && (
                    <div className="border rounded-md p-3 bg-accent/10">
                      <h4 className="text-sm font-semibold mb-2 text-foreground/80">Available to Add</h4>
                      <div className="space-y-2">
                        {definitions
                          .filter(def => !selectedQualityIds.has(def.ID))
                          .map((def) => (
                            <div
                              key={def.ID}
                              className="flex items-center justify-between py-2 px-2 bg-background rounded"
                            >
                              <div className="flex items-center space-x-3">
                                <Button
                                  type="button"
                                  variant="ghost"
                                  size="sm"
                                  className="h-8 w-8 p-0"
                                  onClick={() => toggleQuality(def.ID)}
                                >
                                  <Plus className="h-4 w-4" />
                                </Button>
                                <div>
                                  <div className="font-medium">{def.Name}</div>
                                  <div className="text-sm text-muted-foreground">
                                    {def.MinSize} - {def.MaxSize} MB/min
                                  </div>
                                </div>
                              </div>
                            </div>
                          ))}
                      </div>
                    </div>
                  )}
                </div>
              )}
              <p className="text-sm text-muted-foreground">
                Select qualities to include in this profile. The cutoff quality determines when to stop upgrading.
              </p>
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => onOpenChange(false)} disabled={isLoading}>
              Cancel
            </Button>
            <Button onClick={handleSubmit} disabled={isLoading}>
              {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {profile ? 'Update' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <QualityDefinitionDialog
        open={definitionDialogOpen}
        onOpenChange={setDefinitionDialogOpen}
        defaultMediaType={mediaType === 'series' ? 'episode' : 'movie'}
        lockMediaType={true}
        onPendingCreated={(pendingDef) => {
          setPendingDefinitions(prev => [...prev, pendingDef]);
        }}
      />
    </>
  );
}
