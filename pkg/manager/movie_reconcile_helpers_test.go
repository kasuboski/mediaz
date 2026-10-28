package manager

import (
	"database/sql"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/stretchr/testify/assert"
)

func Test_initialMovieState(t *testing.T) {
	tests := []struct {
		name        string
		releaseDate sql.NullTime
		want        storage.MovieState
	}{
		{name: "null", want: storage.MovieStateUnreleased},
		{name: "invalid with past value", releaseDate: sql.NullTime{Time: time.Now().Add(-time.Hour)}, want: storage.MovieStateUnreleased},
		{name: "zero date", releaseDate: sql.NullTime{Valid: true}, want: storage.MovieStateUnreleased},
		{name: "past", releaseDate: sql.NullTime{Time: time.Now().Add(-time.Hour), Valid: true}, want: storage.MovieStateMissing},
		{name: "future", releaseDate: sql.NullTime{Time: time.Now().Add(time.Hour), Valid: true}, want: storage.MovieStateUnreleased},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, initialMovieState(tt.releaseDate))
		})
	}
}
