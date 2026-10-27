package manager

import (
	"database/sql"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
)

func TestJobResponseGeneratedRows(t *testing.T) {
	createdAt := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Minute)
	for _, tt := range []struct {
		name      string
		err       sql.NullString
		wantError *string
	}{
		{name: "null error"},
		{name: "invalid value is ignored", err: sql.NullString{String: "stale value"}},
		{name: "empty error", err: sql.NullString{Valid: true}, wantError: new(string)},
		{name: "error", err: sql.NullString{String: "failed", Valid: true}, wantError: func() *string { s := "failed"; return &s }()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			row := sqlcdb.GetJobByIDRow{
				Job:       sqlcdb.Job{ID: 1 << 40, Type: string(MovieIndex), CreatedAt: sql.NullTime{Time: createdAt, Valid: true}},
				State:     "error",
				UpdatedAt: sql.NullTime{Time: updatedAt, Valid: true},
				Error:     tt.err,
			}
			want := JobResponse{ID: 1 << 40, Type: string(MovieIndex), State: "error", CreatedAt: createdAt, UpdatedAt: updatedAt, Error: tt.wantError}
			assert.Equal(t, want, toJobResponse(row))
			assert.Equal(t, want, toJobListResponse(sqlcdb.ListJobsRow(row)))
		})
	}
}
