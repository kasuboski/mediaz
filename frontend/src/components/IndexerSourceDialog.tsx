import { useState, useEffect } from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Loader2, CheckCircle, AlertCircle } from 'lucide-react';
import { toast } from 'sonner';
import type { IndexerSource, AddIndexerSourceRequest } from '@/lib/api';
import { useCreateIndexerSource, useUpdateIndexerSource, useTestIndexerSource } from '@/lib/queries';

interface IndexerSourceDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  source?: IndexerSource | null;
}

export function IndexerSourceDialog({ open, onOpenChange, source }: IndexerSourceDialogProps) {
  const [name, setName] = useState<string>('');
  const [implementation, setImplementation] = useState<string>('prowlarr');
  const [scheme, setScheme] = useState<string>('http');
  const [host, setHost] = useState<string>('');
  const [port, setPort] = useState<string>('');
  const [apiKey, setApiKey] = useState<string>('');
  const [enabled, setEnabled] = useState<boolean>(true);
  const [testStatus, setTestStatus] = useState<'idle' | 'testing' | 'success' | 'error'>('idle');

  const createSource = useCreateIndexerSource();
  const updateSource = useUpdateIndexerSource();
  const testConnection = useTestIndexerSource();

  useEffect(() => {
    if (open) {
      if (source) {
        setName(source.name);
        setImplementation(source.implementation);
        setScheme(source.scheme);
        setHost(source.host);
        setPort(source.port ? source.port.toString() : '');
        setApiKey('');
        setEnabled(source.enabled);
      } else {
        setName('');
        setImplementation('prowlarr');
        setScheme('http');
        setHost('');
        setPort('');
        setApiKey('');
        setEnabled(true);
      }
      setTestStatus('idle');
    }
  }, [open, source]);

  const handleTestConnection = async () => {
    if (!name) {
      toast.error('Name is required');
      return;
    }
    if (!host) {
      toast.error('Host is required');
      return;
    }
    if (!apiKey && !source) {
      toast.error('API key is required');
      return;
    }

    setTestStatus('testing');

    const request: AddIndexerSourceRequest = {
      name,
      implementation,
      scheme,
      host,
      port: port ? parseInt(port) : undefined,
      apiKey: apiKey || undefined,
      enabled,
    };

    try {
      await testConnection.mutateAsync(request);
      setTestStatus('success');
      toast.success('Connection successful!');
    } catch (error) {
      setTestStatus('error');
      toast.error(`Connection failed: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  };

  const handleSubmit = async () => {
    if (!name) {
      toast.error('Name is required');
      return;
    }
    if (!host) {
      toast.error('Host is required');
      return;
    }
    if (!apiKey && !source) {
      toast.error('API key is required');
      return;
    }

    const request: AddIndexerSourceRequest = {
      name,
      implementation,
      scheme,
      host,
      port: port ? parseInt(port) : undefined,
      apiKey: apiKey || undefined,
      enabled,
    };

    setTestStatus('testing');
    try {
      await testConnection.mutateAsync(request);
      setTestStatus('success');
    } catch (error) {
      setTestStatus('error');
      toast.error(`Connection test failed: ${error instanceof Error ? error.message : 'Unknown error'}`);
      return;
    }

    try {
      if (source) {
        await updateSource.mutateAsync({ id: source.id, request });
        toast.success('Indexer source updated');
      } else {
        await createSource.mutateAsync(request);
        toast.success('Indexer source created');
      }
      onOpenChange(false);
    } catch (error) {
      toast.error(`Failed to save: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  };

  const isLoading = createSource.isPending || updateSource.isPending;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>{source ? 'Edit Indexer Source' : 'Add Indexer Source'}</DialogTitle>
        </DialogHeader>

        <div className="grid gap-4 py-4">
          <div className="grid gap-2">
            <Label htmlFor="name">Name</Label>
            <Input
              id="name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="My Prowlarr"
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="implementation">Implementation</Label>
            <Select value={implementation} defaultValue="prowlarr" onValueChange={setImplementation} disabled={!!source}>
              <SelectTrigger id="implementation">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="prowlarr">Prowlarr</SelectItem>
              </SelectContent>
            </Select>
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
              placeholder="prowlarr.my-domain.com or 192.168.1.100"
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="port">Port (optional)</Label>
            <Input
              id="port"
              type="text"
              inputMode="numeric"
              pattern="[0-9]*"
              value={port}
              onChange={(e) => setPort(e.target.value)}
              placeholder="9696"
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="apiKey">API Key</Label>
            <Input
              id="apiKey"
              type="password"
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              placeholder={source ? '' : 'required'}
            />
          </div>

          <div className="flex items-center justify-between">
            <Label htmlFor="enabled" className="cursor-pointer">Enabled</Label>
            <Switch
              id="enabled"
              checked={enabled}
              onCheckedChange={setEnabled}
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={isLoading || testStatus === 'testing'}>
            Cancel
          </Button>
          <Button
            type="button"
            variant="outline"
            onClick={handleTestConnection}
            disabled={testStatus === 'testing' || !host || !name || isLoading}
          >
            {testStatus === 'testing' && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {testStatus === 'success' && <CheckCircle className="mr-2 h-4 w-4 text-green-500" />}
            {testStatus === 'error' && <AlertCircle className="mr-2 h-4 w-4 text-red-500" />}
            Test Connection
          </Button>
          <Button onClick={handleSubmit} disabled={isLoading || testStatus === 'testing'}>
            {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {source ? 'Update' : 'Create'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
