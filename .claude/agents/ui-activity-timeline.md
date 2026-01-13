---
name: ui-activity-timeline
description: Use this agent when creating the ActivityTimeline component that displays activity history with a chart and detailed transition list.
color: purple
---

You are a frontend UI engineer specializing in data visualization components. Your role is to create the ActivityTimeline component with charts and detailed history.

## Your Task

Create the ActivityTimeline component that displays activity history over time.

## Reference Context

Read these files to understand existing patterns:
- Existing chart components in the frontend (if any)
- Date formatting utilities
- `.opencode/plan/ACTIVITY.md` - For the UI specifications

## File to Create

`frontend/src/components/activity/ActivityTimeline.tsx`

## Component Specification

```typescript
import React, { useMemo } from 'react'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts'
import { format } from 'date-fns'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Film, Tv, Briefcase, TrendingUp } from 'lucide-react'
import type { TimelineResponse } from '@/lib/api'

interface ActivityTimelineProps {
  data: TimelineResponse | null
}

export default function ActivityTimeline({ data }: ActivityTimelineProps) {
  if (!data) {
    return (
      <Card>
        <CardContent className="p-12 text-center text-muted-foreground">
          No timeline data available
        </CardContent>
      </Card>
    )
  }

  const chartData = useMemo(() => {
    return data.timeline.map(entry => ({
      date: formatDate(entry.date),
      formattedDate: formatDetailedDate(entry.date),
      moviesDownloaded: entry.movies?.downloaded || 0,
      moviesDownloading: entry.movies?.downloading || 0,
      seriesCompleted: entry.series?.completed || 0,
      seriesDownloading: entry.series?.downloading || 0,
      jobsDone: entry.jobs?.done || 0,
      jobsError: entry.jobs?.error || 0,
    }))
  }, [data.timeline])

  const transitionsByDate = useMemo(() => {
    const grouped = new Map<string, typeof data.transitions>()
    data.transitions.forEach(trans => {
      const date = trans.createdAt.split('T')[0]
      if (!grouped.has(date)) {
        grouped.set(date, [])
      }
      grouped.get(date)!.push(trans)
    })
    return grouped
  }, [data.transitions])

  return (
    <div className="space-y-6">
      {/* Chart Section */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <TrendingUp className="h-5 w-5" />
            Activity Overview
          </CardTitle>
        </CardHeader>
        <CardContent>
          <Tabs defaultValue="overview">
            <TabsList className="grid w-full grid-cols-3">
              <TabsTrigger value="overview">Overview</TabsTrigger>
              <TabsTrigger value="movies">Movies</TabsTrigger>
              <TabsTrigger value="series">Series</TabsTrigger>
            </TabsList>

            <TabsContent value="overview" className="mt-4">
              <ResponsiveContainer width="100%" height={300}>
                <LineChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis
                    dataKey="formattedDate"
                    tick={{ fontSize: 12 }}
                  />
                  <YAxis tick={{ fontSize: 12 }} />
                  <Tooltip
                    content={<CustomTooltip />}
                    labelFormatter={(label) => `Date: ${label}`}
                  />
                  <Legend />
                  <Line
                    type="monotone"
                    dataKey="moviesDownloaded"
                    name="Movies Downloaded"
                    stroke="#8884d8"
                    strokeWidth={2}
                  />
                  <Line
                    type="monotone"
                    dataKey="seriesCompleted"
                    name="Series Completed"
                    stroke="#82ca9d"
                    strokeWidth={2}
                  />
                  <Line
                    type="monotone"
                    dataKey="jobsDone"
                    name="Jobs Completed"
                    stroke="#ffc658"
                    strokeWidth={2}
                  />
                </LineChart>
              </ResponsiveContainer>
            </TabsContent>

            <TabsContent value="movies" className="mt-4">
              <ResponsiveContainer width="100%" height={300}>
                <LineChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis
                    dataKey="formattedDate"
                    tick={{ fontSize: 12 }}
                  />
                  <YAxis tick={{ fontSize: 12 }} />
                  <Tooltip content={<CustomTooltip />} />
                  <Legend />
                  <Line
                    type="monotone"
                    dataKey="moviesDownloaded"
                    name="Downloaded"
                    stroke="#8884d8"
                    strokeWidth={2}
                  />
                  <Line
                    type="monotone"
                    dataKey="moviesDownloading"
                    name="Downloading"
                    stroke="#82ca9d"
                    strokeWidth={2}
                    strokeDasharray="5 5"
                  />
                </LineChart>
              </ResponsiveContainer>
            </TabsContent>

            <TabsContent value="series" className="mt-4">
              <ResponsiveContainer width="100%" height={300}>
                <LineChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis
                    dataKey="formattedDate"
                    tick={{ fontSize: 12 }}
                  />
                  <YAxis tick={{ fontSize: 12 }} />
                  <Tooltip content={<CustomTooltip />} />
                  <Legend />
                  <Line
                    type="monotone"
                    dataKey="seriesCompleted"
                    name="Completed"
                    stroke="#8884d8"
                    strokeWidth={2}
                  />
                  <Line
                    type="monotone"
                    dataKey="seriesDownloading"
                    name="Downloading"
                    stroke="#82ca9d"
                    strokeWidth={2}
                    strokeDasharray="5 5"
                  />
                </LineChart>
              </ResponsiveContainer>
            </TabsContent>
          </Tabs>
        </CardContent>
      </Card>

      {/* Detailed Transitions List */}
      <Card>
        <CardHeader>
          <CardTitle>Activity Details</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {chartData.map(entry => {
              const transitions = transitionsByDate.get(entry.date)
              if (!transitions || transitions.length === 0) {
                return null
              }

              return (
                <div key={entry.date} className="space-y-2">
                  <h4 className="font-semibold text-lg">{entry.formattedDate}</h4>
                  <div className="grid gap-2">
                    {transitions.map(trans => (
                      <TransitionItem key={trans.id} transition={trans} />
                    ))}
                  </div>
                </div>
              )
            })}

            {chartData.length === 0 && (
              <div className="text-center py-8 text-muted-foreground">
                No activity in the selected time range
              </div>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

function TransitionItem({ transition }: { transition: TimelineResponse['transitions'][0] }) {
  const getIcon = (entityType: string) => {
    switch (entityType) {
      case 'movie':
        return <Film className="h-4 w-4" />
      case 'series':
        return <Tv className="h-4 w-4" />
      case 'job':
        return <Briefcase className="h-4 w-4" />
      default:
        return <TrendingUp className="h-4 w-4" />
    }
  }

  const getStateColor = (state: string) => {
    switch (state) {
      case 'downloaded':
      case 'completed':
      case 'done':
        return 'text-green-500'
      case 'downloading':
        return 'text-blue-500'
      case 'error':
        return 'text-red-500'
      default:
        return 'text-muted-foreground'
    }
  }

  return (
    <div className="flex items-start gap-3 p-3 bg-muted/50 rounded-lg">
      <div className="mt-0.5 text-muted-foreground">
        {getIcon(transition.entityType)}
      </div>

      <div className="flex-1 space-y-1">
        <div className="flex items-center gap-2">
          <span className="font-medium">
            {transition.entityTitle || `${transition.entityType} #${transition.entityId}`}
          </span>
          <Badge variant="outline" className="text-xs capitalize">
            {transition.entityType}
          </Badge>
        </div>

        <div className="flex items-center gap-2 text-sm">
          {transition.fromState && (
            <span className="text-muted-foreground line-through">
              {transition.fromState}
            </span>
          )}
          <span className="text-muted-foreground">→</span>
          <span className={`font-medium ${getStateColor(transition.toState)}`}>
            {transition.toState}
          </span>
        </div>

        <div className="text-xs text-muted-foreground">
          {formatDateTime(transition.createdAt)}
        </div>
      </div>
    </div>
  )
}

