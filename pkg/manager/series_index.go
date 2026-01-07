package manager

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
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

	episodeFiles, err := m.storage.ListEpisodeFiles(ctx)
	if err != nil {
		return err
	}

	for _, discoveredFile := range discoveredFiles {
		matchedID, matchedPath := matchEpisodeFile(discoveredFile, episodeFiles, log)
		if err := m.upsertEpisodeFile(ctx, discoveredFile, matchedID, matchedPath); err != nil {
			log.Errorf("failed to upsert episode file: %v", err)
		}
	}

	// pull the updated episode file list in case we added anything above
	episodeFiles, err = m.storage.ListEpisodeFiles(ctx)
	if err != nil {
		return err
	}

	for _, f := range episodeFiles {
		if f == nil || f.RelativePath == nil {
			continue
		}
		// rely on discovered EpisodeFile data; library provides series/season parsing
		var df library.EpisodeFile
		for _, d := range discoveredFiles {
			if d.RelativePath == *f.RelativePath {
				df = d
				break
			}
		}
		if df.SeriesName == "" {
			log.Warnf("skipping episode file because series name is empty after matching", zap.String("episode_file_relative_path", *f.RelativePath))
			continue
		}

		// Check if this specific episode file already has an associated episode
		existingEpisode, err := m.storage.GetEpisodeByEpisodeFileID(ctx, int64(f.ID))
		if err == nil && existingEpisode != nil {
			log.Debug("episode file already has associated episode",
				zap.Int32("file_id", f.ID),
				zap.Int32("episode_id", existingEpisode.ID))
			continue
		}
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			log.Debug("error checking existing episode for file", zap.Error(err))
			continue
		}

		seriesID, err := m.ensureSeries(ctx, df.SeriesName)
		if err != nil {
			log.Errorf("couldn't ensure series for discovered file: %w", err)
			continue
		}

		seasonID, err := m.getOrCreateSeason(ctx, seriesID, int32(df.SeasonNumber), nil, storage.SeasonStateDiscovered)
		if err != nil {
			log.Errorf("couldn't ensure season for discovered file: %w", err)
			continue
		}

		// ensure episode exists; parse episode number from the discovered file data
		// Link the episode to the episode file so reconciliation can find the file details
		episode := storage.Episode{Episode: model.Episode{
			SeasonID:      int32(seasonID),
			Monitored:     0,
			EpisodeFileID: &f.ID,
			EpisodeNumber: int32(df.EpisodeNumber),
		}}
		_, err = m.storage.CreateEpisode(ctx, episode, storage.EpisodeStateDiscovered)
		if err != nil {
			log.Warnf("failed to create new episode", zap.Error(err))
			continue
		}

		log.Debug("successfully indexed new episode", zap.Int32("ID", episode.ID), zap.Int("number", df.EpisodeNumber), zap.Int64("Season ID", seasonID))
	}

	return nil
}

func (m MediaManager) ensureSeries(ctx context.Context, seriesName string) (int64, error) {
	log := logger.FromCtx(ctx)

	series, err := m.storage.GetSeries(ctx, table.Series.Path.EQ(sqlite.String(seriesName)))
	if errors.Is(err, storage.ErrNotFound) || series == nil {
		log.Debug("episode file does not have associated series, creating new series")
		seriesModel := storage.Series{Series: model.Series{Path: &seriesName, Monitored: 0}}
		seriesID, err := m.storage.CreateSeries(ctx, seriesModel, storage.SeriesStateDiscovered)
		if err != nil {
			return 0, err
		}
		log.Debug("created new discovered series", zap.Int64("series id", seriesID))
		return seriesID, nil
	}
	if err != nil {
		return 0, err
	}

	seriesID := int64(series.ID)
	log.Debug("using existing series", zap.Int64("series id", seriesID))
	return seriesID, nil
}

