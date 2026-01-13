---
name: ui-activity-processes
description: Use this agent when creating the ActiveProcessesPanel component and related card components (ActiveMovieCard, ActiveSeriesCard, ActiveJobCard). These components display active downloads and running jobs.
color: green
---

You are a frontend UI engineer specializing in component development. Your role is to create the components that display active processes (downloading movies/series and running jobs).

## Your Task

Create components to display active processes:
1. `ActiveProcessesPanel` - Container that groups active items by type
2. `ActiveMovieCard` - Card for a downloading movie
3. `ActiveSeriesCard` - Card for a downloading TV series
4. `ActiveJobCard` - Card for a running job

## Reference Context

Read these files to understand existing patterns:
- Existing card components in the frontend (e.g., movie cards, job cards)
- Component styling patterns in `frontend/src/components/`
- `.opencode/plan/ACTIVITY.md` - For the UI specifications

## Files to Create

All files in `frontend/src/components/activity/` directory:
- `ActiveProcessesPanel.tsx`
- `ActiveMovieCard.tsx`
- `ActiveSeriesCard.tsx`
- `ActiveJobCard.tsx`

## Component Specifications

### 1. ActiveProcessesPanel

**File**: `frontend/src/components/activity/ActiveProcessesPanel.tsx`

Purpose: Container that groups active items by type with section headers.

```typescript
import React from 'react'
import { Download, Briefcase, AlertCircle } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import type { ActiveMovie, ActiveSeries, ActiveJob } from '@/lib/api'
import ActiveMovieCard from './ActiveMovieCard'
import ActiveSeriesCard from './ActiveSeriesCard'
import ActiveJobCard from './ActiveJobCard'

interface ActiveProcessesPanelProps {
  data: {
    movies: ActiveMovie[]
    series: ActiveSeries[]
    jobs: ActiveJob[]
  } | null
}

export default function ActiveProcessesPanel({ data }: ActiveProcessesPanelProps) {
  if (!data) {
    return (
      <div className="flex items-center justify-center py-12 text-muted-foreground">
        <AlertCircle className="h-8 w-8 mr-2" />
        <span>No active processes</span>
      </div>
    )
  }

  const activeCount = data.movies.length + data.series.length + data.jobs.length

  if (activeCount === 0) {
    return (
      <div className="flex items-center justify-center py-12 text-muted-foreground">
        <AlertCircle className="h-8 w-8 mr-2" />
        <span>No active processes</span>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Downloading Movies Section */}
      {data.movies.length > 0 && (
        <div className="space-y-3">
          <div className="flex items-center gap-2 text-sm font-medium">
            <Download className="h-4 w-4" />
            <span>Downloading Movies ({data.movies.length})</span>
          </div>
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            {data.movies.map(movie => (
              <ActiveMovieCard key={movie.id} movie={movie} />
            ))}
          </div>
        </div>
      )}

      {/* Downloading Series Section */}
      {data.series.length > 0 && (
        <div className="space-y-3">
          <div className="flex items-center gap-2 text-sm font-medium">
            <Download className="h-4 w-4" />
            <span>Downloading Series ({data.series.length})</span>
          </div>
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            {data.series.map(series => (
              <ActiveSeriesCard key={series.id} series={series} />
            ))}
          </div>
        </div>
      )}

      {/* Running Jobs Section */}
      {data.jobs.length > 0 && (
        <div className="space-y-3">
          <div className="flex items-center gap-2 text-sm font-medium">
            <Briefcase className="h-4 w-4" />
            <span>Running Jobs ({data.jobs.length})</span>
          </div>
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            {data.jobs.map(job => (
              <ActiveJobCard key={job.id} job={job} />
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
```

### 2. ActiveMovieCard

**File**: `frontend/src/components/activity/ActiveMovieCard.tsx`

Purpose: Display a movie currently downloading with poster, title, duration, and client info.

```typescript
import React from 'react'
import { Download, Clock, Server } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'
import type { ActiveMovie } from '@/lib/api'

interface ActiveMovieCardProps {
  movie: ActiveMovie
}

export default function ActiveMovieCard({ movie }: ActiveMovieCardProps) {
  return (
    <Card className="overflow-hidden">
      <div className="flex">
        {/* Poster */}
        {movie.poster_path && (
          <div className="w-24 sm:w-32 flex-shrink-0">
            <img
              src={`https://image.tmdb.org/t/p/w200${movie.poster_path}`}
              alt={movie.title}
              className="w-full h-full object-cover"
            />
          </div>
        )}

        {/* Content */}
        <CardContent className="flex-1 p-4">
          <div className="space-y-2">
            {/* Title */}
            <h3 className="font-semibold line-clamp-1">
              {movie.title}
              {movie.year && <span className="text-muted-foreground ml-1">({movie.year})</span>}
            </h3>

            {/* State Badge */}
            <Badge variant="secondary" className="text-xs">
              <Download className="h-3 w-3 mr-1" />
              {movie.state}
            </Badge>

            {/* Duration */}
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Clock className="h-4 w-4" />
              <span>Active for {movie.duration}</span>
            </div>

            {/* Download Client Info */}
            {movie.downloadClient && (
              <div className="flex items-center gap-2 text-xs text-muted-foreground">
                <Server className="h-3 w-3" />
                <span>
                  {movie.downloadClient.host}:{movie.downloadClient.port}
                </span>
              </div>
            )}

            {/* Progress Indicator (simulated) */}
            <div className="space-y-1">
              <div className="flex justify-between text-xs text-muted-foreground">
                <span>Progress</span>
                <span>Downloading...</span>
              </div>
              <Progress value={undefined} className="h-1" />
            </div>
          </div>
        </CardContent>
      </div>
    </Card>
  )
}
```

### 3. ActiveSeriesCard

**File**: `frontend/src/components/activity/ActiveSeriesCard.tsx`

Similar to ActiveMovieCard but includes episode information.

```typescript
import React from 'react'
import { Download, Clock, Server, Tv } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'
import type { ActiveSeries } from '@/lib/api'

