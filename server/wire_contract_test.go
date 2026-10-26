// Wire-contract capture tests.
//
// These tests pin the pre-refactor (29f62f9) HTTP response shapes and ensure
// sql.Null values and secrets do not leak onto the wire. Intentional seed,
// secret, transition-history and stable-order deltas are enumerated in
// testdata/wire/README.md; other drift must be fixed, not re-captured.
//
// How it works:
//   - Each test builds the REAL stack: in-memory sqlite storage
//     (sqlite.New(":memory:")), the real MediaManager,
//     and the full mux router from registerRoutes. Only the TMDB client is
//     a gomock (search/details endpoints).
//   - Representative entities are seeded directly through the storage layer:
//     movies with metadata+file+transition history (nullable columns both
//     NULL and non-NULL), series/season/episode tree, indexer + indexer
//     source with api_key set, download client with api_key set, quality
//     profiles/definitions (plus the seeded defaults), jobs with pending and
//     done transitions, and activity-producing downloads.
//   - Each capture asserts the status code, decodes+walks the body to guard
//     against sql.Null leakage (no "Valid"/"String" keys anywhere), applies
//     a documented secrets policy (see TestWireContract_SecretsAudit), and
//     compares the normalized raw JSON against a golden file in
//     testdata/wire/. Normalization only masks known-dynamic values:
//     RFC3339 timestamps ("<timestamp>"), values under "duration" keys
//     ("<duration>"), and timeline bucket dates ("<date>").
//
// To capture a deliberately reviewed contract change (never a blanket fix):
//
//	WIRE_CAPTURE_UPDATE=1 go test ./server/ -run WireContract
package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/manager"
	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	tmdbMocks "github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// Golden capture infrastructure
// ---------------------------------------------------------------------------

func TestMain(m *testing.M) {
	// The activity timeline endpoint compares DB timestamps (UTC) against
	// time.Now() in the process location; capture under UTC so goldens are
	// environment-independent. Must run before time.Local is first resolved.
	_ = os.Setenv("TZ", "UTC")
	os.Exit(m.Run())
}

const wireGoldenUpdateEnv = "WIRE_CAPTURE_UPDATE"

var (
	// Any RFC3339-shaped quoted string (timestamps are dynamic or
	// DB-defaulted; masking preserves presence vs null vs omitted).
	wireRxTimestamp = regexp.MustCompile(`"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})"`)
	// Values under "duration" keys are computed with time.Since at read time.
	wireRxDuration = regexp.MustCompile(`("duration":")[^"]*(")`)
	// Timeline bucket dates are "today" at capture time.
	wireRxDate = regexp.MustCompile(`("date":")\d{4}-\d{2}-\d{2}(")`)
)

func normalizeWireJSON(t *testing.T, raw string) string {
	t.Helper()
	out := wireRxTimestamp.ReplaceAllString(raw, `"<timestamp>"`)
	out = wireRxDuration.ReplaceAllString(out, `${1}<duration>${2}`)
	out = wireRxDate.ReplaceAllString(out, `${1}<date>${2}`)
	return out
}

// assertNoSQLNullLeak walks the decoded response and fails if any object key
// is "Valid" or "String" (the marshaled shape of sql.Null* leaking to the wire).
func assertNoSQLNullLeak(t *testing.T, raw string) {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var v any
	require.NoError(t, dec.Decode(&v), "response is not valid JSON: %s", raw)
	var walk func(x any, path string)
	walk = func(x any, path string) {
		switch val := x.(type) {
		case map[string]any:
			for k, sub := range val {
				require.NotEqual(t, "Valid", k, "sql.Null leak (key %q) at %s in %s", k, path, raw)
				require.NotEqual(t, "String", k, "sql.Null leak (key %q) at %s in %s", k, path, raw)
				walk(sub, path+"."+k)
			}
		case []any:
			for i, sub := range val {
				walk(sub, fmt.Sprintf("%s[%d]", path, i))
			}
		}
	}
	walk(v, "$")
}

type secretPolicy int

const secretMustBeAbsent secretPolicy = 0

type wireApp struct {
	t       *testing.T
	router  *mux.Router
	store   storage.Storage
	tmdb    *tmdbMocks.MockITmdb
	secrets map[string]string // secret value -> where it was seeded

	// ids of entities seeded by builders (deterministic on a fresh DB)
	movieID   int64
	seriesID  int64
	episodeID int64
	jobID     int64
}

// capture performs the request, asserts status + leak guards + secret policy,
// and compares the normalized body against the golden file.
func (app *wireApp) capture(name, method, path, body string, wantStatus int, policy secretPolicy) {
	app.t.Helper()
	t := app.t

	var req *http.Request
	var err error
	if body != "" {
		req, err = http.NewRequest(method, path, strings.NewReader(body))
	} else {
		req, err = http.NewRequest(method, path, nil)
	}
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	app.router.ServeHTTP(rr, req)
	require.Equal(t, wantStatus, rr.Code, "%s %s (golden %s) — body: %s", method, path, name, rr.Body.String())

	raw := rr.Body.String()
	assertNoSQLNullLeak(t, raw)
	if name == "activity_active" {
		// Timestamp normalization must not hide the historical sort_key-as-Unix
		// bug: active media now use the actual latest transition created_at.
		var activity struct {
			Response struct {
				Movies []struct {
					StateSince time.Time `json:"stateSince"`
				} `json:"movies"`
				Series []struct {
					StateSince time.Time `json:"stateSince"`
				} `json:"series"`
			} `json:"response"`
		}
		require.NoError(t, json.Unmarshal([]byte(raw), &activity))
		for _, movie := range activity.Response.Movies {
			require.WithinDuration(t, time.Now().UTC(), movie.StateSince, time.Minute)
		}
		for _, series := range activity.Response.Series {
			require.WithinDuration(t, time.Now().UTC(), series.StateSince, time.Minute)
		}
	}

	switch policy {
	case secretMustBeAbsent:
		for secret, where := range app.secrets {
			require.NotContains(t, raw, secret,
				"SECRET LEAK: value seeded via %s appeared in %s (%s %s)", where, name, method, path)
		}
	}

	normalized := normalizeWireJSON(t, raw) + "\n"
	goldenPath := filepath.Join("testdata", "wire", name+".golden.json")
	if os.Getenv(wireGoldenUpdateEnv) != "" {
		require.NoError(t, os.MkdirAll(filepath.Dir(goldenPath), 0o755))
		require.NoError(t, os.WriteFile(goldenPath, []byte(normalized), 0o644))
		return
	}
	want, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "missing golden %s — regenerate with WIRE_CAPTURE_UPDATE=1", goldenPath)
	assert.Equal(t, string(want), normalized, "wire contract drift in %s (%s %s)", name, method, path)
}

