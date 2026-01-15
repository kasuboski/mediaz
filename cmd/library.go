package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/storage"
	sqliteStorage "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var generateTestlibCmd = &cobra.Command{
	Use:   "testlib",
	Short: "Generate test library based on current database state",
	Long: `Generate a test media library with empty files based on the current database state.
	
This command reads the existing series, seasons, episodes, and movies from the database
and creates corresponding empty media files with proper naming patterns. This allows
you to test reconciliation logic without large media files.

The generated library will include:
- TV series with season/episode structure matching your database
- Movies with various naming patterns
- Empty files with valid media headers (MP4/MKV)
- README documentation of the generated content

Example:
  mediaz generate testlib ./testlib
  mediaz generate testlib /path/to/test/library`,
	Args: cobra.MaximumNArgs(1),
	Run:  runGenerateTestlib,
}

func init() {
	generateCmd.AddCommand(generateTestlibCmd)
	generateTestlibCmd.Flags().Bool("overwrite", false, "Overwrite existing test library directory")
	generateTestlibCmd.Flags().Bool("movies-only", false, "Generate only movies")
	generateTestlibCmd.Flags().Bool("tv-only", false, "Generate only TV shows")
	generateTestlibCmd.Flags().String("db-path", "", "Path to database file (default: from config)")
}

func runGenerateTestlib(cmd *cobra.Command, args []string) {
	ctx := context.Background()
	log := logger.Get()

	// Determine output directory
	outputDir := "./testlib"
	if len(args) > 0 {
		outputDir = args[0]
	}

	// Check flags
	overwrite, _ := cmd.Flags().GetBool("overwrite")
	moviesOnly, _ := cmd.Flags().GetBool("movies-only")
	tvOnly, _ := cmd.Flags().GetBool("tv-only")
	dbPath, _ := cmd.Flags().GetString("db-path")

	// Use database path from config if not specified
	if dbPath == "" {
		dbPath = viper.GetString("storage.filePath")
		if dbPath == "" {
			log.Fatal("database path not found in config or flags")
		}
	}

	// Check if output directory exists
	if _, err := os.Stat(outputDir); err == nil && !overwrite {
		log.Fatalf("Output directory %s already exists. Use --overwrite to replace it.", outputDir)
	}

	log.Infow("generating test library from database",
		"db_path", dbPath,
		"output_dir", outputDir,
		"movies_only", moviesOnly,
		"tv_only", tvOnly)

	// Initialize storage
	store, err := sqliteStorage.New(ctx, dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	// Create output directory structure
	if err := os.RemoveAll(outputDir); err != nil && !os.IsNotExist(err) {
		log.Fatalf("failed to remove existing directory: %v", err)
	}

	moviesDir := filepath.Join(outputDir, "movies")
	tvDir := filepath.Join(outputDir, "tv")

	if !tvOnly {
		if err := os.MkdirAll(moviesDir, 0755); err != nil {
			log.Fatalf("failed to create movies directory: %v", err)
		}
	}

	if !moviesOnly {
		if err := os.MkdirAll(tvDir, 0755); err != nil {
			log.Fatalf("failed to create tv directory: %v", err)
		}
	}

	var movieCount, seriesCount, seasonCount, episodeCount int

	// Generate movies
	if !tvOnly {
		movieCount, err = generateMoviesFromDB(ctx, store, moviesDir, log)
		if err != nil {
			log.Fatalf("failed to generate movies: %v", err)
		}
	}

	// Generate TV shows
	if !moviesOnly {
		seriesCount, seasonCount, episodeCount, err = generateTVFromDB(ctx, store, tvDir, log)
		if err != nil {
			log.Fatalf("failed to generate TV shows: %v", err)
		}
	}

	// Generate README
	readmeContent := generateReadmeFromCounts(movieCount, seriesCount, seasonCount, episodeCount)
	readmePath := filepath.Join(outputDir, "README.md")
	if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
		log.Warnf("failed to create README: %v", err)
	}

	log.Infow("test library generation complete",
		"output_dir", outputDir,
		"movies", movieCount,
		"series", seriesCount,
		"seasons", seasonCount,
		"episodes", episodeCount)

	fmt.Printf("✅ Test library generated at: %s\n", outputDir)
	if !moviesOnly {
		fmt.Printf("📺 TV: %d series, %d seasons, %d episodes\n", seriesCount, seasonCount, episodeCount)
	}
	if !tvOnly {
		fmt.Printf("🎬 Movies: %d titles\n", movieCount)
	}
	fmt.Printf("\nTo test with this library:\n")
	fmt.Printf("  export MEDIAZ_LIBRARY_MOVIE=\"%s\"\n", moviesDir)
	fmt.Printf("  export MEDIAZ_LIBRARY_TV=\"%s\"\n", tvDir)
	fmt.Printf("  ./mediaz discover\n")
}

