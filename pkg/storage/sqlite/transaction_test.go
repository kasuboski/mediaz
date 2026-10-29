package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLite_WithTxCanceledBeginSkipsCallback(t *testing.T) {
	s := sqlcIntegrationStore(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	called := false
	err := s.withTx(ctx, func(*sqlcdb.Queries) error {
		called = true
		return nil
	})
	require.ErrorIs(t, err, context.Canceled)
	assert.False(t, called)
}

func TestSQLite_WithTxPanicRollsBackAndReleasesConnection(t *testing.T) {
	s := sqlcIntegrationStore(t)
	seriesID := tvTestSeries(t, t.Context(), s)
	params := sqlcdb.CreateSeasonParams{SeriesID: seriesID, SeasonNumber: 1, Monitored: 1}
	assert.PanicsWithValue(t, "transaction panic", func() {
		_ = s.withTx(t.Context(), func(q *sqlcdb.Queries) error {
			_, err := q.CreateSeason(t.Context(), params)
			require.NoError(t, err)
			panic("transaction panic")
		})
	})

	// Bound followups so a leaked transaction holding the only connection fails
	// promptly rather than hanging the suite.
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	var count int
	require.NoError(t, s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM season`).Scan(&count))
	assert.Zero(t, count)

	ctx, cancel = context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	var id int64
	require.NoError(t, s.withTx(ctx, func(q *sqlcdb.Queries) error {
		var err error
		id, err = q.CreateSeason(ctx, params)
		return err
	}))
	got, err := s.GetSeasonByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, sqlcdb.GetSeasonByIDRow{Season: sqlcdb.Season{ID: id, SeriesID: seriesID, SeasonNumber: 1, Monitored: 1}}, got)
}
