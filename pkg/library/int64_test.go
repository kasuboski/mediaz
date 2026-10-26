package library

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	mediaio "github.com/kasuboski/mediaz/pkg/io"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddEpisodePreservesInt64Season(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "episode.mkv")
	require.NoError(t, os.WriteFile(source, []byte("episode data"), 0600))
	tvRoot := filepath.Join(root, "tv")
	lib := New(FileSystem{}, FileSystem{Path: tvRoot}, &mediaio.MediaFileSystem{}, true)

	got, err := lib.AddEpisode(context.Background(), "Show", int64(math.MaxInt32)+1, source)
	require.NoError(t, err)
	want := EpisodeFile{
		Name: "episode.mkv", Size: 12,
		RelativePath: "Show/Season 2147483648/episode.mkv",
		AbsolutePath: filepath.Join(tvRoot, "Show", "Season 2147483648", "episode.mkv"),
	}
	assert.Equal(t, want, got)
	sourceInfo, err := os.Stat(source)
	require.NoError(t, err)
	targetInfo, err := os.Stat(want.AbsolutePath)
	require.NoError(t, err)
	assert.True(t, os.SameFile(sourceInfo, targetInfo), "library import should preserve the seeding hardlink")
}