func generateMoviesFromDB(ctx context.Context, store storage.Storage, outputDir string, log *zap.SugaredLogger) (int, error) {
	// Get the movie library path to scan for actual files
	movieLibraryPath := viper.GetString("library.movie")
	if movieLibraryPath == "" {
		log.Warn("No movie library path configured, falling back to generated filenames")
		return generateMoviesWithGeneratedNames(ctx, store, outputDir, log)
	}

	// Scan the actual movie library for files
	actualFiles, err := scanMovieLibrary(movieLibraryPath, log)
	if err != nil {
		log.Warnf("Failed to scan movie library at %s: %v. Falling back to generated filenames", movieLibraryPath, err)
		return generateMoviesWithGeneratedNames(ctx, store, outputDir, log)
	}

	if len(actualFiles) == 0 {
		log.Warn("No movie files found in library, falling back to generated filenames")
		return generateMoviesWithGeneratedNames(ctx, store, outputDir, log)
	}

	// Get all movies from database
	movies, err := store.ListMovies(ctx)
	if err != nil && err != storage.ErrNotFound {
		return 0, fmt.Errorf("failed to list movies: %w", err)
	}

	count := 0
	fileIndex := 0

	for range movies {
		if fileIndex >= len(actualFiles) {
			fileIndex = 0 // Wrap around if we have more database entries than files
		}

		actualFile := actualFiles[fileIndex]
		fileIndex++

		// Create the target path in output directory
		targetPath := filepath.Join(outputDir, actualFile.RelativePath)

		// Create parent directories if needed
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			log.Warnf("failed to create directory for %s: %v", targetPath, err)
			continue
		}

		if err := createEmptyMediaFile(targetPath, actualFile.Extension); err != nil {
			log.Warnf("failed to create movie file %s: %v", targetPath, err)
			continue
		}

		count++
		if count%10 == 0 {
			log.Infof("created %d movie files...", count)
		}
	}

	log.Infof("Used %d actual library files as templates for %d database movies", len(actualFiles), count)
	return count, nil
}

// MovieFile represents a movie file found in the library
type MovieFile struct {
	RelativePath string // Path relative to library root (e.g., "28 Days Later/28 Days Later (2002).mp4")
	Extension    string // File extension (e.g., ".mp4")
	IsInFolder   bool   // Whether the file is in its own folder
}