// wireConfig is the deterministic app config used by all capture tests.
func wireConfig() config.Config {
	return config.Config{
		Library: config.Library{
			MovieDir:         "/data/movies",
			TVDir:            "/data/tv",
			DownloadMountDir: "/data/downloads",
			UseHardlinks:     true,
		},
		Server: config.Server{Port: 8080, DistDir: "dist"},
		Manager: config.Manager{Jobs: config.Jobs{
			MovieReconcile:  10 * time.Minute,
			MovieIndex:      5 * time.Minute,
			SeriesReconcile: 15 * time.Minute,
			SeriesIndex:     7 * time.Minute,
			IndexerSync:     60 * time.Minute,
			MinJobsToKeep:   5,
		}},
	}
}

// newWireAppWithStore builds the full real stack (given in-memory store + real
// manager + full router) with a TMDB mock that fails loudly on unexpected calls.
func newWireAppWithStore(t *testing.T, store storage.Storage) *wireApp {
	t.Helper()

	ctrl := gomock.NewController(t)
	tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

	cfg := wireConfig()
	mgr := manager.New(tmdbMock, nil, nil, store, nil, cfg.Manager, cfg)

	srv := New(zap.NewNop().Sugar(), mgr, cfg.Server)
	router := mux.NewRouter()
	srv.registerRoutes(router)

	return &wireApp{
		t:      t,
		router: router,
		store:  store,
		tmdb:   tmdbMock,
		// Register the complete suite catalog before ANY response is scanned,
		// including secrets supplied by later POST/PUT requests. A leak into
		// an earlier list/get or an unrelated endpoint must fail as well.
		secrets: map[string]string{
			"wire-dc-secret-alpha":        "download_client.api_key",
			"wire-dc-secret-beta":         "download_client.api_key (via POST)",
			"wire-source-secret-gamma":    "indexer_source.api_key",
			"wire-indexer-secret-delta":   "indexer.api_key",
			"wire-indexer-secret-epsilon": "indexer.api_key (source-bound)",
			"wire-indexer-secret-zeta":    "indexer.api_key (via POST)",
			"wire-indexer-secret-eta":     "indexer.api_key (via PUT)",
			"wire-source-secret-theta":    "indexer_source.api_key (via POST)",
		},
	}
}

func newWireStore(t *testing.T) storage.Storage {
	t.Helper()
	ctx := context.Background()
	store, err := mediaSqlite.New(ctx, ":memory:")
	require.NoError(t, err)
	return store
}

func httptestJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

// ---------------------------------------------------------------------------
// Seed helpers (deterministic ids via fixed insert order on a fresh DB)
// ---------------------------------------------------------------------------

func nullString(value string) sql.NullString    { return sql.NullString{String: value, Valid: true} }
func nullInt64(value int64) sql.NullInt64       { return sql.NullInt64{Int64: value, Valid: true} }
func nullTime(value time.Time) sql.NullTime     { return sql.NullTime{Time: value, Valid: true} }
func nullFloat64(value float64) sql.NullFloat64 { return sql.NullFloat64{Float64: value, Valid: true} }

func seedDownloadClient(t *testing.T, ctx context.Context, store storage.Storage, secret string) {
	t.Helper()
	_, err := store.CreateDownloadClient(ctx, sqlcdb.DownloadClient{
		Type:           "transmission",
		Implementation: "TRANSMISSION",
		Scheme:         "http",
		Host:           "dc.local",
		Port:           9091,
		ApiKey:         nullString(secret),
	})
	require.NoError(t, err)
}

func seedMinimalMovieMetadata(t *testing.T, ctx context.Context, store storage.Storage, tmdbID int, title string) sqlcdb.MovieMetadatum {
	t.Helper()
	meta := sqlcdb.MovieMetadatum{
		TmdbID:             int64(tmdbID),
		Title:              title,
		OriginalLanguage:   1,
		Status:             5,
		Runtime:            120,
		Recommendations:    "[]",
		OriginalTitle:      nullString(title),
		SortTitle:          nullString(strings.ToLower(title)),
		CleanTitle:         nullString(strings.ToLower(title)),
		CleanOriginalTitle: nullString(strings.ToLower(title)),
	}
	_, err := store.CreateMovieMetadata(ctx, meta)
	require.NoError(t, err)
	created, err := store.GetMovieMetadataByTmdbID(ctx, int64(tmdbID))
	require.NoError(t, err)
	return created
}

