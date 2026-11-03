package indexer_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/kasuboski/mediaz/pkg/indexer"
	"github.com/kasuboski/mediaz/pkg/indexer/mocks"
	"github.com/kasuboski/mediaz/pkg/newznab"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/oapi-codegen/nullable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func capsFixture() *newznab.Capabilities {
	return &newznab.Capabilities{Max: 100, Searches: map[string]newznab.SearchCapability{"search": {Available: true, Params: []string{"q"}}, "movie-search": {Available: true, Params: []string{"imdbid", "tmdbid"}}, "tv-search": {Available: true, Params: []string{"tvdbid", "season", "ep"}}}}
}
func config() sqlcdb.IndexerSource {
	return sqlcdb.IndexerSource{Name: "NZBGeek", Scheme: "http", Host: "example.test"}
}
func TestNewznabSearchUsesStructuredOnlyWhenUsableAndFallbacks(t *testing.T) {
	ctrl := gomock.NewController(t)
	c := mocks.NewMockNewznabClient(ctrl)
	src := indexer.NewNewznabIndexerSourceWithClient(c, config())
	cap := capsFixture()
	cap.Searches["tv-search"] = newznab.SearchCapability{Available: true, Params: []string{"tvdbid"}}
	c.EXPECT().Caps(gomock.Any()).Return(cap, nil)
	c.EXPECT().Search(gomock.Any(), gomock.Cond(func(v any) bool {
		r, ok := v.(newznab.SearchRequest)
		return ok && r.Type == "search" && r.Query == "Arrival S02E03" && r.TMDBID == ""
	})).Return(&newznab.Page{}, nil)
	season, episode := int64(2), int64(3)
	tmdb := int32(42)
	typ := indexer.TypeTV
	_, err := src.Search(context.Background(), 1, nil, indexer.SearchOptions{Query: "Arrival", Type: &typ, Season: &season, Episode: &episode, TmdbID: &tmdb})
	require.NoError(t, err)
}
func TestNewznabSearchMapsFullRelease(t *testing.T) {
	ctrl := gomock.NewController(t)
	c := mocks.NewMockNewznabClient(ctrl)
	src := indexer.NewNewznabIndexerSourceWithClient(c, config())
	c.EXPECT().Caps(gomock.Any()).Return(capsFixture(), nil)
	c.EXPECT().Search(gomock.Any(), gomock.Any()).Return(&newznab.Page{Items: []newznab.Item{{Title: "  Arrival  ", GUID: "urn:unique:1", EnclosureURL: "/get/abc", Size: 1234, Attributes: map[string][]string{"imdb": {"tt0012345"}, "tmdbid": {"27205"}, "tvdbid": {"7"}}}}, Total: 1}, nil)
	got, err := src.Search(context.Background(), 1, []int32{2000}, indexer.SearchOptions{Query: "Arrival"})
	require.NoError(t, err)
	require.Len(t, got, 1)
	imdb, tmdb, tvdb, id := int32(12345), int32(27205), int32(7), int32(1)
	size := int64(1234)
	protocol := "usenet"
	want := &indexer.Release{Size: &size, Title: nullable.NewNullableWithValue("Arrival"), GUID: nullable.NewNullableWithValue("urn:unique:1"), DownloadURL: nullable.NewNullableWithValue("http://example.test/get/abc"), Indexer: nullable.NewNullableWithValue("NZBGeek"), IndexerID: &id, Protocol: &protocol, ImdbID: &imdb, TmdbID: &tmdb, TvdbID: &tvdb}
	require.Equal(t, want, got[0])
}
func TestNewznabPagesEqualSizedAndRepeatedGuard(t *testing.T) {
	for _, repeat := range []bool{false, true} {
		t.Run(fmt.Sprint("repeat=", repeat), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			c := mocks.NewMockNewznabClient(ctrl)
			src := indexer.NewNewznabIndexerSourceWithClient(c, config())
			cap := capsFixture()
			cap.Max = 2
			c.EXPECT().Caps(gomock.Any()).Return(cap, nil)
			item := func(id string) newznab.Item { return newznab.Item{Title: id, GUID: id, Link: "/" + id, Size: 10} }
			times := 3
			if repeat {
				times = 2
			}
			c.EXPECT().Search(gomock.Any(), gomock.Any()).Times(times).DoAndReturn(func(_ context.Context, r newznab.SearchRequest) (*newznab.Page, error) {
				if r.Offset == 0 {
					return &newznab.Page{Total: 6, Items: []newznab.Item{item("a"), item("b")}}, nil
				}
				if r.Offset == 2 {
					if repeat {
						return &newznab.Page{Total: 6, Items: []newznab.Item{item("a"), item("b")}}, nil
					}
					return &newznab.Page{Total: 6, Items: []newznab.Item{item("c"), item("d")}}, nil
				}
				return &newznab.Page{Total: 6, Items: []newznab.Item{item("e"), item("f")}}, nil
			})
			got, err := src.Search(context.Background(), 1, nil, indexer.SearchOptions{})
			if repeat {
				require.ErrorContains(t, err, "repeated")
				require.Len(t, got, 2)
			} else {
				require.NoError(t, err)
				require.Len(t, got, 6)
			}
		})
	}
}
func TestNewznabRejectsBadItemsAndUnsafeURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	c := mocks.NewMockNewznabClient(ctrl)
	src := indexer.NewNewznabIndexerSourceWithClient(c, config())
	c.EXPECT().Caps(gomock.Any()).Return(capsFixture(), nil)
	c.EXPECT().Search(gomock.Any(), gomock.Any()).Return(&newznab.Page{Items: []newznab.Item{{Title: "missing size", GUID: "a", Link: "/a"}, {Title: "blank target", GUID: "z", Size: 10}, {Title: "invalid size", GUID: "b", Link: "/b", Size: -1}, {Title: "bad scheme", GUID: "c", Link: "javascript:alert(1)", Size: 10}, {Title: "userinfo", GUID: "d", Link: "http://u:p@evil.test/x", Size: 10}}}, nil)
	got, err := src.Search(context.Background(), 1, nil, indexer.SearchOptions{})
	require.NoError(t, err)
	require.Empty(t, got)
}
func TestNewznabRefreshReFetchesCapabilities(t *testing.T) {
	ctrl := gomock.NewController(t)
	c := mocks.NewMockNewznabClient(ctrl)
	src := indexer.NewNewznabIndexerSourceWithClient(c, config())
	first := capsFixture()
	first.Categories = []newznab.Category{{ID: "2000", Name: "Movies"}}
	second := capsFixture()
	second.Categories = []newznab.Category{{ID: "5000", Name: "TV"}}
	c.EXPECT().Caps(gomock.Any()).Return(first, nil)
	c.EXPECT().Caps(gomock.Any()).Return(second, nil)
	a, err := src.ListIndexers(context.Background())
	require.NoError(t, err)
	b, err := src.ListIndexers(context.Background())
	require.NoError(t, err)
	require.Equal(t, int32(2000), *a[0].Categories[0].ID)
	require.Equal(t, int32(5000), *b[0].Categories[0].ID)
}
func TestNewznabShortPageContinuesWhenTotalSaysMore(t *testing.T) {
	ctrl := gomock.NewController(t)
	c := mocks.NewMockNewznabClient(ctrl)
	src := indexer.NewNewznabIndexerSourceWithClient(c, config())
	cap := capsFixture()
	cap.Max = 100
	c.EXPECT().Caps(gomock.Any()).Return(cap, nil)
	mk := func(start, count int) []newznab.Item {
		x := []newznab.Item{}
		for i := 0; i < count; i++ {
			id := fmt.Sprint(start + i)
			x = append(x, newznab.Item{Title: id, GUID: id, Link: "/" + id, Size: 1})
		}
		return x
	}
	c.EXPECT().Search(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r newznab.SearchRequest) (*newznab.Page, error) {
		require.Equal(t, 0, r.Offset)
		return &newznab.Page{Total: 120, Items: mk(0, 50)}, nil
	})
	c.EXPECT().Search(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r newznab.SearchRequest) (*newznab.Page, error) {
		require.Equal(t, 50, r.Offset)
		return &newznab.Page{Total: 120, Items: mk(50, 50)}, nil
	})
	c.EXPECT().Search(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r newznab.SearchRequest) (*newznab.Page, error) {
		require.Equal(t, 100, r.Offset)
		return &newznab.Page{Total: 120, Items: mk(100, 20)}, nil
	})
	got, err := src.Search(context.Background(), 1, nil, indexer.SearchOptions{})
	require.NoError(t, err)
	require.Len(t, got, 120)
}
func TestNewznabFifthPageExhaustionAndTruncation(t *testing.T) {
	tests := []struct {
		name             string
		total, lastCount int
		wantErr          bool
	}{
		{name: "unknown short fifth page", total: 0, lastCount: 1},
		{name: "unknown full fifth page", total: 0, lastCount: 2, wantErr: true},
		{name: "known remaining total short fifth page", total: 10, lastCount: 1, wantErr: true},
		{name: "known exhausted short fifth page", total: 9, lastCount: 1},
		{name: "known exhausted full fifth page", total: 10, lastCount: 2},
		{name: "negative total short fifth page", total: -1, lastCount: 1},
		{name: "negative total full fifth page", total: -1, lastCount: 2, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			c := mocks.NewMockNewznabClient(ctrl)
			src := indexer.NewNewznabIndexerSourceWithClient(c, config())
			cap := capsFixture()
			cap.Max = 2
			c.EXPECT().Caps(gomock.Any()).Return(cap, nil)
			nextOffset := 0
			c.EXPECT().Search(gomock.Any(), gomock.Any()).Times(5).DoAndReturn(func(_ context.Context, r newznab.SearchRequest) (*newznab.Page, error) {
				require.Equal(t, 2, r.Limit)
				require.Equal(t, nextOffset, r.Offset)
				nextOffset += 2
				count := 2
				if r.Offset == 8 {
					count = tt.lastCount
				}
				items := make([]newznab.Item, count)
				for i := range items {
					id := fmt.Sprint(r.Offset + i)
					items[i] = newznab.Item{Title: id, GUID: id, Link: "/x", Size: 1}
				}
				return &newznab.Page{Total: tt.total, Items: items}, nil
			})
			got, err := src.Search(context.Background(), 1, nil, indexer.SearchOptions{})
			wantCount := 8 + tt.lastCount
			require.Len(t, got, wantCount)
			for i, release := range got {
				title := fmt.Sprint(i)
				size, id := int64(1), int32(1)
				protocol := "usenet"
				want := &indexer.Release{Size: &size, Title: nullable.NewNullableWithValue(title), GUID: nullable.NewNullableWithValue(title), DownloadURL: nullable.NewNullableWithValue("http://example.test/x"), Indexer: nullable.NewNullableWithValue("NZBGeek"), IndexerID: &id, Protocol: &protocol}
				require.Equal(t, want, release)
			}
			var truncated *indexer.SearchTruncatedError
			if tt.wantErr {
				require.ErrorAs(t, err, &truncated)
				require.Equal(t, &indexer.SearchTruncatedError{Limit: 10, Results: wantCount}, truncated)
				require.EqualError(t, err, fmt.Sprintf("Newznab results truncated at 10-result page budget (%d releases kept)", wantCount))
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNewznabPageBudgetIsTypedPartialError(t *testing.T) {
	ctrl := gomock.NewController(t)
	c := mocks.NewMockNewznabClient(ctrl)
	src := indexer.NewNewznabIndexerSourceWithClient(c, config())
	cap := capsFixture()
	cap.Max = 1
	c.EXPECT().Caps(gomock.Any()).Return(cap, nil)
	c.EXPECT().Search(gomock.Any(), gomock.Any()).Times(5).DoAndReturn(func(_ context.Context, r newznab.SearchRequest) (*newznab.Page, error) {
		return &newznab.Page{Total: 99, Items: []newznab.Item{{Title: fmt.Sprint(r.Offset), GUID: fmt.Sprint(r.Offset), Link: "/x", Size: 1}}}, nil
	})
	got, err := src.Search(context.Background(), 1, nil, indexer.SearchOptions{})
	var truncated *indexer.SearchTruncatedError
	require.ErrorAs(t, err, &truncated)
	require.Len(t, got, 5)
}
func TestNewznabCapsCachedAndUnavailableSearchErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	c := mocks.NewMockNewznabClient(ctrl)
	src := indexer.NewNewznabIndexerSourceWithClient(c, config())
	c.EXPECT().Caps(gomock.Any()).Return(&newznab.Capabilities{Searches: map[string]newznab.SearchCapability{"movie": {Available: true}}}, nil).Times(1)
	_, err := src.Search(context.Background(), 1, nil, indexer.SearchOptions{})
	require.ErrorContains(t, err, "unavailable")
}
func TestNewznabFixtureHTTPScenarios(t *testing.T) {
	root := filepath.Join("testdata", "newznab", "nzbgeek")
	scenarios := []struct {
		name, query, typ, imdb, tvdb string
		season, episode              *int64
		cats                         []int32
		wantCount                    int
	}{
		{name: "movie-text", query: "step brothers", typ: indexer.TypeMovie, cats: []int32{2010, 2020, 2030, 2040, 2045, 2050, 2060, 2000}, wantCount: 67},
		{name: "movie-imdb", typ: indexer.TypeMovie, imdb: "tt0838283", cats: []int32{2010, 2020, 2030, 2040, 2045, 2050, 2060, 2000}, wantCount: 69},
		{name: "tv-text-mixed-metadata", query: "utopia", typ: indexer.TypeTV, cats: []int32{5020, 5030, 5040, 5045, 5050, 5060, 5070, 5080, 5000}, wantCount: 100},
		{name: "tv-text", query: "the british baking show", typ: indexer.TypeTV, cats: []int32{5020, 5030, 5040, 5045, 5050, 5060, 5070, 5080, 5000}, wantCount: 120},
		{name: "tv-series", typ: indexer.TypeTV, tvdb: "353994", cats: []int32{5020, 5030, 5040, 5045, 5050, 5060, 5070, 5080, 5000}, wantCount: 120},
		{name: "tv-season", typ: indexer.TypeTV, tvdb: "353994", season: ptrInt64(2), cats: []int32{5020, 5030, 5040, 5045, 5050, 5060, 5070, 5080, 5000}, wantCount: 30},
		{name: "tv-episode", typ: indexer.TypeTV, tvdb: "353994", season: ptrInt64(2), episode: ptrInt64(1), cats: []int32{5020, 5030, 5040, 5045, 5050, 5060, 5070, 5080, 5000}, wantCount: 2},
		{name: "tv-page-two", query: "the british baking show", typ: indexer.TypeTV, cats: []int32{5020, 5030, 5040, 5045, 5050, 5060, 5070, 5080, 5000}, wantCount: 120},
	}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			caps, e := os.ReadFile(filepath.Join(root, "caps", "response.xml"))
			require.NoError(t, e)
			firstName := sc.name
			if sc.name == "tv-page-two" {
				firstName = "tv-text"
			}
			first, e := os.ReadFile(filepath.Join(root, firstName, "response.xml"))
			require.NoError(t, e)
			// Utopia has only its captured first 100 results. This synthetic empty end-page
			// tests pagination; it is not a captured or real second page.
			second := []byte(`<rss><channel><newznab:response xmlns:newznab="http://www.newznab.com/DTD/2010/feeds/attributes/" offset="100" total="100" /></channel></rss>`)
			if sc.name == "tv-text" || sc.name == "tv-series" || sc.name == "tv-page-two" {
				second, e = os.ReadFile(filepath.Join(root, "tv-page-two", "response.xml"))
				require.NoError(t, e)
			}
			calls := 0
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				q := r.URL.Query()
				if q.Get("t") == "caps" {
					_, _ = w.Write(caps)
					return
				}
				calls++
				assert.Equal(t, "TEST_API_KEY", q.Get("apikey"))
				assert.Equal(t, "100", q.Get("limit"))
				assert.Equal(t, csvInts(sc.cats), q.Get("cat"))
				if sc.name == "movie-imdb" {
					assert.Equal(t, "movie", q.Get("t"))
					assert.Equal(t, "0838283", q.Get("imdbid"))
					assert.Empty(t, q.Get("q"))
				} else if sc.name == "tv-series" || sc.name == "tv-season" || sc.name == "tv-episode" {
					assert.Equal(t, "tvsearch", q.Get("t"))
					assert.Equal(t, "353994", q.Get("tvdbid"))
					if sc.season != nil {
						assert.Equal(t, "2", q.Get("season"))
					}
					if sc.episode != nil {
						assert.Equal(t, "1", q.Get("ep"))
					}
				} else {
					assert.Equal(t, "search", q.Get("t"))
					assert.Equal(t, sc.query, q.Get("q"))
				}
				if q.Get("offset") != "0" {
					if q.Get("offset") == "100" {
						_, _ = w.Write(second)
					} else {
						_, _ = w.Write([]byte(`<rss><channel/></rss>`))
					}
					return
				}
				_, _ = w.Write(first)
			}))
			defer ts.Close()
			client, e := newznab.New(ts.URL, "TEST_API_KEY")
			require.NoError(t, e)
			cfg := config()
			host, port, _ := net.SplitHostPort(ts.Listener.Addr().String())
			cfg.Host = host
			pnum, _ := strconv.ParseInt(port, 10, 64)
			cfg.Port = sql.NullInt64{Int64: pnum, Valid: true}
			cfg.Scheme = "http"
			cfg.ApiPath = sql.NullString{String: "", Valid: false}
			src := indexer.NewNewznabIndexerSourceWithClient(client, cfg)
			_, e = src.ListIndexers(context.Background())
			require.NoError(t, e)
			opts := indexer.SearchOptions{Type: &sc.typ, Query: sc.query, Season: sc.season, Episode: sc.episode}
			if sc.imdb != "" {
				opts.IMDbID = &sc.imdb
			}
			if sc.tvdb != "" {
				v := int32(353994)
				opts.TvdbID = &v
			}
			got, e := src.Search(context.Background(), 1, sc.cats, opts)
			require.NoError(t, e)
			require.Len(t, got, sc.wantCount)
			switch sc.name {
			case "tv-text-mixed-metadata", "tv-text", "tv-series", "tv-page-two":
				require.Equal(t, 2, calls)
			default:
				require.Equal(t, 1, calls)
			}
			if sc.name == "tv-text-mixed-metadata" {
				// Exactly 100 captured results, not the full 3284-result corpus.
				require.Len(t, got, 100)
			}
			if sc.name == "movie-imdb" {
				size, id, imdb := int64(55106898000), int32(1), int32(838283)
				protocol := "usenet"
				want := &indexer.Release{Size: &size, Title: nullable.NewNullableWithValue("Step.Brothers.2008.Unrated.2160p.UHD.BluRay.REMUX.HDR.HEVC.Atmos-EPSiLON"), GUID: nullable.NewNullableWithValue("https://indexer.example.invalid/details/release-000001"), DownloadURL: nullable.NewNullableWithValue("https://indexer.example.invalid/api?t=get&id=release-000001&apikey=TEST_API_KEY"), Indexer: nullable.NewNullableWithValue("NZBGeek"), IndexerID: &id, Protocol: &protocol, ImdbID: &imdb}
				require.Equal(t, want, got[0])
			}
			if sc.name == "tv-episode" {
				size, id, tvdb := int64(1108072000), int32(1), int32(353994)
				protocol := "usenet"
				want := &indexer.Release{Size: &size, Title: nullable.NewNullableWithValue("The.Great.British.Baking.Show.S02E01.Cakes.720p.WEB.h264-PFa"), GUID: nullable.NewNullableWithValue("https://indexer.example.invalid/details/release-000479"), DownloadURL: nullable.NewNullableWithValue("https://indexer.example.invalid/api?t=get&id=release-000479&apikey=TEST_API_KEY"), Indexer: nullable.NewNullableWithValue("NZBGeek"), IndexerID: &id, Protocol: &protocol, TvdbID: &tvdb}
				require.Equal(t, want, got[0])
			}
		})
	}
}
func ptrInt64(v int64) *int64 { return &v }
func csvInts(v []int32) string {
	parts := make([]string, len(v))
	for i, x := range v {
		parts[i] = strconv.Itoa(int(x))
	}
	return strings.Join(parts, ",")
}
func TestNewznabProtocolErrorPropagates(t *testing.T) {
	ctrl := gomock.NewController(t)
	c := mocks.NewMockNewznabClient(ctrl)
	src := indexer.NewNewznabIndexerSourceWithClient(c, config())
	c.EXPECT().Caps(gomock.Any()).Return(capsFixture(), nil)
	c.EXPECT().Search(gomock.Any(), gomock.Any()).Return(nil, errors.New("provider rate limited"))
	_, err := src.Search(context.Background(), 1, nil, indexer.SearchOptions{})
	require.ErrorContains(t, err, "rate limited")
}
