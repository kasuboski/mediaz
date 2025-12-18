import { useState, useEffect } from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Loader2 } from 'lucide-react';
import { toast } from 'sonner';
import type { Indexer, IndexerRequest } from '@/lib/api';
import { useCreateIndexer, useUpdateIndexer } from '@/lib/queries';

interface IndexerDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  indexer?: Indexer | null;
}

export function IndexerDialog({ open, onOpenChange, indexer }: IndexerDialogProps) {
  const [name, setName] = useState<string>('');
  const [scheme, setScheme] = useState<string>('http');
  const [host, setHost] = useState<string>('');
  const [priority, setPriority] = useState<string>('25');
  const [apiKey, setApiKey] = useState<string>('');

  const createIndexer = useCreateIndexer();
  const updateIndexer = useUpdateIndexer();

  useEffect(() => {
    if (open) {
      if (indexer) {
        setName(indexer.name);
        setPriority(indexer.priority.toString());
        setApiKey('');

        try {
          const url = new URL(indexer.uri);
          setScheme(url.protocol.replace(':', ''));
          setHost(url.host);
        } catch {
          setScheme('http');
          setHost(indexer.uri);
        }
      } else {
        setName('');
        setScheme('http');
        setHost('');
        setPriority('25');
        setApiKey('');
      }
    }
  }, [open, indexer]);

  const handleSubmit = async () => {
    if (!name) {
      toast.error('Name is required');
      return;
    }
    if (!host) {
      toast.error('Host is required');
      return;
    }

    const parsedPriority = parseInt(priority) || 25;
    const uri = `${scheme}://${host}`;

    const request: IndexerRequest = {
      name,
      uri,
      priority: parsedPriority,
    };

    if (apiKey) {
      request.api_key = apiKey;
    }

    try {
      if (indexer) {
        await updateIndexer.mutateAsync({ id: indexer.id, request });
        toast.success('Indexer updated');
      } else {
        await createIndexer.mutateAsync(request);
        toast.success('Indexer created');
      }
      onOpenChange(false);
    } catch (error) {
      toast.error(`Failed to save: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  };

  const isLoading = createIndexer.isPending || updateIndexer.isPending;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>{indexer ? 'Edit Indexer' : 'Add Indexer'}</DialogTitle>
        </DialogHeader>

        <div className="grid gap-4 py-4">
          <div className="grid gap-2">
            <Label htmlFor="name">Name</Label>
            <Input
              id="name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="My Indexer"
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="scheme">Protocol</Label>
            <Select value={scheme} defaultValue="http" onValueChange={setScheme}>
              <SelectTrigger id="scheme">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="http">HTTP</SelectItem>
                <SelectItem value="https">HTTPS</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="host">Host</Label>
            <Input
              id="host"
              value={host}
              onChange={(e) => setHost(e.target.value)}
              placeholder="prowlarr.example.com or 192.168.1.100"
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="priority">Priority</Label>
            <Input
              id="priority"
              type="text"
              inputMode="numeric"
              pattern="[0-9]*"
              value={priority}
              onChange={(e) => setPriority(e.target.value)}
              placeholder="e.g., Low (10), Medium (25), High (50)"
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="apiKey">API Key {indexer && '(optional)'}</Label>
            <Input
              id="apiKey"
              type="password"
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              placeholder={indexer ? 'Enter to update' : ''}
            />
            {indexer && (
              <p className="text-sm text-muted-foreground">
                Leave empty to keep existing API key
              </p>
            )}
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={isLoading}>
            Cancel
          </Button>
          <Button onClick={handleSubmit} disabled={isLoading}>
            {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {indexer ? 'Update' : 'Create'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
