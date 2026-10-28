package manager

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/stretchr/testify/require"
)

func TestSeriesDiscoveryFilesReview(t *testing.T) {
	rows := func(paths ...string) []sqlcdb.EpisodeFile {
		out := make([]sqlcdb.EpisodeFile, 0, len(paths))
		for _, path := range paths {
			out = append(out, sqlcdb.EpisodeFile{RelativePath: sql.NullString{String: path, Valid: path != ""}})
		}
		return out
	}
	tests := []struct {
		name string
		rows []sqlcdb.EpisodeFile
		root string
		want map[string][]string
	}{
		{"relative paths cleaned", rows("Show/Season 01/../Season 01/Show.S01E01.mkv"), "/tv", map[string][]string{"Show": {"Show/Season 01/Show.S01E01.mkv"}}},
		{"absolute path under root", []sqlcdb.EpisodeFile{{OriginalFilePath: sql.NullString{String: "/tv/Show/Season 01/Show.S01E01.mkv", Valid: true}}}, "/tv", map[string][]string{"Show": {"Show/Season 01/Show.S01E01.mkv"}}},
		{"outside root rejected", []sqlcdb.EpisodeFile{{OriginalFilePath: sql.NullString{String: "/else/Show/Season 01/a.mkv", Valid: true}}}, "/tv", map[string][]string{}},
		{"prefix collision rejected", []sqlcdb.EpisodeFile{{OriginalFilePath: sql.NullString{String: "/tv-other/Show/Season 01/a.mkv", Valid: true}}}, "/tv", map[string][]string{}},
		{"traversal rejected", rows("../Show/Season 01/a.mkv"), "/tv", map[string][]string{}},
		{"absolute without root rejected", []sqlcdb.EpisodeFile{{OriginalFilePath: sql.NullString{String: "/tv/Show/Season 01/a.mkv", Valid: true}}}, "", map[string][]string{}},
		{"empty path ignored", rows(""), "/tv", map[string][]string{}},
		{"root-level file ignored", []sqlcdb.EpisodeFile{{OriginalFilePath: sql.NullString{String: "/tv/Show.S01E01.mkv", Valid: true}}}, "/tv", map[string][]string{}},
		{"absolute traversal outside root rejected", []sqlcdb.EpisodeFile{{OriginalFilePath: sql.NullString{String: "/tv/../else/Show/a.mkv", Valid: true}}}, "/tv", map[string][]string{}},
		{"relative path takes precedence", []sqlcdb.EpisodeFile{{RelativePath: sql.NullString{String: "Show/Season 01/a.mkv", Valid: true}, OriginalFilePath: sql.NullString{String: "/tv/Other/Season 01/a.mkv", Valid: true}}}, "/tv", map[string][]string{"Show": {"Show/Season 01/a.mkv"}}},
		{"absolute relative field rejected", []sqlcdb.EpisodeFile{{RelativePath: sql.NullString{String: "/tv/Show/a.mkv", Valid: true}}}, "/tv", map[string][]string{}},
		{"cleaning cannot create a root-level episode", rows("Show/../Show.S01E01.mkv"), "/tv", map[string][]string{}},
		{"relative fallback", []sqlcdb.EpisodeFile{{OriginalFilePath: sql.NullString{String: "Show/Season 01/a.mkv", Valid: true}}}, "/tv", map[string][]string{"Show": {"Show/Season 01/a.mkv"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { require.Equal(t, tt.want, seriesDiscoveryFiles(tt.rows, tt.root)) })
	}
}

type reviewEpisodeFileStore struct {
	storage.Storage
	reads int
	err   error
	empty bool
}

func (s *reviewEpisodeFileStore) ListEpisodeFiles(ctx context.Context) ([]sqlcdb.EpisodeFile, error) {
	s.reads++
	if s.err != nil {
		return nil, s.err
	}
	if s.empty {
		return nil, nil
	}
	return s.Storage.ListEpisodeFiles(ctx)
}

func TestSeriesDiscoveryBatchReview(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		linkedOnly, fail, empty bool
		unlinked, wantReads     int
		wantSearches            int32
	}{
		{"linked only", true, true, false, 0, 0, 0},
		{"mixed loader error", false, true, false, 2, 1, 0},
		{"empty successful load cached", false, false, true, 2, 1, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			base, err := mediaSqlite.New(ctx, ":memory:")
			require.NoError(t, err)
			store := &reviewEpisodeFileStore{Storage: base, empty: tc.empty}
			if tc.fail {
				store.err = errors.New("file listing failed")
			}
			metaID, err := base.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 42, Title: "Linked"})
			require.NoError(t, err)
			seasonMetaID, err := base.CreateSeasonMetadata(ctx, sqlcdb.SeasonMetadatum{TmdbID: 43, SeriesMetadataID: metaID, Number: 1, Title: "Season 1"})
			require.NoError(t, err)
			epMetaID, err := base.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{TmdbID: 44, SeasonMetadataID: seasonMetaID, Number: 1, Title: "Pilot"})
			require.NoError(t, err)
			addEpisode := func(name string, linked bool) {
				series := sqlcdb.Series{Path: sql.NullString{String: name, Valid: true}, QualityProfileID: 1}
				if linked {
					series.SeriesMetadataID = sql.NullInt64{Int64: metaID, Valid: true}
				}
				sid, e := base.CreateSeries(ctx, series, storage.SeriesStateDiscovered)
				require.NoError(t, e)
				sm := sqlcdb.Season{SeriesID: sid, SeasonNumber: 1}
				if linked {
					sm.SeasonMetadataID = sql.NullInt64{Int64: seasonMetaID, Valid: true}
				}
				seasonID, e := base.CreateSeason(ctx, sm, storage.SeasonStateDiscovered)
				require.NoError(t, e)
				fileID, e := base.CreateEpisodeFile(ctx, sqlcdb.EpisodeFile{RelativePath: sql.NullString{String: filepath.Join(name, "Season 01", "x.mkv"), Valid: true}})
				require.NoError(t, e)
				ep := sqlcdb.Episode{SeasonID: seasonID, EpisodeNumber: 1, EpisodeFileID: sql.NullInt64{Int64: fileID, Valid: true}}
				if linked {
					ep.EpisodeMetadataID = sql.NullInt64{Int64: epMetaID, Valid: true}
				}
				_, e = base.CreateEpisode(ctx, ep, storage.EpisodeStateDiscovered)
				require.NoError(t, e)
			}
			if tc.linkedOnly {
				addEpisode("Linked", true)
			} else {
				addEpisode("Unlinked First", false)
				addEpisode("Linked", true)
				for i := 0; i < tc.unlinked-1; i++ {
					addEpisode("Unlinked Other", false)
				}
			}
			before, err := base.ListEpisodes(ctx, storage.EpisodeFilter{States: []storage.EpisodeState{storage.EpisodeStateDiscovered}})
			require.NoError(t, err)
			var searches atomic.Int32
			m := filesystemDiscoveryManager(t, store, func(w http.ResponseWriter, r *http.Request) {
				searches.Add(1)
				if !tc.empty || r.URL.Path != "/3/search/tv" {
					t.Errorf("unexpected HTTP call: %s", r.URL)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				_, _ = w.Write([]byte(`{"results":[]}`))
			})
			m.config.Library.TVDir = "/tv"
			require.NoError(t, m.ReconcileDiscoveredEpisodes(ctx, nil))
			after, err := base.ListEpisodes(ctx, storage.EpisodeFilter{})
			require.NoError(t, err)
			require.Len(t, after, len(before))
			for i := range before {
				if before[i].Episode.EpisodeMetadataID.Valid {
					before[i].State = string(storage.EpisodeStateCompleted)
				}
			}
			require.Equal(t, before, after)
			require.Equal(t, tc.wantReads, store.reads)
			require.Equal(t, tc.wantSearches, searches.Load())
		})
	}
}