func seedRichMovieMetadata(t *testing.T, ctx context.Context, store storage.Storage, tmdbID int, title string) sqlcdb.MovieMetadatum {
	t.Helper()
	meta := sqlcdb.MovieMetadatum{
		TmdbID:             int64(tmdbID),
		Title:              title,
		ImdbID:             nullString(fmt.Sprintf("tt%d", tmdbID)),
		Images:             `{"poster_path":"/gold.jpg","backdrops":[]}`,
		Genres:             nullString("Action,Adventure"),
		OriginalTitle:      nullString(title),
		SortTitle:          nullString(strings.ToLower(title)),
		CleanTitle:         nullString(strings.ToLower(title)),
		CleanOriginalTitle: nullString(strings.ToLower(title)),
		OriginalLanguage:   1,
		Status:             5,
		LastInfoSync:       nullTime(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)),
		Runtime:            132,
		InCinemas:          nullTime(time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC)),
		ReleaseDate:        nullTime(time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC)),
		Year:               nullInt64(2024),
		Ratings:            nullString(`{"imdb":{"votes":1000,"value":7.9}}`),
		Recommendations:    "[]",
		Certification:      nullString("PG-13"),
		YoutubeTrailerID:   nullString("dQw4w9WgXcQ"),
		Studio:             nullString("Gold Studios"),
		Overview:           nullString("A golden movie overview"),
		Website:            nullString("https://gold.example.com"),
		Popularity:         nullFloat64(42.5),
		CollectionTmdbID:   nullInt64(999),
		CollectionTitle:    nullString("Gold Collection"),
	}
	_, err := store.CreateMovieMetadata(ctx, meta)
	require.NoError(t, err)
	created, err := store.GetMovieMetadataByTmdbID(ctx, int64(tmdbID))
	require.NoError(t, err)
	return created
}

// seedSeriesTree creates series metadata + one season (2 episodes) of metadata.
func seedSeriesTree(t *testing.T, ctx context.Context, store storage.Storage, tmdbID int, title string, rich bool) (seriesMetaID, seasonMetaID int64) {
	t.Helper()
	meta := sqlcdb.SeriesMetadatum{
		TmdbID:       int64(tmdbID),
		Title:        title,
		SeasonCount:  1,
		EpisodeCount: 2,
		Status:       "Continuing",
		LastInfoSync: nullTime(time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)),
		FirstAirDate: nullTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)),
		LastAirDate:  nullTime(time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)),
	}
	if rich {
		meta.Overview = nullString("A golden series overview")
		meta.PosterPath = nullString("/series-poster.jpg")
		meta.ExternalIds = nullString(`{"imdb_id":"tt22001","tvdb_id":22001}`)
		meta.WatchProviders = nullString(`{"US":{"flatrate":[{"provider_id":8,"provider_name":"Netflix","logo_path":"/net.png"}]}}`)
	}
	_, err := store.CreateSeriesMetadata(ctx, meta)
	require.NoError(t, err)
	created, err := store.GetSeriesMetadataByTmdbID(ctx, int64(tmdbID))
	require.NoError(t, err)

	season := sqlcdb.SeasonMetadatum{
		SeriesMetadataID: created.ID,
		Number:           1,
		TmdbID:           int64(tmdbID*100 + 1),
		Title:            "Season 1",
		Overview:         nullString("The first season"),
		AirDate:          nullTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)),
	}
	seasonID64, err := store.CreateSeasonMetadata(ctx, season)
	require.NoError(t, err)

	for _, n := range []int64{1, 2} {
		ep := sqlcdb.EpisodeMetadatum{
			SeasonMetadataID: seasonID64,
			Number:           n,
			TmdbID:           int64(tmdbID*100 + 10 + int(n)),
			Title:            fmt.Sprintf("Episode %d", n),
		}
		if rich {
			ep.Overview = nullString(fmt.Sprintf("Plot of episode %d", n))
			ep.AirDate = nullTime(time.Date(2020, 1, 1+int(n), 0, 0, 0, 0, time.UTC))
			ep.Runtime = nullInt64(42)
			ep.StillPath = nullString(fmt.Sprintf("/ep%d.jpg", n))
		}
		_, err := store.CreateEpisodeMetadata(ctx, ep)
		require.NoError(t, err)
	}
	return created.ID, seasonID64
}

// seedSeriesLibraryRows creates the library season row (plus its episode rows)
// for a series that was linked to metadata from seedSeriesTree.
func seedSeriesLibraryRows(t *testing.T, ctx context.Context, store storage.Storage, seriesID int64, seasonMetaID int64) (seasonID int64, episodeIDs []int64) {
	t.Helper()
	seasonID64, err := store.CreateSeason(ctx, sqlcdb.Season{
		SeriesID:         seriesID,
		SeasonMetadataID: nullInt64(seasonMetaID),
		Monitored:        1,
	}, storage.SeasonStateMissing)
	require.NoError(t, err)

	seasonMetas, err := store.ListEpisodeMetadataBySeasonMetadataID(ctx, seasonMetaID)
	require.NoError(t, err)
	for _, em := range seasonMetas {
		epID, err := store.CreateEpisode(ctx, sqlcdb.Episode{
			SeasonID:          seasonID64,
			EpisodeMetadataID: nullInt64(em.ID),
			EpisodeNumber:     em.Number,
			Monitored:         1,
		}, storage.EpisodeStateMissing)
		require.NoError(t, err)
		episodeIDs = append(episodeIDs, epID)
	}
	require.Len(t, episodeIDs, 2, "expected 2 episode rows from seedSeriesTree metadata")
	return seasonID64, episodeIDs
}

// ---------------------------------------------------------------------------
// Movies: list / detail / add / patch / refresh / delete
// ---------------------------------------------------------------------------

