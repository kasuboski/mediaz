# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Mediaz is a self-hosted media management platform written in Go that helps organize and automate movie/TV show collections. It provides metadata indexing from TMDB, download integration with clients like SABnzbd/Transmission, and a unified REST API.

## Development Commands

### Building & Running
```bash
go build -o mediaz main.go  # Build the binary
./mediaz serve              # Start the media server
```

### Code Generation
```bash
make generate               # Generate all code (TMDB & Prowlarr clients)
go generate ./...           # Alternative method
```

### Testing
```bash
go test ./...               # Run all tests
go test -v ./pkg/manager/   # Run specific package tests with verbose output
go test -race ./...         # Run tests with race detection
go test -cover ./...        # Run tests with coverage
```

### Development Workflow
1. Make requested changes
2. Run `go fmt ./...` to format the code
3. Run `go test ./...` to run tests
4. Fix any errors
5. Repeat

### Database
```bash
./mediaz generate schema    # Create database schema
```

## Architecture Overview

### Core Components

**Main Entry Point**: `main.go` → `cmd/root.go` using Cobra for CLI commands

**HTTP Server**: `server/server.go` provides REST API with Gorilla Mux router
- All endpoints under `/api/v1/`
- CORS enabled for all origins
- JSON responses with `GenericResponse` wrapper

**Media Manager**: `pkg/manager/manager.go` is the core business logic orchestrator
- Coordinates between TMDB, Prowlarr, storage, and download clients
- Handles movie/TV metadata fetching and library management
- Manages indexers, quality profiles, and download clients

**Storage Layer**: `pkg/storage/sqlite/` using Jet ORM
- SQLite database with generated models in `schema/gen/`
- Schema defined in `schema/schema.sql` and `schema/defaults.sql`

**External Integrations**:
- `pkg/tmdb/` - TMDB API client (generated from schema)
- `pkg/prowlarr/` - Prowlarr API client (generated from schema)
- `pkg/download/` - Download clients (SABnzbd, Transmission)

### Configuration Management

Uses Viper with hierarchy: CLI flags → ENV vars (MEDIAZ_*) → config file → defaults

Key config sections defined in `cmd/root.go:initConfig()`:
- TMDB API settings
- Prowlarr integration
- Server port (default 8080)
- Library paths for movies/TV
- Storage (SQLite file path and schemas)
- Manager job intervals (default 10 minutes)

### Generated Code

The project uses code generation extensively:
- TMDB client from `tmdb.schema.json` (fetched from TMDB OpenAPI)
- Prowlarr client from `prowlarr.schema.json` (fetched from Prowlarr repo)
- Database models via Jet ORM from SQL schema
- Mock interfaces via go:generate directives

Run `make generate` to regenerate all external schemas and code.

### Key Patterns

**Dependency Injection**: Main components injected into MediaManager constructor
**Interface-based Design**: All external clients use interfaces (see `generate.go` files for mock generation)
**Context Propagation**: All operations use context.Context for cancellation and logging
**Structured Logging**: Uses Zap logger with context-aware logging via `pkg/logger`

### Testing Strategy

The project uses `gomock` library to avoid external dependencies during testing.

**Storage Testing**: Prefer using an in-memory SQLite database over mocks. Use mocks only if the scenario would be hard to recreate in an in-memory SQLite database.

**External Components**: Components that interact with the outside world have generated mocks with `gomock`. These mocks are in a `mocks` folder next to the component.

**Mock Setup Example**:
```go
ctrl := gomock.NewController(t)
store := mocks.NewMockStorage(ctrl)
m := New(nil, nil, nil, store, nil, config.Manager{})
store.EXPECT().ListMoviesByState(ctx, storage.MovieStateDiscovered).Return([]*storage.Movie{}, nil)
store.EXPECT().ListMoviesByState(ctx, storage.MovieStateDownloaded).Return([]*storage.Movie{}, nil)
// use components that you are testing
```

### CLI Commands Available

**Core Operations**:
- `mediaz serve` - Start the media server
- `mediaz discover` - Find new media content

**Movie Management**:
- `mediaz index movies` - Refresh movie metadata
- `mediaz list movies` - Show indexed collection
- `mediaz search movie <title>` - Find TMDB entries

**TV Management**:
- `mediaz list tv <path>` - List TV episodes in library
- `mediaz search tv <title>` - Find TV shows in TMDB

**Indexer Management**:
- `mediaz list indexer` - List configured indexers
- `mediaz search indexer <query>` - Search indexers for content

**System Management**:
- `mediaz generate schema` - Create DB schema
- `mediaz --config <path>` - Specify config file
