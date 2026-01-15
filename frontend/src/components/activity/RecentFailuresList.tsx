import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { queryKeys } from '@/lib/queries';
import { type FailureItem } from '@/lib/api';
import {
  AlertTriangle,
  Server,
  Film,
  Tv,
  RefreshCw,
  ChevronDown,
  ChevronUp,
  CheckCircle2,
} from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { cn } from '@/lib/utils';

interface RecentFailuresListProps {
  failures: FailureItem[];
}

function getFailureIcon(type: string) {
  switch (type) {
    case 'job':
      return Server;
    case 'movie':
      return Film;
    case 'series':
      return Tv;
    default:
      return AlertTriangle;
  }
}

function getSeverityBadgeVariant(state: string): 'default' | 'secondary' | 'destructive' | 'outline' {
  if (state === 'error') return 'destructive';
  if (state === 'stuck') return 'secondary';
  return 'default';
}

function formatRelativeTime(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMins / 60);
  const diffDays = Math.floor(diffHours / 24);

  if (diffMins < 1) return 'just now';
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays < 7) return `${diffDays}d ago`;
  return date.toLocaleDateString();
}

function formatAbsoluteTime(dateString: string): string {
  return new Date(dateString).toLocaleString();
}

function FailureCard({ failure }: { failure: FailureItem }) {
  const [isExpanded, setIsExpanded] = useState(false);
  const [isRetrying, setIsRetrying] = useState(false);
  const queryClient = useQueryClient();

  const Icon = getFailureIcon(failure.type);
  const badgeVariant = getSeverityBadgeVariant(failure.state);

  const handleRetry = async () => {
    setIsRetrying(true);
    console.log('Retrying failure:', failure);
    console.log('  - Type:', failure.type);
    console.log('  - ID:', failure.id);
    console.log('  - Title:', failure.title);
    
    await queryClient.invalidateQueries({ queryKey: queryKeys.activity.failures(24) });
    setIsRetrying(false);
  };

  return (
    <Card className="overflow-hidden">
      <div className="p-4">
        <div className="flex items-center justify-between gap-4">
          <div className="flex items-center gap-3 flex-1 min-w-0">
            <Icon className="h-5 w-5 text-destructive flex-shrink-0" />
            <div className="flex flex-col min-w-0">
              <div className="flex items-center gap-2">
                <h4 className="font-medium truncate">{failure.title}</h4>
                <Badge variant={badgeVariant} className="text-xs">
                  {failure.state}
                </Badge>
              </div>
              <p className="text-sm text-muted-foreground truncate">
                {failure.subtitle}
              </p>
            </div>
          </div>
          
          <div className="flex items-center gap-2 flex-shrink-0">
            <span className="text-sm text-muted-foreground">
              {formatRelativeTime(failure.failedAt)}
            </span>
            
            {failure.retryable && (
              <Button
                variant="ghost"
                size="sm"
                onClick={handleRetry}
                disabled={isRetrying}
                className="h-8 px-2"
              >
                <RefreshCw className={cn('h-4 w-4', isRetrying && 'animate-spin')} />
              </Button>
            )}
            
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setIsExpanded(!isExpanded)}
              className="h-8 px-2"
            >
              {isExpanded ? (
                <ChevronUp className="h-4 w-4" />
              ) : (
                <ChevronDown className="h-4 w-4" />
              )}
            </Button>
          </div>
        </div>
        
        {isExpanded && (
          <div className="mt-4 space-y-4">
            <div>
              <p className="text-xs text-muted-foreground mb-2">Error Message</p>
              <div className="rounded-md bg-muted p-3">
                <code className="text-sm break-all font-mono">{failure.error}</code>
              </div>
            </div>
            
            <div className="grid grid-cols-2 gap-4 text-sm">
              <div>
                <p className="text-xs text-muted-foreground mb-1">Type</p>
                <p className="font-medium capitalize">{failure.type}</p>
              </div>
              <div>
                <p className="text-xs text-muted-foreground mb-1">ID</p>
                <p className="font-medium">{failure.id}</p>
              </div>
              <div>
                <p className="text-xs text-muted-foreground mb-1">State</p>
                <p className="font-medium">{failure.state}</p>
              </div>
              <div>
                <p className="text-xs text-muted-foreground mb-1">Failed At</p>
                <p className="font-medium">{formatAbsoluteTime(failure.failedAt)}</p>
              </div>
            </div>
            
            {failure.retryable && (
              <div className="flex justify-end">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleRetry}
                  disabled={isRetrying}
                >
                  <RefreshCw className={cn('mr-2 h-4 w-4', isRetrying && 'animate-spin')} />
                  Retry
                </Button>
              </div>
            )}
          </div>
        )}
      </div>
    </Card>
  );
}

export function RecentFailuresList({ failures }: RecentFailuresListProps) {
  if (!failures || failures.length === 0) {
    return (
      <Card className="p-8">
        <div className="flex flex-col items-center justify-center gap-3 text-center">
          <CheckCircle2 className="h-12 w-12 text-muted-foreground" />
          <h3 className="font-semibold text-lg">No Recent Failures</h3>
          <p className="text-sm text-muted-foreground max-w-sm">
            Everything is running smoothly. Check back later if you encounter any issues.
          </p>
        </div>
      </Card>
    );
  }

  return (
    <div className="space-y-3">
      {failures.map((failure) => (
        <FailureCard key={`${failure.type}-${failure.id}`} failure={failure} />
      ))}
    </div>
  );
}
