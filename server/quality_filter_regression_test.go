package server

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/require"
)

func TestHTTPQualityProfilesMixedMediaFiltering(t *testing.T) {
	app := newWireAppWithStore(t, newWireStore(t))
	ctx := context.Background()
	const path = "/api/v1/quality/profiles"
	// Keep the seeded profiles intact and verify their responses remain unchanged.
	baseline := make(map[string][]qualityProfileResponse)
	for _, query := range []string{"", "?type=movie", "?type=series"} {
		baseline[query] = httpDatabaseResponse[struct {
			Items      []qualityProfileResponse `json:"items"`
			Pagination struct {
				Page       int `json:"page"`
				PageSize   int `json:"pageSize"`
				TotalItems int `json:"totalItems"`
				TotalPages int `json:"totalPages"`
			} `json:"pagination"`
		}](t, app, http.MethodGet, path+query, "", http.StatusOK).Items
		for _, p := range baseline[query] {
			for _, q := range p.Qualities {
				require.Less(t, q.MinSize, float64(10000))
			}
		}
	}
	definition := func(name, mediaType string, minSize float64) profileQualityResponse {
		id, err := app.store.CreateQualityDefinition(ctx, sqlcdb.QualityDefinition{
			QualityID: 99, Name: name, MediaType: mediaType,
			MinSize: minSize, PreferredSize: minSize + 1, MaxSize: minSize + 2,
		})
		require.NoError(t, err)
		return profileQualityResponse{ID: id, Name: name, MediaType: mediaType, MinSize: minSize, PreferredSize: minSize + 1, MaxSize: minSize + 2}
	}
	movieLow := definition("Movie low", "movie", 20000)
	movieHigh := definition("Movie high", "movie", 30000)
	episodeHigh := definition("Episode high", "episode", 50000)
	movieOther := definition("Movie other", "movie", 40000)
	episodeLow := definition("Episode low", "episode", 10000)
	profile := func(name string, cutoff *int64, qualities ...profileQualityResponse) qualityProfileResponse {
		p := sqlcdb.QualityProfile{Name: name, UpgradeAllowed: cutoff != nil}
		if cutoff != nil {
			p.CutoffQualityID = nullInt64(*cutoff)
		}
		id, err := app.store.CreateQualityProfile(ctx, p)
		require.NoError(t, err)
		for _, q := range qualities {
			_, err = app.store.CreateQualityProfileItem(ctx, sqlcdb.QualityProfileItem{ProfileID: id, QualityID: q.ID})
			require.NoError(t, err)
		}
		return qualityProfileResponse{ID: id, Name: name, CutoffQualityID: cutoff, UpgradeAllowed: p.UpgradeAllowed, Qualities: qualities}
	}
	// Insert items out of size order. The opposite media's maximum must not
	// determine either the filtered profile order or the qualities it contains.
	a := profile("Mixed A", &episodeHigh.ID, movieLow, episodeHigh, movieHigh)
	a.Qualities = []profileQualityResponse{episodeHigh, movieHigh, movieLow}
	b := profile("Mixed B", nil, episodeLow, movieOther)
	b.Qualities = []profileQualityResponse{movieOther, episodeLow}
	empty := profile("Empty", nil)
	movieA, movieB, seriesA, seriesB := a, b, a, b
	movieA.Qualities, movieB.Qualities = []profileQualityResponse{movieHigh, movieLow}, []profileQualityResponse{movieOther}
	seriesA.Qualities, seriesB.Qualities = []profileQualityResponse{episodeHigh}, []profileQualityResponse{episodeLow}
	for _, tc := range []struct {
		name, query string
		profiles    []qualityProfileResponse
	}{
		{"movie", "?type=movie", []qualityProfileResponse{movieB, movieA}},
		{"series maps to episode", "?type=series", []qualityProfileResponse{seriesA, seriesB}},
		{"unfiltered", "", []qualityProfileResponse{a, b}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := append(tc.profiles, baseline[tc.query]...)
			if tc.query == "" {
				want = append(want, empty)
			}
			got := httpDatabaseResponse[struct {
				Items      []qualityProfileResponse `json:"items"`
				Pagination struct {
					Page       int `json:"page"`
					PageSize   int `json:"pageSize"`
					TotalItems int `json:"totalItems"`
					TotalPages int `json:"totalPages"`
				} `json:"pagination"`
			}](t, app, http.MethodGet, path+tc.query, "", http.StatusOK)
			require.Equal(t, want, got.Items)
			require.Equal(t, 1, got.Pagination.Page)
			require.Equal(t, 50, got.Pagination.PageSize)
			require.Equal(t, len(want), got.Pagination.TotalItems)
			require.Equal(t, 1, got.Pagination.TotalPages)
		})
	}
	for _, p := range []qualityProfileResponse{a, b, empty} {
		t.Run(fmt.Sprintf("complete profile %d", p.ID), func(t *testing.T) {
			require.Equal(t, p, httpDatabaseResponse[qualityProfileResponse](t, app, http.MethodGet, fmt.Sprintf("%s/%d", path, p.ID), "", http.StatusOK))
		})
	}
}
