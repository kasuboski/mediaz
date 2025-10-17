import { useQuery } from "@tanstack/react-query";
import { libraryApi } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Loader2, Film, Tv, FolderOpen, Clock, CheckCircle, Eye, AlertCircle } from "lucide-react";

export default function LibraryPage() {
  const {
    data: config,
    isLoading: configLoading,
    error: configError,
  } = useQuery({
    queryKey: ["library-config"],
    queryFn: libraryApi.getConfig,
  });

  const {
    data: stats,
    isLoading: statsLoading,
    error: statsError,
  } = useQuery({
    queryKey: ["library-stats"],
    queryFn: libraryApi.getLibraryStats,
  });

  if (configLoading || statsLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="h-8 w-8 animate-spin" />
      </div>
    );
  }

  if (configError || statsError) {
    return (
      <div className="text-center text-destructive p-8">
        <p>Error loading library information</p>
      </div>
    );
  }

  return (
    <div className="container mx-auto px-4 py-8 space-y-8">
      <div>
        <h1 className="text-3xl font-bold mb-2">Library Configuration</h1>
        <p className="text-muted-foreground">
          View your library paths, server settings, and media statistics
        </p>
      </div>

      {/* Configuration Section */}
      <div className="grid gap-6 md:grid-cols-2">
        <Card>
          <CardHeader className="flex flex-row items-center space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Library Configuration</CardTitle>
            <FolderOpen className="ml-auto h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-3">
              <div>
                <p className="text-xs text-muted-foreground mb-1">Movies Directory</p>
                <div className="flex items-center gap-2">
                  <Film className="h-4 w-4 text-blue-500" />
                  <p className="text-sm font-mono bg-muted px-2 py-1 rounded">
                    {config?.library.movieDir || "Using default location"}
                  </p>
                  {!config?.library.movieDir && (
                    <AlertCircle className="h-4 w-4 text-yellow-500" title="Using default or relative path" />
                  )}
                </div>
              </div>
              <div>
                <p className="text-xs text-muted-foreground mb-1">TV Shows Directory</p>
                <div className="flex items-center gap-2">
                  <Tv className="h-4 w-4 text-orange-500" />
                  <p className="text-sm font-mono bg-muted px-2 py-1 rounded">
                    {config?.library.tvDir || "Using default location"}
                  </p>
                  {!config?.library.tvDir && (
                    <AlertCircle className="h-4 w-4 text-yellow-500" title="Using default or relative path" />
                  )}
                </div>
              </div>
            </div>
            {!config?.library.movieDir && !config?.library.tvDir && (
              <div className="text-xs text-muted-foreground bg-blue-500/10 border border-blue-500/20 rounded p-2">
                <AlertCircle className="h-3 w-3 inline mr-1" />
                Library paths are using defaults or relative paths. Your library may still be functional.
              </div>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Job Schedule</CardTitle>
            <Clock className="ml-auto h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="grid grid-cols-2 gap-3">
              <div>
                <p className="text-xs text-muted-foreground">Movie Index</p>
                <p className="text-sm font-semibold text-blue-400">{config?.jobs.movieIndex}</p>
              </div>
              <div>
                <p className="text-xs text-muted-foreground">Movie Reconcile</p>
                <p className="text-sm font-semibold text-blue-400">{config?.jobs.movieReconcile}</p>
              </div>
              <div>
                <p className="text-xs text-muted-foreground">Series Index</p>
                <p className="text-sm font-semibold text-orange-400">{config?.jobs.seriesIndex}</p>
              </div>
              <div>
                <p className="text-xs text-muted-foreground">Series Reconcile</p>
                <p className="text-sm font-semibold text-orange-400">{config?.jobs.seriesReconcile}</p>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Statistics Section */}
      <div>
        <h2 className="text-2xl font-bold mb-6">Library Statistics</h2>
        
        <div className="grid gap-6 md:grid-cols-2">
          {/* Movies Stats */}
          <Card>
            <CardHeader className="flex flex-row items-center space-y-0">
              <Film className="h-5 w-5" />
              <CardTitle className="ml-2">Movies</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-2xl font-bold text-blue-400">{stats?.movies.total}</p>
                  <p className="text-xs text-muted-foreground">Total Movies</p>
                </div>
                <Film className="h-8 w-8 text-blue-500/20" />
              </div>
              <div className="space-y-2">
                <p className="text-sm font-medium text-muted-foreground">Breakdown by Status</p>
                <div className="space-y-1">
                  {Object.entries(stats?.movies.byState || {}).map(([state, count]) => {
                    const percentage = Math.round((count / (stats?.movies.total || 1)) * 100);
                    const getColor = (state: string) => {
                      switch (state) {
                        case 'downloaded': return 'bg-green-500';
                        case 'downloading': return 'bg-blue-500';
                        case 'discovered': return 'bg-gray-500';
                        case 'missing': return 'bg-red-500';
                        case 'unreleased': return 'bg-yellow-500';
                        default: return 'bg-gray-500';
                      }
                    };
                    
                    return (
                      <div key={state} className="flex items-center gap-3">
                        <div className="w-20 text-xs text-muted-foreground capitalize">{state}</div>
                        <div className="flex-1 bg-muted rounded-full h-2 overflow-hidden">
                          <div 
                            className={`h-full ${getColor(state)} transition-all duration-500`}
                            style={{ width: `${percentage}%` }}
                          />
                        </div>
                        <div className="w-12 text-xs text-right font-medium">{count}</div>
                        <div className="w-10 text-xs text-muted-foreground text-right">{percentage}%</div>
                      </div>
                    );
                  })}
                </div>
              </div>
            </CardContent>
          </Card>

          {/* TV Stats */}
          <Card>
            <CardHeader className="flex flex-row items-center space-y-0">
              <Tv className="h-5 w-5" />
              <CardTitle className="ml-2">TV Shows</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-2xl font-bold text-orange-400">{stats?.tv.total}</p>
                  <p className="text-xs text-muted-foreground">Total Series</p>
                </div>
                <Tv className="h-8 w-8 text-orange-500/20" />
              </div>
              <div className="space-y-2">
                <p className="text-sm font-medium text-muted-foreground">Breakdown by Status</p>
                <div className="space-y-1">
                  {Object.entries(stats?.tv.byState || {}).map(([state, count]) => {
                    const percentage = Math.round((count / (stats?.tv.total || 1)) * 100);
                    const getColor = (state: string) => {
                      switch (state) {
                        case 'completed': return 'bg-green-500';
                        case 'continuing': return 'bg-blue-500';
                        case 'discovered': return 'bg-gray-500';
                        case 'missing': return 'bg-red-500';
                        default: return 'bg-gray-500';
                      }
                    };
                    
                    return (
                      <div key={state} className="flex items-center gap-3">
                        <div className="w-20 text-xs text-muted-foreground capitalize">{state}</div>
                        <div className="flex-1 bg-muted rounded-full h-2 overflow-hidden">
                          <div 
                            className={`h-full ${getColor(state)} transition-all duration-500`}
                            style={{ width: `${percentage}%` }}
                          />
                        </div>
                        <div className="w-12 text-xs text-right font-medium">{count}</div>
                        <div className="w-10 text-xs text-muted-foreground text-right">{percentage}%</div>
                      </div>
                    );
                  })}
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>

      {/* Summary Cards */}
      <div className="grid gap-4 md:grid-cols-3">
        <Card className="bg-gradient-to-br from-blue-500/10 to-blue-600/5 border-blue-500/20">
          <CardContent className="p-6">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-blue-400">Library Overview</p>
                <p className="text-2xl font-bold text-blue-400">
                  {((stats?.movies.total || 0) + (stats?.tv.total || 0)).toLocaleString()}
                </p>
                <p className="text-xs text-muted-foreground">Total Items</p>
              </div>
              <div className="flex -space-x-2">
                <Film className="h-8 w-8 text-blue-500" />
                <Tv className="h-8 w-8 text-orange-500" />
              </div>
            </div>
          </CardContent>
        </Card>

        <Card className="bg-gradient-to-br from-green-500/10 to-green-600/5 border-green-500/20">
          <CardContent className="p-6">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-green-400">Available to Watch</p>
                <p className="text-2xl font-bold text-green-400">
                  {((stats?.movies.byState.discovered || 0) + (stats?.tv.byState.completed || 0) + (stats?.tv.byState.continuing || 0)).toLocaleString()}
                </p>
                <p className="text-xs text-muted-foreground">Ready Now</p>
              </div>
              <CheckCircle className="h-8 w-8 text-green-500" />
            </div>
            <div className="mt-3 space-y-1 text-xs text-muted-foreground">
              <div>• {stats?.movies.byState.discovered} movies discovered</div>
              <div>• {(stats?.tv.byState.completed || 0) + (stats?.tv.byState.continuing || 0)} TV series available</div>
            </div>
          </CardContent>
        </Card>

        <Card className="bg-gradient-to-br from-orange-500/10 to-orange-600/5 border-orange-500/20">
          <CardContent className="p-6">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-orange-400">Still Missing</p>
                <p className="text-2xl font-bold text-orange-400">
                  {(stats?.tv.byState.missing || 0).toLocaleString()}
                </p>
                <p className="text-xs text-muted-foreground">Content to Find</p>
              </div>
              <AlertCircle className="h-8 w-8 text-orange-500" />
            </div>
            <div className="mt-3 space-y-1 text-xs text-muted-foreground">
              <div>• {stats?.tv.byState.missing} TV series missing</div>
              <div>• {stats?.movies.byState.downloaded || 0} movies downloaded by system</div>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}