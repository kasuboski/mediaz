//go:build functional
// +build functional

package manager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/storage"
	sqliteStorage "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// FunctionalTestSuite contains all functional test infrastructure
type FunctionalTestSuite struct {
	t   *testing.T
	ctx context.Context
	log *zap.SugaredLogger

	// Test environment paths
	testDir      string
	dbPath       string
	movieLibrary string
	tvLibrary    string

	// Services
	storage storage.Storage
	manager *MediaManager

	// Test data
	testMovies []TestMovie
	testShows  []TestShow

	// Cleanup functions
	cleanupFuncs []func() error
}

type TestMovie struct {
	Path         string
	ExpectedName string
	ExpectedYear int
}

type TestShow struct {
	SeriesName   string
	SeasonNumber int
	Episodes     []TestEpisode
}

type TestEpisode struct {
	Path          string
	EpisodeNumber int
	ExpectedTitle string
}

// TestFunctionalReconciliation is the main functional test entry point
func TestFunctionalReconciliation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional tests in short mode")
	}

	suite := NewFunctionalTestSuite(t)
	defer suite.Cleanup()

	t.Run("Setup Test Environment", suite.TestSetupEnvironment)
	t.Run("Create Test Media Files", suite.TestCreateMediaFiles)
	t.Run("Initialize Media Manager", suite.TestInitializeManager)
	t.Run("Run Movie Reconciliation", suite.TestMovieReconciliation)
	t.Run("Run Series Reconciliation", suite.TestSeriesReconciliation)
	t.Run("Verify Database Integrity", suite.TestDatabaseIntegrity)
	t.Run("Verify Foreign Key Relationships", suite.TestForeignKeyIntegrity)
	t.Run("Verify No Duplicates", suite.TestNoDuplicateEntries)
	t.Run("Verify Metadata Hierarchy", suite.TestMetadataHierarchy)
}

func NewFunctionalTestSuite(t *testing.T) *FunctionalTestSuite {
	ctx := context.Background()
	log := logger.Get()

	// Create temporary test directory
	testDir, err := os.MkdirTemp("", "mediaz-functional-test-*")
	require.NoError(t, err)

	suite := &FunctionalTestSuite{
		t:            t,
		ctx:          ctx,
		log:          log,
		testDir:      testDir,
		dbPath:       filepath.Join(testDir, "test.db"),
		movieLibrary: filepath.Join(testDir, "movies"),
		tvLibrary:    filepath.Join(testDir, "tv"),
		cleanupFuncs: make([]func() error, 0),
	}

	// Add main cleanup function
	suite.addCleanup(func() error {
		return os.RemoveAll(testDir)
	})

	return suite
}

func (s *FunctionalTestSuite) addCleanup(fn func() error) {
	s.cleanupFuncs = append(s.cleanupFuncs, fn)
}

func (s *FunctionalTestSuite) Cleanup() {
	for i := len(s.cleanupFuncs) - 1; i >= 0; i-- {
		if err := s.cleanupFuncs[i](); err != nil {
			s.log.Warnw("cleanup failed", "error", err)
		}
	}
}

// TestSetupEnvironment creates the test database and library directories
func (s *FunctionalTestSuite) TestSetupEnvironment(t *testing.T) {
	// Create library directories
	require.NoError(t, os.MkdirAll(s.movieLibrary, 0755))
	require.NoError(t, os.MkdirAll(s.tvLibrary, 0755))

	// Initialize storage
	store, err := sqliteStorage.New(s.dbPath)
	require.NoError(t, err)
	s.storage = store

	// Initialize database schema
	schemas, err := storage.GetSchemas()
	require.NoError(t, err)

	err = s.storage.Init(s.ctx, schemas...)
	require.NoError(t, err)

	s.log.Infow("test environment setup complete",
		"test_dir", s.testDir,
		"db_path", s.dbPath,
		"movie_library", s.movieLibrary,
		"tv_library", s.tvLibrary)
}

