package manager

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/pagination"
	"github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// The generated sequence must remain explicitly int64, even on platforms where
// int happens to be 64 bits. Exercise every generated-row history mapping.
func TestActivityHistorySequenceInt64(t *testing.T) {
	const sequence int64 = math.MaxInt32 + 123
	const entityID int64 = math.MaxInt32 + 456
	ctx := context.Background()
	stamp := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	title := sql.NullString{String: "History", Valid: true}
	poster := sql.NullString{String: "/poster.jpg", Valid: true}
	from := sql.NullString{String: "missing", Valid: true}
	created := sql.NullTime{Time: stamp, Valid: true}

	for _, entityType := range []string{"movie", "series", "season", "episode", "job"} {
		t.Run(entityType, func(t *testing.T) {
			store := mocks.NewMockStorage(gomock.NewController(t))
			params := sqlcdb.GetMovieHistoryPageParams{EntityID: entityID, Offs: 0, Lim: 50}
			switch entityType {
			case "movie":
				store.EXPECT().GetMovieHistoryPage(ctx, params).Return([]sqlcdb.GetMovieHistoryPageRow{{
					EntityTitle: title, PosterPath: poster, ToState: "downloading", FromState: from, CreatedAt: created, SortKey: sequence,
				}}, nil)
				store.EXPECT().CountMovieHistory(ctx, entityID).Return(int64(1), nil)
			case "series":
				store.EXPECT().GetSeriesHistoryPage(ctx, sqlcdb.GetSeriesHistoryPageParams(params)).Return([]sqlcdb.GetSeriesHistoryPageRow{{
					EntityTitle: title, PosterPath: poster, ToState: "downloading", FromState: from, CreatedAt: created, SortKey: sequence,
				}}, nil)
				store.EXPECT().CountSeriesHistory(ctx, entityID).Return(int64(1), nil)
			case "season":
				store.EXPECT().GetSeasonHistoryPage(ctx, sqlcdb.GetSeasonHistoryPageParams(params)).Return([]sqlcdb.GetSeasonHistoryPageRow{{
					EntityTitle: title, PosterPath: poster, ToState: "downloading", FromState: from, CreatedAt: created, SortKey: sequence,
				}}, nil)
				store.EXPECT().CountSeasonHistory(ctx, entityID).Return(int64(1), nil)
			case "episode":
				store.EXPECT().GetEpisodeHistoryPage(ctx, sqlcdb.GetEpisodeHistoryPageParams(params)).Return([]sqlcdb.GetEpisodeHistoryPageRow{{
					EntityTitle: title, PosterPath: poster, ToState: "downloading", FromState: from, CreatedAt: created, SortKey: sequence,
				}}, nil)
				store.EXPECT().CountEpisodeHistory(ctx, entityID).Return(int64(1), nil)
			case "job":
				store.EXPECT().GetJobHistoryPage(ctx, sqlcdb.GetJobHistoryPageParams(params)).Return([]sqlcdb.GetJobHistoryPageRow{{
					EntityTitle: title.String, ToState: "downloading", FromState: from, CreatedAt: created, SortKey: sequence,
				}}, nil)
				store.EXPECT().CountJobHistory(ctx, entityID).Return(int64(1), nil)
			}

			svc := &JobService{activityStorage: store}
			// Test the complete mapped response before wall-clock duration decoration.
			got, err := svc.getEntityTransitions(ctx, entityType, entityID, 0, 50)
			require.NoError(t, err)
			wantPoster := poster.String
			if entityType == "job" {
				wantPoster = ""
			}
			want := &HistoryResponse{
				Entity:     &EntityInfo{Type: entityType, ID: entityID, Title: title.String, PosterPath: wantPoster},
				History:    []*HistoryEntry{{SortKey: sequence, ToState: "downloading", FromState: &from.String, CreatedAt: stamp, Duration: formatDuration(time.Now().UTC().Sub(stamp))}},
				Pagination: pagination.Meta{Page: 1, PageSize: 50, TotalItems: 1, TotalPages: 1},
			}
			require.Equal(t, want, got)
			require.IsType(t, int64(0), got.History[0].SortKey)

			encoded, err := json.Marshal(got)
			require.NoError(t, err)
			var wire struct {
				History []struct {
					SortKey json.RawMessage `json:"sortKey"`
				} `json:"history"`
			}
			require.NoError(t, json.Unmarshal(encoded, &wire))
			require.Len(t, wire.History, 1)
			// Raw token equality proves both the exact value and an unquoted JSON
			// number (without a float64 decoder masking representation changes).
			require.Equal(t, "2147483770", string(wire.History[0].SortKey))
		})
	}
}
