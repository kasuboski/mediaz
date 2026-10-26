package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/mattn/go-sqlite3"
)

// These acceptance tests deliberately use real, private SQLite files. Raw SQL
// below is test-only: it probes database invariants and injects failures which
// cannot be produced through a valid application request.
func freshAcceptanceDB(t *testing.T) (*SQLite, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	t.Cleanup(cancel)
	store, err := New(ctx, filepath.Join(t.TempDir(), "acceptance.db"))
	if err != nil {
		t.Fatal(err)
	}
	s := store.(*SQLite)
	t.Cleanup(func() {
		if err := s.db.Close(); err != nil {
			t.Error(err)
		}
	})
	return s, ctx
}

func freshRequireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func freshCount(t *testing.T, s *SQLite, query string, args ...any) int {
	t.Helper()
	var n int
	freshRequireNoError(t, s.db.QueryRow(query, args...).Scan(&n))
	return n
}

func TestFreshAcceptanceForeignKeysOnEveryPooledConnection(t *testing.T) {
	s, ctx := freshAcceptanceDB(t)
	const poolSize = 8
	s.db.SetMaxOpenConns(poolSize)
	s.db.SetMaxIdleConns(poolSize)
	// Hold all connections simultaneously: releasing between probes would only
	// test the same connection repeatedly.
	for round := 0; round < 2; round++ {
		conns := make([]*sql.Conn, 0, poolSize)
		for i := 0; i < poolSize; i++ {
			conn, err := s.db.Conn(ctx)
			freshRequireNoError(t, err)
			conns = append(conns, conn)
		}
		for i, conn := range conns {
			var enabled int
			freshRequireNoError(t, conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&enabled))
			if enabled != 1 {
				t.Errorf("round %d connection %d: foreign_keys=%d", round, i, enabled)
			}
			_, err := conn.ExecContext(ctx, `INSERT INTO movie_transition(movie_id,to_state,most_recent,sort_key) VALUES(-999,'missing',1,1)`)
			var constraint sqlite3.Error
			if !errors.As(err, &constraint) || constraint.ExtendedCode != sqlite3.ErrConstraintForeignKey {
				t.Errorf("round %d connection %d: orphan insertion error = %v; want FK constraint", round, i, err)
			}
		}
		for _, conn := range conns {
			freshRequireNoError(t, conn.Close())
		}
		// Force the second round to open entirely new underlying connections.
		s.db.SetMaxIdleConns(0)
		s.db.SetMaxIdleConns(poolSize)
	}
	if n := freshCount(t, s, "SELECT count(*) FROM movie_transition"); n != 0 {
		t.Fatalf("orphan transitions persisted: %d", n)
	}
}

