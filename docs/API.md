# Mediaz HTTP API
This document is created by analyzing the code in the `server` package.

Base URL: `/`

## Health Check

### GET /healthz
- Status: 200 OK
- Response: `{ "response": "ok" }`

---

## API v1
Base path: `/api/v1`

### Movies

#### GET /library/movies
- Status: 200 OK
- Response: `{ response: [ MovieFile ] }`

#### POST /library/movies
- Request (JSON): `{ tmdbID: int, qualityProfileID: int }`
- Status: 200 OK
- Response: `{ response: Movie }`

#### GET /discover/movie
- Query: `query=string`
- Status: 200 OK
- Response: `{ response: SearchMediaResponse }`

---

### TV Shows

#### GET /library/tv
- Status: 200 OK
- Response: `{ response: [ string ] }`  
(list of episode paths)

#### POST /library/tv
- Request (JSON): `{ tmdbID: int, qualityProfileID: int }`
- Status: 200 OK
- Response: `{ response: Series }`

#### GET /discover/tv
- Query: `query=string`
- Status: 200 OK
- Response: `{ response: SearchMediaResponse }`

---

### Indexers

#### GET /indexers
- Status: 200 OK
- Response: `{ response: [ Indexer ] }`

#### POST /indexers
- Request (JSON): `Indexer { id?: int, name: string, priority: int, uri: string, apiKey?: string }`
- Status: 201 Created
- Response: `{ response: Indexer }`

#### DELETE /indexers
- Request (JSON): `{ id: int }`
- Status: 200 OK
- Response: `{ response: { id: int } }`

---

### Download Clients

#### GET /download/clients
- Status: 200 OK
- Response: `{ response: [ DownloadClient ] }`

#### GET /download/clients/{id}
- Status: 200 OK
- Response: `{ response: DownloadClient }`

#### POST /download/clients
- Request (JSON): `DownloadClient { type: string, implementation: string, scheme: string, host: string, port: int, apiKey?: string }`
- Status: 201 Created
- Response: `{ response: DownloadClient }`

#### DELETE /download/clients/{id}
- Status: 200 OK
- Response: `{ response: { id: int } }`

---

### Quality Definitions

#### GET /quality/definitions
- Status: 200 OK
- Response: `{ response: [ QualityDefinition ] }`

#### GET /quality/definitions/{id}
- Status: 200 OK
- Response: `{ response: QualityDefinition }`

#### POST /quality/definitions
- Request (JSON): `QualityDefinition { name: string, media_type: string, preferredSize: float, minSize: float, maxSize: float }`
- Status: 201 Created
- Response: `{ response: QualityDefinition }`

#### DELETE /quality/definitions
- Request (JSON): `{ id: int }`
- Status: 200 OK
- Response: `{ response: { id: int } }`

---

### Quality Profiles

#### GET /quality/profiles
- Status: 200 OK
- Response: `{ response: [ QualityProfile ] }`

#### GET /quality/profiles/{id}
- Status: 200 OK
- Response: `{ response: QualityProfile }`

---

## Schemas

### MovieFile
- `name`: string
- `path`: string
- `absolutePath`: string
- `size`: int

### Movie
- `ID`: int
- `path?`: string
- `monitored`: int
- `qualityProfileID`: int
- `added?`: datetime
- `tags?`: string
- `movieFileID?`: int
- `movieMetadataID?`: int
- `state`: string

### Series
- same fields as Movie with `Series` semantics

### SearchMediaResponse
- `page?`, `total_pages?`, `total_results?`: int
- `results`: [ `SearchMediaResult` ]

### SearchMediaResult
- `adult?`: bool
- `backdrop_path?`: string
- `genre_ids?`: [int]
- `id?`: int
- `original_language?`: string
- `original_title?`: string
- `overview?`: string
- `popularity?`: float
- `poster_path?`: string
- `release_date?`: string
- `title?`: string
- `video?`: bool
- `vote_average?`: float
- `vote_count?`: int

### Indexer
- `id`: int
- `name`: string
- `priority`: int
- `uri`: string
- `apiKey?`: string
- `status?`: string
- `categories?`: array

### DownloadClient
- `id`: int
- `type`: string
- `implementation`: string
- `scheme`: string
- `host`: string
- `port`: int
- `apiKey?`: string

### QualityDefinition
- `name`: string
- `type`: string
- `preferredSize`: float
- `minSize`: float
- `maxSize`: float

### QualityProfile
- `id`: int
- `name`: string
- `qualities`: [ `QualityDefinition` ]
- `cutoff_quality_id`: int
- `upgradeAllowed`: bool
