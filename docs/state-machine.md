# State Machine System

This document describes Mediaz's state machine system, which tracks the lifecycle of movies, TV series, and jobs with complete audit trails.

## Overview

The state machine system provides a robust framework for managing entity state transitions throughout their lifecycle. It ensures:

- **Type-safe transitions** using Go generics
- **Immutable history** of all state changes
- **Validation** of allowed transitions before execution
- **Efficient queries** for current state and full history
- **Audit trail** for debugging and monitoring

Each state change creates a new database record, preserving the complete history of how an entity moved through its lifecycle.

## Core Concepts

### State

A state is a string value representing the current status of an entity. All states are defined as constants in `pkg/storage/storage.go`.

```go
type MovieState string

const (
    MovieStateNew         MovieState = ""
    MovieStateUnreleased  MovieState = "unreleased"
    MovieStateMissing     MovieState = "missing"
    MovieStateDiscovered  MovieState = "discovered"
    MovieStateDownloading MovieState = "downloading"
    MovieStateDownloaded  MovieState = "downloaded"
)
```

### Transition

A transition represents moving from one state to another. Each transition is stored as a separate database record with:

- `to_state`: The destination state (required)
- `from_state`: The source state (nullable for initial state)
- `created_at`: Timestamp when the transition occurred
- `sort_key`: Sequential number maintaining chronological order

### State Machine

The state machine validates whether a transition is allowed before it's executed. It uses a generic implementation with Go generics:

```go
type StateMachine[S State] struct {
    fromState S
    toStates  []Allowable[S]
}
```

Each entity implements a `Machine()` method that returns a configured state machine with all allowed transitions.

### Most Recent Flag

Every transition record has a `most_recent` boolean flag. Only one transition per entity has `most_recent = true`, identifying the current state. This flag is indexed for efficient queries.

### Sort Key

The `sort_key` is an incrementing integer that maintains the chronological order of transitions. The first transition has `sort_key = 1`, and each subsequent transition increments by 1.

## State-Managed Entities

### Movies

Movies have 6 states:

| State         | Description                                     |
| ------------- | ----------------------------------------------- |
| `""` (New)    | Initial state when a movie record is created    |
| `unreleased`  | Movie exists but release date is in the future  |
| `missing`     | Movie is monitored but not yet downloaded       |
| `discovered`  | Movie file found in library (scanned from disk) |
| `downloading` | Movie is actively being downloaded              |
| `downloaded`  | Download is complete and file is in library     |

**Valid Transitions:**

- `""` → `unreleased`, `missing`, `discovered`
- `unreleased` → `discovered`, `missing`
- `missing` → `discovered`, `downloading`
- `downloading` → `downloaded`

### TV Series / Seasons / Episodes

TV content has 7 states at each level (series, season, episode):

| State         | Description                                         |
| ------------- | --------------------------------------------------- |
| `""` (New)    | Initial state when record is created                |
| `unreleased`  | Not yet aired (for future episodes)                 |
| `missing`     | Monitored, released content not downloaded          |
| `discovered`  | Content found in library, awaiting metadata linking |
| `continuing`  | Ongoing series/season with mixed states             |
| `downloading` | Active downloads in progress                        |
| `completed`   | All monitored content downloaded                    |

**Valid Transitions:**

- `""` → `unreleased`, `missing`, `discovered`
- `unreleased` → `discovered`, `missing`
- `missing` → `discovered`, `downloading`
- `discovered` → `missing`, `continuing`, `completed`
- `downloading` → `continuing`, `completed`
- `continuing` → `completed`, `missing`

**Cascading State Evaluation:**

When an episode's state changes, the season state is re-evaluated based on all its episodes. Similarly, season state changes trigger series state re-evaluation. This ensures parent states accurately reflect their children's status.

**Parent State Determination:**

- **Season** state is calculated from its episodes:
  - All episodes done → `completed`
  - Any downloading → `downloading`
  - Mixed done/unreleased or discovered+done → `continuing`
  - Missing and no unreleased → `missing`
  - All unreleased → `unreleased`
  - Only discovered → `discovered`

- **Series** state is calculated from its seasons:
  - All seasons completed + TMDB status "ended/cancelled" → `completed`
  - Any downloading → `downloading`
  - All discovered + series ended → `discovered`
  - Continuing series → `continuing`
  - All unreleased → `unreleased`
  - Default → `missing`

### Jobs

Jobs have 6 states tracking their execution lifecycle:

