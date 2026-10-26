package server

import (
	"database/sql"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// Input credentials remain scalar strings, not sql.NullString objects.
// Case-insensitive decoding retains the historical PascalCase request keys.
type downloadClientRequest struct {
	ID             int64   `json:"ID"`
	Type           string  `json:"Type"`
	Implementation string  `json:"Implementation"`
	Scheme         string  `json:"Scheme"`
	Host           string  `json:"Host"`
	Port           int64   `json:"Port"`
	APIKey         *string `json:"APIKey"`
}

func (r downloadClientRequest) row() sqlcdb.DownloadClient {
	c := sqlcdb.DownloadClient{ID: r.ID, Type: r.Type, Implementation: r.Implementation, Scheme: r.Scheme, Host: r.Host, Port: r.Port}
	if r.APIKey != nil {
		c.ApiKey = sql.NullString{String: *r.APIKey, Valid: true}
	}
	return c
}

type indexerRequest struct {
	ID              int64   `json:"ID"`
	Name            string  `json:"Name"`
	Priority        int64   `json:"Priority"`
	URI             string  `json:"URI"`
	APIKey          *string `json:"APIKey"`
	IndexerSourceID *int64  `json:"IndexerSourceID"`
}

func (r indexerRequest) row() sqlcdb.Indexer {
	c := sqlcdb.Indexer{ID: r.ID, Name: r.Name, Priority: r.Priority, Uri: r.URI}
	if r.APIKey != nil {
		c.ApiKey = sql.NullString{String: *r.APIKey, Valid: true}
	}
	if r.IndexerSourceID != nil {
		c.IndexerSourceID = sql.NullInt64{Int64: *r.IndexerSourceID, Valid: true}
	}
	return c
}
