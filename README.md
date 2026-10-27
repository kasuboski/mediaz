# mediaz

## Project Overview

Mediaz is a self-hosted media management platform that helps organize and automate your movie/TV show collections. Key features include:

- **Metadata Indexing** - Automatic fetching from TMDB
- **Download Integration** - Handoff to clients like SABnzbd/Transmission
- **Unified API** - REST interface for managing your media
- **CLI First** - All operations available via command line

## CLI Commands

### Core Operations
```
$ mediaz serve       # Start the media server
$ mediaz discover    # Find new media content
```

### Movie Management
```
$ mediaz index movies    # Refresh movie metadata
$ mediaz list movies    # Show indexed collection
$ mediaz search movie <title>  # Find TMDB entries
```

### TV Management
```
$ mediaz list tv <path>  # List TV episodes in library
$ mediaz search tv <title>  # Find TV shows in TMDB
```

### Indexer Management
```
$ mediaz list indexer    # List configured indexers
$ mediaz search indexer <query>  # Search indexers for content
```

### System Management
```
$ mediaz generate testlib [path]  # Generate test library from database
$ mediaz --config <path>  # Specify config file
```

## HTTP API Endpoints

### Movies API
- `GET /api/v1/library/movies` - List all movies
- `GET /api/v1/discover/movie?query=<title>` - Search for movies
- `POST /api/v1/library/movies` - Add movie to library

### TV Shows API
- `GET /api/v1/library/tv` - List all TV shows
- `GET /api/v1/discover/tv?query=<title>` - Search for TV shows
- `POST /api/v1/library/tv` - Add a show to library

### Indexers API
- `GET /api/v1/indexers` - List all indexers
- `POST /api/v1/indexers` - Create an indexer
- `DELETE /api/v1/indexers` - Delete an indexer

### Download Clients API
- `GET /api/v1/download/clients` - List all download clients
- `GET /api/v1/download/clients/{id}` - Get download client details
- `POST /api/v1/download/clients` - Create a download client
- `DELETE /api/v1/download/clients/{id}` - Delete a download client

### Quality API
- `GET /api/v1/quality/profiles` - List all quality profiles
- `GET /api/v1/quality/profiles/{id}` - Get quality profile details
- `GET /api/v1/quality/definitions` - List all quality definitions
- `GET /api/v1/quality/definitions/{id}` - Get quality definition details
- `POST /api/v1/quality/definitions` - Create a quality definition
- `DELETE /api/v1/quality/definitions` - Delete a quality definition

### System API  
- `GET /healthz` - Service status

## Test Library Generation

The `generate testlib` command creates a test media library with empty files based on your current database state. This is useful for testing reconciliation logic without large media files.

```bash
# Generate test library using actual filenames from configured library paths
$ mediaz generate testlib ./testlib

# Use specific config file  
$ mediaz --config=local.yaml generate testlib ./testlib

# Available options
$ mediaz generate testlib --help
  --overwrite      Overwrite existing test library directory
  --movies-only    Generate only movies
  --tv-only        Generate only TV shows
  --db-path        Path to database file (default: from config)
```

**Features:**
- Uses actual filenames and directory structure from your configured library paths
- Creates empty media files with valid headers (MP4/MKV)
- Preserves folder organization (e.g., "Movie Title/movie.mp4")
- Falls back to generated names if library paths aren't configured
- Generates README with library statistics

**Setup:**
```bash
export MEDIAZ_LIBRARY_MOVIE="/path/to/movies"
export MEDIAZ_LIBRARY_TV="/path/to/tv"
./mediaz generate testlib ./test-output
```

## Configuration

Configuration uses Viper with this priority order:
1. Command-line flags
2. Environment variables (MEDIAZ_*)
3. Config file
4. Default values

See `--help` on any command for specific configuration options.

## Database Reset (Breaking Change)

This release ships a single squashed baseline migration. Databases created by
earlier mediaz releases (old migration versions 1-7, or databases from before
the migration system existed) are **not** supported and are **not** upgraded
automatically.

Startup refuses databases with migration versions above the new baseline (for
example, version 7) and nonempty databases without migration history, with an
error that includes the database file path and reset instructions. An old
version-1 database cannot be distinguished by its version number alone: reset
all databases created by earlier releases, even if startup does not refuse them.
To upgrade:

1. Stop mediaz.
2. Back up the database if needed, then delete it yourself (the path from
   `storage.filePath` / `MEDIAZ_STORAGE_FILEPATH`, default `mediaz.sqlite`).
   With mediaz stopped, also remove any matching `-wal` and `-shm` sidecars;
   do not restore old sidecars alongside the recreated database.
3. Restart mediaz — the database is recreated at the baseline schema version
   with default quality profiles and definitions.

Media files in the configured library directories are **never** touched; only
the database index is recreated. This discards database-only settings and
history (including configured indexers, download clients, monitoring choices,
and custom quality profiles); configure them again after the reset. mediaz
never deletes the database file itself.

The legacy `storage.schemas` and `storage.tableValueSchemas` configuration
options have been removed; the schema now comes solely from the embedded
baseline migration.