func buildMoviesApp(t *testing.T) *wireApp {
	t.Helper()
	ctx := context.Background()
	store := newWireStore(t)
	app := newWireAppWithStore(t, store)

	seedDownloadClient(t, ctx, store, "wire-dc-secret-alpha")
	app.secrets["wire-dc-secret-alpha"] = "download_client.api_key"

	// Movie A: full metadata + file + transition history (downloading w/ download info).
	metaA := seedRichMovieMetadata(t, ctx, store, 11001, "Golden Movie A")
	movieAID, err := store.CreateMovie(ctx, sqlcdb.Movie{
		Path:             nullString("/movies/Golden Movie A (2024)"),
		Monitored:        1,
		QualityProfileID: 1,
		Added:            nullTime(time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)),
		MovieMetadataID:  nullInt64(metaA.ID),
	}, storage.MovieStateMissing)
	require.NoError(t, err)
	require.NoError(t, store.UpdateMovieState(ctx, movieAID, storage.MovieStateDownloading, &storage.TransitionStateMetadata{
		DownloadID:       ptr.To("sab-1234"),
		DownloadClientID: ptr.To(int64(1)),
	}))
	fileID, err := store.CreateMovieFile(ctx, sqlcdb.MovieFile{
		Quality:          "WEBDL-1080p",
		Size:             2147483648,
		DateAdded:        time.Date(2024, 1, 6, 0, 0, 0, 0, time.UTC),
		SceneName:        nullString("Golden.Movie.A.2024.WEBDL-1080p-GOLD"),
		MediaInfo:        nullString(`{"audioBitrate":640}`),
		ReleaseGroup:     nullString("GOLD"),
		RelativePath:     nullString("Golden Movie A (2024)/Golden.Movie.A.2024.WEBDL-1080p-GOLD.mkv"),
		Edition:          nullString("Theatrical"),
		Languages:        "eng",
		IndexerFlags:     0,
		OriginalFilePath: nullString("/downloads/Golden.Movie.A.2024.WEBDL-1080p-GOLD.mkv"),
	})
	require.NoError(t, err)
	require.NoError(t, store.UpdateMovieMovieFileID(ctx, movieAID, fileID))
	require.NoError(t, store.UpdateMovieState(ctx, movieAID, storage.MovieStateDownloaded, nil))
	app.movieID = movieAID

	// Movie B: minimal metadata, every nullable column NULL except identity.
	metaB := seedMinimalMovieMetadata(t, ctx, store, 11002, "Minimal Movie B")
	_, err = store.CreateMovie(ctx, sqlcdb.Movie{
		Monitored:        0,
		QualityProfileID: 1,
		MovieMetadataID:  nullInt64(metaB.ID),
	}, storage.MovieStateDiscovered)
	require.NoError(t, err)

	// Metadata for a movie NOT yet in the library (used by POST /library/movies).
	seedRichMovieMetadata(t, ctx, store, 11003, "Fresh Movie C")

	// POST /movies/refresh triggers a TMDB detail fetch + metadata update.
	app.tmdb.EXPECT().GetMovieDetails(gomock.Any(), 11001).Return(&tmdb.MediaDetails{
		ID:          11001,
		Title:       ptr.To("Golden Movie A (Updated)"),
		Runtime:     ptr.To(137),
		Overview:    ptr.To("An updated overview"),
		PosterPath:  ptr.To("/goldA-updated.jpg"),
		ReleaseDate: ptr.To("2024-01-05"),
	}, nil).AnyTimes()

	return app
}

func TestWireContract_Movies(t *testing.T) {
	app := buildMoviesApp(t)

	app.capture("movies_list", "GET", "/api/v1/library/movies", "", http.StatusOK, secretMustBeAbsent)
	app.capture("movie_detail_11001", "GET", "/api/v1/movie/11001", "", http.StatusOK, secretMustBeAbsent)
	app.capture("movie_detail_minimal", "GET", "/api/v1/movie/11002", "", http.StatusOK, secretMustBeAbsent)
	app.capture("movie_add", "POST", "/api/v1/library/movies", `{"tmdbID":11003,"qualityProfileID":1}`, http.StatusOK, secretMustBeAbsent)
	app.capture("movie_patch_monitored", "PATCH", "/api/v1/library/movies/2/monitored", `{"monitored":true}`, http.StatusOK, secretMustBeAbsent)
	app.capture("movie_patch_quality", "PATCH", "/api/v1/library/movies/1/quality", `{"qualityProfileId":2}`, http.StatusOK, secretMustBeAbsent)
	app.capture("movie_refresh", "POST", "/api/v1/movies/refresh", `{"tmdbIds":[11001]}`, http.StatusOK, secretMustBeAbsent)
	app.capture("movie_delete", "DELETE", "/api/v1/library/movies/2", "", http.StatusOK, secretMustBeAbsent)
}

// ---------------------------------------------------------------------------
// Series / seasons / episodes
// ---------------------------------------------------------------------------

func buildSeriesApp(t *testing.T) *wireApp {
	t.Helper()
	ctx := context.Background()
	store := newWireStore(t)
	app := newWireAppWithStore(t, store)

	seedDownloadClient(t, ctx, store, "wire-dc-secret-alpha")
	app.secrets["wire-dc-secret-alpha"] = "download_client.api_key"

	// Series A (rich): metadata + season metadata + episode metadata,
	// transition history on series/episode incl. download info.
	seriesMetaID, seasonMetaID := seedSeriesTree(t, ctx, store, 22001, "Golden Series", true)
	seriesID, err := store.CreateSeries(ctx, sqlcdb.Series{
		Path:              nullString("/tv/Golden Series (2020)"),
		Monitored:         1,
		Added:             nullTime(time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)),
		QualityProfileID:  1,
		SeriesMetadataID:  nullInt64(seriesMetaID),
		MonitorNewSeasons: 1,
	}, storage.SeriesStateDiscovered)
	require.NoError(t, err)
	require.NoError(t, store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateMissing, nil))
	app.seriesID = seriesID

	_, episodeIDs := seedSeriesLibraryRows(t, ctx, store, seriesID, seasonMetaID)

	// Episode 1 has a full download history and a file; episode 2 stays missing.
	require.NoError(t, store.UpdateEpisodeState(ctx, episodeIDs[0], storage.EpisodeStateDownloading, &storage.TransitionStateMetadata{
		DownloadID:       ptr.To("tr-77"),
		DownloadClientID: ptr.To(int64(1)),
	}))
	epFileID, err := store.CreateEpisodeFile(ctx, sqlcdb.EpisodeFile{
		Quality:          "WEBDL-1080p",
		Size:             1073741824,
		Added:            time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
		RelativePath:     nullString("Golden Series (2020)/Season 1/Golden.Series.S01E01.WEBDL-1080p.mkv"),
		OriginalFilePath: nullString("/downloads/Golden.Series.S01E01.WEBDL-1080p.mkv"),
	})
	require.NoError(t, err)
	require.NoError(t, store.UpdateEpisodeEpisodeFileID(ctx, episodeIDs[0], epFileID))
	require.NoError(t, store.UpdateEpisodeState(ctx, episodeIDs[0], storage.EpisodeStateDownloaded, nil))
	app.episodeID = episodeIDs[0]

	// Second series, NOT in the library yet (used by POST /library/tv).
	seedSeriesTree(t, ctx, store, 22002, "Fresh Series B", false)

	// TMDB series details are fetched even when metadata is cached.
	app.tmdb.EXPECT().TvSeriesDetails(gomock.Any(), int32(22001), gomock.Any()).
		Return(httptestJSONResponse(http.StatusOK, `{
			"id": 22001,
			"name": "Golden Series",
			"poster_path": "/tmdb-poster.jpg",
			"backdrop_path": "/series-bg.jpg",
			"adult": false,
			"popularity": 55.5,
			"networks": [{"name": "HBO", "logo_path": "/hbo.png"}],
			"genres": [{"name": "Drama"}, {"name": "Crime"}],
			"production_countries": [{"iso_3166_1": "US", "name": "United States"}],
			"status": "Returning Series",
			"next_episode_to_air": {"air_date": "2024-06-01"},
			"original_language": "en",
			"vote_average": 8.1,
			"vote_count": 432,
			"number_of_seasons": 1,
			"number_of_episodes": 2
		}`), nil).AnyTimes()

	return app
}