| State       | Description                          |
| ----------- | ------------------------------------ |
| `""` (New)  | Initial state when job is created    |
| `pending`   | Job is queued and waiting to execute |
| `running`   | Job is currently executing           |
| `error`     | Job failed during execution          |
| `done`      | Job completed successfully           |
| `cancelled` | Job was cancelled before completion  |

**Valid Transitions:**

- `""` → `pending`
- `pending` → `running`, `error`, `cancelled`
- `running` → `error`, `done`, `cancelled`

**Job Types:**

- `MovieIndex` - Index the movie library
- `MovieReconcile` - Reconcile movie status
- `SeriesIndex` - Index the TV series library
- `SeriesReconcile` - Reconcile series status
- `IndexerSync` - Sync with Prowlarr indexers

**Error Tracking:**

When a job transitions to `error` state, the error message is stored in the transition's `error` field. This preserves failure context for debugging and retry operations.

## State Machine Pattern

### Generic Implementation

The state machine is defined in `pkg/machine/machine.go` using Go generics with a type constraint:

```go
type State interface {
    ~string
}

type StateMachine[S State] struct {
    fromState S
    toStates  []Allowable[S]
}
```

### Defining Allowed Transitions

The builder pattern is used to configure which states can transition to which other states:

```go
func (m *Movie) Machine() *machine.StateMachine[MovieState] {
    return machine.New(m.State,
        machine.From(MovieStateNew).To(MovieStateUnreleased, MovieStateMissing, MovieStateDiscovered),
        machine.From(MovieStateUnreleased).To(MovieStateDiscovered, MovieStateMissing),
        machine.From(MovieStateMissing).To(MovieStateDiscovered, MovieStateDownloading),
        machine.From(MovieStateDownloading).To(MovieStateDownloaded),
    )
}
```

### Validating Transitions

Before performing a state update, the state machine validates the transition:

```go
func (m *StateMachine[S]) ToState(s S) error {
    for _, transition := range m.toStates {
        if transition.from != m.fromState {
            continue
        }
        if slices.Contains(transition.to, s) {
            return nil  // Valid transition
        }
    }
    return ErrInvalidTransition
}
```

If the transition is invalid, `ErrInvalidTransition` is returned and no database changes occur.

## Database Schema Overview

### Transition Table Pattern

Each entity has a corresponding `{entity}_transition` table. All transition tables share a common structure:

```sql
CREATE TABLE "{entity}_transition" (
    id INTEGER PRIMARY KEY,
    {entity}_id INTEGER NOT NULL REFERENCES "{entity}"(id) ON DELETE CASCADE,
    to_state TEXT NOT NULL,
    from_state TEXT,
    most_recent BOOLEAN NOT NULL,
    sort_key INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### Entity-Specific Columns

Some transition tables include additional fields relevant to that entity:

**Movies, Seasons, Episodes:**

- `download_client_id` - Reference to download client handling the download
- `download_id` - External download client's identifier

**Seasons, Episodes:**

- `is_entire_season_download` - Boolean flag for season pack downloads

**Jobs:**

- `type` - Job type (duplicated from parent for query efficiency)
- `error` - Error message if transition resulted in failure

### Indexing Strategy

Two critical indexes ensure efficient queries:

```sql
-- Partial index for current state queries (only indexes most_recent=1 rows)
CREATE UNIQUE INDEX idx_{entity}_transitions_by_parent_most_recent
ON {entity}_transition({entity}_id, most_recent) WHERE most_recent = 1;

