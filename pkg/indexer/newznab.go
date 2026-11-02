package indexer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kasuboski/mediaz/pkg/newznab"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/oapi-codegen/nullable"
)

// NewznabClient is the protocol surface used by the adapter.
type NewznabClient interface {
	Caps(context.Context) (*newznab.Capabilities, error)
	Search(context.Context, newznab.SearchRequest) (*newznab.Page, error)
}

type NewznabIndexerSource struct {
	client        NewznabClient
	config        sqlcdb.IndexerSource
	mu            sync.Mutex
	gate          chan struct{}
	caps          *newznab.Capabilities
	cooldownUntil time.Time
	nextRequest   time.Time
	minInterval   time.Duration
}

func NewNewznabSource(config sqlcdb.IndexerSource) (*NewznabIndexerSource, error) {
	endpoint, err := newznabEndpoint(config)
	if err != nil {
		return nil, err
	}
	client, err := newznab.New(endpoint, config.ApiKey.String)
	if err != nil {
		return nil, fmt.Errorf("create Newznab client: %w", err)
	}
	return NewNewznabIndexerSourceWithClient(client, config), nil
}

func NewNewznabIndexerSourceWithClient(client NewznabClient, config sqlcdb.IndexerSource) *NewznabIndexerSource {
	return &NewznabIndexerSource{client: client, config: config, gate: make(chan struct{}, 1), minInterval: 100 * time.Millisecond}
}

