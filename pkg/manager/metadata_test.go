package manager

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	mockTmdb "github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestMediaManager_GetSeriesMetadata(t *testing.T) {
	// t.Run("get series metadata", func(t *testing.T) {
	// 	ctx := context.Background()
	// 	ctrl := gomock.NewController(t)

	// 	store := storeMocks.NewMockStorage(ctrl)
	// 	store.EXPECT().GetSeriesMetadata(ctx, gomock.Any()).Return(nil, storage.ErrNotFound)

	// 	tmdbHttpMock := tmdbMock.NewMockITmdb(ctrl)
	// 	tmdbHttpMock.EXPECT().GetSeriesDetails(gomock.Any(), gomock.Any()).Return(&tmdb.SeriesDetails{
	// 		ID:           1,
	// 		Name:         "Test Series",
	// 		FirstAirDate: "2023-01-01",
	// 	}, nil)

	// 	m := MediaManager{
	// 		tmdb:    tmdbHttpMock,
	// 		library: nil,
	// 		storage: store,
	// 		factory: nil,
	// 		configs: config.Manager{},
	// 	}

	// 	details, err := m.GetSeriesMetadata(ctx, 0)
	// 	require.NoError(t, err)

	// 	require.NotNil(t, details)
	// 	require.Equal(t, "Test Series", details.Title)
	// 	require.Equal(t, int32(1), details.TmdbID)
	// })
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
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FromSeriesDetails() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestFromSeriesSeasons(t *testing.T) {
	tests := []struct {
		name    string
		input   tmdb.Season
		want    model.SeasonMetadata
		wantErr bool
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
				Runtime:  func() *int32 { r := int32(45); return &r }(),
				Overview: func() *string { o := "Test overview"; return &o }(),
			},
			wantErr: false,
		},
		{
			name: "invalid date",
			input: tmdb.Season{
				ID:           123,
				Name:         "Season 1",
				AirDate:      "invalid-date",
				SeasonNumber: 1,
			},
			want:    model.SeasonMetadata{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromSeriesSeasons(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("FromSeriesSeasons() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FromSeriesSeasons() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFromSeriesEpisodes(t *testing.T) {
	tests := []struct {
		name    string
		input   tmdb.Episode
		want    model.EpisodeMetadata
		wantErr bool
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
			wantErr: false,
		},
		{
			name: "invalid date",
			input: tmdb.Episode{
				ID:            123,
				Name:          "Test Episode",
				AirDate:       "invalid-date",
				EpisodeNumber: 1,
			},
			want:    model.EpisodeMetadata{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromSeriesEpisodes(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("FromSeriesEpisodes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FromSeriesEpisodes() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestMediaManager_loadSeriesMetadata(t *testing.T) {
	t.Run("error loading series metadata", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)

		tmdbHttpMock := mockTmdb.NewMockITmdb(ctrl)
		tmdbHttpMock.EXPECT().GetSeriesDetails(gomock.Any(), gomock.Any()).Return(&tmdb.SeriesDetails{
			ID:           1,
			Name:         "Test Series",
			FirstAirDate: "2023-01-01",
		}, nil)

		m := MediaManager{
			tmdb:    tmdbHttpMock,
			library: nil,
			storage: st,
			factory: nil,
			configs: config.Manager{},
		}
		_, err := m.loadSeriesMetadata(ctx, 0)
		require.Error(t, err)
		require.Equal(t, errors.New("no series metadata"), err)
	})
}
