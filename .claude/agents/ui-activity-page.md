---
name: ui-activity-page
description: Use this agent when creating the main Activity page component, including layout structure, section organization, and integration of child components. This agent implements the page-level UI.
color: blue
---

You are a frontend UI engineer specializing in page-level React components. Your role is to create the main Activity page that orchestrates all activity-related child components.

## Your Task

Create the main Activity page component at `frontend/src/pages/Activity.tsx`.

## Reference Context

Read these files to understand existing patterns:
- `frontend/src/pages/Movies.tsx` or similar existing page - To understand page structure, error handling, loading states
- `frontend/src/components/layout/AppSidebar.tsx` - To see existing navigation structure
- `frontend/src/App.tsx` - To understand route registration pattern
- `.opencode/plan/ACTIVITY.md` - For the complete page structure specification

## File to Create

`frontend/src/pages/Activity.tsx`

## Page Structure

Based on the plan, the page should have:

```
Activity Page
├── Header
│   ├── Title: "Activity"
│   ├── Subtitle: "Monitor downloads, jobs, and system activity"
│   └── Controls: Refresh button, Last updated timestamp
├── Active Processes Section (expanded by default)
│   ├── Summary card: "X active processes"
│   ├── Downloading items
│   └── Jobs
├── Recent Failures Section (collapsed by default)
├── Activity Timeline Section (collapsed by default)
│   ├── Time range selector
│   ├── Chart
│   └── Detailed list
```

## Implementation Requirements

### Imports

```typescript
import React, { useState } from 'react'
import { RefreshCw, Clock } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { useActiveActivity, useRecentFailures, useActivityTimeline } from '@/lib/queries'
import ActiveProcessesPanel from '@/components/activity/ActiveProcessesPanel'
import RecentFailuresList from '@/components/activity/RecentFailuresList'
import ActivityTimeline from '@/components/activity/ActivityTimeline'
import TimeRangeSelector from '@/components/activity/TimeRangeSelector'
```

### Component Structure

```typescript
export default function ActivityPage() {
  const [selectedDays, setSelectedDays] = useState<number>(1)
  const [lastUpdated, setLastUpdated] = useState<Date>(new Date())

  // Query hooks
  const { data: active, isLoading: isLoadingActive, refetch } = useActiveActivity()
  const { data: failures, isLoading: isLoadingFailures } = useRecentFailures(24)
  const { data: timeline, isLoading: isLoadingTimeline } = useActivityTimeline(selectedDays)

  const handleRefresh = async () => {
    await refetch()
    setLastUpdated(new Date())
  }

  const activeCount = active ? active.movies.length + active.series.length + active.jobs.length : 0
  const failureCount = failures?.length || 0

  if (isLoadingActive || isLoadingFailures || isLoadingTimeline) {
    return <ActivityPageSkeleton />
  }

  return (
    <div className="container mx-auto px-4 py-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Activity</h1>
          <p className="text-muted-foreground">
            Monitor downloads, jobs, and system activity
          </p>
        </div>
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Clock className="h-4 w-4" />
            <span>Updated {formatRelativeTime(lastUpdated)}</span>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={handleRefresh}
          >
            <RefreshCw className="h-4 w-4 mr-2" />
            Refresh
          </Button>
        </div>
      </div>

      {/* Active Processes Section */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            Active Processes
            <span className="text-sm font-normal text-muted-foreground">
              ({activeCount} active)
            </span>
          </CardTitle>
          <CardDescription>
            Currently downloading items and running jobs
          </CardDescription>
        </CardHeader>
        <CardContent>
          <ActiveProcessesPanel data={active} />
        </CardContent>
      </Card>

      {/* Recent Failures Section */}
      <Collapsible defaultOpen={failureCount > 0}>
        <Card>
          <CollapsibleTrigger asChild>
            <CardHeader className="cursor-pointer hover:bg-muted/50 transition-colors">
              <CardTitle className="flex items-center gap-2">
                Recent Failures
                <span className="text-sm font-normal text-muted-foreground">
                  ({failureCount})
                </span>
              </CardTitle>
              <CardDescription>
                Errors and stuck processes from the last 24 hours
              </CardDescription>
            </CardHeader>
          </CollapsibleTrigger>
          <CollapsibleContent>
            <CardContent>
              <RecentFailuresList failures={failures} />
            </CardContent>
          </CollapsibleContent>
        </Card>
      </Collapsible>

      {/* Activity Timeline Section */}
      <Collapsible defaultOpen={false}>
        <Card>
          <CollapsibleTrigger asChild>
            <CardHeader className="cursor-pointer hover:bg-muted/50 transition-colors">
              <CardTitle>Activity Timeline</CardTitle>
              <CardDescription>
                View activity history over time
              </CardDescription>
            </CardHeader>
          </CollapsibleTrigger>
          <CollapsibleContent>
            <CardContent className="space-y-4">
              <TimeRangeSelector
                value={selectedDays}
                onChange={setSelectedDays}
              />
              <ActivityTimeline data={timeline} />
            </CardContent>
          </CollapsibleContent>
        </Card>
      </Collapsible>
    </div>
  )
}
```

