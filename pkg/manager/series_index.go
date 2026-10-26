package manager

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/storage"
	sqlcdb "github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"go.uber.org/zap"
)

// IndexSeriesLibrary indexes the tv library directory for new files that are not yet monitored. The episodes are then stored with a state of discovered.
func (m MediaManager) IndexSeriesLibrary(ctx context.Context) error {
	log := logger.FromCtx(ctx).With("indexer", "series")

	discoveredFiles, err := m.library.FindEpisodes(ctx)
	if err != nil {
		return err
	}

	if len(discoveredFiles) == 0 {
		log.Debug("no files discovered")
		return nil
	}

	episodeFiles, err := m.seriesStorage.ListEpisodeFiles(ctx)
	if err != nil {
		return err
	}

	for _, discoveredFile := range discoveredFiles {
		matchedID, matchedPath := matchEpisodeFile(discoveredFile, episodeFiles, log)
		if err := m.upsertEpisodeFile(ctx, discoveredFile, matchedID, matchedPath); err != nil {
			log.Error("failed to upsert episode file", zap.Error(err))
		}
	}

	// pull the updated episode file list in case we added anything above
	episodeFiles, err = m.seriesStorage.ListEpisodeFiles(ctx)
	if err != nil {
		return err
	}

	for _, f := range episodeFiles {
		if !f.RelativePath.Valid {
			continue
		}
		// rely on discovered EpisodeFile data; library provides series/season parsing
		var df library.EpisodeFile
		for _, d := range discoveredFiles {
			if d.RelativePath == f.RelativePath.String {
				df = d
				break
			}
		}
		if df.SeriesName == "" {
			log.Warn("skipping episode file because series name is empty after matching", zap.String("episode_file_relative_path", f.RelativePath.String))
			continue
		}

		// Check if this specific episode file already has an associated episode
		if _, err := m.seriesStorage.GetEpisodeByEpisodeFileID(ctx, f.ID); err == nil {
			log.Debug("episode file already has associated episode", zap.Int64("file_id", f.ID))
			continue
		}
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			log.Debug("error checking existing episode for file", zap.Error(err))
			continue
		}

		seriesID, err := m.ensureSeries(ctx, df.SeriesName)
		if err != nil {
			log.Error("couldn't ensure series for discovered file", zap.Error(err))
			continue
		}

		seasonID, err := m.getOrCreateSeason(ctx, seriesID, int64(df.SeasonNumber), nil, storage.SeasonStateDiscovered)
		if err != nil {
			log.Error("couldn't ensure season for discovered file", zap.Error(err))
			continue
		}

		// ensure episode exists; parse episode number from the discovered file data
		// Link the episode to the episode file so reconciliation can find the file details
		episode := sqlcdb.Episode{
			SeasonID:      seasonID,
			Monitored:     0,
			EpisodeFileID: sql.NullInt64{Int64: f.ID, Valid: true},
			EpisodeNumber: int64(df.EpisodeNumber),
		}
		episodeID, err := m.seriesStorage.CreateEpisode(ctx, episode, storage.EpisodeStateDiscovered)
		if err != nil {
			log.Warn("failed to create new episode", zap.Error(err))
			continue
		}

		log.Debug("successfully indexed new episode", zap.Int64("ID", episodeID), zap.Int("number", df.EpisodeNumber), zap.Int64("Season ID", seasonID))
	}

	return nil
}

func (m MediaManager) ensureSeries(ctx context.Context, seriesName string) (int64, error) {
	log := logger.FromCtx(ctx)

	series, err := m.seriesStorage.GetSeriesByPath(ctx, seriesName)
	if errors.Is(err, storage.ErrNotFound) {
		log.Debug("episode file does not have associated series, creating new series")
		// Discovery does not have a user-selected profile, but the required
		// foreign key still needs a real TV profile. Choose the first available
		// episode profile rather than relying on the old invalid zero ID.
		profiles, err := m.qualityService.qualityStorage.ListQualityProfilesByMediaType(ctx, "episode")
		if err != nil {
			return 0, fmt.Errorf("list TV quality profiles for discovery: %w", err)
		}
		if len(profiles) == 0 {
			return 0, fmt.Errorf("cannot discover series without a TV quality profile")
		}
		seriesModel := sqlcdb.Series{Path: sql.NullString{String: seriesName, Valid: true}, Monitored: 0, QualityProfileID: profiles[0].ID}
		seriesID, err := m.seriesStorage.CreateSeries(ctx, seriesModel, storage.SeriesStateDiscovered)
		if err != nil {
			return 0, err
		}
		log.Debug("created new discovered series", zap.Int64("series id", seriesID))
		return seriesID, nil
	}
	if err != nil {
		return 0, err
	}

	seriesID := series.Series.ID
	log.Debug("using existing series", zap.Int64("series id", seriesID))
	return seriesID, nil
}

