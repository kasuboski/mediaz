package indexer

import (
	"context"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/oapi-codegen/nullable"
)

const (
	TypeTV    = "tv"
	TypeMovie = "movie"
)

type RowID int64

type DownloadProtocol string

const (
	DownloadProtocolTorrent DownloadProtocol = "torrent"
	DownloadProtocolUsenet  DownloadProtocol = "usenet"
	DownloadProtocolUnknown DownloadProtocol = "unknown"
)

type SourceID int32

// ProwlarrID is retained for source compatibility; new code should use SourceID.
type ProwlarrID = SourceID

type Release struct {
	Age         *int32                        `json:"age,omitempty"`
	Categories  nullable.Nullable[[]Category] `json:"categories,omitempty"`
	CommentURL  nullable.Nullable[string]     `json:"commentUrl,omitempty"`
	DownloadURL nullable.Nullable[string]     `json:"downloadUrl,omitempty"`
	FileName    nullable.Nullable[string]     `json:"fileName,omitempty"`
	GUID        nullable.Nullable[string]     `json:"guid,omitempty"`
	ID          *int32                        `json:"id,omitempty"`
	ImdbID      *int32                        `json:"imdbId,omitempty"`
	Indexer     nullable.Nullable[string]     `json:"indexer,omitempty"`
	IndexerID   *int32                        `json:"indexerId,omitempty"`
	InfoHash    nullable.Nullable[string]     `json:"infoHash,omitempty"`
	InfoURL     nullable.Nullable[string]     `json:"infoUrl,omitempty"`
	Leechers    nullable.Nullable[int32]      `json:"leechers,omitempty"`
	MagnetURL   nullable.Nullable[string]     `json:"magnetUrl,omitempty"`
	PosterURL   nullable.Nullable[string]     `json:"posterUrl,omitempty"`
	Protocol    *string                       `json:"protocol,omitempty"`
	PublishDate *time.Time                    `json:"publishDate,omitempty"`
	Seeders     nullable.Nullable[int32]      `json:"seeders,omitempty"`
	Size        *int64                        `json:"size,omitempty"`
	Title       nullable.Nullable[string]     `json:"title,omitempty"`
	TmdbID      *int32                        `json:"tmdbId,omitempty"`
	TvdbID      *int32                        `json:"tvdbId,omitempty"`
}

type Category struct {
	Description   nullable.Nullable[string]     `json:"description,omitempty"`
	ID            *int32                        `json:"id,omitempty"`
	Name          nullable.Nullable[string]     `json:"name,omitempty"`
	SubCategories nullable.Nullable[[]Category] `json:"subCategories,omitempty"`
}

type Status struct {
	DisabledTill      nullable.Nullable[time.Time] `json:"disabledTill,omitempty"`
	ID                *int32                       `json:"id,omitempty"`
	IndexerID         *int32                       `json:"indexerId,omitempty"`
	InitialFailure    nullable.Nullable[time.Time] `json:"initialFailure,omitempty"`
	MostRecentFailure nullable.Nullable[time.Time] `json:"mostRecentFailure,omitempty"`
}

type IndexerSource interface {
	ListIndexers(ctx context.Context) ([]SourceIndexer, error)
	Search(ctx context.Context, indexerID SourceID, categories []int32, opts SearchOptions) ([]*Release, error)
}

type SourceIndexer struct {
	ID         SourceID
	Name       string
	URI        string
	Priority   int32
	Categories []Category
	Status     *Status
}

type SearchOptions struct {
	Query   string
	Season  *int64
	Episode *int64
	Type    *string
	TmdbID  *int32
	IMDbID  *string
	TvdbID  *int32
}

type Factory interface {
	NewIndexerSource(config sqlcdb.IndexerSource) (IndexerSource, error)
}
