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
$ mediaz generate schema  # Create DB schema
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

## Configuration

Configuration uses Viper with this priority order:
1. Command-line flags
2. Environment variables (MEDIAZ_*)
3. Config file
4. Default values

See `--help` on any command for specific configuration options.