func TestWireContract_Series(t *testing.T) {
	app := buildSeriesApp(t)

	app.capture("tv_list", "GET", "/api/v1/library/tv", "", http.StatusOK, secretMustBeAbsent)
	app.capture("tv_detail", "GET", "/api/v1/tv/22001", "", http.StatusOK, secretMustBeAbsent)
	app.capture("series_add", "POST", "/api/v1/library/tv", `{"tmdbID":22002,"qualityProfileID":1,"monitorNewSeasons":true}`, http.StatusOK, secretMustBeAbsent)
	app.capture("series_patch_monitored", "PATCH", "/api/v1/library/tv/2/monitored", `{"monitored":false}`, http.StatusOK, secretMustBeAbsent)
	app.capture("series_delete", "DELETE", "/api/v1/library/tv/2", "", http.StatusOK, secretMustBeAbsent)
}

// ---------------------------------------------------------------------------
// Indexers + indexer sources (secrets audit: api_key must NOT leak here)
// ---------------------------------------------------------------------------

func buildIndexersApp(t *testing.T) *wireApp {
	t.Helper()
	ctx := context.Background()
	store := newWireStore(t)
	app := newWireAppWithStore(t, store)

	sourceID, err := store.CreateIndexerSource(ctx, sqlcdb.IndexerSource{
		Name:           "Prowlarr",
		Implementation: "prowlarr",
		Scheme:         "http",
		Host:           "prowlarr.local",
		Port:           nullInt64(9696),
		ApiKey:         nullString("wire-source-secret-gamma"),
		Enabled:        true,
	})
	require.NoError(t, err)
	app.secrets["wire-source-secret-gamma"] = "indexer_source.api_key"

	_, err = store.CreateIndexer(ctx, sqlcdb.Indexer{
		Name:     "Internal Indexer",
		Priority: 25,
		Uri:      "http://internal/",
		ApiKey:   nullString("wire-indexer-secret-delta"),
	})
	require.NoError(t, err)
	app.secrets["wire-indexer-secret-delta"] = "indexer.api_key"

	_, err = store.CreateIndexer(ctx, sqlcdb.Indexer{
		Name:            "Prowlarr Indexer",
		Priority:        20,
		Uri:             "http://prowlarr.local/1",
		ApiKey:          nullString("wire-indexer-secret-epsilon"),
		IndexerSourceID: nullInt64(sourceID),
	})
	require.NoError(t, err)
	app.secrets["wire-indexer-secret-epsilon"] = "indexer.api_key"

	return app
}

func TestWireContract_IndexersAndSources(t *testing.T) {
	app := buildIndexersApp(t)

	// List omits source-bound indexers that are not cached.
	app.capture("indexers_list", "GET", "/api/v1/indexers", "", http.StatusOK, secretMustBeAbsent)
	// Incoming secrets were registered at app construction, before even the
	// initial list response (not merely before their create/update responses).
	app.capture("indexer_create", "POST", "/api/v1/indexers",
		`{"Name":"Manual Indexer","Priority":50,"URI":"http://manual/","APIKey":"wire-indexer-secret-zeta"}`,
		http.StatusCreated, secretMustBeAbsent)
	app.capture("indexer_update", "PUT", "/api/v1/indexers/1",
		`{"name":"Internal Indexer Updated","priority":30,"uri":"http://internal2/","api_key":"wire-indexer-secret-eta"}`,
		http.StatusOK, secretMustBeAbsent)
	app.capture("indexer_delete", "DELETE", "/api/v1/indexers/3", "", http.StatusOK, secretMustBeAbsent)

	app.capture("indexer_sources_list", "GET", "/api/v1/indexer-sources", "", http.StatusOK, secretMustBeAbsent)
	app.capture("indexer_source_get", "GET", "/api/v1/indexer-sources/1", "", http.StatusOK, secretMustBeAbsent)
	// Enabled=false so creation does not trigger a prowlarr refresh call.
	app.capture("indexer_source_create", "POST", "/api/v1/indexer-sources",
		`{"name":"Prowlarr2","implementation":"prowlarr","scheme":"http","host":"prowlarr2.local","port":9697,"apiKey":"wire-source-secret-theta","enabled":false}`,
		http.StatusCreated, secretMustBeAbsent)
	// Empty apiKey preserves the stored secret.
	app.capture("indexer_source_update", "PUT", "/api/v1/indexer-sources/1",
		`{"name":"Prowlarr Renamed","implementation":"prowlarr","scheme":"http","host":"prowlarr.local","port":9696,"apiKey":"","enabled":false}`,
		http.StatusOK, secretMustBeAbsent)
	app.capture("indexer_source_delete", "DELETE", "/api/v1/indexer-sources/2", "", http.StatusOK, secretMustBeAbsent)
}