// request serializes endpoint traffic and applies context-aware spacing/cooldown.
func (n *NewznabIndexerSource) request(ctx context.Context, call func() error) error {
	select {
	case n.gate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-n.gate }()
	n.mu.Lock()
	cooldown := time.Until(n.cooldownUntil)
	spacing := time.Until(n.nextRequest)
	n.mu.Unlock()
	if cooldown > 0 {
		return &RetryableCooldownError{RetryAfter: cooldown}
	}
	if spacing > 0 {
		timer := time.NewTimer(spacing)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	err := call()
	n.mu.Lock()
	n.nextRequest = time.Now().Add(n.minInterval)
	var pe *newznab.ProtocolError
	if errors.As(err, &pe) {
		delay := pe.RetryAfter
		switch {
		case pe.Code == "500" || pe.Code == "501":
			if delay <= 0 {
				delay = 5 * time.Minute
			}
		case pe.Status == 429 || pe.Status == 503:
			if delay <= 0 {
				delay = time.Minute
			}
		}
		if delay > 5*time.Minute {
			delay = 5 * time.Minute
		}
		if delay > 0 {
			until := time.Now().Add(delay)
			if until.After(n.cooldownUntil) {
				n.cooldownUntil = until
			}
		}
	}
	n.mu.Unlock()
	return err
}

// RetryableCooldownError reports a provider cooldown without blocking refresh/search callers.
type RetryableCooldownError struct{ RetryAfter time.Duration }

func (e *RetryableCooldownError) Error() string {
	return fmt.Sprintf("Newznab provider cooldown active; retry after %s", e.RetryAfter.Round(time.Second))
}

func (n *NewznabIndexerSource) capabilities(ctx context.Context) (*newznab.Capabilities, error) {
	n.mu.Lock()
	cached := n.caps
	n.mu.Unlock()
	if cached != nil {
		return cached, nil
	}
	var c *newznab.Capabilities
	err := n.request(ctx, func() error { var e error; c, e = n.client.Caps(ctx); return e })
	if err != nil {
		return nil, fmt.Errorf("fetch Newznab capabilities: %w", err)
	}
	if c == nil {
		return nil, errors.New("Newznab returned empty capabilities")
	}
	n.mu.Lock()
	if n.caps != nil {
		c = n.caps
	} else {
		n.caps = c
	}
	n.mu.Unlock()
	return c, nil
}

func (n *NewznabIndexerSource) ListIndexers(ctx context.Context) ([]SourceIndexer, error) {
	// RefreshIndexers is the explicit capability refresh boundary used by source refresh.
	n.mu.Lock()
	n.caps = nil
	n.mu.Unlock()
	caps, err := n.capabilities(ctx)
	if err != nil {
		return nil, err
	}
	cats := make([]Category, 0, len(caps.Categories))
	for _, c := range caps.Categories {
		id, _ := strconv.Atoi(c.ID)
		cat := Category{ID: &[]int32{int32(id)}[0], Name: nullable.NewNullableWithValue(c.Name)}
		for _, sub := range c.Subcategories {
			sid, _ := strconv.Atoi(sub.ID)
			child := Category{ID: &[]int32{int32(sid)}[0], Name: nullable.NewNullableWithValue(sub.Name)}
			cat.SubCategories = nullable.NewNullableWithValue(appendNullableCategory(cat.SubCategories, child))
		}
		cats = append(cats, cat)
	}
	return []SourceIndexer{{ID: 1, Name: n.config.Name, URI: n.endpoint(), Categories: cats}}, nil
}

func appendNullableCategory(v nullable.Nullable[[]Category], c Category) []Category {
	if v.IsSpecified() {
		x, _ := v.Get()
		return append(x, c)
	}
	return []Category{c}
}
func (n *NewznabIndexerSource) endpoint() string { s, _ := newznabEndpoint(n.config); return s }

func newznabEndpoint(config sqlcdb.IndexerSource) (string, error) {
	path := "/api"
	if config.ApiPath.Valid && strings.TrimSpace(config.ApiPath.String) != "" {
		path = config.ApiPath.String
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	host := strings.Trim(config.Host, "[]")
	if config.Port.Valid {
		host = net.JoinHostPort(host, strconv.FormatInt(config.Port.Int64, 10))
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	u := url.URL{Scheme: config.Scheme, Host: host, Path: path}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", errors.New("invalid Newznab endpoint")
	}
	return u.String(), nil
}

func (n *NewznabIndexerSource) Search(ctx context.Context, id SourceID, categories []int32, opts SearchOptions) ([]*Release, error) {
	if id != 1 {
		return nil, fmt.Errorf("unknown Newznab indexer ID %d", id)
	}
	caps, err := n.capabilities(ctx)
	if err != nil {
		return nil, err
	}
	typ := "search"
	requested := "search"
	if opts.Type != nil {
		requested = *opts.Type
	}
	structured := ""
	if requested == TypeMovie {
		structured = "movie-search"
	} else if requested == TypeTV {
		structured = "tv-search"
	}
	params := newznab.SearchCapability{}
	if structured != "" {
		params = caps.Searches[structured]
	}
	hasParam := func(p newznab.SearchCapability, name string) bool {
		for _, v := range p.Params {
			if strings.EqualFold(v, name) {
				return true
			}
		}
		return false
	}
	usableID := (opts.TmdbID != nil && hasParam(params, "tmdbid")) || (opts.IMDbID != nil && hasParam(params, "imdbid")) || (opts.TvdbID != nil && hasParam(params, "tvdbid"))
	usableTV := requested == TypeTV && ((opts.Season != nil && hasParam(params, "season")) || (opts.Episode != nil && hasParam(params, "ep")))
	if structured != "" && params.Available && (usableID || usableTV) {
		if requested == TypeMovie {
			typ = "movie"
		} else {
			typ = "tvsearch"
		}
	} else {
		params = caps.Searches["search"]
		if !params.Available {
			return nil, errors.New("Newznab text search is unavailable")
		}
	}
	if typ != "search" && !params.Available {
		return nil, fmt.Errorf("Newznab %s search is unavailable", typ)
	}
	if typ == "search" && !caps.Searches["search"].Available {
		return nil, errors.New("Newznab text search is unavailable")
	}
	if typ == "search" && requested == TypeTV {
		var suffix string
		if opts.Season != nil {
			suffix = fmt.Sprintf(" S%02d", *opts.Season)
			if opts.Episode != nil {
				suffix += fmt.Sprintf("E%02d", *opts.Episode)
			}
		}
		if suffix != "" {
			reqQuery := opts.Query + suffix
			opts.Query = reqQuery
		}
	}
	req := newznab.SearchRequest{Type: typ, Query: opts.Query, Limit: 100}
	has := func(s string) bool { return hasParam(params, s) }
	if opts.TmdbID != nil && has("tmdbid") {
		req.TMDBID = strconv.Itoa(int(*opts.TmdbID))
	}
	if opts.IMDbID != nil && has("imdbid") {
		req.IMDBID = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(*opts.IMDbID)), "tt")
		if _, e := strconv.ParseInt(req.IMDBID, 10, 64); e != nil {
			req.IMDBID = ""
		}
	}
	if opts.TvdbID != nil && has("tvdbid") {
		req.TVDBID = strconv.Itoa(int(*opts.TvdbID))
	}
	if req.TMDBID != "" || req.IMDBID != "" || req.TVDBID != "" {
		req.Query = ""
	}
	if opts.Season != nil && has("season") {
		x := int(*opts.Season)
		req.Season = &x
	}
	if opts.Episode != nil && has("ep") {
		x := int(*opts.Episode)
		req.Episode = &x
	}
	for _, c := range categories {
		req.Categories = append(req.Categories, strconv.Itoa(int(c)))
	}
	limit := caps.Max
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 100
	}
	req.Limit = limit
	out := []*Release{}
	seen := map[string]bool{}
	pages := map[string]bool{}
	offset := 0
	for page := 0; page < 5; page++ {
		req.Offset = offset
		var p *newznab.Page
		e := n.request(ctx, func() error { var callErr error; p, callErr = n.client.Search(ctx, req); return callErr })
		if e != nil {
			return out, fmt.Errorf("Newznab search: %w", e)
		}
		if p == nil {
			return out, errors.New("Newznab returned empty page")
		}
		hash := ""
		for _, item := range p.Items {
			hash += "|" + item.GUID + "|" + item.Title + "|" + item.EnclosureURL + "|" + item.Link
		}
		if pages[hash] && len(p.Items) > 0 {
			return out, errors.New("Newznab repeated a page")
		}
		pages[hash] = true
		for _, item := range p.Items {
			title := strings.TrimSpace(item.Title)
			if title == "" || item.Size <= 0 {
				continue
			}
			target := strings.TrimSpace(item.EnclosureURL)
			if target == "" {
				target = strings.TrimSpace(item.Link)
			}
			if target == "" {
				continue
			}
			target, e = resolveSafe(n.endpoint(), target)
			if e != nil {
				continue
			}
			guid := strings.TrimSpace(item.GUID)
			if guid == "" {
				guid = target
			}
			if seen[guid] {
				continue
			}
			seen[guid] = true
			size := item.Size
			protocol := "usenet"
			rel := &Release{Size: &size, Title: nullable.NewNullableWithValue(title), GUID: nullable.NewNullableWithValue(guid), DownloadURL: nullable.NewNullableWithValue(target), Indexer: nullable.NewNullableWithValue(n.config.Name), IndexerID: func() *int32 { x := int32(1); return &x }(), Protocol: &protocol}
			if x := attr(item, "imdb"); x != "" {
				x = strings.TrimPrefix(strings.ToLower(x), "tt")
				if v, e := strconv.ParseInt(x, 10, 32); e == nil {
					z := int32(v)
					rel.ImdbID = &z
				}
			}
			if x := attr(item, "tvdbid"); x != "" {
				if v, e := strconv.ParseInt(x, 10, 32); e == nil {
					z := int32(v)
					rel.TvdbID = &z
				}
			}
			if x := attr(item, "tmdbid"); x != "" {
				if v, e := strconv.ParseInt(x, 10, 32); e == nil {
					z := int32(v)
					rel.TmdbID = &z
				}
			}
			out = append(out, rel)
		}
		offset += len(p.Items)
		if page == 4 && len(p.Items) > 0 && (p.Total == 0 || offset < p.Total) {
			return out, &SearchTruncatedError{Limit: limit * 5, Results: len(out)}
		}
		if len(p.Items) == 0 || (p.Total > 0 && offset >= p.Total) || (p.Total <= 0 && len(p.Items) < limit) {
			break
		}
	}
	return out, nil
}

type SearchTruncatedError struct{ Limit, Results int }

func (e *SearchTruncatedError) Error() string {
	return fmt.Sprintf("Newznab results truncated at %d pages (%d releases)", e.Limit, e.Results)
}
func attr(i newznab.Item, key string) string {
	for k, v := range i.Attributes {
		if strings.EqualFold(k, key) && len(v) > 0 {
			return strings.TrimSpace(v[0])
		}
	}
	return ""
}
func resolveSafe(base, raw string) (string, error) {
	u, e := url.Parse(strings.TrimSpace(raw))
	if e != nil {
		return "", e
	}
	b, e := url.Parse(base)
	if e != nil {
		return "", e
	}
	u = b.ResolveReference(u)
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", errors.New("unsafe download URL")
	}
	if u.User != nil {
		return "", errors.New("unsafe download URL")
	}
	return u.String(), nil
}
