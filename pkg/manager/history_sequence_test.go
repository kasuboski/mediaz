package manager

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"testing"
	"time"

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
			switch entityType {
			case "movie":
				store.EXPECT().GetEntityTransitionsMovie(ctx, entityID).Return([]sqlcdb.GetEntityTransitionsMovieRow{{
					EntityTitle: title, PosterPath: poster, ToState: "downloading", FromState: from, CreatedAt: created, SortKey: sequence,
				}}, nil)
			case "series":
				store.EXPECT().GetEntityTransitionsSeries(ctx, entityID).Return([]sqlcdb.GetEntityTransitionsSeriesRow{{
					EntityTitle: title, PosterPath: poster, ToState: "downloading", FromState: from, CreatedAt: created, SortKey: sequence,
				}}, nil)
			case "season":
				store.EXPECT().GetEntityTransitionsSeason(ctx, entityID).Return([]sqlcdb.GetEntityTransitionsSeasonRow{{
					EntityTitle: title, PosterPath: poster, ToState: "downloading", FromState: from, CreatedAt: created, SortKey: sequence,
				}}, nil)
			case "episode":
				store.EXPECT().GetEntityTransitionsEpisode(ctx, entityID).Return([]sqlcdb.GetEntityTransitionsEpisodeRow{{
					EntityTitle: title, PosterPath: poster, ToState: "downloading", FromState: from, CreatedAt: created, SortKey: sequence,
				}}, nil)
			case "job":
				store.EXPECT().GetEntityTransitionsJob(ctx, entityID).Return([]sqlcdb.GetEntityTransitionsJobRow{{
					EntityTitle: title.String, ToState: "downloading", FromState: from, CreatedAt: created, SortKey: sequence,
				}}, nil)
			}

			svc := &JobService{activityStorage: store}
			// Test the complete mapped response before wall-clock duration decoration.
			got, err := svc.getEntityTransitions(ctx, entityType, entityID)
			require.NoError(t, err)
			wantPoster := poster.String
			if entityType == "job" {
				wantPoster = ""
			}
			want := &HistoryResponse{
				Entity:  &EntityInfo{Type: entityType, ID: entityID, Title: title.String, PosterPath: wantPoster},
				History: []*HistoryEntry{{SortKey: sequence, ToState: "downloading", FromState: &from.String, CreatedAt: stamp}},
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