func TestLinkSeriesMetadataOriginalAbsolutePathReview(t *testing.T) {
	ctx := context.Background()
	store, err := mediaSqlite.New(ctx, ":memory:")
	require.NoError(t, err)
	id, err := store.CreateSeries(ctx, sqlcdb.Series{Path: sql.NullString{String: "Untitled", Valid: true}, QualityProfileID: 1}, storage.SeriesStateDiscovered)
	require.NoError(t, err)
	_, err = store.CreateEpisodeFile(ctx, sqlcdb.EpisodeFile{OriginalFilePath: sql.NullString{String: "/tv/Untitled/Season 01/Show.{tmdb-42}.S01E01.mkv", Valid: true}})
	require.NoError(t, err)
	before, err := store.GetSeriesByID(ctx, id)
	require.NoError(t, err)
	m := filesystemDiscoveryManager(t, store, func(w http.ResponseWriter, r *http.Request) { t.Errorf("unexpected search: %s", r.URL) })
	m.config.Library.TVDir = "/tv"
	metaID, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 42, Title: "Show"})
	require.NoError(t, err)
	require.NoError(t, m.linkSeriesMetadata(ctx, before, logger.FromCtx(ctx)))
	after, err := store.GetSeriesByID(ctx, id)
	require.NoError(t, err)
	before.Series.SeriesMetadataID = sql.NullInt64{Int64: metaID, Valid: true}
	require.Equal(t, before, after)
}
