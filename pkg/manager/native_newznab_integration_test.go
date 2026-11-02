package manager

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/kasuboski/mediaz/pkg/download"
	"github.com/kasuboski/mediaz/pkg/indexer"
	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/oapi-codegen/nullable"
	"github.com/stretchr/testify/require"
)

// TestNativeNewznabIndexerServiceIntegration exercises native Newznab from
// source configuration through real HTTP requests and SQLite-backed searches.
// Its fixture is deliberately local; this test must never contact a provider.
func TestNativeNewznabIndexerServiceIntegration(t *testing.T) {
	ctx := context.Background()
	var requests []url.Values
	var requestsMu sync.Mutex
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api", r.URL.Path)
		q := r.URL.Query()
		requestsMu.Lock()
		requests = append(requests, q)
		requestsMu.Unlock()
		w.Header().Set("Content-Type", "application/rss+xml")
		switch q.Get("t") {
		case "caps":
			fmt.Fprint(w, `<?xml version="1.0"?><caps><server title="fixture"/><limits max="100" default="100"/><searching><search available="yes" supportedParams="q"/><tv-search available="yes" supportedParams="q,season,ep"/><movie-search available="yes" supportedParams="q,imdbid"/></searching><categories><category id="2000" name="Movies"><subcat id="2040" name="HD"/></category></categories></caps>`)
		case "search", "movie", "tvsearch":
			offset, _ := strconv.Atoi(q.Get("offset"))
			count := 100
			if offset == 100 {
				count = 20
			}
			fmt.Fprint(w, `<?xml version="1.0"?><rss><channel>`)
			for i := 0; i < count; i++ {
				n := offset + i
				fmt.Fprintf(w, `<item><title>Fixture Release %03d</title><guid isPermaLink="false">opaque-guid-%03d</guid><link>http://fixture.local/get?nzb=%03d</link><pubDate>Mon, 02 Jan 2006 15:04:05 +0000</pubDate><enclosure url="http://fixture.local/download/%03d.nzb" length="734003200" type="application/x-nzb"/><category>Movies</category><attr name="category" value="2040"/><attr name="seeders" value="9"/><attr name="leechers" value="2"/></item>`, n, n, n, n)
			}
			fmt.Fprint(w, `</channel></rss>`)
		default:
			http.Error(w, "unexpected Newznab operation", http.StatusBadRequest)
		}
	}))
	defer feed.Close()

	host, portText, err := net.SplitHostPort(feed.Listener.Addr().String())
	require.NoError(t, err)
	port, err := strconv.ParseInt(portText, 10, 32)
	require.NoError(t, err)
	port32 := int32(port)

	store, err := sqlite.New(ctx, ":memory:")
	require.NoError(t, err)
	service := NewIndexerService(store, store, indexer.NewIndexerSourceFactory())
	create := func(name string, enabled bool) IndexerSourceResponse {
		t.Helper()
		source, err := service.CreateIndexerSource(ctx, AddIndexerSourceRequest{
			Name: name, Implementation: "newznab", Scheme: "http", Host: host, Port: &port32,
			APIPath: "/api", APIKey: stringPtr("fixture-key"), Enabled: enabled,
		})
		require.NoError(t, err)
		return source
	}

	first := create("fixture-a", true)
	second := create("fixture-b", true)
	require.NotEqual(t, first.ID, second.ID)
	// Both independent adapters expose Newznab-local indexer ID 1. Their
	// persisted row IDs must nevertheless be unique and stable across refresh.
	idsForSource := func(sourceID int64) []int64 {
		t.Helper()
		rows, err := store.ListIndexers(ctx)
		require.NoError(t, err)
		var ids []int64
		for _, row := range rows {
			if row.IndexerSourceID.Valid && row.IndexerSourceID.Int64 == sourceID {
				ids = append(ids, row.ID)
			}
		}
		return ids
	}
	firstIDs, secondIDs := idsForSource(first.ID), idsForSource(second.ID)
	require.Len(t, firstIDs, 1)
	require.Len(t, secondIDs, 1)
	require.NotEqual(t, firstIDs[0], secondIDs[0])
	require.NoError(t, service.RefreshIndexerSource(ctx, first.ID))
	require.NoError(t, service.RefreshIndexerSource(ctx, second.ID))
	require.Equal(t, firstIDs, idsForSource(first.ID))
	require.Equal(t, secondIDs, idsForSource(second.ID))

	// Recreate the service to model process restart and rebuild its cache from
	// the enabled source rows persisted in SQLite.
	service = NewIndexerService(store, store, indexer.NewIndexerSourceFactory())
	require.NoError(t, service.RefreshAllIndexerSources(ctx))
	require.Equal(t, firstIDs, idsForSource(first.ID))
	require.Equal(t, secondIDs, idsForSource(second.ID))

	// Isolate the disabled-source phase from setup caps traffic.
	requestsMu.Lock()
	requests = nil
	requestsMu.Unlock()
	disabled := create("fixture-disabled", false)
	require.NoError(t, service.RefreshIndexerSource(ctx, disabled.ID))
	requestsMu.Lock()
	require.Empty(t, requests, "disabled source must not be contacted")
	requestsMu.Unlock()
	storedDisabled, err := service.GetIndexerSource(ctx, disabled.ID)
	require.NoError(t, err)
	require.False(t, storedDisabled.Enabled)
	require.Equal(t, "/api", storedDisabled.APIPath)

	// Paginate beyond the 100-result Newznab cap and check a complete public
	// release value against independent expectations.
	options := indexer.SearchOptions{Query: "deep space"}
	releases, err := service.SearchIndexers(ctx, []indexer.RowID{indexer.RowID(firstIDs[0])}, []int32{2040}, options)
	require.NoError(t, err)
	require.Len(t, releases, 120)
	require.Equal(t, "Fixture Release 000", releases[0].Title.MustGet())
	last := releases[len(releases)-1]
	wantProtocol := "usenet"
	wantIndexerID := int32(1)
	wantRelease := &indexer.Release{
		Title:       nullable.NewNullableWithValue("Fixture Release 119"),
		GUID:        nullable.NewNullableWithValue("opaque-guid-119"),
		DownloadURL: nullable.NewNullableWithValue("http://fixture.local/download/119.nzb"),
		Indexer:     nullable.NewNullableWithValue("fixture-a"),
		IndexerID:   &wantIndexerID,
		Protocol:    &wantProtocol,
		Size:        ptr.To(int64(734003200)),
	}
	require.Equal(t, wantRelease, last, "representative native Newznab release domain mapping")

	// SABnzbd should receive the opaque download URL, not the independent GUID.
	var sabURL string
	sabClient := download.NewSabnzbdClient(nativeNewznabSabHTTPClient(func(r *http.Request) (*http.Response, error) {
		var body string
		switch r.URL.Query().Get("mode") {
		case "addurl":
			sabURL = r.URL.Query().Get("name")
			body = `{"nzo_ids":["NZB-1"]}`
		case "queue":
			body = `{"queue":{"slots":[{"nzo_id":"NZB-1","filename":"Fixture Release 119","percentage":"0","mb":"700"}],"speed":"0 KB/s"}}`
		case "history":
			body = `{"history":{"slots":[]}}`
		default:
			return nil, fmt.Errorf("unexpected SABnzbd mode %q", r.URL.Query().Get("mode"))
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	}), "http", "sab.fixture.local", "", "fixture-sab-key")
	status, err := sabClient.Add(ctx, download.AddRequest{Release: last})
	require.NoError(t, err)
	require.Equal(t, "NZB-1", status.ID)
	require.Equal(t, "http://fixture.local/download/119.nzb", sabURL)
	require.NotEqual(t, last.GUID.MustGet(), sabURL)

	requestsMu.Lock()
	requestsForSearch := append([]url.Values(nil), requests...)
	requestsMu.Unlock()
	var offsets []string
	for _, request := range requestsForSearch {
		require.Equal(t, "search", request.Get("t"))
		require.Equal(t, "100", request.Get("limit"))
		require.Equal(t, "deep space", request.Get("q"))
		offsets = append(offsets, request.Get("offset"))
	}
	sort.Strings(offsets)
	require.Equal(t, []string{"0", "100"}, offsets)

	stored, err := service.GetIndexerSource(ctx, first.ID)
	require.NoError(t, err)
	require.Equal(t, "/api", stored.APIPath)
	require.Equal(t, "newznab", stored.Implementation)
}

type nativeNewznabSabHTTPClient func(*http.Request) (*http.Response, error)

func (f nativeNewznabSabHTTPClient) Do(r *http.Request) (*http.Response, error) { return f(r) }

func stringPtr(value string) *string { return &value }
