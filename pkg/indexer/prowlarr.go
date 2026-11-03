package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/oapi-codegen/nullable"
	"go.uber.org/zap"
)

// ProwlarrClient defines the subset of the Prowlarr API used by ProwlarrIndexerSource.
type ProwlarrClient interface {
	GetAPIV1Indexer(ctx context.Context, reqEditors ...prowlarr.RequestEditorFn) (*http.Response, error)
	GetAPIV1Search(ctx context.Context, params *prowlarr.GetAPIV1SearchParams, reqEditors ...prowlarr.RequestEditorFn) (*http.Response, error)
	GetAPIKey() string
	GetAPIURL() string
}

type ProwlarrIndexerSource struct {
	client ProwlarrClient
	config sqlcdb.IndexerSource
}

func NewProwlarrSource(config sqlcdb.IndexerSource) (*ProwlarrIndexerSource, error) {
	if !config.ApiKey.Valid {
		return nil, fmt.Errorf("prowlarr source requires api_key")
	}

	url := fmt.Sprintf("%s://%s", config.Scheme, config.Host)
	if config.Port.Valid && config.Port.Int64 != 80 && config.Port.Int64 != 443 {
		url = fmt.Sprintf("%s:%d", url, config.Port.Int64)
	}

	client, err := prowlarr.New(url, config.ApiKey.String)
	if err != nil {
		return nil, fmt.Errorf("failed to create prowlarr client: %w", err)
	}

	return &ProwlarrIndexerSource{
		client: client,
		config: config,
	}, nil
}

// NewProwlarrIndexerSourceWithClient creates a ProwlarrIndexerSource with a
// pre-configured client, useful for testing.
func NewProwlarrIndexerSourceWithClient(client ProwlarrClient, config sqlcdb.IndexerSource) *ProwlarrIndexerSource {
	return &ProwlarrIndexerSource{
		client: client,
		config: config,
	}
}

func (p *ProwlarrIndexerSource) GetAPIKey() string { return p.client.GetAPIKey() }
func (p *ProwlarrIndexerSource) GetAPIURL() string { return p.client.GetAPIURL() }

func (p *ProwlarrIndexerSource) ListIndexers(ctx context.Context) ([]SourceIndexer, error) {
	resp, err := p.client.GetAPIV1Indexer(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch indexers: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var prowlarrIndexers []prowlarr.IndexerResource
	if err := json.Unmarshal(body, &prowlarrIndexers); err != nil {
		return nil, err
	}

	indexers := make([]SourceIndexer, 0, len(prowlarrIndexers))
	for _, pi := range prowlarrIndexers {
		name, _ := pi.Name.Get()

		var uri string
		if pi.IndexerUrls.IsSpecified() {
			urls, _ := pi.IndexerUrls.Get()
			if len(urls) > 0 {
				uri = urls[0]
			}
		}

		var categories []Category
		if pi.Capabilities != nil && pi.Capabilities.Categories.IsSpecified() {
			cats, _ := pi.Capabilities.Categories.Get()
			categories = mapCategories(cats)
		}
		var status *Status
		if pi.Status != nil {
			status = &Status{DisabledTill: pi.Status.DisabledTill, ID: pi.Status.ID, IndexerID: pi.Status.IndexerID, InitialFailure: pi.Status.InitialFailure, MostRecentFailure: pi.Status.MostRecentFailure}
		}
		indexers = append(indexers, SourceIndexer{
			ID: SourceID(*pi.ID), Name: name, URI: uri, Priority: *pi.Priority,
			Categories: categories, Status: status,
		})
	}

	return indexers, nil
}

func mapCategories(in []prowlarr.IndexerCategory) []Category {
	out := make([]Category, len(in))
	for i, c := range in {
		out[i] = Category{ID: c.ID, Name: c.Name, Description: c.Description}
		if c.SubCategories.IsSpecified() {
			children, _ := c.SubCategories.Get()
			out[i].SubCategories = nullable.NewNullableWithValue(mapCategories(children))
		}
	}
	return out
}

func mapRelease(r *prowlarr.ReleaseResource) *Release {
	if r == nil {
		return nil
	}
	out := &Release{Age: r.Age, CommentURL: r.CommentURL, DownloadURL: r.DownloadURL, FileName: r.FileName, GUID: r.GUID, ID: r.ID, ImdbID: r.ImdbID, Indexer: r.Indexer, IndexerID: r.IndexerID, InfoHash: r.InfoHash, InfoURL: r.InfoURL, Leechers: r.Leechers, MagnetURL: r.MagnetURL, PosterURL: r.PosterURL, PublishDate: r.PublishDate, Seeders: r.Seeders, Size: r.Size, Title: r.Title, TmdbID: r.TmdbID, TvdbID: r.TvdbID}
	if r.Protocol != nil {
		v := string(*r.Protocol)
		out.Protocol = &v
	}
	if r.Categories.IsSpecified() {
		cs, _ := r.Categories.Get()
		out.Categories = nullable.NewNullableWithValue(mapCategories(cs))
	}
	return out
}

func (p *ProwlarrIndexerSource) Search(ctx context.Context, indexerID SourceID, categories []int32, opts SearchOptions) ([]*Release, error) {
	log := logger.FromCtx(ctx)

	query := opts.Query
	if opts.Season != nil {
		query = fmt.Sprintf("%s S%02d", query, *opts.Season)
		if opts.Episode != nil {
			query = fmt.Sprintf("%sE%02d", query, *opts.Episode)
		}
	}

	log.Debug("searching indexer", zap.Int32("indexer_id", int32(indexerID)), zap.String("query", query))

	resp, err := p.client.GetAPIV1Search(ctx, &prowlarr.GetAPIV1SearchParams{
		IndexerIds: &[]int32{int32(indexerID)},
		Query:      &query,
		Categories: &categories,
		Limit:      ptr.To(int32(100)),
		Type:       opts.Type,
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var wireReleases []*prowlarr.ReleaseResource
	if err := json.Unmarshal(body, &wireReleases); err != nil {
		return nil, err
	}
	releases := make([]*Release, 0, len(wireReleases))
	for _, r := range wireReleases {
		releases = append(releases, mapRelease(r))
	}
	return releases, nil
}