func TestFreshAcceptanceCascadeSetNullAndMediaSafety(t *testing.T) {
	s, ctx := freshAcceptanceDB(t)
	mediaDir := t.TempDir()
	moviePath := filepath.Join(mediaDir, "movie.mkv")
	episodePath := filepath.Join(mediaDir, "episode.mkv")
	contents := []byte("fake media: must never be removed or rewritten by storage")
	for _, path := range []string{moviePath, episodePath} {
		freshRequireNoError(t, os.WriteFile(path, contents, 0600))
	}
	movieFile, err := s.CreateMovieFile(ctx, sqlcdb.MovieFile{Quality: "HD", Languages: "[]", RelativePath: sql.NullString{String: moviePath, Valid: true}})
	freshRequireNoError(t, err)
	movie, err := s.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1, Path: sql.NullString{String: moviePath, Valid: true}, MovieFileID: sql.NullInt64{Int64: movieFile, Valid: true}}, storage.MovieStateMissing)
	freshRequireNoError(t, err)
	series, err := s.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1, Path: sql.NullString{String: mediaDir, Valid: true}}, storage.SeriesStateMissing)
	freshRequireNoError(t, err)
	season, err := s.CreateSeason(ctx, sqlcdb.Season{SeriesID: series, SeasonNumber: 1}, storage.SeasonStateMissing)
	freshRequireNoError(t, err)
	episodeFile, err := s.CreateEpisodeFile(ctx, sqlcdb.EpisodeFile{Quality: "HD", RelativePath: sql.NullString{String: episodePath, Valid: true}})
	freshRequireNoError(t, err)
	episode, err := s.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: season, EpisodeNumber: 1, EpisodeFileID: sql.NullInt64{Int64: episodeFile, Valid: true}}, storage.EpisodeStateMissing)
	freshRequireNoError(t, err)
	client, err := s.CreateDownloadClient(ctx, sqlcdb.DownloadClient{Type: "torrent", Implementation: "Transmission", Scheme: "http", Host: "localhost", Port: 9091})
	freshRequireNoError(t, err)
	downloadID := "retained-download-id"
	meta := &storage.TransitionStateMetadata{DownloadClientID: &client, DownloadID: &downloadID}
	freshRequireNoError(t, s.UpdateMovieState(ctx, movie, storage.MovieStateDownloading, meta))
	freshRequireNoError(t, s.UpdateSeasonState(ctx, season, storage.SeasonStateDownloading, meta))
	freshRequireNoError(t, s.UpdateEpisodeState(ctx, episode, storage.EpisodeStateDownloading, meta))
	freshRequireNoError(t, s.DeleteDownloadClient(ctx, client))
	for _, table := range []string{"movie_transition", "season_transition", "episode_transition"} {
		var gotClient sql.NullInt64
		var gotDownload sql.NullString
		freshRequireNoError(t, s.db.QueryRowContext(ctx, "SELECT download_client_id,download_id FROM "+table+" WHERE most_recent=1").Scan(&gotClient, &gotDownload))
		if gotClient.Valid || gotDownload != (sql.NullString{String: downloadID, Valid: true}) {
			t.Errorf("%s after client deletion: client=%+v download=%+v", table, gotClient, gotDownload)
		}
		if n := freshCount(t, s, "SELECT count(*) FROM "+table); n != 2 {
			t.Errorf("%s history count=%d, want 2", table, n)
		}
	}
	freshRequireNoError(t, s.DeleteMovie(ctx, movie))
	freshRequireNoError(t, s.DeleteSeries(ctx, series))
	for _, table := range []string{"movie", "movie_transition", "series", "series_transition", "season", "season_transition", "episode", "episode_transition"} {
		if n := freshCount(t, s, "SELECT count(*) FROM "+table); n != 0 {
			t.Errorf("%s cascade left %d rows", table, n)
		}
	}
	// File records have separate lifecycles; neither record deletion nor parent
	// deletion is permission to touch bytes on disk.
	freshRequireNoError(t, s.DeleteMovieFile(ctx, movieFile))
	freshRequireNoError(t, s.DeleteEpisodeFile(ctx, episodeFile))
	for _, path := range []string{moviePath, episodePath} {
		got, err := os.ReadFile(path)
		freshRequireNoError(t, err)
		if !reflect.DeepEqual(got, contents) {
			t.Errorf("media changed at %s", path)
		}
	}
	if _, err := s.GetMovieByPath(ctx, moviePath); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("missing movie by path: %v", err)
	}
	if _, err := s.GetSeasonByID(ctx, season); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("deleted season: %v", err)
	}
	if _, err := s.GetEpisodeByID(ctx, episode); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("deleted episode: %v", err)
	}
}