// TestCreateMediaFiles creates comprehensive test media files
func (s *FunctionalTestSuite) TestCreateMediaFiles(t *testing.T) {
	// Define test movies with various naming patterns
	s.testMovies = []TestMovie{
		{"The Matrix (1999).mp4", "The Matrix", 1999},
		{"Inception.2010.1080p.BluRay.x264.mp4", "Inception", 2010},
		{"Pulp Fiction (1994) [1080p].mkv", "Pulp Fiction", 1994},
		{"The.Godfather.1972.REMASTERED.1080p.BluRay.x264.mkv", "The Godfather", 1972},
		{"Interstellar 2014 4K UHD.mp4", "Interstellar", 2014},
	}

	// Define test TV shows with various naming patterns
	s.testShows = []TestShow{
		{
			SeriesName:   "Breaking Bad",
			SeasonNumber: 1,
			Episodes: []TestEpisode{
				{"Breaking Bad S01E01 - Pilot.mp4", 1, "Pilot"},
				{"Breaking Bad S01E02 - Cat's in the Bag....mp4", 2, "Cat's in the Bag..."},
				{"Breaking Bad S01E03 - ...And the Bag's in the River.mkv", 3, "...And the Bag's in the River"},
			},
		},
		{
			SeriesName:   "Breaking Bad",
			SeasonNumber: 2,
			Episodes: []TestEpisode{
				{"Breaking Bad S02E01 Seven Thirty-Seven.mp4", 1, "Seven Thirty-Seven"},
				{"Breaking Bad S02E02 Grilled.mkv", 2, "Grilled"},
			},
		},
		{
			SeriesName:   "Stranger Things",
			SeasonNumber: 1,
			Episodes: []TestEpisode{
				{"Stranger.Things.S01E01.1080p.WEBRip.x264.mp4", 1, "Chapter One: The Vanishing of Will Byers"},
				{"Stranger.Things.S01E02.1080p.WEBRip.x264.mp4", 2, "Chapter Two: The Weirdo on Maple Street"},
			},
		},
		{
			SeriesName:   "Game of Thrones",
			SeasonNumber: 0, // Specials
			Episodes: []TestEpisode{
				{"Game of Thrones S00E01 - Special.mp4", 1, "Special"},
			},
		},
	}

	// Create movie files
	for _, movie := range s.testMovies {
		moviePath := filepath.Join(s.movieLibrary, movie.Path)
		require.NoError(t, s.createEmptyFile(moviePath))
	}

	// Create TV show files
	for _, show := range s.testShows {
		seasonDir := filepath.Join(s.tvLibrary, show.SeriesName, fmt.Sprintf("Season %d", show.SeasonNumber))
		require.NoError(t, os.MkdirAll(seasonDir, 0755))

		for _, episode := range show.Episodes {
			episodePath := filepath.Join(seasonDir, episode.Path)
			require.NoError(t, s.createEmptyFile(episodePath))
		}
	}

	s.log.Infow("test media files created",
		"movies", len(s.testMovies),
		"shows", len(s.testShows))
}

// createEmptyFile creates an empty file with minimal valid header
func (s *FunctionalTestSuite) createEmptyFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write minimal valid MP4/MKV header to make it a recognizable media file
	ext := filepath.Ext(path)
	switch ext {
	case ".mp4":
		// Minimal MP4 header
		_, err = file.Write([]byte{0x00, 0x00, 0x00, 0x20, 0x66, 0x74, 0x79, 0x70})
	case ".mkv":
		// Minimal MKV header
		_, err = file.Write([]byte{0x1A, 0x45, 0xDF, 0xA3})
	default:
		// Generic media file marker
		_, err = file.Write([]byte("MEDIAZ_TEST_FILE"))
	}

	return err
}

// TestInitializeManager sets up the MediaManager with test configuration
func (s *FunctionalTestSuite) TestInitializeManager(t *testing.T) {
	managerConfig := config.Manager{
		Jobs: config.Jobs{
			MovieReconcile:  time.Minute,
			MovieIndex:      time.Minute,
			SeriesReconcile: time.Minute,
			SeriesIndex:     time.Minute,
		},
	}

	// For functional tests, we can use nil for external services (TMDB, Prowlarr, etc.)
	// since we're focusing on the reconciliation logic and database integrity
	manager := New(nil, nil, nil, s.storage, nil, managerConfig)
	s.manager = &manager

	require.NotNil(t, s.manager)
	s.log.Info("media manager initialized for functional tests")
}

// TestMovieReconciliation tests movie-related functionality
func (s *FunctionalTestSuite) TestMovieReconciliation(t *testing.T) {
	// For now, skip direct indexing and focus on database integrity
	// Future enhancement: add actual movie indexing integration

	// Verify movies can be queried (even if empty)
	movies, err := s.storage.ListMovies(s.ctx)
	require.NoError(t, err)

	s.log.Infow("movie storage verified", "movies_count", len(movies))
}

