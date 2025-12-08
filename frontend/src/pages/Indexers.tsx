import { useState } from 'react';
import { useIndexers, useDeleteIndexer } from '@/lib/queries';
import type { Indexer } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog';
import { Loader2, AlertCircle, RefreshCw, Plus, Pencil, Trash2, Database } from 'lucide-react';
import { toast } from 'sonner';
import { IndexerDialog } from '@/components/IndexerDialog';

export default function Indexers() {
  const { data: indexers, isLoading, error, refetch } = useIndexers();
  const deleteIndexer = useDeleteIndexer();

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingIndexer, setEditingIndexer] = useState<Indexer | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [indexerToDelete, setIndexerToDelete] = useState<Indexer | null>(null);

  const handleAddIndexer = () => {
    setEditingIndexer(null);
    setDialogOpen(true);
  };

  const handleEditIndexer = (indexer: Indexer) => {
    setEditingIndexer(indexer);
    setDialogOpen(true);
  };

  const handleDeleteClick = (indexer:   Indexer) => {
    setIndexerToDelete(indexer);
    setDeleteDialogOpen(true);
  };

  const handleDeleteConfirm = async () => {
    if (!indexerToDelete) return;

    try {
      await deleteIndexer.mutateAsync(indexerToDelete.id);
      toast.success('Indexer deleted');
      setDeleteDialogOpen(false);
      setIndexerToDelete(null);
    } catch (error) {
      toast.error(`Failed to delete: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  };

  return (
    <div className="container mx-auto px-6 py-8">
      <div className="mb-8 flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-foreground mb-2">Indexers</h1>
          <p className="text-muted-foreground">
            Manage indexers
          </p>
        </div>
        <Button onClick={handleAddIndexer} className="bg-gradient-primary hover:opacity-90">
          <Plus className="mr-2 h-4 w-4" />
          Add Indexer
        </Button>
      </div>

      {isLoading && (
        <Card>
          <CardContent className="flex items-center justify-center py-12">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </CardContent>
        </Card>
      )}

      {error && (
        <Card className="border-destructive">
          <CardContent className="flex flex-col items-center justify-center py-12 gap-4">
            <AlertCircle className="h-12 w-12 text-destructive" />
            <div className="text-center">
              <p className="font-semibold text-destructive mb-1">Failed to load indexers</p>
              <p className="text-sm text-muted-foreground mb-4">{error.message}</p>
              <Button onClick={() => refetch()} variant="outline" size="sm">
                <RefreshCw className="h-4 w-4 mr-2" />
                Try Again
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {!isLoading && !error && (!indexers || indexers.length === 0) && (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <Database className="h-12 w-12 text-muted-foreground mb-4" />
            <p className="text-lg font-semibold text-foreground mb-2">No indexers configured</p>
            <Button onClick={handleAddIndexer} className="bg-gradient-primary hover:opacity-90">
              <Plus className="mr-2 h-4 w-4" />
              Add Indexer
            </Button>
          </CardContent>
        </Card>
      )}

      {!isLoading && !error && indexers && indexers.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Configured Indexers</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>URL</TableHead>
                  <TableHead>Priority</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {indexers.map((indexer) => (
                  <TableRow key={indexer.id}>
                    <TableCell className="font-medium">{indexer.name}</TableCell>
                    <TableCell>{indexer.uri}</TableCell>
                    <TableCell>{indexer.priority}</TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => handleEditIndexer(indexer)}
                        >
                          <Pencil className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="destructive"
                          size="sm"
                          onClick={() => handleDeleteClick(indexer)}
                          disabled={deleteIndexer.isPending}
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

      <IndexerDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        indexer={editingIndexer}
      />

      <AlertDialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Indexer</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete the indexer{' '}
              <span className="font-semibold">{indexerToDelete?.name}</span>?
              This action cannot be undone.
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
