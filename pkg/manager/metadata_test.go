package manager

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/config"
	storeMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	tmdbMocks "github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestMediaManager_GetSeriesMetadata(t *testing.T) {
	t.Run("error getting storage series metadata", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)

		store := storeMocks.NewMockStorage(ctrl)
		store.EXPECT().GetSeriesMetadata(ctx, gomock.Any()).Return(nil, errors.New("expected testing error"))

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		m := MediaManager{
			tmdb:    tmdbMock,
			library: nil,
			storage: store,
			factory: nil,
			configs: config.Manager{},
		}

		details, err := m.GetSeriesMetadata(ctx, 0)
		require.Error(t, err)
		require.Equal(t, "expected testing error", err.Error())

		assert.Nil(t, details)
	})

	t.Run("success getting storage series metadata", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)

		store := newStore(t, ctx)

		_, err := store.CreateSeriesMetadata(ctx, model.SeriesMetadata{
			ID:           1,
			TmdbID:       1234,
			Title:        "Test Series",
			FirstAirDate: ptr(time.Now().Add(-time.Hour * 2)),
		})
		require.NoError(t, err)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		m := MediaManager{
			tmdb:    tmdbMock,
			library: nil,
			storage: store,
			factory: nil,
			configs: config.Manager{},
		}

		details, err := m.GetSeriesMetadata(ctx, 1234)
		require.NoError(t, err)
		require.NotNil(t, details)
		require.Equal(t, "Test Series", details.Title)
		require.Equal(t, int32(1234), details.TmdbID)
		assert.Equal(t, int32(1), details.ID)
	})

	t.Run("success getting series metadata from tdmb", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)

		store := newStore(t, ctx)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbMock.EXPECT().GetSeriesDetails(ctx, 1234).Return(&tmdb.SeriesDetails{
			ID:              1234,
			Name:            "Test Series",
			FirstAirDate:    "2023-01-01",
			NumberOfSeasons: 2,
			Seasons: []tmdb.Season{
				{
					ID:           1,
					Name:         "Test Season 1",
					AirDate:      "2023-01-01",
					SeasonNumber: 1,
					Episodes: []tmdb.Episode{
						{
							ID:            1,
							Name:          "Test Season 1 Episode 1",
							AirDate:       "2023-01-01",
							EpisodeNumber: 1,
							Runtime:       45,
						},
						{
							ID:            2,
							Name:          "Test Season 1 Episode 2",
							AirDate:       "2023-01-02",
							EpisodeNumber: 2,
							Runtime:       42,
						},
					},
				},
				{
					ID:           2,
					Name:         "Test Season 2",
					AirDate:      "2023-01-08",
					SeasonNumber: 2,
					Episodes: []tmdb.Episode{
						{
							ID:            3,
							Name:          "Test Season 2 Episode 1",
							AirDate:       "2023-01-08",
							EpisodeNumber: 1,
							Runtime:       45,
						},
						{
							ID:            4,
							Name:          "Test Season 2 Episode 2",
							AirDate:       "2023-01-09",
							EpisodeNumber: 2,
							Runtime:       43,
						},
					},
				},
			},
		}, nil)

		m := MediaManager{
			tmdb:    tmdbMock,
			library: nil,
			storage: store,
			factory: nil,
			configs: config.Manager{},
		}

		details, err := m.GetSeriesMetadata(ctx, 1234)
		require.Nil(t, err)
		require.NotNil(t, details)
		require.Equal(t, "Test Series", details.Title)
		require.Equal(t, int32(1234), details.TmdbID)
		assert.Equal(t, int32(1), details.ID)

		seasons, err := store.ListSeasonMetadata(ctx)
		require.NoError(t, err)
		require.Len(t, seasons, 2)

		assert.Equal(t, int32(1), seasons[0].ID)
		assert.Equal(t, int32(1), seasons[0].Number)
		assert.Equal(t, "Test Season 1", seasons[0].Title)

		assert.Equal(t, int32(2), seasons[1].ID)
		assert.Equal(t, int32(2), seasons[1].Number)
		assert.Equal(t, "Test Season 2", seasons[1].Title)

		episodes, err := store.ListEpisodeMetadata(ctx)
		require.NoError(t, err)
		require.Len(t, episodes, 4)
		assert.Equal(t, int32(1), episodes[0].ID)
		assert.Equal(t, int32(1), episodes[0].Number)
		assert.Equal(t, "Test Season 1 Episode 1", episodes[0].Title)
		assert.Equal(t, ptr(int32(45)), episodes[0].Runtime)
		assert.Equal(t, int32(2), episodes[1].ID)
		assert.Equal(t, int32(2), episodes[1].Number)
		assert.Equal(t, "Test Season 1 Episode 2", episodes[1].Title)
		assert.Equal(t, ptr(int32(42)), episodes[1].Runtime)
		assert.Equal(t, int32(3), episodes[2].ID)
		assert.Equal(t, int32(1), episodes[2].Number)
		assert.Equal(t, "Test Season 2 Episode 1", episodes[2].Title)
		assert.Equal(t, ptr(int32(45)), episodes[2].Runtime)
		assert.Equal(t, int32(4), episodes[3].ID)
		assert.Equal(t, int32(2), episodes[3].Number)
		assert.Equal(t, "Test Season 2 Episode 2", episodes[3].Title)
		assert.Equal(t, ptr(int32(43)), episodes[3].Runtime)
	})
}