// ---------------------------------------------------------------------------
// Download clients (api_key values must be absent from all responses)
// ---------------------------------------------------------------------------

func buildDownloadClientsApp(t *testing.T) *wireApp {
	t.Helper()
	ctx := context.Background()
	store := newWireStore(t)
	app := newWireAppWithStore(t, store)

	seedDownloadClient(t, ctx, store, "wire-dc-secret-alpha")
	app.secrets["wire-dc-secret-alpha"] = "download_client.api_key"

	return app
}

func TestWireContract_DownloadClients(t *testing.T) {
	app := buildDownloadClientsApp(t)

	app.capture("download_clients_list", "GET", "/api/v1/download/clients", "", http.StatusOK, secretMustBeAbsent)
	app.capture("download_client_get", "GET", "/api/v1/download/clients/1", "", http.StatusOK, secretMustBeAbsent)
	// Create/Update request bodies use PascalCase keys, captured as-is.
	app.capture("download_client_create", "POST", "/api/v1/download/clients",
		`{"Type":"sabnzbd","Implementation":"SABNZBD","Scheme":"https","Host":"sab.local","Port":8080,"APIKey":"wire-dc-secret-beta"}`,
		http.StatusCreated, secretMustBeAbsent)
	// Update without APIKey preserves the stored secret.
	app.capture("download_client_update", "PUT", "/api/v1/download/clients/1",
		`{"Type":"transmission","Implementation":"TRANSMISSION","Scheme":"http","Host":"localhost","Port":9091}`,
		http.StatusOK, secretMustBeAbsent)
	app.capture("download_client_delete", "DELETE", "/api/v1/download/clients/2", "", http.StatusOK, secretMustBeAbsent)
}

// ---------------------------------------------------------------------------
// Quality definitions + profiles (seeded defaults included)
// ---------------------------------------------------------------------------

func TestWireContract_Quality(t *testing.T) {
	app := newWireAppWithStore(t, newWireStore(t))

	app.capture("quality_definitions_list", "GET", "/api/v1/quality/definitions", "", http.StatusOK, secretMustBeAbsent)
	app.capture("quality_definition_get", "GET", "/api/v1/quality/definitions/5", "", http.StatusOK, secretMustBeAbsent)
	// Note the response shape: raw quality-definition row → PascalCase keys.
	app.capture("quality_definition_create", "POST", "/api/v1/quality/definitions",
		`{"name":"Gold Custom","type":"movie","preferredSize":2000,"minSize":10.5,"maxSize":2500}`,
		http.StatusCreated, secretMustBeAbsent)
	app.capture("quality_definition_update", "PUT", "/api/v1/quality/definitions/29",
		`{"name":"Gold Custom 2","type":"movie","preferredSize":2100,"minSize":11.5,"maxSize":2600}`,
		http.StatusOK, secretMustBeAbsent)
	app.capture("quality_definition_delete", "DELETE", "/api/v1/quality/definitions/29", "", http.StatusOK, secretMustBeAbsent)

	app.capture("quality_profiles_list", "GET", "/api/v1/quality/profiles", "", http.StatusOK, secretMustBeAbsent)
	app.capture("quality_profiles_movie", "GET", "/api/v1/quality/profiles?type=movie", "", http.StatusOK, secretMustBeAbsent)
	app.capture("quality_profiles_series", "GET", "/api/v1/quality/profiles?type=series", "", http.StatusOK, secretMustBeAbsent)
	app.capture("quality_profile_get", "GET", "/api/v1/quality/profiles/2", "", http.StatusOK, secretMustBeAbsent)
	app.capture("quality_profile_create", "POST", "/api/v1/quality/profiles",
		`{"name":"Gold Profile","cutoffQualityId":2,"upgradeAllowed":true,"qualityIds":[1,2]}`,
		http.StatusCreated, secretMustBeAbsent)
	app.capture("quality_profile_update", "PUT", "/api/v1/quality/profiles/7",
		`{"name":"Gold Profile 2","cutoffQualityId":1,"upgradeAllowed":false,"qualityIds":[1,2,3]}`,
		http.StatusOK, secretMustBeAbsent)
	app.capture("quality_profile_delete", "DELETE", "/api/v1/quality/profiles/7", "", http.StatusOK, secretMustBeAbsent)
}

// ---------------------------------------------------------------------------
// Jobs (pending + done + cancelled transitions)
// ---------------------------------------------------------------------------

func buildJobsApp(t *testing.T) *wireApp {
	t.Helper()
	ctx := context.Background()
	store := newWireStore(t)
	app := newWireAppWithStore(t, store)

	jobA, err := store.CreateJob(ctx, sqlcdb.Job{Type: "MovieIndex"}, storage.JobStatePending)
	require.NoError(t, err)
	require.NoError(t, store.UpdateJobState(ctx, jobA, storage.JobStateRunning, nil))
	require.NoError(t, store.UpdateJobState(ctx, jobA, storage.JobStateDone, nil))

	_, err = store.CreateJob(ctx, sqlcdb.Job{Type: "SeriesIndex"}, storage.JobStatePending)
	require.NoError(t, err)

	jobC, err := store.CreateJob(ctx, sqlcdb.Job{Type: "MovieReconcile"}, storage.JobStatePending)
	require.NoError(t, err)
	app.jobID = jobC

	return app
}