// scanMovieLibrary scans the movie library and returns all movie files
func scanMovieLibrary(libraryPath string, log *zap.SugaredLogger) ([]MovieFile, error) {
	var files []MovieFile

	err := filepath.Walk(libraryPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Check if it's a movie file
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".mp4" && ext != ".mkv" && ext != ".avi" && ext != ".mov" {
			return nil
		}

		// Get relative path from library root
		relPath, err := filepath.Rel(libraryPath, path)
		if err != nil {
			return err
		}

		// Determine if file is in its own folder
		isInFolder := strings.Contains(relPath, string(os.PathSeparator))

		files = append(files, MovieFile{
			RelativePath: relPath,
			Extension:    ext,
			IsInFolder:   isInFolder,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	log.Infof("Found %d movie files in library at %s", len(files), libraryPath)
	return files, nil
}

// generateMoviesWithGeneratedNames is the fallback function using generated names
func generateMoviesWithGeneratedNames(ctx context.Context, store storage.Storage, outputDir string, log *zap.SugaredLogger) (int, error) {
	// Get all movies with metadata
	movies, err := store.ListMovies(ctx)
	if err != nil && err != storage.ErrNotFound {
		return 0, fmt.Errorf("failed to list movies: %w", err)
	}

	count := 0
	for _, movie := range movies {
		// Get movie metadata if available
		var title string
		var year int

		if movie.MovieMetadataID != nil {
			metadata, err := store.GetMovieMetadata(ctx, table.MovieMetadata.ID.EQ(sqlite.Int32(*movie.MovieMetadataID)))
			if err == nil {
				title = metadata.Title
				if metadata.ReleaseDate != nil {
					year = metadata.ReleaseDate.Year()
				}
			}
		}

		// Fallback to movie path if no metadata
		if title == "" {
			if movie.Path != nil {
				title = extractTitleFromPath(*movie.Path)
			} else {
				title = fmt.Sprintf("Movie %d", movie.ID)
			}
		}

		// Generate filename with various naming patterns for variety
		var filename string
		switch count % 4 {
		case 0:
			if year > 0 {
				filename = fmt.Sprintf("%s (%d).mp4", title, year)
			} else {
				filename = fmt.Sprintf("%s.mp4", title)
			}
		case 1:
			if year > 0 {
				filename = fmt.Sprintf("%s.%d.1080p.BluRay.x264.mp4", strings.ReplaceAll(title, " ", "."), year)
			} else {
				filename = fmt.Sprintf("%s.1080p.BluRay.x264.mp4", strings.ReplaceAll(title, " ", "."))
			}
		case 2:
			if year > 0 {
				filename = fmt.Sprintf("%s (%d) [1080p].mkv", title, year)
			} else {
				filename = fmt.Sprintf("%s [1080p].mkv", title)
			}
		case 3:
			if year > 0 {
				filename = fmt.Sprintf("%s %d 4K UHD.mkv", title, year)
			} else {
				filename = fmt.Sprintf("%s 4K UHD.mkv", title)
			}
		}

		// Clean filename
		filename = cleanFilename(filename)

		// Create movie directory (e.g., "Movie Title (2020)/")
		dirName := title
		if year > 0 {
			dirName = fmt.Sprintf("%s (%d)", title, year)
		}
		dirName = cleanFilename(dirName)
		movieDir := filepath.Join(outputDir, dirName)
		if err := os.MkdirAll(movieDir, 0755); err != nil {
			log.Warnf("failed to create movie directory %s: %v", movieDir, err)
			continue
		}

		filePath := filepath.Join(movieDir, filename)

		if err := createEmptyMediaFile(filePath, filepath.Ext(filename)); err != nil {
			log.Warnf("failed to create movie file %s: %v", filename, err)
			continue
		}

		count++
		if count%10 == 0 {
			log.Infof("created %d movie files...", count)
		}
	}

	return count, nil
}

func generateTVFromDB(ctx context.Context, store storage.Storage, outputDir string, log *zap.SugaredLogger) (int, int, int, error) {
	// Get all series
	allSeries, err := store.ListSeries(ctx, nil)
	if err != nil && err != storage.ErrNotFound {
		return 0, 0, 0, fmt.Errorf("failed to list series: %w", err)
	}

	seriesCount := 0
	totalSeasonCount := 0
	totalEpisodeCount := 0

	for _, series := range allSeries {
		// Get series title
		var seriesTitle string
		if series.SeriesMetadataID != nil {
			metadata, err := store.GetSeriesMetadata(ctx, table.SeriesMetadata.ID.EQ(sqlite.Int32(*series.SeriesMetadataID)))
			if err == nil {
				seriesTitle = metadata.Title
			}
		}

		// Fallback to path-based title
		if seriesTitle == "" {
			if series.Path != nil {
				seriesTitle = extractTitleFromPath(*series.Path)
			} else {
				seriesTitle = fmt.Sprintf("Series %d", series.ID)
			}
		}

		// Get seasons for this series
		seasons, err := store.ListSeasons(ctx, table.Season.SeriesID.EQ(sqlite.Int32(series.ID)))
		if err != nil && err != storage.ErrNotFound {
			log.Warnf("failed to list seasons for series %s: %v", seriesTitle, err)
			continue
		}

		if len(seasons) == 0 {
			continue // Skip series with no seasons
		}

		seriesCount++
		seasonCount := 0

		for _, season := range seasons {
			// Create season directory
			seasonDir := filepath.Join(outputDir, cleanFilename(seriesTitle), fmt.Sprintf("Season %d", season.SeasonNumber))
			if err := os.MkdirAll(seasonDir, 0755); err != nil {
				log.Warnf("failed to create season directory %s: %v", seasonDir, err)
				continue
			}

			// Get episodes for this season
			episodes, err := store.ListEpisodes(ctx, table.Episode.SeasonID.EQ(sqlite.Int32(season.ID)))
			if err != nil && err != storage.ErrNotFound {
				log.Warnf("failed to list episodes for season %s S%d: %v", seriesTitle, season.SeasonNumber, err)
				continue
			}

			seasonCount++
			episodeCount := 0

			for _, episode := range episodes {
				// Get episode title from metadata if available
				var episodeTitle string
				if episode.EpisodeMetadataID != nil {
					metadata, err := store.GetEpisodeMetadata(ctx, table.EpisodeMetadata.ID.EQ(sqlite.Int32(*episode.EpisodeMetadataID)))
					if err == nil {
						episodeTitle = metadata.Title
					}
				}

				if episodeTitle == "" {
					episodeTitle = fmt.Sprintf("Episode %d", episode.EpisodeNumber)
				}

				// Generate episode filename with different patterns
				var filename string
				switch seriesCount % 3 {
				case 0:
					filename = fmt.Sprintf("%s S%02dE%02d - %s.mp4",
						seriesTitle, season.SeasonNumber, episode.EpisodeNumber, episodeTitle)
				case 1:
					filename = fmt.Sprintf("%s.S%02dE%02d.1080p.WEBRip.x264.mp4",
						strings.ReplaceAll(seriesTitle, " ", "."), season.SeasonNumber, episode.EpisodeNumber)
				case 2:
					filename = fmt.Sprintf("%s S%02dE%02d (%dp).mkv",
						seriesTitle, season.SeasonNumber, episode.EpisodeNumber, 1080)
				}

				filename = cleanFilename(filename)
				filePath := filepath.Join(seasonDir, filename)

				if err := createEmptyMediaFile(filePath, filepath.Ext(filename)); err != nil {
					log.Warnf("failed to create episode file %s: %v", filename, err)
					continue
				}

				episodeCount++
				totalEpisodeCount++
			}

			if episodeCount > 0 {
				totalSeasonCount++
			}
		}

		if seriesCount%5 == 0 {
			log.Infof("created %d series...", seriesCount)
		}
	}

	return seriesCount, totalSeasonCount, totalEpisodeCount, nil
}

func extractTitleFromPath(path string) string {
	// Extract title from file path
	base := filepath.Base(path)

	// Remove file extension
	name := strings.TrimSuffix(base, filepath.Ext(base))

	// Simple cleanup for common patterns
	name = strings.TrimSpace(name)
	if strings.Contains(name, ".") {
		name = strings.ReplaceAll(name, ".", " ")
	}

	// Clean up extra spaces
	words := strings.Fields(name)
	return strings.Join(words, " ")
}

func cleanFilename(name string) string {
	// Replace invalid filename characters
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	clean := name

	for _, char := range invalid {
		clean = strings.ReplaceAll(clean, char, "-")
	}

	return clean
}

func createEmptyMediaFile(path, ext string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write minimal valid media file headers
	switch ext {
	case ".mp4":
		// Minimal MP4 header (ftyp box)
		_, err = file.Write([]byte{
			0x00, 0x00, 0x00, 0x20, // Box size (32 bytes)
			0x66, 0x74, 0x79, 0x70, // 'ftyp'
			0x69, 0x73, 0x6F, 0x6D, // 'isom' major brand
			0x00, 0x00, 0x02, 0x00, // Minor version
			0x69, 0x73, 0x6F, 0x6D, // 'isom' compatible brand
			0x69, 0x73, 0x6F, 0x32, // 'iso2' compatible brand
			0x61, 0x76, 0x63, 0x31, // 'avc1' compatible brand
			0x6D, 0x70, 0x34, 0x31, // 'mp41' compatible brand
		})
	case ".mkv":
		// Minimal MKV/WebM header (EBML)
		_, err = file.Write([]byte{
			0x1A, 0x45, 0xDF, 0xA3, // EBML signature
			0x93, 0x42, 0x82, 0x88, // DocType = "matroska"
			0x6D, 0x61, 0x74, 0x72, 0x6F, 0x73, 0x6B, 0x61,
		})
	default:
		// Generic media file marker
		_, err = file.Write([]byte("MEDIAZ_TEST_FILE_" + ext))
	}

	return err
}

func generateReadmeFromCounts(movies, series, seasons, episodes int) string {
	return fmt.Sprintf(`# Generated Test Library

This test library was generated from the current database state to enable testing of reconciliation logic without large media files.

## 📊 Library Statistics

- **Movies**: %d titles
- **TV Series**: %d shows  
- **Seasons**: %d total
- **Episodes**: %d total

## 📁 Structure

The library is organized as follows:
- movies/ - Movie files with various naming patterns
- tv/[Series Name]/Season N/ - TV episodes organized by series and season

## 🧪 Usage

1. **Set library paths**:
   export MEDIAZ_LIBRARY_MOVIE="$(pwd)/movies"
   export MEDIAZ_LIBRARY_TV="$(pwd)/tv"

2. **Run discovery**:
   ./mediaz discover

3. **Start reconciliation**:
   ./mediaz serve

4. **Test database integrity**:
   go test -tags functional ./pkg/manager/ -v

## 📝 Notes

- All files are empty with valid media headers (MP4/MKV)
- Filenames use various patterns to test parser robustness  
- Structure matches the current database state
- Files can be safely deleted after testing

## 🔄 Regeneration

To regenerate this library with updated database content:
./mediaz generate testlib --overwrite ./path/to/testlib

---
Generated by: mediaz generate testlib
`, movies, series, seasons, episodes)
}
