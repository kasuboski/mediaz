import { useState, useEffect } from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Loader2, CheckCircle, AlertCircle } from 'lucide-react';
import { toast } from 'sonner';
import type { DownloadClient, CreateDownloadClientRequest } from '@/lib/api';
import { useCreateDownloadClient, useUpdateDownloadClient, useTestDownloadClient } from '@/lib/queries';

interface DownloadClientDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  client?: DownloadClient | null;
}

export function DownloadClientDialog({ open, onOpenChange, client }: DownloadClientDialogProps) {
  const [implementation, setImplementation] = useState<string>('transmission');
  const [scheme, setScheme] = useState<string>('http');
  const [host, setHost] = useState<string>('');
  const [port, setPort] = useState<string>('');
  const [apiKey, setApiKey] = useState<string>('');
  const [testStatus, setTestStatus] = useState<'idle' | 'testing' | 'success' | 'error'>('idle');

  const createClient = useCreateDownloadClient();
  const updateClient = useUpdateDownloadClient();
  const testConnection = useTestDownloadClient();

  useEffect(() => {
    if (open) {
      if (client) {
        setImplementation(client.Implementation);
        setScheme(client.Scheme);
        setHost(client.Host);
        setPort(client.Port ? client.Port.toString() : '');
        setApiKey('');
      } else {
        setImplementation('transmission');
        setScheme('http');
        setHost('');
        setPort('');
        setApiKey('');
      }
      setTestStatus('idle');
    }
  }, [open, client]);

  const handleTestConnection = async () => {
    if (!host) {
      toast.error('Host is required');
      return;
    }
    if (implementation === 'sabnzbd' && !apiKey && !client) {
      toast.error('API key is required for SABnzbd');
      return;
    }

    setTestStatus('testing');

    const request: CreateDownloadClientRequest = {
      type: implementation === 'transmission' ? 'torrent' : 'usenet',
      implementation,
      scheme,
      host,
      port: port ? parseInt(port) : 0,
      apiKey: implementation === 'sabnzbd' ? (apiKey || client?.APIKey || null) : null,
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
    if (!host) {
      toast.error('Host is required');
      return;
    }
    if (implementation === 'sabnzbd' && !apiKey && !client) {
      toast.error('API key is required for SABnzbd');
      return;
    }

    const request: CreateDownloadClientRequest = {
      type: implementation === 'transmission' ? 'torrent' : 'usenet',
      implementation,
      scheme,
      host,
      port: port ? parseInt(port) : 0,
      apiKey: implementation === 'sabnzbd' ? (apiKey || null) : null,
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
      if (client) {
        await updateClient.mutateAsync({ id: client.ID, request: { ...request, id: client.ID } });
        toast.success('Download client updated');
      } else {
        await createClient.mutateAsync(request);
        toast.success('Download client created');
      }
      onOpenChange(false);
    } catch (error) {
      toast.error(`Failed to save: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  };

  const isLoading = createClient.isPending || updateClient.isPending;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>{client ? 'Edit Download Client' : 'Add Download Client'}</DialogTitle>
        </DialogHeader>

        <div className="grid gap-4 py-4">
          <div className="grid gap-2">
            <Label htmlFor="implementation">Implementation</Label>
            <Select value={implementation} onValueChange={setImplementation} disabled={!!client}>
              <SelectTrigger id="implementation">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="transmission">Transmission (BitTorrent)</SelectItem>
                <SelectItem value="sabnzbd">SABnzbd (Usenet)</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="scheme">Protocol</Label>
            <Select value={scheme} onValueChange={setScheme}>
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
              placeholder="transmission.my-domain.com or 192.168.1.100"
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
              placeholder={implementation === 'transmission' ? '9091' : '8080'}
            />
          </div>

          {implementation === 'sabnzbd' && (
            <div className="grid gap-2">
              <Label htmlFor="apiKey">API Key</Label>
              <Input
                id="apiKey"
                type="password"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                placeholder={client ? 'Enter new API key to update' : 'Required for SABnzbd'}
              />
              {client && (
                <p className="text-sm text-muted-foreground">
                  Leave empty to keep existing API key
                </p>
              )}
            </div>
          )}

        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={isLoading || testStatus === 'testing'}>
            Cancel
          </Button>
          <Button
            type="button"
            variant="outline"
            onClick={handleTestConnection}
            disabled={testStatus === 'testing' || !host || isLoading}
          >
            {testStatus === 'testing' && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {testStatus === 'success' && <CheckCircle className="mr-2 h-4 w-4 text-green-500" />}
            {testStatus === 'error' && <AlertCircle className="mr-2 h-4 w-4 text-red-500" />}
            Test Connection
          </Button>
          <Button onClick={handleSubmit} disabled={isLoading || testStatus === 'testing'}>
            {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {client ? 'Update' : 'Create'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
