package manager

import (
	"context"
	"errors"
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
	log := logger.FromCtx(ctx)

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
		isTracked := false
		for _, ef := range episodeFiles {
			if ef == nil {
				continue
			}
			if ef.RelativePath != nil && strings.EqualFold(*ef.RelativePath, discoveredFile.RelativePath) {
				log.Debug("discovered file relative path matches monitored episode file relative path",
					zap.String("discovered file relative path", discoveredFile.RelativePath),
					zap.String("monitored file relative path", *ef.RelativePath))
				isTracked = true
				break
			}
			if ef.OriginalFilePath != nil && strings.EqualFold(*ef.OriginalFilePath, discoveredFile.AbsolutePath) {
				log.Debug("discovered file absolute path matches monitored episode file original path",
					zap.String("discovered file absolute path", discoveredFile.RelativePath),
					zap.String("monitored file original path", *ef.RelativePath))
				isTracked = true
				break
			}
		}

		if isTracked {
			continue
		}

		ef := modelEpisodeFile(discoveredFile)
		log.Debug("discovered new episode file", zap.String("path", discoveredFile.RelativePath))
		_, err := m.storage.CreateEpisodeFile(ctx, ef)
		if err != nil {
			log.Errorf("couldn't store episode file: %w", err)
			continue
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

		seriesName := library.MovieNameFromFilepath(*f.RelativePath)
		_ = seriesName

		// rely on discovered EpisodeFile data; library provides series/season parsing
		var df library.EpisodeFile
		for _, d := range discoveredFiles {
			if d.RelativePath == *f.RelativePath {
				df = d
				break
			}
		}
		if df.SeriesName == "" {
			continue
		}

		series, err := m.storage.GetSeries(ctx, table.Series.Path.EQ(sqlite.String(df.SeriesName)))
		if err == nil && series != nil {
			log.Debug("episode file associated with series already", zap.Any("series id", series.ID))
			continue
		}
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			log.Debug("error fetching series", zap.Error(err))
			continue
		}

		log.Debug("episode file does not have associated series")

		seriesModel := storage.Series{Series: model.Series{Path: &df.SeriesName, Monitored: 1}}
		seriesID, err := m.storage.CreateSeries(ctx, seriesModel, storage.SeriesStateDiscovered)
		if err != nil {
			log.Errorf("couldn't create new series for discovered file: %w", err)
			continue
		}
		log.Debug("created new discovered series", zap.Int64("series id", seriesID))

		// ensure season exists for the specific season number parsed from path
		season, err := m.storage.GetSeason(ctx, table.Season.SeriesID.EQ(sqlite.Int64(seriesID)).AND(table.Season.SeasonMetadataID.IS_NOT_NULL()))
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			log.Debug("failed to get season", zap.Error(err))
			continue
		}
		var seasonID int64
		if season == nil || errors.Is(err, storage.ErrNotFound) {
			created, cerr := m.storage.CreateSeason(ctx, storage.Season{Season: model.Season{SeriesID: int32(seriesID), Monitored: 1}}, storage.SeasonStateDiscovered)
			if cerr != nil {
				log.Debug("failed to create season", zap.Error(cerr))
				continue
			}
			seasonID = created
		} else {
			seasonID = int64(season.ID)
		}

		// ensure episode exists; we don't parse episode number here, just create placeholder
		episode := storage.Episode{Episode: model.Episode{SeasonID: int32(seasonID), Monitored: 1}}
		_, _ = m.storage.CreateEpisode(ctx, episode, storage.EpisodeStateDiscovered)
	}

	return nil
}

func modelEpisodeFile(df library.EpisodeFile) model.EpisodeFile {
	return model.EpisodeFile{
		OriginalFilePath: &df.AbsolutePath,
		RelativePath:     &df.RelativePath,
		Size:             df.Size,
	}
}

func modelSeriesFromMeta(meta *model.SeriesMetadata) model.Series {
	return model.Series{
		SeriesMetadataID: &meta.ID,
		QualityProfileID: 0,
		Monitored:        1,
		TmdbID:           int32(meta.TmdbID),
		Path:             &meta.Title,
	}
}