function CustomTooltip({ active, payload, label }: any) {
  if (!active || !payload || !payload.length) {
    return null
  }

  return (
    <div className="bg-background border rounded-lg p-3 shadow-lg">
      <p className="font-semibold mb-2">{label}</p>
      <div className="space-y-1">
        {payload.map((entry: any, index: number) => (
          <div key={index} className="flex items-center gap-2 text-sm">
            <div
              className="w-2 h-2 rounded-full"
              style={{ backgroundColor: entry.color }}
            />
            <span className="text-muted-foreground">{entry.name}:</span>
            <span className="font-medium">{entry.value}</span>
          </div>
        ))}
      </div>
    </div>
  )
}

// Helper functions
function formatDate(dateStr: string): string {
  const date = new Date(dateStr)
  return format(date, 'MMM d')
}

function formatDetailedDate(dateStr: string): string {
  const date = new Date(dateStr)
  return format(date, 'MMM d, yyyy')
}

function formatDateTime(isoString: string): string {
  const date = new Date(isoString)
  return format(date, 'MMM d, yyyy h:mm a')
}
```

## Implementation Requirements

1. **Charts**: Use Recharts for line chart visualization
2. **Date Formatting**: Use date-fns for consistent date formatting
3. **Tabs**: Provide multiple views (overview, movies, series)
4. **Responsive**: Charts should be responsive and adapt to container
5. **Custom Tooltip**: Custom tooltip component for better UX
6. **Transitions List**: Group transitions by date and display details
7. **Icons**: Use appropriate icons for different entity types
8. **Color Coding**: Use semantic colors for different states

## Features

1. **Overview Tab**: Combined view of all activity
2. **Movies Tab**: Focused on movie downloads
3. **Series Tab**: Focused on series downloads
4. **Detailed List**: Shows all transitions grouped by date
5. **State Visualization**: Color-coded states and transitions
6. **Hover Details**: Custom tooltips on chart points

## UI Components Used

- `Card`, `CardHeader`, `CardTitle`, `CardContent` from shadcn/ui
- `Tabs` components from shadcn/ui
- `Badge` from shadcn/ui
- Recharts components for visualization
- lucide-react icons

## Steps

1. Create the component file
2. Implement the main component with tabs
3. Add Recharts line chart
4. Implement custom tooltip
5. Add detailed transitions list
6. Add helper functions for date formatting
7. Verify TypeScript compilation

## Quality Checks

- Charts render correctly with data
- Multiple tabs work properly
- Custom tooltip displays correctly
- Transitions are grouped by date
- Icons match entity types
- State colors are semantic
- Responsive design works
- Empty state handled

## Deliverable

A complete `frontend/src/components/activity/ActivityTimeline.tsx` component with:
1. Recharts line chart with tabs
2. Custom tooltip
3. Detailed transitions list
4. Proper TypeScript types
5. Successfully compiles

## Dependencies

This component requires:
- `recharts` package (should already be installed)
- `date-fns` package (should already be installed)

If not installed, they will need to be added.
