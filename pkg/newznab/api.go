// Package newznab implements the Newznab XML-over-HTTP protocol.
package newznab

import (
	"context"
	"net/http"
)

// HTTPDoer is the injectable transport seam used by Client.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type Client struct {
	endpoint, apiKey string
	http             HTTPDoer
}

// New constructs a client for a complete Newznab API endpoint.
func New(endpoint, apiKey string) (*Client, error) { return NewWithHTTP(endpoint, apiKey, nil) }

// NewWithHTTP is New with an injectable HTTP transport.
func NewWithHTTP(endpoint, apiKey string, doer HTTPDoer) (*Client, error) {
	return newClient(endpoint, apiKey, doer)
}

type Capabilities struct {
	Max, Default int
	Searches     map[string]SearchCapability
	Categories   []Category
}
type SearchCapability struct {
	Available bool
	Params    []string
}
type Category struct {
	ID, Name      string
	Subcategories []Category
}
type SearchRequest struct {
	Type                                         string // search, movie, or tvsearch
	Query, IMDBID, TVDBID, TVMazeID, RID, TMDBID string
	Season, Episode                              *int
	Categories                                   []string
	Limit, Offset                                int
}
type Page struct {
	Offset, Total int
	TotalKnown    bool
	Items         []Item
}
type Item struct {
	Title, GUID, Link, EnclosureURL, EnclosureType string
	Size                                           int64
	Attributes                                     map[string][]string
}

// Caps fetches endpoint capabilities.
func (c *Client) Caps(ctx context.Context) (*Capabilities, error) { return c.caps(ctx) }

// Search runs one bounded Newznab request.
func (c *Client) Search(ctx context.Context, request SearchRequest) (*Page, error) {
	return c.search(ctx, request)
}