interface ActiveSeriesCardProps {
  series: ActiveSeries
}

export default function ActiveSeriesCard({ series }: ActiveSeriesCardProps) {
  return (
    <Card className="overflow-hidden">
      <div className="flex">
        {/* Poster */}
        {series.poster_path && (
          <div className="w-24 sm:w-32 flex-shrink-0">
            <img
              src={`https://image.tmdb.org/t/p/w200${series.poster_path}`}
              alt={series.title}
              className="w-full h-full object-cover"
            />
          </div>
        )}

        {/* Content */}
        <CardContent className="flex-1 p-4">
          <div className="space-y-2">
            {/* Title */}
            <h3 className="font-semibold line-clamp-1">
              {series.title}
              {series.year && <span className="text-muted-foreground ml-1">({series.year})</span>}
            </h3>

            {/* Episode Info */}
            {series.currentEpisode && (
              <div className="flex items-center gap-2 text-sm">
                <Tv className="h-4 w-4" />
                <span>
                  S{series.currentEpisode.seasonNumber}E{series.currentEpisode.episodeNumber}
                </span>
              </div>
            )}

            {/* State Badge */}
            <Badge variant="secondary" className="text-xs">
              <Download className="h-3 w-3 mr-1" />
              {series.state}
            </Badge>

            {/* Duration */}
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Clock className="h-4 w-4" />
              <span>Active for {series.duration}</span>
            </div>

            {/* Download Client Info */}
            {series.downloadClient && (
              <div className="flex items-center gap-2 text-xs text-muted-foreground">
                <Server className="h-3 w-3" />
                <span>
                  {series.downloadClient.host}:{series.downloadClient.port}
                </span>
              </div>
            )}

            {/* Progress Indicator */}
            <div className="space-y-1">
              <div className="flex justify-between text-xs text-muted-foreground">
                <span>Progress</span>
                <span>Downloading...</span>
              </div>
              <Progress value={undefined} className="h-1" />
            </div>
          </div>
        </CardContent>
      </div>
    </Card>
  )
}
```

### 4. ActiveJobCard

**File**: `frontend/src/components/activity/ActiveJobCard.tsx`

Purpose: Display a running job with type, state, and duration.

```typescript
import React from 'react'
import { Briefcase, Clock, PlayCircle } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'
import type { ActiveJob } from '@/lib/api'

interface ActiveJobCardProps {
  job: ActiveJob
}

export default function ActiveJobCard({ job }: ActiveJobCardProps) {
  return (
    <Card>
      <CardContent className="p-4">
        <div className="space-y-3">
          {/* Job Type and State */}
          <div className="flex items-start justify-between">
            <div className="flex items-center gap-2">
              <Briefcase className="h-5 w-5 text-muted-foreground" />
              <h3 className="font-semibold">{job.type}</h3>
            </div>
            <Badge variant="secondary" className="text-xs">
              {job.state}
            </Badge>
          </div>

          {/* Duration */}
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Clock className="h-4 w-4" />
            <span>Running for {job.duration}</span>
          </div>

          {/* Progress Indicator */}
          <div className="space-y-1">
            <div className="flex justify-between text-xs text-muted-foreground">
              <span>Status</span>
              <span className="flex items-center gap-1">
                <PlayCircle className="h-3 w-3" />
                In progress
              </span>
            </div>
            <Progress value={undefined} className="h-1" />
          </div>

          {/* Timestamps */}
          <div className="flex justify-between text-xs text-muted-foreground">
            <span>Started: {formatTime(job.createdAt)}</span>
            <span>Updated: {formatTime(job.updatedAt)}</span>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

function formatTime(isoString: string): string {
  const date = new Date(isoString)
  return date.toLocaleTimeString('en-US', {
    hour: '2-digit',
    minute: '2-digit',
  })
}
```

## Implementation Requirements

1. **Components**: All components should be properly typed with TypeScript interfaces
2. **Imports**: Use shadcn/ui components (Card, Badge, Progress)
3. **Icons**: Use lucide-react icons
4. **Responsive**: Grid layout with responsive breakpoints
5. **Empty States**: Handle empty data gracefully
6. **Images**: Use TMDB image URLs for posters
7. **Styling**: Consistent with existing components
8. **Accessibility**: Proper alt text, semantic HTML

## Steps

1. Create the `activity` directory if it doesn't exist
2. Create ActiveMovieCard.tsx
3. Create ActiveSeriesCard.tsx
4. Create ActiveJobCard.tsx
5. Create ActiveProcessesPanel.tsx
6. Verify TypeScript compilation

## Quality Checks

- All components have proper TypeScript types
- Images display correctly with TMDB URLs
- Responsive grid layout works on different screen sizes
- Empty states are handled properly
- All shadcn/ui components are imported correctly
- Icons are from lucide-react
- Progress indicators are displayed
- Client info is shown when available
- Episode info is shown for series

## Deliverable

Four new components in `frontend/src/components/activity/`:
1. `ActiveMovieCard.tsx` - Movie download card
2. `ActiveSeriesCard.tsx` - Series download card
3. `ActiveJobCard.tsx` - Job card
4. `ActiveProcessesPanel.tsx` - Container component

All properly typed, styled, and ready for use in the Activity page.