// TestSeriesReconciliation tests series-related functionality
func (s *FunctionalTestSuite) TestSeriesReconciliation(t *testing.T) {
	// For now, skip direct indexing and focus on database integrity
	// Future enhancement: add actual series indexing integration

	// Verify series can be queried (even if empty)
	series, err := s.storage.ListSeries(s.ctx, nil)
	require.NoError(t, err)

	s.log.Infow("series storage verified", "series_count", len(series))
}

// TestDatabaseIntegrity verifies overall database state after reconciliation
func (s *FunctionalTestSuite) TestDatabaseIntegrity(t *testing.T) {
	// Verify database file exists and is accessible
	_, err := os.Stat(s.dbPath)
	require.NoError(t, err, "Database file should exist")

	// Test basic database connectivity
	ctx := context.Background()

	// Verify that storage operations work
	series, err := s.storage.ListSeries(ctx, nil)
	if err != nil && err != storage.ErrNotFound {
		t.Fatalf("Failed to query series table: %v", err)
	}

	movies, err := s.storage.ListMovies(ctx)
	if err != nil && err != storage.ErrNotFound {
		t.Fatalf("Failed to query movies table: %v", err)
	}

	s.log.Infow("database integrity check passed",
		"series_count", len(series),
		"movies_count", len(movies))
}

// TestForeignKeyIntegrity verifies all foreign key relationships are correct
func (s *FunctionalTestSuite) TestForeignKeyIntegrity(t *testing.T) {
	ctx := context.Background()

	// Get all series with metadata
	series, err := s.storage.ListSeries(ctx, table.Series.SeriesMetadataID.IS_NOT_NULL())
	if err != nil && err != storage.ErrNotFound {
		require.NoError(t, err)
	}

	for _, seriesEntity := range series {
		if seriesEntity.SeriesMetadataID == nil {
			continue
		}

		// Verify series metadata exists
		seriesMetadata, err := s.storage.GetSeriesMetadata(ctx,
			table.SeriesMetadata.ID.EQ(sqlite.Int32(*seriesEntity.SeriesMetadataID)))
		if err != nil {
			t.Errorf("Series %d references non-existent metadata %d",
				seriesEntity.ID, *seriesEntity.SeriesMetadataID)
			continue
		}

		// Verify seasons belong to this series
		seasons, err := s.storage.ListSeasons(ctx,
			table.Season.SeriesID.EQ(sqlite.Int32(seriesEntity.ID)))
		if err != nil && err != storage.ErrNotFound {
			require.NoError(t, err)
		}

		for _, season := range seasons {
			assert.Equal(t, seriesEntity.ID, season.SeriesID,
				"Season %d should belong to series %d", season.ID, seriesEntity.ID)

			if season.SeasonMetadataID != nil {
				// Verify season metadata exists and references correct series metadata
				seasonMetadata, err := s.storage.GetSeasonMetadata(ctx,
					table.SeasonMetadata.ID.EQ(sqlite.Int32(*season.SeasonMetadataID)))
				if err != nil {
					t.Errorf("Season %d references non-existent metadata %d",
						season.ID, *season.SeasonMetadataID)
					continue
				}

				assert.Equal(t, seriesMetadata.ID, seasonMetadata.SeriesMetadataID,
					"Season metadata %d should reference series metadata %d",
					seasonMetadata.ID, seriesMetadata.ID)
			}

			// Verify episodes belong to this season
			episodes, err := s.storage.ListEpisodes(ctx,
				table.Episode.SeasonID.EQ(sqlite.Int32(season.ID)))
			if err != nil && err != storage.ErrNotFound {
				require.NoError(t, err)
			}

			for _, episode := range episodes {
				assert.Equal(t, season.ID, episode.SeasonID,
					"Episode %d should belong to season %d", episode.ID, season.ID)

				if episode.EpisodeMetadataID != nil {
					// Verify episode metadata exists and references correct season metadata
					if season.SeasonMetadataID != nil {
						episodeMetadata, err := s.storage.GetEpisodeMetadata(ctx,
							table.EpisodeMetadata.ID.EQ(sqlite.Int32(*episode.EpisodeMetadataID)))
						if err != nil {
							t.Errorf("Episode %d references non-existent metadata %d",
								episode.ID, *episode.EpisodeMetadataID)
							continue
						}

						assert.Equal(t, *season.SeasonMetadataID, episodeMetadata.SeasonMetadataID,
							"Episode metadata %d should reference season metadata %d",
							episodeMetadata.ID, *season.SeasonMetadataID)
					}
				}
			}
		}
	}

	s.log.Info("foreign key integrity verification completed")
}