-- Index for chronological ordering
CREATE UNIQUE INDEX idx_{entity}_transitions_by_parent_sort_key
ON {entity}_transition({entity}_id, sort_key);
```

### Key Design Decisions

**Immutable History:**

- Past transitions are never modified or deleted
- Each state change creates a new transition record
- Only the `most_recent` flag changes on previous transitions

**Transaction Safety:**

- State updates are wrapped in database transactions
- Updates to previous transition and insertion of new transition are atomic
- Mutex locks prevent concurrent modifications

**Cascade Deletion:**

- Transitions are automatically deleted when parent entity is removed
- Enforced via `ON DELETE CASCADE` foreign key constraint

## Transition Workflow

### State Update Process

All state update methods follow this pattern:

1. **Fetch current entity** with its current state via JOIN with transition table where `most_recent = true`
2. **Validate transition** using `entity.Machine().ToState(newState)`
3. **Begin transaction** with mutex lock for thread safety
4. **Mark previous transition as non-recent:**
   - Set `most_recent = false` on the current transition
   - Update `updated_at` timestamp
5. **Create new transition record:**
   - Set `most_recent = true`
   - Set `from_state` from previous transition
   - Set `to_state` to the new state
   - Increment `sort_key` (previous `sort_key + 1`)
   - Attach optional metadata (download client/id, season download flag, error message)
6. **Commit transaction**

### Example: Movie Download Lifecycle

**Step 1: Create Movie (Initial State)**

```go
movieID, err := storage.CreateMovie(ctx, storage.Movie{
    Movie: model.Movie{
        Path:          "Inception (2010)",
        Monitored:     1,
        QualityProfileID: 1,
    },
}, storage.MovieStateMissing)
```

Creates:

- `movie` row with basic information
- `movie_transition` with `to_state="missing"`, `most_recent=true`, `sort_key=1`

**Step 2: Start Download (Missing → Downloading)**

```go
err = storage.UpdateMovieState(ctx, movieID, storage.MovieStateDownloading,
    &storage.TransitionStateMetadata{
        DownloadID:       "abc123",
        DownloadClientID: 5,
    })
```

Performs:

1. Validates transition via `movie.Machine().ToState(MovieStateDownloading)`
2. Updates previous transition: `most_recent = false`
3. Creates new transition: `to_state="downloading"`, `download_client_id=5`, `download_id="abc123"`, `most_recent=true`, `sort_key=2`

**Step 3: Download Complete (Downloading → Downloaded)**

```go
err = storage.UpdateMovieState(ctx, movieID, storage.MovieStateDownloaded, nil)
```

Performs:

1. Validates transition via `movie.Machine().ToState(MovieStateDownloaded)`
2. Updates previous transition: `most_recent = false`
3. Creates new transition: `to_state="downloaded"`, `most_recent=true`, `sort_key=3`

## Querying State Data

### Current State Queries

To query the current state of entities, join with the transition table filtering on `most_recent = 1`:

```sql
SELECT m.*, mt.to_state, mt.created_at, mt.download_id
FROM movie m
INNER JOIN movie_transition mt ON (m.id = mt.movie_id AND mt.most_recent = 1)
WHERE m.monitored = 1
ORDER BY mt.created_at DESC
```

Storage methods provide convenient wrappers:

- `GetMovie(ctx, id)` - Single movie with current state
- `ListMovies(ctx)` - All movies with current states
- `ListMoviesByState(ctx, state)` - Movies in specific state
- `ListMoviesByPath(ctx, path)` - Find movie by filesystem path

### Transition History Queries

To get the full audit trail for an entity, exclude the `most_recent` filter:

```sql
SELECT to_state, from_state, created_at, sort_key
FROM movie_transition
WHERE movie_id = ?
ORDER BY sort_key ASC
```

This returns all transitions in chronological order, showing the complete lifecycle.

### State Statistics

To count entities by their current state:

```sql
SELECT to_state, COUNT(*) AS count
FROM movie m
INNER JOIN movie_transition mt ON (m.id = mt.movie_id AND mt.most_recent = 1)
GROUP BY to_state
ORDER BY to_state
```

Useful for dashboards and monitoring.

### Active Process Queries

To find entities in active states (e.g., currently downloading):

```sql
SELECT m.id, m.path, mt.to_state, mt.created_at, mt.download_id
FROM movie m
INNER JOIN movie_transition mt ON (m.id = mt.movie_id AND mt.most_recent = 1)
WHERE mt.to_state IN ('downloading', 'missing')
ORDER BY mt.created_at ASC
```

### Duration Calculations

Calculate time spent in current state:

```sql
SELECT
    m.id,
    mt.to_state,
    mt.created_at AS state_started,
    datetime('now') - mt.created_at AS duration
FROM movie m
INNER JOIN movie_transition mt ON (m.id = mt.movie_id AND mt.most_recent = 1)
WHERE mt.to_state = 'downloading'
```

Calculate duration between historical transitions:

```sql
SELECT
    to_state,
    from_state,
    created_at,
    LEAD(created_at) OVER (PARTITION BY movie_id ORDER BY sort_key) AS next_state_at,
    LEAD(created_at) OVER (PARTITION BY movie_id ORDER BY sort_key) - created_at AS duration