func TestFromSeriesDetails(t *testing.T) {
	tests := []struct {
		name    string
		input   tmdb.SeriesDetails
		want    model.SeriesMetadata
		wantErr bool
	}{
		{
			name: "valid series details",
			input: tmdb.SeriesDetails{
				ID:               123,
				Name:             "Test Series",
				NumberOfSeasons:  2,
				NumberOfEpisodes: 20,
				FirstAirDate:     "2023-01-01",
			},
			want: model.SeriesMetadata{
				TmdbID:       123,
				Title:        "Test Series",
				SeasonCount:  2,
				EpisodeCount: 20,
				FirstAirDate: func() *time.Time {
					t, _ := time.Parse(tmdb.ReleaseDateFormat, "2023-01-01")
					return &t
				}(),
			},
			wantErr: false,
		},
		{
			name: "invalid date format",
			input: tmdb.SeriesDetails{
				ID:               123,
				Name:             "Test Series",
				NumberOfSeasons:  2,
				NumberOfEpisodes: 20,
				FirstAirDate:     "invalid-date",
			},
			want:    model.SeriesMetadata{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromSeriesDetails(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("FromSeriesDetails() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestFromSeriesSeasons(t *testing.T) {
	tests := []struct {
		name  string
		input tmdb.Season
		want  model.SeasonMetadata
	}{
		{
			name: "valid season",
			input: tmdb.Season{
				ID:           123,
				Name:         "Season 1",
				AirDate:      "2023-01-01",
				SeasonNumber: 1,
				Runtime:      45,
				Overview:     "Test overview",
			},
			want: model.SeasonMetadata{
				TmdbID: 123,
				Title:  "Season 1",
				AirDate: func() *time.Time {
					t, _ := time.Parse(tmdb.ReleaseDateFormat, "2023-01-01")
					return &t
				}(),
				Number:   1,
				Overview: func() *string { o := "Test overview"; return &o }(),
			},
		},
		{
			name: "invalid date",
			input: tmdb.Season{
				ID:           123,
				Name:         "Season 1",
				AirDate:      "",
				SeasonNumber: 1,
				Runtime:      45,
				Overview:     "Test overview",
			},
			want: model.SeasonMetadata{
				TmdbID:   123,
				Title:    "Season 1",
				Number:   1,
				Overview: func() *string { o := "Test overview"; return &o }(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromSeriesSeasons(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFromSeriesEpisodes(t *testing.T) {
	tests := []struct {
		name  string
		input tmdb.Episode
		want  model.EpisodeMetadata
	}{
		{
			name: "valid episode",
			input: tmdb.Episode{
				ID:            123,
				Name:          "Test Episode",
				AirDate:       "2023-01-01",
				EpisodeNumber: 1,
				Runtime:       45,
				Overview:      "Test overview",
			},
			want: model.EpisodeMetadata{
				TmdbID: 123,
				Title:  "Test Episode",
				AirDate: func() *time.Time {
					t, _ := time.Parse(tmdb.ReleaseDateFormat, "2023-01-01")
					return &t
				}(),
				Number:   1,
				Runtime:  func() *int32 { r := int32(45); return &r }(),
				Overview: func() *string { o := "Test overview"; return &o }(),
			},
		},
		{
			name: "invalid date",
			input: tmdb.Episode{
				ID:            123,
				Name:          "Test Episode",
				AirDate:       "",
				EpisodeNumber: 1,
				Runtime:       45,
				Overview:      "Test overview",
			},
			want: model.EpisodeMetadata{
				TmdbID:   123,
				Title:    "Test Episode",
				Number:   1,
				Runtime:  func() *int32 { r := int32(45); return &r }(),
				Overview: func() *string { o := "Test overview"; return &o }(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromSeriesEpisodes(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFromMediaDetails(t *testing.T) {
	tests := []struct {
		name string
		det  tmdb.MediaDetails
		want model.MovieMetadata
	}{
		{
			name: "all fields present",
			det: tmdb.MediaDetails{
				ID:                  123,
				Title:               ptr("Title"),
				OriginalTitle:       ptr("Orig"),
				ImdbID:              ptr("tt100"),
				Runtime:             ptr(100),
				Overview:            ptr("Overview"),
				PosterPath:          ptr("path.jpg"),
				Genres:              &[]tmdb.Genre{{Name: "G1"}, {Name: "G2"}},
				Homepage:            ptr("http://"),
				Popularity:          ptr(float32(1.1)),
				ProductionCompanies: &[]tmdb.ProductionCompany{{Name: ptr("Studio")}},
				BelongsToCollection: func() *any {
					v := any(map[string]any{"id": float64(5), "name": "Coll"})
					return &v
				}(),
				ReleaseDate: ptr("2020-02-03"),
			},
			want: model.MovieMetadata{
				TmdbID:           123,
				ImdbID:           ptr("tt100"),
				Images:           "path.jpg",
				Genres:           ptr("G1,G2"),
				Title:            "Title",
				OriginalTitle:    ptr("Orig"),
				Runtime:          100,
				Overview:         ptr("Overview"),
				Website:          ptr("http://"),
				Popularity:       ptr(float64(1.1)),
				Studio:           ptr("Studio"),
				CollectionTmdbID: ptr(int32(5)),
				CollectionTitle:  ptr("Coll"),
				ReleaseDate: func() *time.Time {
					t, _ := time.Parse(tmdb.ReleaseDateFormat, "2020-02-03")
					return &t
				}(),
				Year: func() *int32 {
					y := int32(2020)
					return &y
				}(),
			},
		},
		{
			name: "missing optional fields",
			det: tmdb.MediaDetails{
				ID:       456,
				Title:    ptr("NoOpt"),
				Runtime:  ptr(80),
				Overview: ptr("No optional"),
			},
			want: model.MovieMetadata{
				TmdbID:           456,
				ImdbID:           nil,
				Images:           "",
				Genres:           nil,
				Title:            "NoOpt",
				OriginalTitle:    nil,
				Runtime:          80,
				Overview:         ptr("No optional"),
				Website:          nil,
				Popularity:       nil,
				Studio:           nil,
				CollectionTmdbID: nil,
				CollectionTitle:  nil,
				ReleaseDate:      nil,
				Year:             nil,
			},
		},
		{
			name: "invalid release date",
			det: tmdb.MediaDetails{
				ID:          789,
				Title:       ptr("BadDate"),
				Runtime:     ptr(60),
				Overview:    ptr("Bad date"),
				ReleaseDate: ptr("invalid"),
			},
			want: model.MovieMetadata{
				TmdbID:      789,
				Title:       "BadDate",
				Runtime:     60,
				Overview:    ptr("Bad date"),
				ReleaseDate: nil,
				Year:        nil,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FromMediaDetails(tc.det)
			require.Equal(t, tc.want.TmdbID, got.TmdbID)
			require.Equal(t, tc.want.Images, got.Images)
			require.Equal(t, tc.want.Title, got.Title)
			require.Equal(t, tc.want.Runtime, got.Runtime)

			assert.Equal(t, tc.want.ImdbID, got.ImdbID)
			assert.Equal(t, tc.want.Genres, got.Genres)
			assert.Equal(t, tc.want.OriginalTitle, got.OriginalTitle)
			assert.Equal(t, tc.want.Overview, got.Overview)
			assert.Equal(t, tc.want.Website, got.Website)
			assert.Equal(t, tc.want.Studio, got.Studio)
			assert.Equal(t, tc.want.CollectionTmdbID, got.CollectionTmdbID)
			assert.Equal(t, tc.want.CollectionTitle, got.CollectionTitle)

			if tc.want.Popularity == nil {
				assert.Nil(t, got.Popularity)
			} else {
				require.NotNil(t, got.Popularity)
				assert.InDelta(t, *tc.want.Popularity, *got.Popularity, 0.001)
			}

			if tc.want.ReleaseDate == nil {
				assert.Nil(t, got.ReleaseDate)
			} else {
				require.NotNil(t, got.ReleaseDate)
				assert.True(t, tc.want.ReleaseDate.Equal(*got.ReleaseDate), "Expected ReleaseDate %v, got %v", *tc.want.ReleaseDate, *got.ReleaseDate)
			}

			assert.Equal(t, tc.want.Year, got.Year)
		})
	}
}
