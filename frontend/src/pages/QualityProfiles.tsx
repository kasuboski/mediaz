import { useState } from 'react';
import { useQualityProfiles, useDeleteQualityProfile, useQualityDefinitions, useDeleteQualityDefinition } from '@/lib/queries';
import type { QualityProfile, QualityDefinition } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog';
import { Loader2, AlertCircle, RefreshCw, Plus, Pencil, Trash2, Layers, ChevronDown, ChevronRight } from 'lucide-react';
import { toast } from 'sonner';
import { QualityProfileDialog } from '@/components/QualityProfileDialog';
import { QualityDefinitionDialog } from '@/components/QualityDefinitionDialog';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';

export default function QualityProfiles() {
  const [mediaType, setMediaType] = useState<'movie' | 'series'>('movie');
  const { data: profiles, isLoading: profilesLoading, error: profilesError, refetch: refetchProfiles } = useQualityProfiles(mediaType);
  const { data: definitions, isLoading: definitionsLoading, error: definitionsError, refetch: refetchDefinitions } = useQualityDefinitions();

  const deleteProfile = useDeleteQualityProfile();
  const deleteDefinition = useDeleteQualityDefinition();

  const [profileDialogOpen, setProfileDialogOpen] = useState(false);
  const [editingProfile, setEditingProfile] = useState<QualityProfile | null>(null);
  const [deleteProfileDialogOpen, setDeleteProfileDialogOpen] = useState(false);
  const [profileToDelete, setProfileToDelete] = useState<QualityProfile | null>(null);

  const [definitionDialogOpen, setDefinitionDialogOpen] = useState(false);
  const [editingDefinition, setEditingDefinition] = useState<QualityDefinition | null>(null);
  const [deleteDefinitionDialogOpen, setDeleteDefinitionDialogOpen] = useState(false);
  const [definitionToDelete, setDefinitionToDelete] = useState<QualityDefinition | null>(null);

  const [expandedProfiles, setExpandedProfiles] = useState<Set<number>>(new Set());

  const toggleProfileExpanded = (id: number) => {
    setExpandedProfiles(prev => {
      const newSet = new Set(prev);
      if (newSet.has(id)) {
        newSet.delete(id);
      } else {
        newSet.add(id);
      }
      return newSet;
    });
  };

  const handleAddProfile = () => {
    setEditingProfile(null);
    setProfileDialogOpen(true);
  };

  const handleEditProfile = (profile: QualityProfile) => {
    setEditingProfile(profile);
    setProfileDialogOpen(true);
  };

  const handleDeleteProfileClick = (profile: QualityProfile) => {
    setProfileToDelete(profile);
    setDeleteProfileDialogOpen(true);
  };

  const handleDeleteProfileConfirm = async () => {
    if (!profileToDelete) return;

    try {
      await deleteProfile.mutateAsync(profileToDelete.id);
      toast.success('Quality profile deleted');
      setDeleteProfileDialogOpen(false);
      setProfileToDelete(null);
    } catch (error) {
      toast.error(`Failed to delete: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  };

  const handleAddDefinition = () => {
    setEditingDefinition(null);
    setDefinitionDialogOpen(true);
  };

  const handleEditDefinition = (definition: QualityDefinition) => {
    setEditingDefinition(definition);
    setDefinitionDialogOpen(true);
  };

  const handleDeleteDefinitionClick = (definition: QualityDefinition) => {
    setDefinitionToDelete(definition);
    setDeleteDefinitionDialogOpen(true);
  };

  const handleDeleteDefinitionConfirm = async () => {
    if (!definitionToDelete) return;

    try {
      await deleteDefinition.mutateAsync(definitionToDelete.ID);
      toast.success('Quality definition deleted');
      setDeleteDefinitionDialogOpen(false);
      setDefinitionToDelete(null);
    } catch (error) {
      toast.error(`Failed to delete: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  };

  const filteredDefinitions = definitions?.filter(d =>
    d.MediaType === (mediaType === 'series' ? 'episode' : 'movie')
  ) || [];

  return (
    <div className="container mx-auto px-6 py-8">
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-foreground mb-2">Quality Profiles</h1>
        <p className="text-muted-foreground">
          Manage quality profiles and definitions for your media
        </p>
      </div>

      <Tabs value={mediaType} onValueChange={(v) => setMediaType(v as 'movie' | 'series')}>
        <TabsList className="mb-4">
          <TabsTrigger value="movie">Movies</TabsTrigger>
          <TabsTrigger value="series">TV Series</TabsTrigger>
        </TabsList>

        <TabsContent value={mediaType}>
          <Card className="mb-6">
            <CardHeader className="flex flex-row items-center justify-between">
              <CardTitle>Quality Profiles</CardTitle>
              <Button onClick={handleAddProfile} className="bg-gradient-primary hover:opacity-90">
                <Plus className="mr-2 h-4 w-4" />
                Add Profile
              </Button>
            </CardHeader>
            <CardContent>
              {profilesLoading && (
                <div className="flex items-center justify-center py-12">
                  <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
                </div>
              )}

              {profilesError && (
                <div className="flex flex-col items-center justify-center py-12 gap-4">
                  <AlertCircle className="h-12 w-12 text-destructive" />
                  <div className="text-center">
                    <p className="font-semibold text-destructive mb-1">Failed to load profiles</p>
                    <p className="text-sm text-muted-foreground mb-4">{profilesError.message}</p>
                    <Button onClick={() => refetchProfiles()} variant="outline" size="sm">
                      <RefreshCw className="h-4 w-4 mr-2" />
                      Try Again
                    </Button>
                  </div>
                </div>
              )}

              {!profilesLoading && !profilesError && (!profiles || profiles.length === 0) && (
                <div className="flex flex-col items-center justify-center py-12">
                  <Layers className="h-12 w-12 text-muted-foreground mb-4" />
                  <p className="text-lg font-semibold text-foreground mb-2">No profiles configured</p>
                  <Button onClick={handleAddProfile} className="bg-gradient-primary hover:opacity-90">
                    <Plus className="mr-2 h-4 w-4" />
                    Add Profile
                  </Button>
                </div>
              )}

              {!profilesLoading && !profilesError && profiles && profiles.length > 0 && (
                <div className="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
                  {profiles.map((profile) => (
                    <Card key={profile.id}>
                      <CardContent className="p-4">
                        <div className="flex items-start justify-between mb-3">
                          <div className="flex-1">
                            <div className="font-semibold text-lg mb-1">{profile.name}</div>
                            <div className="text-sm text-muted-foreground mb-2">
                              {profile.qualities.length} quality definitions
                              {profile.upgradeAllowed && (
                                <Badge variant="outline" className="ml-2">
                                  Upgrades Allowed
                                </Badge>
                              )}
                            </div>
                          </div>
                          <div className="flex gap-1">
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => handleEditProfile(profile)}
                              className="h-8 w-8 p-0"
                            >
                              <Pencil className="h-3 w-3" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => handleDeleteProfileClick(profile)}
                              disabled={deleteProfile.isPending}
                              className="h-8 w-8 p-0 text-destructive hover:text-destructive"
                            >
                              <Trash2 className="h-3 w-3" />
                            </Button>
                          </div>
                        </div>
                        <div className="space-y-1.5 text-sm">
                          <div className="font-medium text-muted-foreground">Qualities:</div>
                          {profile.qualities.map((quality, idx) => (
                            <div key={idx} className="flex items-center justify-between py-1 px-2 bg-accent/50 rounded text-xs">
                              <span className="font-medium">{quality.name}</span>
                            </div>
                          ))}
                        </div>
                      </CardContent>
                    </Card>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <CardTitle>Quality Definitions</CardTitle>
              <Button onClick={handleAddDefinition} className="bg-gradient-primary hover:opacity-90">
                <Plus className="mr-2 h-4 w-4" />
                Add Definition
              </Button>
            </CardHeader>
            <CardContent>
              {definitionsLoading && (
                <div className="flex items-center justify-center py-12">
                  <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
                </div>
              )}

              {definitionsError && (
                <div className="flex flex-col items-center justify-center py-12 gap-4">
                  <AlertCircle className="h-12 w-12 text-destructive" />
                  <div className="text-center">
                    <p className="font-semibold text-destructive mb-1">Failed to load definitions</p>
                    <p className="text-sm text-muted-foreground mb-4">{definitionsError.message}</p>
                    <Button onClick={() => refetchDefinitions()} variant="outline" size="sm">
                      <RefreshCw className="h-4 w-4 mr-2" />
                      Try Again
                    </Button>
                  </div>
                </div>
              )}

              {!definitionsLoading && !definitionsError && filteredDefinitions.length === 0 && (
                <div className="flex flex-col items-center justify-center py-12">
                  <Layers className="h-12 w-12 text-muted-foreground mb-4" />
                  <p className="text-lg font-semibold text-foreground mb-2">No definitions available</p>
                  <Button onClick={handleAddDefinition} variant="outline">
                    <Plus className="mr-2 h-4 w-4" />
                    Add Definition
                  </Button>
                </div>
              )}

              {!definitionsLoading && !definitionsError && filteredDefinitions.length > 0 && (
                <div className="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
                  {filteredDefinitions.map((def) => (
                    <Card key={def.ID}>
                      <CardContent className="p-4">
                        <div className="flex items-start justify-between mb-2">
                          <div className="font-semibold">{def.Name}</div>
                          <div className="flex gap-1">
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => handleEditDefinition(def)}
                              className="h-8 w-8 p-0"
                            >
                              <Pencil className="h-3 w-3" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => handleDeleteDefinitionClick(def)}
                              disabled={deleteDefinition.isPending}
                              className="h-8 w-8 p-0 text-destructive hover:text-destructive"
                            >
                              <Trash2 className="h-3 w-3" />
                            </Button>
                          </div>
                        </div>
                        <div className="space-y-1 text-sm text-muted-foreground">
                          <div>Min: {def.MinSize} MB/min</div>
                          <div>Preferred: {def.PreferredSize} MB/min</div>
                          <div>Max: {def.MaxSize} MB/min</div>
                        </div>
                      </CardContent>
                    </Card>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      <QualityProfileDialog
        open={profileDialogOpen}
        onOpenChange={setProfileDialogOpen}
        profile={editingProfile}
        mediaType={mediaType}
      />

      <QualityDefinitionDialog
        open={definitionDialogOpen}
        onOpenChange={setDefinitionDialogOpen}
        definition={editingDefinition}
        defaultMediaType={mediaType === 'series' ? 'episode' : 'movie'}
      />

      <AlertDialog open={deleteProfileDialogOpen} onOpenChange={setDeleteProfileDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Quality Profile</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete the profile{' '}
              <span className="font-semibold">{profileToDelete?.name}</span>?
              This action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handleDeleteProfileConfirm} className="bg-destructive">
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={deleteDefinitionDialogOpen} onOpenChange={setDeleteDefinitionDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Quality Definition</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete the definition{' '}
              <span className="font-semibold">{definitionToDelete?.Name}</span>?
              This will affect all profiles using this definition.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handleDeleteDefinitionConfirm} className="bg-destructive">
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