func TestFreshAcceptanceAtomicTransitionRollback(t *testing.T) {
	s, ctx := freshAcceptanceDB(t)
	id, err := s.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1}, storage.MovieStateMissing)
	freshRequireNoError(t, err)
	before, err := s.GetMovie(ctx, id)
	freshRequireNoError(t, err)
	// A real FK failure happens after DeactivateMovieTransition, unlike invalid
	// state validation which returns before entering the transaction.
	invalidClient := int64(-999)
	download := "invalid-client"
	err = s.UpdateMovieState(ctx, id, storage.MovieStateDownloading, &storage.TransitionStateMetadata{DownloadClientID: &invalidClient, DownloadID: &download})
	var constraint sqlite3.Error
	if !errors.As(err, &constraint) || constraint.ExtendedCode != sqlite3.ErrConstraintForeignKey {
		t.Fatalf("want injected FK failure, got %v", err)
	}
	after, err := s.GetMovie(ctx, id)
	freshRequireNoError(t, err)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("failed transition changed row:\nbefore: %+v\nafter: %+v", before, after)
	}
	if n := freshCount(t, s, "SELECT count(*) FROM movie_transition WHERE movie_id=?", id); n != 1 {
		t.Fatalf("failed transition left %d history rows", n)
	}
	freshRequireNoError(t, s.UpdateMovieState(ctx, id, storage.MovieStateDownloading, nil))
	freshRequireNoError(t, s.UpdateMovieState(ctx, id, storage.MovieStateDownloaded, nil))
	rows, err := s.db.QueryContext(ctx, "SELECT sort_key,most_recent FROM movie_transition WHERE movie_id=? ORDER BY sort_key", id)
	freshRequireNoError(t, err)
	defer rows.Close()
	for want := 1; want <= 3; want++ {
		if !rows.Next() {
			t.Fatalf("missing sort key %d", want)
		}
		var key int
		var latest bool
		freshRequireNoError(t, rows.Scan(&key, &latest))
		if key != want || latest != (want == 3) {
			t.Errorf("transition (%d,%t), want (%d,%t)", key, latest, want, want == 3)
		}
	}
	if rows.Next() {
		t.Error("extra transition")
	}
	freshRequireNoError(t, rows.Err())
}

func TestFreshAcceptanceCreateAndJobRollback(t *testing.T) {
	s, ctx := freshAcceptanceDB(t)
	_, err := s.db.ExecContext(ctx, `CREATE TRIGGER acceptance_fail_movie BEFORE INSERT ON movie_transition BEGIN SELECT RAISE(ABORT,'injected transition failure'); END`)
	freshRequireNoError(t, err)
	if _, err := s.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1}, storage.MovieStateMissing); err == nil {
		t.Fatal("create succeeded through abort trigger")
	}
	if n := freshCount(t, s, "SELECT count(*) FROM movie"); n != 0 {
		t.Fatalf("failed create left %d parent rows", n)
	}
	job, err := s.CreateJob(ctx, sqlcdb.Job{Type: "rollback"}, storage.JobStatePending)
	freshRequireNoError(t, err)
	before, err := s.GetJob(ctx, job)
	freshRequireNoError(t, err)
	_, err = s.db.ExecContext(ctx, `CREATE TRIGGER acceptance_fail_job BEFORE INSERT ON job_transition BEGIN SELECT RAISE(ABORT,'injected job transition failure'); END`)
	freshRequireNoError(t, err)
	if err := s.UpdateJobState(ctx, job, storage.JobStateRunning, nil); err == nil {
		t.Fatal("job update succeeded through abort trigger")
	}
	after, err := s.GetJob(ctx, job)
	freshRequireNoError(t, err)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("job rollback changed row: before=%+v after=%+v", before, after)
	}
	if _, err := s.CreateJob(ctx, sqlcdb.Job{Type: "new-job-rollback"}, storage.JobStatePending); err == nil {
		t.Fatal("job create succeeded through abort trigger")
	}
	if n := freshCount(t, s, "SELECT count(*) FROM job"); n != 1 {
		t.Fatalf("failed job create left %d jobs, want original only", n)
	}
	if n := freshCount(t, s, "SELECT count(*) FROM job_transition"); n != 1 {
		t.Fatalf("failed job writes left %d transitions", n)
	}
	_, err = s.db.ExecContext(ctx, "DROP TRIGGER acceptance_fail_job")
	freshRequireNoError(t, err)
	if err := s.UpdateJobState(ctx, job, storage.JobStateDone, nil); err == nil {
		t.Fatal("invalid pending->done transition accepted")
	}
	freshRequireNoError(t, s.UpdateJobState(ctx, job, storage.JobStateRunning, nil))
}

