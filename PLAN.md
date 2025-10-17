# Development Plan: Library Configuration & Stats Page

## Overview
Implement a readonly Library page (`/library`) that displays:
- Library configuration (paths, server settings, job intervals)
- Library statistics (movie/TV counts by state)

## Current State
- **Frontend**: `/library` route returns 404
- **Backend**: No endpoints exist to expose library config or aggregate statistics
- **Available Data**: Config is loaded at startup; database tracks movies/TV shows by state, quality profiles, indexers, and download clients

---

## Backend Changes (Minimal)

### 1. Add Response Types
**File**: `pkg/manager/manager_types.go`

Add new type definitions:
```go
// Configuration summary (excludes sensitive data like API keys)
type ConfigSummary struct {
    Library LibraryConfig `json:"library"`
    Server  ServerConfig  `json:"server"`
    Jobs    JobsConfig    `json:"jobs"`
}

type LibraryConfig struct {
    MovieDir         string `json:"movieDir"`
    TVDir            string `json:"tvDir"`
    DownloadMountDir string `json:"downloadMountDir"`
}

type ServerConfig struct {
    Port int `json:"port"`
}

type JobsConfig struct {
    MovieReconcile  string `json:"movieReconcile"`
    MovieIndex      string `json:"movieIndex"`
    SeriesReconcile string `json:"seriesReconcile"`
    SeriesIndex     string `json:"seriesIndex"`
}

// Library statistics
type LibraryStats struct {
    Movies MovieStats `json:"movies"`
    TV     TVStats    `json:"tv"`
}

type MovieStats struct {
    Total   int            `json:"total"`
    ByState map[string]int `json:"byState"`
}

type TVStats struct {
    Total   int            `json:"total"`
    ByState map[string]int `json:"byState"`
}
```

### 2. Add Manager Methods
**File**: `pkg/manager/manager.go`

Add method to get config summary:
```go
func (m MediaManager) GetConfigSummary() ConfigSummary {
    return ConfigSummary{
        Library: LibraryConfig{
            MovieDir:         m.config.Library.MovieDir,
            TVDir:            m.config.Library.TVDir,
            DownloadMountDir: m.config.Library.DownloadMountDir,
        },
        Server: ServerConfig{
            Port: m.config.Server.Port,
        },
        Jobs: JobsConfig{
            MovieReconcile:  m.config.Manager.Jobs.MovieReconcile.String(),
            MovieIndex:      m.config.Manager.Jobs.MovieIndex.String(),
            SeriesReconcile: m.config.Manager.Jobs.SeriesReconcile.String(),
            SeriesIndex:     m.config.Manager.Jobs.SeriesIndex.String(),
        },
    }
}
```

Add method to get library stats:
```go
func (m MediaManager) GetLibraryStats(ctx context.Context) (*LibraryStats, error) {
    // Query database for aggregate statistics
    movieStats, err := m.getMovieStats(ctx)
    if err != nil {
        return nil, err
    }

    tvStats, err := m.getTVStats(ctx)
    if err != nil {
        return nil, err
    }

    return &LibraryStats{
        Movies: movieStats,
        TV:     tvStats,
    }, nil
}

func (m MediaManager) getMovieStats(ctx context.Context) (MovieStats, error) {
    stats := MovieStats{
        ByState: make(map[string]int),
    }

    // Count movies by each state
    for _, state := range []storage.MovieState{
        storage.MovieStateMissing,
        storage.MovieStateDiscovered,
        storage.MovieStateUnreleased,
        storage.MovieStateDownloading,
        storage.MovieStateDownloaded,
    } {
        movies, err := m.storage.ListMoviesByState(ctx, state)
        if err != nil {
            return stats, err
        }
        stats.ByState[string(state)] = len(movies)
        stats.Total += len(movies)
    }

    return stats, nil
}

func (m MediaManager) getTVStats(ctx context.Context) (TVStats, error) {
    stats := TVStats{
        ByState: make(map[string]int),
    }

    // Count series by each state
    for _, state := range []storage.SeriesState{
        storage.SeriesStateMissing,
        storage.SeriesStateDiscovered,
        storage.SeriesStateDownloading,
        storage.SeriesStateDownloaded,
        storage.SeriesStateCompleted,
        storage.SeriesStateContinuing,
    } {
        series, err := m.storage.ListSeriesByState(ctx, state)
        if err != nil {
            return stats, err
        }
        stats.ByState[string(state)] = len(series)
        stats.Total += len(series)
    }

    return stats, nil
}
```

### 3. Add Server Handlers
**File**: `server/server.go`

Add handler methods:
```go
func (s *Server) GetConfig() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        result := s.mm.GetConfigSummary()
        GenericResponse{Response: result}.Write(w)
    }
}

func (s *Server) GetLibraryStats() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()
        stats, err := s.mm.GetLibraryStats(ctx)
        if err != nil {
            GenericResponse{Error: err.Error()}.Write(w)
            return
        }
        GenericResponse{Response: stats}.Write(w)
    }
}
```

### 4. Register Routes
**File**: `server/server.go`