FROM movie_transition
WHERE movie_id = ?
ORDER BY sort_key ASC
```

## Best Practices

### When to Use State Machine

Use the state machine for entities that:

- Have defined lifecycle stages with clear transitions
- Need an audit trail of state changes
- Require validation to prevent invalid state moves
- Are queried frequently by current state

### Defining States

1. **Use descriptive names**: State names should clearly indicate the entity's status
2. **Consider future states**: Plan for states that may be needed as features evolve
3. **Document business logic**: Explain what each state means and when transitions occur
4. **Keep states simple**: Avoid combining multiple concepts in one state
5. **Think about queries**: Design states that make filtering meaningful

### Transition Validation

1. **Always validate before database writes**: Use the state machine's `ToState()` method
2. **Handle errors gracefully**: Return clear error messages for invalid transitions
3. **Log state changes**: Include context about why a transition occurred
4. **Test edge cases**: Verify all possible transition paths

### Performance Considerations

1. **Leverage `most_recent` index**: This partial index makes current state queries very fast
2. **Limit history queries**: Filter by date range when querying transition history
3. **Use pagination**: For lists of entities, use `LIMIT` and `OFFSET`
4. **Consider caching**: Cache frequently accessed statistics (refresh periodically)
5. **Add indexes strategically**: Add indexes only for queries that need them

### Error Handling

1. **Validate transitions early**: Catch invalid transitions before starting transactions
2. **Preserve error context**: Store error messages in transition records for debugging
3. **Log failures**: Include entity ID, attempted transition, and error message
4. **Provide recovery options**: For jobs, allow retrying failed operations

## Common Patterns

### Adding a New State-Managed Entity

1. **Define state type and constants** in `pkg/storage/storage.go`
2. **Create transition table schema** in `pkg/storage/sqlite/schema/schema.sql`
3. **Implement `Machine()` method** on the entity struct
4. **Add state update methods** to storage interface:
   - `Create{Entity}(ctx, entity, initialState)`
   - `Update{Entity}State(ctx, id, state, metadata)`
5. **Implement SQLite update logic** with transaction pattern
6. **Add query methods** for filtering by state and getting history

### Adding a New State to Existing Entity

1. **Add state constant** to the entity's state type
2. **Update `Machine()` method** with new transition rules
3. **Update database schema** if new columns are needed for metadata
4. **Add reconcile logic** if the state requires background processing
5. **Update UI components** to display the new state

### Implementing State Reconciliation

Many states require background processing to move entities to the next state:

```go
func (m *MediaManager) ReconcileMissingMovies(ctx context.Context) {
    movies, _ := m.store.ListMoviesByState(ctx, storage.MovieStateMissing)

    for _, movie := range movies {
        // Search indexers
        release, err := m.searchIndexer(movie)
        if err != nil {
            continue
        }

        // Start download
        downloadID, err := m.startDownload(release)
        if err != nil {
            continue
        }

        // Transition to downloading
        _ = m.store.UpdateMovieState(ctx, movie.ID, storage.MovieStateDownloading,
            &storage.TransitionStateMetadata{
                DownloadID:       downloadID,
                DownloadClientID: release.DownloadClientID,
            })
    }
}
```

Reconciliation methods are scheduled to run periodically to process entities in states that require action.

## Related Files Reference

### Core Implementation

- `pkg/machine/machine.go` - Generic state machine implementation
- `pkg/storage/storage.go` - State type definitions and storage interfaces
- `pkg/machine/allowable.go` - Allowable transition configuration

### Entity Implementations

- `pkg/storage/storage.go` (lines 99-106) - Movie state machine definition
- `pkg/storage/storage.go` (lines 164-170) - Job state machine definition
- `pkg/storage/storage.go` (lines 276-290) - Series state machine definition
- `pkg/storage/storage.go` (lines 301-312) - Season state machine definition
- `pkg/storage/storage.go` (lines 327-337) - Episode state machine definition

### Storage Layer

- `pkg/storage/sqlite/sqlite.go` - Movie state updates (UpdateMovieState)
- `pkg/storage/sqlite/series.go` - TV state updates (UpdateSeriesState, UpdateSeasonState, UpdateEpisodeState)
- `pkg/storage/sqlite/job.go` - Job state updates (UpdateJobState)

### Database Schema

- `pkg/storage/sqlite/schema/schema.sql` - Transition table definitions
- `pkg/storage/sqlite/schema/gen/model/` - Generated transition models

### Manager Layer

- `pkg/manager/movie_reconcile.go` - Movie state reconciliation logic
- `pkg/manager/series_reconcile.go` - TV state reconciliation logic
- `pkg/manager/scheduler.go` - Job execution and state management