func TestFreshAcceptancePendingJobConcurrentDedupAndRelease(t *testing.T) {
	s, ctx := freshAcceptanceDB(t)
	s.db.SetMaxOpenConns(8)
	type result struct {
		id  int64
		err error
	}
	const contenders = 20
	start := make(chan struct{})
	results := make(chan result, contenders)
	var wg sync.WaitGroup
	for i := 0; i < contenders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			id, err := s.CreateJob(ctx, sqlcdb.Job{Type: "concurrent"}, storage.JobStatePending)
			results <- result{id, err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	var winner int64
	for r := range results {
		if r.err == nil {
			successes++
			winner = r.id
		} else if !errors.Is(r.err, storage.ErrJobAlreadyPending) {
			t.Errorf("loser returned non-domain error: %v", r.err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful concurrent pending creates=%d, want 1", successes)
	}
	if n := freshCount(t, s, "SELECT count(*) FROM job"); n != 1 {
		t.Fatalf("dedup leaked parent jobs: %d", n)
	}
	// Exercise the partial unique index directly, independently of CreateJob's
	// check-then-insert. The failed insert must not replace the pending winner.
	tx, err := s.db.BeginTx(ctx, nil)
	freshRequireNoError(t, err)
	defer tx.Rollback()
	inserted, err := tx.ExecContext(ctx, `INSERT INTO job(type) VALUES('concurrent')`)
	freshRequireNoError(t, err)
	other, err := inserted.LastInsertId()
	freshRequireNoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO job_transition(job_id,type,to_state,most_recent,sort_key) VALUES(?,'concurrent','pending',1,1)`, other)
	var unique sqlite3.Error
	if !errors.As(err, &unique) || unique.ExtendedCode != sqlite3.ErrConstraintUnique {
		t.Errorf("database pending-type uniqueness error = %v", err)
	}
	freshRequireNoError(t, tx.Rollback())
	freshRequireNoError(t, s.UpdateJobState(ctx, winner, storage.JobStateRunning, nil))
	next, err := s.CreateJob(ctx, sqlcdb.Job{Type: "concurrent"}, storage.JobStatePending)
	freshRequireNoError(t, err)
	freshRequireNoError(t, s.UpdateJobState(ctx, next, storage.JobStateCancelled, nil))
	third, err := s.CreateJob(ctx, sqlcdb.Job{Type: "concurrent"}, storage.JobStatePending)
	freshRequireNoError(t, err)
	freshRequireNoError(t, s.DeleteJob(ctx, third))
	_, err = s.CreateJob(ctx, sqlcdb.Job{Type: "concurrent"}, storage.JobStatePending)
	freshRequireNoError(t, err)
	if n := freshCount(t, s, "SELECT count(*) FROM job_transition WHERE job_id=?", third); n != 0 {
		t.Errorf("job deletion left %d transitions", n)
	}
}

func TestFreshAcceptanceJobPaginationFiltersAndSafeBulkDelete(t *testing.T) {
	s, ctx := freshAcceptanceDB(t)
	const total = 200
	ids := make([]int64, total)
	for i := range ids {
		id, err := s.CreateJob(ctx, sqlcdb.Job{Type: fmt.Sprintf("page-%03d", i)}, storage.JobStatePending)
		freshRequireNoError(t, err)
		ids[i] = id
		if i%2 == 0 {
			freshRequireNoError(t, s.UpdateJobState(ctx, id, storage.JobStateRunning, nil))
		}
	}
	// Guarantee ties rather than relying on the speed of the test machine.
	_, err := s.db.ExecContext(ctx, "UPDATE job SET created_at='2024-01-02 03:04:05'")
	freshRequireNoError(t, err)
	for pass := 0; pass < 2; pass++ {
		var got []int64
		for offset := 0; offset < total; offset += 17 {
			page, err := s.ListJobs(ctx, offset, 17, storage.JobFilter{})
			freshRequireNoError(t, err)
			for _, row := range page {
				got = append(got, row.Job.ID)
			}
		}
		want := make([]int64, total)
		for i := range want {
			want[i] = ids[total-i-1]
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("pass %d pagination IDs differ: got %v want %v", pass, got, want)
		}
	}
	pending, typ := string(storage.JobStatePending), "page-001"
	for _, filter := range []storage.JobFilter{{}, {State: &pending}, {Type: &typ}, {Type: &typ, State: &pending}} {
		all, err := s.ListJobs(ctx, 0, 0, filter)
		freshRequireNoError(t, err)
		count, err := s.CountJobs(ctx, filter)
		freshRequireNoError(t, err)
		want := total
		if filter.State != nil {
			want = total / 2
		}
		if filter.Type != nil {
			want = 1
		}
		if count != want || len(all) != want {
			t.Errorf("filter %+v count=%d list=%d want %d", filter, count, len(all), want)
		}
	}
	page, err := s.ListJobs(ctx, 17, 0, storage.JobFilter{})
	freshRequireNoError(t, err)
	if len(page) != total-17 {
		t.Errorf("zero limit with offset returned %d", len(page))
	}
	for _, empty := range [][]int64{nil, {}} {
		n, err := s.DeleteJobsByIDs(ctx, empty)
		freshRequireNoError(t, err)
		if n != 0 {
			t.Errorf("empty delete affected %d", n)
		}
	}
	n, err := s.DeleteJobsByIDs(ctx, []int64{ids[0], ids[0], ids[1], -999})
	freshRequireNoError(t, err)
	if n != 2 {
		t.Errorf("bulk delete affected %d, want 2", n)
	}
	if n := freshCount(t, s, "SELECT count(*) FROM job"); n != total-2 {
		t.Errorf("remaining jobs=%d", n)
	}
	if n := freshCount(t, s, "SELECT count(*) FROM job_transition WHERE job_id IN (?,?)", ids[0], ids[1]); n != 0 {
		t.Errorf("bulk delete orphaned %d transitions", n)
	}
}

func TestFreshAcceptanceUnboundedStateFilters(t *testing.T) {
	s, ctx := freshAcceptanceDB(t)
	// Each fixture takes a legal path from New; all six nonempty states are
	// represented. The fifth and sixth selections must not be silently dropped.
	states := []string{"missing", "unreleased", "discovered", "downloading", "continuing", "completed"}
	seriesIDs := make([]int64, len(states))
	seasonIDs := make([]int64, len(states))
	episodeIDs := make([]int64, len(states))
	episodeStates := []string{"missing", "unreleased", "discovered", "downloading", "downloaded", "completed"}
	for i, state := range states {
		initial := storage.SeriesState(state)
		if i >= 3 {
			initial = storage.SeriesStateMissing
		}
		id, err := s.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1}, initial)
		freshRequireNoError(t, err)
		seriesIDs[i] = id
		season, err := s.CreateSeason(ctx, sqlcdb.Season{SeriesID: id, SeasonNumber: 1}, storage.SeasonState(initial))
		freshRequireNoError(t, err)
		seasonIDs[i] = season
		if i >= 3 {
			freshRequireNoError(t, s.UpdateSeriesState(ctx, id, storage.SeriesStateDownloading, nil))
			freshRequireNoError(t, s.UpdateSeasonState(ctx, season, storage.SeasonStateDownloading, nil))
			if i >= 4 {
				freshRequireNoError(t, s.UpdateSeriesState(ctx, id, storage.SeriesState(state), nil))
				freshRequireNoError(t, s.UpdateSeasonState(ctx, season, storage.SeasonState(state), nil))
			}
		}
		epInitial := storage.EpisodeState(episodeStates[i])
		if i >= 3 {
			epInitial = storage.EpisodeStateMissing
		}
		episode, err := s.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: season, EpisodeNumber: 1}, epInitial)
		freshRequireNoError(t, err)
		episodeIDs[i] = episode
		if i >= 3 {
			freshRequireNoError(t, s.UpdateEpisodeState(ctx, episode, storage.EpisodeStateDownloading, nil))
		}
		if i >= 4 {
			freshRequireNoError(t, s.UpdateEpisodeState(ctx, episode, storage.EpisodeStateDownloaded, nil))
		}
		if i >= 5 {
			freshRequireNoError(t, s.UpdateEpisodeState(ctx, episode, storage.EpisodeStateCompleted, nil))
		}
	}
	for _, size := range []int{5, 6, 7} {
		sf, snf, ef := storage.SeriesFilter{}, storage.SeasonFilter{}, storage.EpisodeFilter{}
		expected := 0
		for i := 0; i < size; i++ {
			if i < len(states) {
				sf.States = append(sf.States, storage.SeriesState(states[i]))
				snf.States = append(snf.States, storage.SeasonState(states[i]))
				ef.States = append(ef.States, storage.EpisodeState(episodeStates[i]))
				expected++
			} else {
				sf.States = append(sf.States, "not-a-state")
				snf.States = append(snf.States, "not-a-state")
				ef.States = append(ef.States, "not-a-state")
			}
		}
		series, err := s.ListSeries(ctx, sf)
		freshRequireNoError(t, err)
		seasons, err := s.ListSeasons(ctx, snf)
		freshRequireNoError(t, err)
		episodes, err := s.ListEpisodes(ctx, ef)
		freshRequireNoError(t, err)
		gotSeries, gotSeasons, gotEpisodes := map[int64]string{}, map[int64]string{}, map[int64]string{}
		for _, row := range series {
			gotSeries[row.Series.ID] = row.State
		}
		for _, row := range seasons {
			gotSeasons[row.Season.ID] = row.State
		}
		for _, row := range episodes {
			gotEpisodes[row.Episode.ID] = row.State
		}
		wantSeries, wantSeasons, wantEpisodes := map[int64]string{}, map[int64]string{}, map[int64]string{}
		for i := 0; i < expected; i++ {
			wantSeries[seriesIDs[i]] = states[i]
			wantSeasons[seasonIDs[i]] = states[i]
			wantEpisodes[episodeIDs[i]] = episodeStates[i]
		}
		if !reflect.DeepEqual(gotSeries, wantSeries) || len(series) != expected {
			t.Errorf("%d-state series filter: got %v want %v", size, gotSeries, wantSeries)
		}
		if !reflect.DeepEqual(gotSeasons, wantSeasons) || len(seasons) != expected {
			t.Errorf("%d-state season filter: got %v want %v", size, gotSeasons, wantSeasons)
		}
		if !reflect.DeepEqual(gotEpisodes, wantEpisodes) || len(episodes) != expected {
			t.Errorf("%d-state episode filter: got %v want %v", size, gotEpisodes, wantEpisodes)
		}
	}
	// Parent predicates must be ANDed with the entire state slice, not only its
	// first element. Pick the last fixture, beyond the old four-slot cap.
	parent := seriesIDs[5]
	seasons, err := s.ListSeasons(ctx, storage.SeasonFilter{SeriesID: &parent, States: []storage.SeasonState{"missing", "unreleased", "discovered", "downloading", "continuing", "completed"}})
	freshRequireNoError(t, err)
	if len(seasons) != 1 || seasons[0].Season.ID != seasonIDs[5] {
		t.Fatalf("parent + state filter returned %+v", seasons)
	}
}

func TestFreshAcceptanceUTCTimestampsAndDefaults(t *testing.T) {
	// Run this under TZ=Pacific/Auckland as well as the ordinary suite. Bounds
	// compare instants, so a local wall-clock string incorrectly stored as UTC
	// cannot pass merely because all fields share the same wrong timezone.
	s, ctx := freshAcceptanceDB(t)
	start := time.Now().UTC().Add(-2 * time.Second)
	movie, err := s.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1}, storage.MovieStateMissing)
	freshRequireNoError(t, err)
	freshRequireNoError(t, s.UpdateMovieState(ctx, movie, storage.MovieStateDownloading, nil))
	series, err := s.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1}, storage.SeriesStateMissing)
	freshRequireNoError(t, err)
	job, err := s.CreateJob(ctx, sqlcdb.Job{Type: "utc"}, storage.JobStatePending)
	freshRequireNoError(t, err)
	freshRequireNoError(t, s.UpdateJobState(ctx, job, storage.JobStateRunning, nil))
	metadata, err := s.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 701, Title: "UTC", Status: "continuing"})
	freshRequireNoError(t, err)
	row, err := s.GetSeriesMetadataByID(ctx, metadata)
	freshRequireNoError(t, err)
	freshRequireNoError(t, s.UpdateSeriesMetadata(ctx, row))
	movieFile, err := s.CreateMovieFile(ctx, sqlcdb.MovieFile{Quality: "HD", Languages: "[]"})
	freshRequireNoError(t, err)
	episodeFile, err := s.CreateEpisodeFile(ctx, sqlcdb.EpisodeFile{Quality: "HD"})
	freshRequireNoError(t, err)
	end := time.Now().UTC().Add(2 * time.Second)
	for _, probe := range []struct {
		query string
		id    int64
	}{
		{"SELECT added FROM movie WHERE id=?", movie},
		{"SELECT added FROM series WHERE id=?", series},
		{"SELECT created_at FROM job WHERE id=?", job},
		{"SELECT updated_at FROM movie_transition WHERE movie_id=? AND most_recent=1", movie},
		{"SELECT updated_at FROM job_transition WHERE job_id=? AND most_recent=1", job},
		{"SELECT last_info_sync FROM series_metadata WHERE id=?", metadata},
		{"SELECT date_added FROM movie_file WHERE id=?", movieFile},
		{"SELECT added FROM episode_file WHERE id=?", episodeFile},
	} {
		var value sql.NullTime
		freshRequireNoError(t, s.db.QueryRowContext(ctx, probe.query, probe.id).Scan(&value))
		if !value.Valid || value.Time.Before(start) || value.Time.After(end) {
			t.Errorf("%s: timestamp %+v outside UTC bounds [%s,%s]", probe.query, value, start, end)
		}
		_, offset := value.Time.Zone()
		if offset != 0 {
			t.Errorf("%s: timestamp has non-UTC offset %d", probe.query, offset)
		}
	}
}

func TestFreshAcceptanceIndexerCascadeRollback(t *testing.T) {
	s, ctx := freshAcceptanceDB(t)
	source, err := s.CreateIndexerSource(ctx, sqlcdb.IndexerSource{Name: "source", Implementation: "Prowlarr", Scheme: "http", Host: "localhost", Enabled: true})
	freshRequireNoError(t, err)
	for _, name := range []string{"one", "two"} {
		_, err := s.CreateIndexer(ctx, sqlcdb.Indexer{Name: name, Uri: "http://localhost", IndexerSourceID: sql.NullInt64{Int64: source, Valid: true}})
		freshRequireNoError(t, err)
	}
	_, err = s.db.ExecContext(ctx, `CREATE TRIGGER acceptance_fail_source_delete BEFORE DELETE ON indexer_source BEGIN SELECT RAISE(ABORT,'injected source delete failure'); END`)
	freshRequireNoError(t, err)
	if err := s.DeleteIndexerSourceCascade(ctx, source); err == nil {
		t.Fatal("source delete succeeded through abort trigger")
	}
	if n := freshCount(t, s, "SELECT count(*) FROM indexer_source WHERE id=?", source); n != 1 {
		t.Errorf("rollback lost source: %d", n)
	}
	if n := freshCount(t, s, "SELECT count(*) FROM indexer WHERE indexer_source_id=?", source); n != 2 {
		t.Errorf("rollback lost child indexers: %d", n)
	}
	_, err = s.db.ExecContext(ctx, "DROP TRIGGER acceptance_fail_source_delete")
	freshRequireNoError(t, err)
	freshRequireNoError(t, s.DeleteIndexerSourceCascade(ctx, source))
	if n := freshCount(t, s, "SELECT count(*) FROM indexer"); n != 0 {
		t.Errorf("cascade left %d indexers", n)
	}
	if _, err := s.GetIndexerSource(ctx, source); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("deleted source lookup: %v", err)
	}
}