Add routes in `Serve()` method (around line 110):
```go
v1.HandleFunc("/config", s.GetConfig()).Methods(http.MethodGet)
v1.HandleFunc("/library/stats", s.GetLibraryStats()).Methods(http.MethodGet)
```

---

## Frontend Changes

### 1. Create API Client Methods
**File**: `frontend/src/services/api.ts` (create if doesn't exist)

Add type definitions and API methods:
```typescript
// Types matching backend responses
export interface ConfigSummary {
  library: {
    movieDir: string;
    tvDir: string;
    downloadMountDir: string;
  };
  server: {
    port: number;
  };
  jobs: {
    movieReconcile: string;
    movieIndex: string;
    seriesReconcile: string;
    seriesIndex: string;
  };
}

export interface LibraryStats {
  movies: {
    total: number;
    byState: Record<string, number>;
  };
  tv: {
    total: number;
    byState: Record<string, number>;
  };
}

// API client
const API_BASE = '/api/v1';

export async function getConfig(): Promise<ConfigSummary> {
  const response = await fetch(`${API_BASE}/config`);
  if (!response.ok) throw new Error('Failed to fetch config');
  const data = await response.json();
  return data.response;
}

export async function getLibraryStats(): Promise<LibraryStats> {
  const response = await fetch(`${API_BASE}/library/stats`);
  if (!response.ok) throw new Error('Failed to fetch library stats');
  const data = await response.json();
  return data.response;
}
```

### 2. Create Library Page Component
**File**: `frontend/src/pages/LibraryPage.tsx` (new file)

Create the page component:
```tsx
import { useEffect, useState } from 'react';
import { getConfig, getLibraryStats, ConfigSummary, LibraryStats } from '../services/api';

export default function LibraryPage() {
  const [config, setConfig] = useState<ConfigSummary | null>(null);
  const [stats, setStats] = useState<LibraryStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function fetchData() {
      try {
        setLoading(true);
        const [configData, statsData] = await Promise.all([
          getConfig(),
          getLibraryStats()
        ]);
        setConfig(configData);
        setStats(statsData);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load library data');
      } finally {
        setLoading(false);
      }
    }

    fetchData();
  }, []);

  if (loading) return <div className="p-8">Loading...</div>;
  if (error) return <div className="p-8 text-red-600">Error: {error}</div>;
  if (!config || !stats) return null;

  return (
    <div className="p-8">
      <h1 className="text-3xl font-bold mb-8">Library</h1>

      {/* Configuration Section */}
      <section className="mb-8">
        <h2 className="text-2xl font-semibold mb-4">Configuration</h2>
        <div className="bg-gray-100 p-6 rounded-lg space-y-3">
          <ConfigItem label="Movie Directory" value={config.library.movieDir} />
          <ConfigItem label="TV Directory" value={config.library.tvDir} />
          <ConfigItem label="Download Mount Directory" value={config.library.downloadMountDir} />
          <ConfigItem label="Server Port" value={config.server.port.toString()} />
        </div>

        <h3 className="text-xl font-semibold mt-6 mb-3">Job Intervals</h3>
        <div className="bg-gray-100 p-6 rounded-lg space-y-3">
          <ConfigItem label="Movie Reconcile" value={config.jobs.movieReconcile} />
          <ConfigItem label="Movie Index" value={config.jobs.movieIndex} />
          <ConfigItem label="Series Reconcile" value={config.jobs.seriesReconcile} />
          <ConfigItem label="Series Index" value={config.jobs.seriesIndex} />
        </div>
      </section>

      {/* Statistics Section */}
      <section>
        <h2 className="text-2xl font-semibold mb-4">Statistics</h2>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {/* Movie Stats */}
          <div className="bg-blue-50 p-6 rounded-lg">
            <h3 className="text-xl font-semibold mb-4">Movies</h3>
            <div className="text-3xl font-bold mb-4">{stats.movies.total}</div>
            <div className="space-y-2">
              {Object.entries(stats.movies.byState).map(([state, count]) => (
                <StatItem key={state} label={formatStateName(state)} value={count} />
              ))}
            </div>
          </div>

          {/* TV Stats */}
          <div className="bg-purple-50 p-6 rounded-lg">
            <h3 className="text-xl font-semibold mb-4">TV Shows</h3>
            <div className="text-3xl font-bold mb-4">{stats.tv.total}</div>
            <div className="space-y-2">
              {Object.entries(stats.tv.byState).map(([state, count]) => (
                <StatItem key={state} label={formatStateName(state)} value={count} />
              ))}
            </div>
          </div>
        </div>
      </section>
    </div>
  );
}

function ConfigItem({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between items-center">
      <span className="font-medium text-gray-700">{label}:</span>
      <span className="text-gray-900 font-mono">{value || 'Not set'}</span>
    </div>
  );
}

function StatItem({ label, value }: { label: string; value: number }) {
  return (
    <div className="flex justify-between items-center text-sm">
      <span className="text-gray-700">{label}:</span>
      <span className="font-semibold">{value}</span>
    </div>
  );
}

function formatStateName(state: string): string {
  return state
    .split('_')
    .map(word => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ');
}
```

### 3. Add Library Route
**File**: `frontend/src/App.tsx`

Add route to the router:
```tsx
import LibraryPage from './pages/LibraryPage';

// Inside your router configuration:
<Route path="/library" element={<LibraryPage />} />
```

### 4. Update Navigation (if applicable)
Add a link to the Library page in the main navigation component.

---

## Testing Plan

### Backend Testing

1. **Unit Tests**: Add tests in `pkg/manager/manager_test.go`
   ```go
   func TestGetConfigSummary(t *testing.T) {
       // Test that config summary is returned correctly
   }

   func TestGetLibraryStats(t *testing.T) {
       // Test stats calculation with mock storage
   }
   ```

2. **Manual API Testing**:
   ```bash
   # Start the server
   ./mediaz serve

   # Test config endpoint
   curl http://localhost:8080/api/v1/config | jq

   # Test stats endpoint
   curl http://localhost:8080/api/v1/library/stats | jq
   ```

### Frontend Testing

1. **Manual Browser Testing**:
   - Navigate to `http://localhost:8080/library`
   - Verify configuration displays correctly
   - Verify statistics display correctly
   - Test error states (stop backend)
   - Test loading states (throttle network)

2. **Visual Verification**:
   - Check responsive design on mobile/tablet
   - Verify styling matches app design
   - Check for any layout issues

---

## Implementation Order

1. **Backend Implementation** (~30-45 minutes):
   - Add response types to `pkg/manager/manager_types.go`
   - Add manager methods to `pkg/manager/manager.go`
   - Add handlers to `server/server.go`
   - Register routes in `server/server.go`
   - Test with curl

2. **Frontend Implementation** (~30-45 minutes):
   - Create `api.ts` with type definitions and fetch functions
   - Create `LibraryPage.tsx` component
   - Add route to `App.tsx`
   - Add navigation link (if applicable)
   - Test in browser

3. **Testing & Refinement** (~15-30 minutes):
   - Write unit tests
   - Test error handling
   - Verify responsive design
   - Code review and cleanup

**Total Estimated Time**: 1.5 - 2 hours

---

## API Endpoints Summary

### New Endpoints

| Endpoint | Method | Description | Response |
|----------|--------|-------------|----------|
| `/api/v1/config` | GET | Get library configuration (non-sensitive) | `ConfigSummary` |
| `/api/v1/library/stats` | GET | Get library statistics (counts by state) | `LibraryStats` |

### Response Examples

**GET /api/v1/config**:
```json
{
  "response": {
    "library": {
      "movieDir": "/media/movies",
      "tvDir": "/media/tv",
      "downloadMountDir": "/downloads"
    },
    "server": {
      "port": 8080
    },
    "jobs": {
      "movieReconcile": "10m0s",
      "movieIndex": "10m0s",
      "seriesReconcile": "10m0s",
      "seriesIndex": "10m0s"
    }
  }
}
```

**GET /api/v1/library/stats**:
```json
{
  "response": {
    "movies": {
      "total": 150,
      "byState": {
        "discovered": 120,
        "downloaded": 25,
        "downloading": 3,
        "missing": 2,
        "unreleased": 0
      }
    },
    "tv": {
      "total": 45,
      "byState": {
        "discovered": 30,
        "downloaded": 10,
        "downloading": 2,
        "continuing": 3,
        "completed": 0,
        "missing": 0
      }
    }
  }
}
```

---

## Design Decisions & Rationale

### Why Two Separate Endpoints?

**Alternative Considered**: Single `/api/v1/library/overview` endpoint returning both config and stats

**Decision**: Use two separate endpoints

**Rationale**:
- Config changes rarely (long cache TTL possible)
- Stats change frequently (short/no cache)
- Separation of concerns (static vs dynamic data)
- More flexible for future clients
- Marginal additional complexity worth the architectural benefits

### Why Not Use Existing List Endpoints?

**Alternative Considered**: Fetch `/api/v1/library/movies` and `/api/v1/library/tv`, count client-side

**Decision**: Create dedicated stats endpoint

**Rationale**:
- Performance: Counting thousands of items in browser is inefficient
- Network: Large payloads when only counts are needed
- No access to config from list endpoints
- Stats endpoint can be optimized independently

### Security: Why Not Expose Full Config?

**Decision**: Only expose non-sensitive config fields

**Rationale**:
- API keys (TMDB, Prowlarr) should never reach frontend
- Storage paths are safe to expose (self-hosted app)
- Job intervals are useful for users to understand system behavior
- Easy to add more fields later if needed

---

## Future Enhancements (Out of Scope for V1)

- Real-time updates using WebSockets or polling
- Storage usage statistics (disk space per quality tier)
- Download progress tracking with live updates
- Historical trends (movies added per month)
- Quality profile distribution charts
- Editable configuration (PATCH /config endpoint)
- Export statistics to CSV/JSON
- Refresh button to manually update stats

---

## Notes

- This implementation uses existing storage methods (`ListMoviesByState`, `ListSeriesByState`)
- No new database queries or schema changes required
- All state types are already defined in `pkg/storage/storage.go`
- Manager already has access to full config via `config.Config` field
- Frontend assumes Tailwind CSS is available (adjust styling if using different framework)
