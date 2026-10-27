package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/stretchr/testify/require"
)

// Use 2^32+1, not an SDK/TMDB identity or merely a large JSON field. Narrowing
// this database primary key to int32 would alias the existing row with ID 1.
const httpDatabaseHighID int64 = 1<<32 + 1

func newHTTPDatabaseIDApp(t *testing.T) (*wireApp, *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "http-ids.sqlite")
	store, err := mediaSqlite.New(context.Background(), path)
	require.NoError(t, err)
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on&_busy_timeout=5000")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return newWireAppWithStore(t, store), db
}

// Advance only the test database's allocator; application inserts still use
// the real generated queries, foreign keys, and transition transactions.
func advanceHTTPDatabaseID(t *testing.T, db *sql.DB, table string) {
	t.Helper()
	_, err := db.Exec("DELETE FROM sqlite_sequence WHERE name = ?", table)
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO sqlite_sequence(name, seq) VALUES (?, ?)", table, httpDatabaseHighID-1)
	require.NoError(t, err)
}

func httpDatabaseResponse[T any](t *testing.T, app *wireApp, method, path, body string, status int) T {
	t.Helper()
	rr := httptest.NewRecorder()
	app.router.ServeHTTP(rr, httptest.NewRequest(method, path, strings.NewReader(body)))
	require.Equal(t, status, rr.Code, "%s %s: %s", method, path, rr.Body.String())
	assertNoSQLNullLeak(t, rr.Body.String())
	for secret, where := range app.secrets {
		require.NotContains(t, rr.Body.String(), secret, "secret from %s in %s %s", where, method, path)
	}
	var envelope struct {
		Response T `json:"response"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
	return envelope.Response
}

func TestHTTPDatabasePathIDsAboveMaxInt32_MovieAndSeries(t *testing.T) {
	app, db := newHTTPDatabaseIDApp(t)
	ctx := context.Background()
	added := nullTime(time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC))
	movie := sqlcdb.Movie{QualityProfileID: 1, Added: added}
	lowMovie, err := app.store.CreateMovie(ctx, movie, storage.MovieStateMissing)
	require.NoError(t, err)
	require.Equal(t, int64(1), lowMovie)
	series := sqlcdb.Series{QualityProfileID: 1, Added: added}
	lowSeries, err := app.store.CreateSeries(ctx, series, storage.SeriesStateDiscovered)
	require.NoError(t, err)
	require.Equal(t, int64(1), lowSeries)
	lowMovieBefore, err := app.store.GetMovie(ctx, lowMovie)
	require.NoError(t, err)
	lowSeriesBefore, err := app.store.GetSeriesByID(ctx, lowSeries)
	require.NoError(t, err)

	advanceHTTPDatabaseID(t, db, "quality_profile")
	profileID, err := app.store.CreateQualityProfile(ctx, sqlcdb.QualityProfile{Name: "Large profile"})
	require.NoError(t, err)
	require.Equal(t, httpDatabaseHighID, profileID)
	advanceHTTPDatabaseID(t, db, "movie")
	movieID, err := app.store.CreateMovie(ctx, movie, storage.MovieStateMissing)
	require.NoError(t, err)
	require.Equal(t, httpDatabaseHighID, movieID)
	advanceHTTPDatabaseID(t, db, "series")
	seriesID, err := app.store.CreateSeries(ctx, series, storage.SeriesStateDiscovered)
	require.NoError(t, err)
	require.Equal(t, httpDatabaseHighID, seriesID)

	moviePath := fmt.Sprintf("/api/v1/library/movies/%d", movieID)
	wantMovie := movieResponse{ID: movieID, Monitored: 1, QualityProfileID: 1, Added: &added.Time, State: "missing"}
	require.Equal(t, wantMovie, httpDatabaseResponse[movieResponse](t, app, http.MethodPatch, moviePath+"/monitored", `{"monitored":true}`, http.StatusOK))
	wantMovie.QualityProfileID = profileID
	require.Equal(t, wantMovie, httpDatabaseResponse[movieResponse](t, app, http.MethodPatch, moviePath+"/quality", fmt.Sprintf(`{"qualityProfileId":%d}`, profileID), http.StatusOK))
	persistedMovie, err := app.store.GetMovie(ctx, movieID)
	require.NoError(t, err)
	movie.ID, movie.Monitored, movie.QualityProfileID = movieID, 1, profileID
	require.Equal(t, sqlcdb.GetMovieByIDRow{Movie: movie, State: "missing"}, persistedMovie)

	seriesPath := fmt.Sprintf("/api/v1/library/tv/%d", seriesID)
	wantSeries := seriesResponse{ID: seriesID, Monitored: 1, QualityProfileID: 1, Added: &added.Time, State: "discovered"}
	require.Equal(t, wantSeries, httpDatabaseResponse[seriesResponse](t, app, http.MethodPatch, seriesPath+"/monitored", `{"monitored":true}`, http.StatusOK))
	persistedSeries, err := app.store.GetSeriesByID(ctx, seriesID)
	require.NoError(t, err)
	series.ID, series.Monitored = seriesID, 1
	require.Equal(t, sqlcdb.GetSeriesRow{Series: series, State: "discovered"}, persistedSeries)

	httpDatabaseResponse[any](t, app, http.MethodDelete, moviePath, "", http.StatusOK)
	httpDatabaseResponse[any](t, app, http.MethodDelete, seriesPath, "", http.StatusOK)
	_, err = app.store.GetMovie(ctx, movieID)
	require.ErrorIs(t, err, storage.ErrNotFound)
	_, err = app.store.GetSeriesByID(ctx, seriesID)
	require.ErrorIs(t, err, storage.ErrNotFound)
	lowMovieAfter, err := app.store.GetMovie(ctx, lowMovie)
	require.NoError(t, err)
	require.Equal(t, lowMovieBefore, lowMovieAfter)
	lowSeriesAfter, err := app.store.GetSeriesByID(ctx, lowSeries)
	require.NoError(t, err)
	require.Equal(t, lowSeriesBefore, lowSeriesAfter)
}

func TestHTTPDatabasePathIDsAboveMaxInt32_DownloadClient(t *testing.T) {
	app, db := newHTTPDatabaseIDApp(t)
	ctx := context.Background()
	client := sqlcdb.DownloadClient{Type: "transmission", Implementation: "TRANSMISSION", Scheme: "http", Host: "dc.local", Port: 9091, ApiKey: nullString("wire-dc-secret-alpha")}
	lowID, err := app.store.CreateDownloadClient(ctx, client)
	require.NoError(t, err)
	require.Equal(t, int64(1), lowID)
	lowBefore, err := app.store.GetDownloadClient(ctx, lowID)
	require.NoError(t, err)
	advanceHTTPDatabaseID(t, db, "download_client")
	id, err := app.store.CreateDownloadClient(ctx, client)
	require.NoError(t, err)
	require.Equal(t, httpDatabaseHighID, id)
	path := fmt.Sprintf("/api/v1/download/clients/%d", id)
	want := downloadClientResponse{ID: id, Type: client.Type, Implementation: client.Implementation, Scheme: client.Scheme, Host: client.Host, Port: client.Port}
	require.Equal(t, want, httpDatabaseResponse[downloadClientResponse](t, app, http.MethodGet, path, "", http.StatusOK))
	want.Host = "updated.local"
	require.Equal(t, want, httpDatabaseResponse[downloadClientResponse](t, app, http.MethodPut, path, `{"Type":"transmission","Implementation":"TRANSMISSION","Scheme":"http","Host":"updated.local","Port":9091,"APIKey":"wire-dc-secret-beta"}`, http.StatusOK))
	client.ID, client.Host, client.ApiKey = id, want.Host, nullString("wire-dc-secret-beta")
	stored, err := app.store.GetDownloadClient(ctx, id)
	require.NoError(t, err)
	require.Equal(t, client, stored)
	require.Equal(t, want, httpDatabaseResponse[downloadClientResponse](t, app, http.MethodGet, path, "", http.StatusOK))
	require.Equal(t, id, httpDatabaseResponse[int64](t, app, http.MethodDelete, path, "", http.StatusOK))
	_, err = app.store.GetDownloadClient(ctx, id)
	require.ErrorIs(t, err, storage.ErrNotFound)
	lowAfter, err := app.store.GetDownloadClient(ctx, lowID)
	require.NoError(t, err)
	require.Equal(t, lowBefore, lowAfter)
}

func TestHTTPDatabasePathIDsAboveMaxInt32_Quality(t *testing.T) {
	app, db := newHTTPDatabaseIDApp(t)
	ctx := context.Background()
	lowDefinition, err := app.store.GetQualityDefinition(ctx, 1)
	require.NoError(t, err)
	lowProfile, err := app.store.GetQualityProfile(ctx, 1)
	require.NoError(t, err)
	advanceHTTPDatabaseID(t, db, "quality_definition")
	definition := sqlcdb.QualityDefinition{QualityID: 99, Name: "Large definition", PreferredSize: 100, MinSize: 10, MaxSize: 200, MediaType: "movie"}
	id, err := app.store.CreateQualityDefinition(ctx, definition)
	require.NoError(t, err)
	require.Equal(t, httpDatabaseHighID, id)
	path := fmt.Sprintf("/api/v1/quality/definitions/%d", id)
	want := qualityDefinitionResponse{ID: id, QualityID: 99, Name: definition.Name, PreferredSize: 100, MinSize: 10, MaxSize: 200, MediaType: "movie"}
	require.Equal(t, want, httpDatabaseResponse[qualityDefinitionResponse](t, app, http.MethodGet, path, "", http.StatusOK))
	want.Name, want.PreferredSize, want.MinSize, want.MaxSize = "Updated definition", 120, 12, 220
	require.Equal(t, want, httpDatabaseResponse[qualityDefinitionResponse](t, app, http.MethodPut, path, `{"name":"Updated definition","type":"movie","preferredSize":120,"minSize":12,"maxSize":220}`, http.StatusOK))
	definition.ID, definition.Name, definition.PreferredSize, definition.MinSize, definition.MaxSize = id, want.Name, 120, 12, 220
	storedDefinition, err := app.store.GetQualityDefinition(ctx, id)
	require.NoError(t, err)
	require.Equal(t, definition, storedDefinition)

	advanceHTTPDatabaseID(t, db, "quality_profile")
	profileID, err := app.store.CreateQualityProfile(ctx, sqlcdb.QualityProfile{Name: "Large profile", CutoffQualityID: nullInt64(id), UpgradeAllowed: true})
	require.NoError(t, err)
	require.Equal(t, httpDatabaseHighID, profileID)
	_, err = app.store.CreateQualityProfileItem(ctx, sqlcdb.QualityProfileItem{ProfileID: profileID, QualityID: id})
	require.NoError(t, err)
	profilePath := fmt.Sprintf("/api/v1/quality/profiles/%d", profileID)
	wantProfile := qualityProfileResponse{ID: profileID, Name: "Large profile", CutoffQualityID: &id, UpgradeAllowed: true, Qualities: []profileQualityResponse{{ID: id, Name: want.Name, MediaType: "movie", PreferredSize: 120, MinSize: 12, MaxSize: 220}}}
	require.Equal(t, wantProfile, httpDatabaseResponse[qualityProfileResponse](t, app, http.MethodGet, profilePath, "", http.StatusOK))
	wantProfile.Name, wantProfile.UpgradeAllowed = "Updated profile", false
	require.Equal(t, wantProfile, httpDatabaseResponse[qualityProfileResponse](t, app, http.MethodPut, profilePath, fmt.Sprintf(`{"name":"Updated profile","cutoffQualityId":%d,"upgradeAllowed":false,"qualityIds":[%d]}`, id, id), http.StatusOK))
	storedProfile, err := app.store.GetQualityProfile(ctx, profileID)
	require.NoError(t, err)
	require.Equal(t, sqlcdb.QualityProfile{ID: profileID, Name: wantProfile.Name, CutoffQualityID: nullInt64(id)}, storedProfile)
	httpDatabaseResponse[any](t, app, http.MethodDelete, profilePath, "", http.StatusOK)
	_, err = app.store.GetQualityProfile(ctx, profileID)
	require.ErrorIs(t, err, storage.ErrNotFound)
	require.Equal(t, map[string]int64{"id": id}, httpDatabaseResponse[map[string]int64](t, app, http.MethodDelete, path, "", http.StatusOK))
	_, err = app.store.GetQualityDefinition(ctx, id)
	require.ErrorIs(t, err, storage.ErrNotFound)
	lowDefinitionAfter, err := app.store.GetQualityDefinition(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, lowDefinition, lowDefinitionAfter)
	lowProfileAfter, err := app.store.GetQualityProfile(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, lowProfile, lowProfileAfter)
}