func TestWireContract_Jobs(t *testing.T) {
	app := buildJobsApp(t)

	app.capture("jobs_list", "GET", "/api/v1/jobs", "", http.StatusOK, secretMustBeAbsent)
	app.capture("jobs_list_paginated", "GET", "/api/v1/jobs?page=1&pageSize=2", "", http.StatusOK, secretMustBeAbsent)
	app.capture("job_get", "GET", "/api/v1/jobs/1", "", http.StatusOK, secretMustBeAbsent)
	app.capture("job_create", "POST", "/api/v1/jobs", `{"type":"IndexerSync"}`, http.StatusCreated, secretMustBeAbsent)
	app.capture("job_cancel", "POST", fmt.Sprintf("/api/v1/jobs/%d/cancel", app.jobID), "", http.StatusOK, secretMustBeAbsent)
}

// ---------------------------------------------------------------------------
// Activity: active downloads, failures, timeline, per-entity history
// ---------------------------------------------------------------------------

func buildActivityApp(t *testing.T) *wireApp {
	t.Helper()
	ctx := context.Background()
	store := newWireStore(t)
	app := newWireAppWithStore(t, store)

	seedDownloadClient(t, ctx, store, "wire-dc-secret-alpha")
	app.secrets["wire-dc-secret-alpha"] = "download_client.api_key"

	// A movie actively downloading with download client + download id.
	metaA := seedMinimalMovieMetadata(t, ctx, store, 11001, "Activity Movie")
	metaA.Images = `{"poster_path":"/activity.jpg"}`
	movieID, err := store.CreateMovie(ctx, sqlcdb.Movie{
		Path:             nullString("/movies/Activity Movie (2021)"),
		Monitored:        1,
		QualityProfileID: 1,
		MovieMetadataID:  nullInt64(metaA.ID),
	}, storage.MovieStateMissing)
	require.NoError(t, err)
	require.NoError(t, store.UpdateMovieState(ctx, movieID, storage.MovieStateDownloading, &storage.TransitionStateMetadata{
		DownloadID:       ptr.To("act-dl-1"),
		DownloadClientID: ptr.To(int64(1)),
	}))
	app.movieID = movieID

	// A series/season/episode actively downloading.
	seriesMetaID, seriesSeasonMetaID := seedSeriesTree(t, ctx, store, 22001, "Activity Series", false)
	seriesID, err := store.CreateSeries(ctx, sqlcdb.Series{
		Path:             nullString("/tv/Activity Series"),
		Monitored:        1,
		QualityProfileID: 1,
		SeriesMetadataID: nullInt64(seriesMetaID),
	}, storage.SeriesStateDiscovered)
	require.NoError(t, err)
	require.NoError(t, store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateMissing, nil))
	require.NoError(t, store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateDownloading, &storage.TransitionStateMetadata{
		DownloadID:       ptr.To("act-dl-2"),
		DownloadClientID: ptr.To(int64(1)),
	}))
	app.seriesID = seriesID

	seasonID, episodeIDs := seedSeriesLibraryRows(t, ctx, store, seriesID, seriesSeasonMetaID)
	require.NoError(t, store.UpdateSeasonState(ctx, seasonID, storage.SeasonStateDownloading, &storage.TransitionStateMetadata{
		DownloadID:       ptr.To("act-dl-2"),
		DownloadClientID: ptr.To(int64(1)),
	}))
	require.NoError(t, store.UpdateEpisodeState(ctx, episodeIDs[0], storage.EpisodeStateDownloading, &storage.TransitionStateMetadata{
		DownloadID:       ptr.To("act-dl-2"),
		DownloadClientID: ptr.To(int64(1)),
	}))
	app.episodeID = episodeIDs[0]

	// Jobs: running (active), error (failure), done (history).
	jobR, err := store.CreateJob(ctx, sqlcdb.Job{Type: "SeriesReconcile"}, storage.JobStatePending)
	require.NoError(t, err)
	require.NoError(t, store.UpdateJobState(ctx, jobR, storage.JobStateRunning, nil))

	jobE, err := store.CreateJob(ctx, sqlcdb.Job{Type: "IndexerSync"}, storage.JobStatePending)
	require.NoError(t, err)
	require.NoError(t, store.UpdateJobState(ctx, jobE, storage.JobStateError, ptr.To("boom: prowlarr unreachable")))

	jobD, err := store.CreateJob(ctx, sqlcdb.Job{Type: "MovieIndex"}, storage.JobStatePending)
	require.NoError(t, err)
	require.NoError(t, store.UpdateJobState(ctx, jobD, storage.JobStateRunning, nil))
	require.NoError(t, store.UpdateJobState(ctx, jobD, storage.JobStateDone, nil))
	app.jobID = jobE

	return app
}

func TestWireContract_Activity(t *testing.T) {
	app := buildActivityApp(t)

	app.capture("activity_active", "GET", "/api/v1/activity/active", "", http.StatusOK, secretMustBeAbsent)
	app.capture("activity_failures", "GET", "/api/v1/activity/failures", "", http.StatusOK, secretMustBeAbsent)
	app.capture("activity_timeline", "GET", "/api/v1/activity/timeline?days=1", "", http.StatusOK, secretMustBeAbsent)
	app.capture("activity_history_movie", "GET", fmt.Sprintf("/api/v1/activity/history/movie/%d", app.movieID), "", http.StatusOK, secretMustBeAbsent)
	app.capture("activity_history_series", "GET", fmt.Sprintf("/api/v1/activity/history/series/%d", app.seriesID), "", http.StatusOK, secretMustBeAbsent)
	app.capture("activity_history_episode", "GET", fmt.Sprintf("/api/v1/activity/history/episode/%d", app.episodeID), "", http.StatusOK, secretMustBeAbsent)
	app.capture("activity_history_job", "GET", fmt.Sprintf("/api/v1/activity/history/job/%d", app.jobID), "", http.StatusOK, secretMustBeAbsent)
}

// ---------------------------------------------------------------------------
// Config, stats, health
// ---------------------------------------------------------------------------