// getOrCreateSeason unified function to get or create a season, with optional metadata linking
// This prevents the duplicate creation issues between discovery and metadata refresh phases
func (m MediaManager) getOrCreateSeason(ctx context.Context, seriesID int64, seasonNumber int64, seasonMetadataID *int64, initialState storage.SeasonState) (int64, error) {
	log := logger.FromCtx(ctx).With(
		zap.Int64("series_id", seriesID),
		zap.Int64("season_number", seasonNumber),
	)

	// First try to find existing season by series_id + season_number (our unique constraint)
	season, err := m.seriesStorage.GetSeasonBySeriesIDAndNumber(ctx, seriesID, seasonNumber)

	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return 0, err
	}

	if err == nil {
		// Season exists, update metadata link if provided and missing
		if seasonMetadataID != nil && !season.Season.SeasonMetadataID.Valid {
			err = m.seriesStorage.LinkSeasonMetadata(ctx, season.Season.ID, *seasonMetadataID)
			if err != nil {
				log.Error("failed to link season metadata", zap.Error(err))
				return 0, err
			}
			log.Debug("linked existing season to metadata",
				zap.Int64("season_id", season.Season.ID),
				zap.Int64("season_metadata_id", *seasonMetadataID))
		}
		return season.Season.ID, nil
	}

	// Season doesn't exist, create new one with metadata link if available
	newSeason := sqlcdb.Season{
		SeriesID:     seriesID,
		SeasonNumber: seasonNumber,
		Monitored:    0,
	}
	if seasonMetadataID != nil {
		newSeason.SeasonMetadataID = sql.NullInt64{Int64: *seasonMetadataID, Valid: true}
	}

	seasonID, err := m.seriesStorage.CreateSeason(ctx, newSeason, initialState)
	if err != nil {
		return 0, err
	}

	log.Debug("created new season",
		zap.Int64("season_id", seasonID),
		zap.String("initial_state", string(initialState)),
		zap.Any("season_metadata_id", seasonMetadataID))

	return seasonID, nil
}

func episodeFileFromLibrary(df library.EpisodeFile) sqlcdb.EpisodeFile {
	return sqlcdb.EpisodeFile{
		OriginalFilePath: sql.NullString{String: df.AbsolutePath, Valid: true},
		RelativePath:     sql.NullString{String: df.RelativePath, Valid: true},
		Size:             df.Size,
	}
}

func matchEpisodeFile(discoveredFile library.EpisodeFile, episodeFiles []sqlcdb.EpisodeFile, log *zap.SugaredLogger) (int64, string) {
	for _, ef := range episodeFiles {

		hasRelativePath := ef.RelativePath.Valid && ef.RelativePath.String != ""
		if hasRelativePath && strings.EqualFold(ef.RelativePath.String, discoveredFile.RelativePath) {
			log.Debug("discovered file relative path matches monitored episode file relative path",
				zap.String("discovered file relative path", discoveredFile.RelativePath),
				zap.String("monitored file relative path", ef.RelativePath.String))
			var path string
			if ef.OriginalFilePath.Valid {
				path = ef.OriginalFilePath.String
			}
			return ef.ID, path
		}

		hasAbsolutePath := ef.OriginalFilePath.Valid && ef.OriginalFilePath.String != ""
		hasDiscoveredAbsolutePath := discoveredFile.AbsolutePath != ""
		if hasAbsolutePath && hasDiscoveredAbsolutePath && strings.EqualFold(ef.OriginalFilePath.String, discoveredFile.AbsolutePath) {
			log.Debug("discovered file absolute path matches monitored episode file original path",
				zap.String("discovered file absolute path", discoveredFile.AbsolutePath),
				zap.String("monitored file original path", ef.OriginalFilePath.String))
			return ef.ID, ef.OriginalFilePath.String
		}
	}

	return 0, ""
}

func (m *MediaManager) upsertEpisodeFile(ctx context.Context, discoveredFile library.EpisodeFile, matchedID int64, matchedPath string) error {
	log := logger.FromCtx(ctx).With(
		zap.String("relative_path", discoveredFile.RelativePath),
		zap.String("discovered_absolute_path", discoveredFile.AbsolutePath),
		zap.Int64("matched_id", matchedID))

	if matchedID == 0 {
		ef := episodeFileFromLibrary(discoveredFile)
		log.Debug("creating new episode file", zap.Int64("size", discoveredFile.Size))
		_, err := m.seriesStorage.CreateEpisodeFile(ctx, ef)
		if err != nil {
			return fmt.Errorf("couldn't store episode file: %w", err)
		}
		log.Debug("successfully created episode file")
		return nil
	}

	log.Debug("episode file matched existing record", zap.String("matched_path", matchedPath))

	if matchedPath == "" {
		log.Debug("skipping update: matched path is empty")
		return nil
	}

	if discoveredFile.AbsolutePath == "" {
		log.Debug("skipping update: discovered absolute path is empty")
		return nil
	}

	if strings.EqualFold(matchedPath, discoveredFile.AbsolutePath) {
		log.Debug("absolute paths match, no update needed")
		return nil
	}

	log.Info("updating episode file absolute path",
		zap.String("old_absolute_path", matchedPath),
		zap.String("new_absolute_path", discoveredFile.AbsolutePath))

	existingFile, err := m.seriesStorage.GetEpisodeFile(ctx, matchedID)
	if err != nil {
		return fmt.Errorf("failed to get episode file for update: %w", err)
	}

	existingFile.OriginalFilePath = sql.NullString{String: discoveredFile.AbsolutePath, Valid: true}
	err = m.seriesStorage.UpdateEpisodeFile(ctx, matchedID, existingFile)
	if err != nil {
		return fmt.Errorf("failed to update episode file absolute path: %w", err)
	}

	log.Debug("successfully updated episode file absolute path")

	return nil
}
