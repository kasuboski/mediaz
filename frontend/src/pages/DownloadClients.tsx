import { useState } from 'react';
import { useDownloadClients, useDeleteDownloadClient } from '@/lib/queries';
import type { DownloadClient } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog';
import { Loader2, AlertCircle, RefreshCw, Plus, Pencil, Trash2, Download } from 'lucide-react';
import { toast } from 'sonner';
import { DownloadClientDialog } from '@/components/DownloadClientDialog';

export default function DownloadClients() {
  const { data: clients, isLoading, error, refetch } = useDownloadClients();
  const deleteClient = useDeleteDownloadClient();

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingClient, setEditingClient] = useState<DownloadClient | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [clientToDelete, setClientToDelete] = useState<DownloadClient | null>(null);

  const handleAddClient = () => {
    setEditingClient(null);
    setDialogOpen(true);
  };

  const handleEditClient = (client: DownloadClient) => {
    setEditingClient(client);
    setDialogOpen(true);
  };

  const handleDeleteClick = (client: DownloadClient) => {
    setClientToDelete(client);
    setDeleteDialogOpen(true);
  };

  const handleDeleteConfirm = async () => {
    if (!clientToDelete) return;

    try {
      await deleteClient.mutateAsync(clientToDelete.ID);
      toast.success('Download client deleted');
      setDeleteDialogOpen(false);
      setClientToDelete(null);
    } catch (error) {
      toast.error(`Failed to delete: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  };

  const getImplementationBadge = (Implementation: string) => {
    if (Implementation === 'transmission') {
      return <Badge variant="outline" className="bg-blue-500/10 text-blue-500 border-blue-500/20">Transmission</Badge>;
    }
    return <Badge variant="outline" className="bg-purple-500/10 text-purple-500 border-purple-500/20">SABnzbd</Badge>;
  };

  return (
    <div className="container mx-auto px-6 py-8">
      <div className="mb-8 flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-foreground mb-2">Download Clients</h1>
          <p className="text-muted-foreground">
            Manage download clients
          </p>
        </div>
        <Button onClick={handleAddClient} className="bg-gradient-primary hover:opacity-90">
          <Plus className="mr-2 h-4 w-4" />
          Add Download Client
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
              <p className="font-semibold text-destructive mb-1">Failed to load download clients</p>
              <p className="text-sm text-muted-foreground mb-4">{error.message}</p>
              <Button onClick={() => refetch()} variant="outline" size="sm">
                <RefreshCw className="h-4 w-4 mr-2" />
                Try Again
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {!isLoading && !error && (!clients || clients.length === 0) && (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <Download className="h-12 w-12 text-muted-foreground mb-4" />
            <p className="text-lg font-semibold text-foreground mb-2">No download clients configured</p>
            <Button onClick={handleAddClient} className="bg-gradient-primary hover:opacity-90">
              <Plus className="mr-2 h-4 w-4" />
              Add Download Client
            </Button>
          </CardContent>
        </Card>
      )}

      {!isLoading && !error && clients && clients.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Configured Clients</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Implementation</TableHead>
                  <TableHead>Connection</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {clients.map((client) => (
                  <TableRow key={client.ID}>
                    <TableCell>{getImplementationBadge(client.Implementation)}</TableCell>
                    <TableCell className="font-mono text-sm">
                      {client.Scheme}://{client.Host}{client.Port ? `:${client.Port}` : ''}
                    </TableCell>
                    <TableCell className="capitalize">{client.Type}</TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => handleEditClient(client)}
                        >
                          <Pencil className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="destructive"
                          size="sm"
                          onClick={() => handleDeleteClick(client)}
                          disabled={deleteClient.isPending}
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

      <DownloadClientDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        client={editingClient}
      />

      <AlertDialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Download Client</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete the download client{' '}
              <span className="font-semibold">{clientToDelete?.Implementation}</span> at{' '}
              <span className="font-mono">{clientToDelete?.Host}{clientToDelete?.Port ? `:${clientToDelete.Port}` : ''}</span>?
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