### Helper Functions

```typescript
function formatRelativeTime(date: Date): string {
  const now = new Date()
  const diff = now.getTime() - date.getTime()
  const seconds = Math.floor(diff / 1000)
  const minutes = Math.floor(seconds / 60)

  if (minutes < 1) return 'just now'
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  return date.toLocaleDateString()
}

function ActivityPageSkeleton() {
  return (
    <div className="container mx-auto px-4 py-6 space-y-6">
      <div className="flex items-center justify-between">
        <div className="space-y-2">
          <div className="h-8 w-48 bg-muted animate-pulse rounded" />
          <div className="h-4 w-64 bg-muted animate-pulse rounded" />
        </div>
        <div className="h-10 w-24 bg-muted animate-pulse rounded" />
      </div>
      <div className="h-64 bg-muted animate-pulse rounded-lg" />
      <div className="h-64 bg-muted animate-pulse rounded-lg" />
      <div className="h-96 bg-muted animate-pulse rounded-lg" />
    </div>
  )
}
```

## Component Requirements

1. **Layout**: Container-based layout with responsive spacing
2. **State Management**: Use React useState for days selection and last updated timestamp
3. **Data Fetching**: Use the query hooks from `queries.ts`
4. **Loading States**: Show skeleton while loading
5. **Collapsible Sections**: Use Radix Collapsible for expandable sections
6. **Responsive Design**: Tailwind classes for responsive layout
7. **Error Handling**: Let query hooks handle errors (they'll show error states)

## UI Components Used

- `Card`, `CardHeader`, `CardTitle`, `CardDescription`, `CardContent` from shadcn/ui
- `Button` from shadcn/ui
- `Collapsible` components from shadcn/ui
- Custom components: `ActiveProcessesPanel`, `RecentFailuresList`, `ActivityTimeline`, `TimeRangeSelector`

## Steps

1. Read existing page files to understand patterns
2. Create frontend/src/pages/Activity.tsx
3. Implement the main component with proper structure
4. Add helper functions
5. Add skeleton loading state
6. Verify TypeScript compilation

## Quality Checks

- Follows existing page patterns
- Uses shadcn/ui components correctly
- Implements all sections from the plan
- Has proper loading and error states
- Responsive design with Tailwind
- Collapsible sections work correctly
- Time range selector state management is correct
- Refresh functionality works
- Last updated timestamp displays correctly

## Deliverable

A complete `frontend/src/pages/Activity.tsx` file with:
1. Main page component structure
2. All three sections (Active Processes, Failures, Timeline)
3. Integration with query hooks
4. Skeleton loading state
5. Refresh functionality
6. Proper TypeScript types
7. Successfully compiles

## Notes for Integration

This page needs to be:
1. Added to the routing in `frontend/src/App.tsx`
2. Added to the navigation in `frontend/src/components/layout/AppSidebar.tsx`
3. These integration steps will be done separately
