import { useState } from 'react';
import { useIndexers, useIndexerSources, useDeleteIndexerSource, useRefreshIndexerSource } from '@/lib/queries';
import type { IndexerSource } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog';
import { Loader2, AlertCircle, RefreshCw, Plus, Pencil, Trash2, ScanSearch } from 'lucide-react';
import { toast } from 'sonner';
import { IndexerSourceDialog } from '@/components/IndexerSourceDialog';

export default function Indexers() {
  const { data: indexers, isLoading: indexersLoading, error: indexersError } = useIndexers();
  const { data: sources, isLoading: sourcesLoading, error: sourcesError, refetch: refetchSources } = useIndexerSources();
  const deleteSource = useDeleteIndexerSource();
  const refreshSource = useRefreshIndexerSource();

  const [sourceDialogOpen, setSourceDialogOpen] = useState(false);
  const [editingSource, setEditingSource] = useState<IndexerSource | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [sourceToDelete, setSourceToDelete] = useState<IndexerSource | null>(null);

  const handleAddSource = () => {
    setEditingSource(null);
    setSourceDialogOpen(true);
  };

  const handleEditSource = (source: IndexerSource) => {
    setEditingSource(source);
    setSourceDialogOpen(true);
  };

  const handleDeleteClick = (source: IndexerSource) => {
    setSourceToDelete(source);
    setDeleteDialogOpen(true);
  };

  const handleDeleteConfirm = async () => {
    if (!sourceToDelete) return;

    try {
      await deleteSource.mutateAsync(sourceToDelete.id);
      toast.success('Indexer source deleted');
      setDeleteDialogOpen(false);
      setSourceToDelete(null);
    } catch (error) {
      toast.error(`Failed to delete: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  };

  const handleRefreshSource = async (id: number) => {
    try {
      await refreshSource.mutateAsync(id);
      toast.success('Indexer source refreshed');
    } catch (error) {
      toast.error(`Failed to refresh: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  };

  return (
    <div className="container mx-auto px-6 py-8">
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-foreground mb-2">Indexers</h1>
        <p className="text-muted-foreground">
          Manage indexer sources
        </p>
      </div>

      {/* Indexer Sources Section */}
      <div className="mb-8">
        {sourcesLoading && (
          <Card>
            <CardContent className="flex items-center justify-center py-12">
              <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            </CardContent>
          </Card>
        )}

        {sourcesError && (
          <Card className="border-destructive">
            <CardContent className="flex flex-col items-center justify-center py-12 gap-4">
              <AlertCircle className="h-12 w-12 text-destructive" />
              <div className="text-center">
                <p className="font-semibold text-destructive mb-1">Failed to load indexer sources</p>
                <p className="text-sm text-muted-foreground mb-4">{sourcesError.message}</p>
                <Button onClick={() => refetchSources()} variant="outline" size="sm">
                  <RefreshCw className="h-4 w-4 mr-2" />
                  Try Again
                </Button>
              </div>
            </CardContent>
          </Card>
        )}

        {!sourcesLoading && !sourcesError && (!sources || sources.length === 0) && (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12">
              <ScanSearch className="h-12 w-12 text-muted-foreground mb-4" />
              <p className="text-lg font-semibold text-foreground mb-2">No indexer sources configured</p>
              <Button onClick={handleAddSource} className="bg-gradient-primary hover:opacity-90">
                <Plus className="mr-2 h-4 w-4" />
                Add Source
              </Button>
            </CardContent>
          </Card>
        )}

        {!sourcesLoading && !sourcesError && sources && sources.length > 0 && (
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle>Indexer Sources</CardTitle>
                  <CardDescription>Configure external indexer providers</CardDescription>
                </div>
                <Button onClick={handleAddSource} className="bg-gradient-primary hover:opacity-90">
                  <Plus className="mr-2 h-4 w-4" />
                  Add Source
                </Button>
              </div>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Implementation</TableHead>
                    <TableHead>Host</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {sources.map((source) => (
                    <TableRow key={source.id}>
                      <TableCell className="font-medium">{source.name}</TableCell>
                      <TableCell>
                        <Badge variant="secondary">{source.implementation}</Badge>
                      </TableCell>
                      <TableCell>{source.scheme}://{source.host}{source.port ? `:${source.port}` : ''}</TableCell>
                      <TableCell>
                        <Badge variant={source.enabled ? 'default' : 'outline'}>
                          {source.enabled ? 'Enabled' : 'Disabled'}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex justify-end gap-2">
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => handleRefreshSource(source.id)}
                            disabled={refreshSource.isPending}
                          >
                            <RefreshCw className="h-4 w-4" />
                          </Button>
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => handleEditSource(source)}
                          >
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Button
                            variant="destructive"
                            size="sm"
                            onClick={() => handleDeleteClick(source)}
                            disabled={deleteSource.isPending}
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        )}
      </div>

      {/* Indexers Section */}
      <div>
        {indexersLoading && (
          <Card>
            <CardContent className="flex items-center justify-center py-12">
              <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            </CardContent>
          </Card>
        )}

        {indexersError && (
          <Card className="border-destructive">
            <CardContent className="flex flex-col items-center justify-center py-12 gap-4">
              <AlertCircle className="h-12 w-12 text-destructive" />
              <div className="text-center">
                <p className="font-semibold text-destructive mb-1">Failed to load indexers</p>
                <p className="text-sm text-muted-foreground">{indexersError.message}</p>
              </div>
            </CardContent>
          </Card>
        )}

        {!indexersLoading && !indexersError && (!indexers || indexers.length === 0) && (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12">
              <ScanSearch className="h-12 w-12 text-muted-foreground mb-4" />
              <p className="text-lg font-semibold text-foreground mb-2">No indexers available</p>
              <p className="text-sm text-muted-foreground">Add and sync an indexer source to see indexers here</p>
            </CardContent>
          </Card>
        )}

        {!indexersLoading && !indexersError && indexers && indexers.length > 0 && (
          <Card>
            <CardHeader>
              <CardTitle>Available Indexers</CardTitle>
              <CardDescription>{indexers.length} indexer{indexers.length !== 1 ? 's' : ''} from configured sources</CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Source</TableHead>
                    <TableHead>URI</TableHead>
                    <TableHead>Priority</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {indexers.map((indexer) => (
                    <TableRow key={indexer.id}>
                      <TableCell className="font-medium">{indexer.name}</TableCell>
                      <TableCell><Badge variant="secondary">{indexer.source}</Badge></TableCell>
                      <TableCell>{indexer.uri}</TableCell>
                      <TableCell>{indexer.priority}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        )}
      </div>

      <IndexerSourceDialog
        open={sourceDialogOpen}
        onOpenChange={setSourceDialogOpen}
        source={editingSource}
      />

      <AlertDialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Indexer Source</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete the indexer source{' '}
              <span className="font-semibold">{sourceToDelete?.name}</span>?
              This will also remove all indexers from this source. This action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handleDeleteConfirm} className="bg-destructive">
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