func buildConfigApp(t *testing.T) *wireApp {
	t.Helper()
	ctx := context.Background()
	store := newWireStore(t)
	app := newWireAppWithStore(t, store)

	// Two movies in different states, one series, for /library/stats.
	metaA := seedMinimalMovieMetadata(t, ctx, store, 11001, "Stats Movie A")
	movieA, err := store.CreateMovie(ctx, sqlcdb.Movie{MovieMetadataID: nullInt64(metaA.ID), QualityProfileID: 1, Monitored: 1}, storage.MovieStateMissing)
	require.NoError(t, err)
	require.NoError(t, store.UpdateMovieState(ctx, movieA, storage.MovieStateDownloaded, nil))

	metaB := seedMinimalMovieMetadata(t, ctx, store, 11002, "Stats Movie B")
	_, err = store.CreateMovie(ctx, sqlcdb.Movie{MovieMetadataID: nullInt64(metaB.ID), QualityProfileID: 1, Monitored: 0}, storage.MovieStateDiscovered)
	require.NoError(t, err)

	seriesMetaID, _ := seedSeriesTree(t, ctx, store, 22001, "Stats Series", false)
	_, err = store.CreateSeries(ctx, sqlcdb.Series{SeriesMetadataID: nullInt64(seriesMetaID), QualityProfileID: 1, Monitored: 1}, storage.SeriesStateDiscovered)
	require.NoError(t, err)

	return app
}

func TestWireContract_ConfigStatsHealth(t *testing.T) {
	app := buildConfigApp(t)

	app.capture("healthz", "GET", "/healthz", "", http.StatusOK, secretMustBeAbsent)
	app.capture("config_get", "GET", "/api/v1/config", "", http.StatusOK, secretMustBeAbsent)
	app.capture("library_stats", "GET", "/api/v1/library/stats", "", http.StatusOK, secretMustBeAbsent)
}

// ---------------------------------------------------------------------------
// Search passthroughs (TMDB-mocked)
// ---------------------------------------------------------------------------

func TestWireContract_Search(t *testing.T) {
	app := newWireAppWithStore(t, newWireStore(t))

	movieSearchBody := `{
		"page": 1,
		"results": [
			{
				"adult": false,
				"backdrop_path": "/movie-bg.jpg",
				"genre_ids": [28, 12],
				"id": 11001,
				"original_language": "en",
				"original_title": "Gold Rush",
				"overview": "A rush for gold",
				"popularity": 95.3,
				"poster_path": "/movie-poster.jpg",
				"release_date": "2024-01-05",
				"title": "Gold Rush",
				"video": false,
				"vote_average": 7.2,
				"vote_count": 1234
			}
		],
		"total_pages": 1,
		"total_results": 1
	}`
	app.tmdb.EXPECT().SearchMovie(gomock.Any(), gomock.Any()).
		Return(httptestJSONResponse(http.StatusOK, movieSearchBody), nil).AnyTimes()

	tvSearchBody := `{
		"page": 1,
		"results": [
			{
				"adult": false,
				"backdrop_path": "/tv-bg.jpg",
				"genre_ids": [18],
				"id": 22001,
				"original_language": "en",
				"original_name": "Golden Series",
				"overview": "A golden series",
				"popularity": 55.5,
				"poster_path": "/tv-poster.jpg",
				"first_air_date": "2020-01-01",
				"name": "Golden Series",
				"vote_average": 8.1,
				"vote_count": 432
			}
		],
		"total_pages": 1,
		"total_results": 1
	}`
	app.tmdb.EXPECT().SearchTv(gomock.Any(), gomock.Any()).
		Return(httptestJSONResponse(http.StatusOK, tvSearchBody), nil).AnyTimes()

	app.capture("discover_movie", "GET", "/api/v1/discover/movie?query=gold", "", http.StatusOK, secretMustBeAbsent)
	app.capture("discover_tv", "GET", "/api/v1/discover/tv?query=gold", "", http.StatusOK, secretMustBeAbsent)
}

// ---------------------------------------------------------------------------
// Secrets audit
// ---------------------------------------------------------------------------

// TestWireContract_SecretCatalogReadyBeforeResponses pins registration timing:
// even a fresh app, before seeding or issuing any request, must know every
// seeded and incoming credential used by the suite. Registering a credential
// only immediately before its POST/PUT would leave earlier responses unaudited.
func TestWireContract_SecretCatalogReadyBeforeResponses(t *testing.T) {
	app := newWireAppWithStore(t, newWireStore(t))
	for _, secret := range []string{
		"wire-dc-secret-alpha", "wire-dc-secret-beta",
		"wire-source-secret-gamma", "wire-indexer-secret-delta",
		"wire-indexer-secret-epsilon", "wire-indexer-secret-zeta",
		"wire-indexer-secret-eta", "wire-source-secret-theta",
	} {
		require.Contains(t, app.secrets, secret, "credential must be registered before the first response")
		require.NotEmpty(t, app.secrets[secret], "credential must identify its origin")
	}
	app.capture("healthz", "GET", "/healthz", "", http.StatusOK, secretMustBeAbsent)
}

// TestWireContract_SecretsAudit ensures secret values are absent from
// download-client, indexer, and indexer-source responses.
func TestWireContract_SecretsAudit(t *testing.T) {
	t.Run("download clients omit api_key", func(t *testing.T) {
		app := buildDownloadClientsApp(t)
		app.capture("audit_download_clients_list", "GET", "/api/v1/download/clients", "", http.StatusOK, secretMustBeAbsent)
		app.capture("audit_download_client_get", "GET", "/api/v1/download/clients/1", "", http.StatusOK, secretMustBeAbsent)
	})

	t.Run("indexers and indexer sources never expose api_key", func(t *testing.T) {
		app := buildIndexersApp(t)
		app.capture("audit_indexers_list", "GET", "/api/v1/indexers", "", http.StatusOK, secretMustBeAbsent)
		app.capture("audit_indexer_sources_list", "GET", "/api/v1/indexer-sources", "", http.StatusOK, secretMustBeAbsent)
		app.capture("audit_indexer_source_get", "GET", "/api/v1/indexer-sources/1", "", http.StatusOK, secretMustBeAbsent)
	})
}