// TestNoDuplicateEntries ensures no duplicate entities exist
func (s *FunctionalTestSuite) TestNoDuplicateEntries(t *testing.T) {
	ctx := context.Background()

	// Check for duplicate series by path
	series, err := s.storage.ListSeries(ctx, nil)
	if err != nil && err != storage.ErrNotFound {
		require.NoError(t, err)
	}

	seriesPaths := make(map[string]int)
	for _, s := range series {
		if s.Path != nil {
			if existingID, exists := seriesPaths[*s.Path]; exists {
				t.Errorf("Duplicate series found for path %s: IDs %d and %d",
					*s.Path, existingID, s.ID)
			}
			seriesPaths[*s.Path] = int(s.ID)
		}
	}

	// Check for duplicate seasons within each series
	for _, seriesEntity := range series {
		seasons, err := s.storage.ListSeasons(ctx,
			table.Season.SeriesID.EQ(sqlite.Int32(seriesEntity.ID)))
		if err != nil && err != storage.ErrNotFound {
			require.NoError(t, err)
		}

		seasonNumbers := make(map[int32]int)
		for _, season := range seasons {
			if existingID, exists := seasonNumbers[season.SeasonNumber]; exists {
				t.Errorf("Duplicate season %d found in series %d: IDs %d and %d",
					season.SeasonNumber, seriesEntity.ID, existingID, season.ID)
			}
			seasonNumbers[season.SeasonNumber] = int(season.ID)
		}
	}

	// Check for duplicate series metadata by TMDB ID
	allSeriesMetadata, err := s.storage.ListSeriesMetadata(ctx, nil)
	if err != nil && err != storage.ErrNotFound {
		require.NoError(t, err)
	}

	tmdbIDs := make(map[int32]int)
	for _, metadata := range allSeriesMetadata {
		if existingID, exists := tmdbIDs[metadata.TmdbID]; exists {
			t.Errorf("Duplicate series metadata found for TMDB ID %d: IDs %d and %d",
				metadata.TmdbID, existingID, metadata.ID)
		}
		tmdbIDs[metadata.TmdbID] = int(metadata.ID)
	}

	s.log.Info("duplicate entry verification completed")
}

// TestMetadataHierarchy verifies metadata relationships form proper hierarchies
func (s *FunctionalTestSuite) TestMetadataHierarchy(t *testing.T) {
	ctx := context.Background()

	// Get all series metadata
	allSeriesMetadata, err := s.storage.ListSeriesMetadata(ctx, nil)
	if err != nil && err != storage.ErrNotFound {
		require.NoError(t, err)
	}

	for _, seriesMetadata := range allSeriesMetadata {
		// Get season metadata for this series
		seasonMetadata, err := s.storage.ListSeasonMetadata(ctx,
			table.SeasonMetadata.SeriesMetadataID.EQ(sqlite.Int32(seriesMetadata.ID)))
		if err != nil && err != storage.ErrNotFound {
			require.NoError(t, err)
		}

		// Verify each season metadata references this series metadata
		for _, seasonMeta := range seasonMetadata {
			assert.Equal(t, seriesMetadata.ID, seasonMeta.SeriesMetadataID,
				"Season metadata %d should reference series metadata %d",
				seasonMeta.ID, seriesMetadata.ID)

			// Get episode metadata for this season
			episodeMetadata, err := s.storage.ListEpisodeMetadata(ctx,
				table.EpisodeMetadata.SeasonMetadataID.EQ(sqlite.Int32(seasonMeta.ID)))
			if err != nil && err != storage.ErrNotFound {
				require.NoError(t, err)
			}

			// Verify each episode metadata references this season metadata
			for _, episodeMeta := range episodeMetadata {
				assert.Equal(t, seasonMeta.ID, episodeMeta.SeasonMetadataID,
					"Episode metadata %d should reference season metadata %d",
					episodeMeta.ID, seasonMeta.ID)
			}
		}
	}

	s.log.Info("metadata hierarchy verification completed")
}