// getOrCreateSeason unified function to get or create a season, with optional metadata linking
// This prevents the duplicate creation issues between discovery and metadata refresh phases
func (m MediaManager) getOrCreateSeason(ctx context.Context, seriesID int64, seasonNumber int32, seasonMetadataID *int32, initialState storage.SeasonState) (int64, error) {
	log := logger.FromCtx(ctx).With(
		zap.Int64("series_id", seriesID),
		zap.Int32("season_number", seasonNumber),
	)

	// First try to find existing season by series_id + season_number (our unique constraint)
	season, err := m.storage.GetSeason(ctx,
		table.Season.SeriesID.EQ(sqlite.Int64(seriesID)).
			AND(table.Season.SeasonNumber.EQ(sqlite.Int32(seasonNumber))))

	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return 0, err
	}

	if season != nil {
		// Season exists, update metadata link if provided and missing
		if seasonMetadataID != nil && season.SeasonMetadataID == nil {
			err = m.storage.LinkSeasonMetadata(ctx, int64(season.ID), *seasonMetadataID)
			if err != nil {
				log.Error("failed to link season metadata", zap.Error(err))
				return 0, err
			}
			log.Debug("linked existing season to metadata",
				zap.Int64("season_id", int64(season.ID)),
				zap.Int32("season_metadata_id", *seasonMetadataID))
		}
		return int64(season.ID), nil
	}

	// Season doesn't exist, create new one with metadata link if available
	newSeason := storage.Season{
		Season: model.Season{
			SeriesID:         int32(seriesID),
			SeasonNumber:     seasonNumber,
			SeasonMetadataID: seasonMetadataID,
			Monitored:        0,
		},
	}

	seasonID, err := m.storage.CreateSeason(ctx, newSeason, initialState)
	if err != nil {
		return 0, err
	}

	log.Debug("created new season",
		zap.Int64("season_id", seasonID),
		zap.String("initial_state", string(initialState)),
		zap.Any("season_metadata_id", seasonMetadataID))

	return seasonID, nil
}

func modelEpisodeFile(df library.EpisodeFile) model.EpisodeFile {
	return model.EpisodeFile{
		OriginalFilePath: &df.AbsolutePath,
		RelativePath:     &df.RelativePath,
		Size:             df.Size,
	}
}

func matchEpisodeFile(discoveredFile library.EpisodeFile, episodeFiles []*model.EpisodeFile, log *zap.SugaredLogger) (int32, string) {
	for _, ef := range episodeFiles {
		if ef == nil {
			continue
		}

		hasRelativePath := ef.RelativePath != nil && *ef.RelativePath != ""
		if hasRelativePath && strings.EqualFold(*ef.RelativePath, discoveredFile.RelativePath) {
			log.Debug("discovered file relative path matches monitored episode file relative path",
				zap.String("discovered file relative path", discoveredFile.RelativePath),
				zap.String("monitored file relative path", *ef.RelativePath))
			var path string
			if ef.OriginalFilePath != nil {
				path = *ef.OriginalFilePath
			}
			return ef.ID, path
		}

		hasAbsolutePath := ef.OriginalFilePath != nil && *ef.OriginalFilePath != ""
		hasDiscoveredAbsolutePath := discoveredFile.AbsolutePath != ""
		if hasAbsolutePath && hasDiscoveredAbsolutePath && strings.EqualFold(*ef.OriginalFilePath, discoveredFile.AbsolutePath) {
			log.Debug("discovered file absolute path matches monitored episode file original path",
				zap.String("discovered file absolute path", discoveredFile.AbsolutePath),
				zap.String("monitored file original path", *ef.OriginalFilePath))
			return ef.ID, *ef.OriginalFilePath
		}
	}

	return 0, ""
}

func (m *MediaManager) upsertEpisodeFile(ctx context.Context, discoveredFile library.EpisodeFile, matchedID int32, matchedPath string) error {
	log := logger.FromCtx(ctx).With(
		zap.String("relative_path", discoveredFile.RelativePath),
		zap.String("discovered_absolute_path", discoveredFile.AbsolutePath),
		zap.Int32("matched_id", matchedID))

	if matchedID == 0 {
		ef := modelEpisodeFile(discoveredFile)
		log.Debug("creating new episode file", zap.Int64("size", discoveredFile.Size))
		_, err := m.storage.CreateEpisodeFile(ctx, ef)
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

	log.Infow("updating episode file absolute path",
		"old_absolute_path", matchedPath,
		"new_absolute_path", discoveredFile.AbsolutePath)

	existingFile, err := m.storage.GetEpisodeFile(ctx, matchedID)
	if err != nil {
		return fmt.Errorf("failed to get episode file for update: %w", err)
	}

	existingFile.OriginalFilePath = &discoveredFile.AbsolutePath
	err = m.storage.UpdateEpisodeFile(ctx, matchedID, *existingFile)
	if err != nil {
		return fmt.Errorf("failed to update episode file absolute path: %w", err)
	}

	log.Debug("successfully updated episode file absolute path")

	return nil
}
