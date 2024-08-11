package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestServer_Healthz(t *testing.T) {
	t.Run("healthz", func(t *testing.T) {
		s := Server{baseLogger: zap.NewNop().Sugar()}

		req, err := http.NewRequest("GET", "/healthz", nil)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.Healthz()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)

		assert.NoError(t, err)
		assert.Equal(t, "ok", response.Response)
	})
}

func TestNew(t *testing.T) {
	type args struct {
		tmbdClient     TMDBClientInterface
		prowlarrClient ProwlarrClientInterface
		library        library.Library
		logger         *zap.SugaredLogger
	}
	tests := []struct {
		name string
		args args
		want Server
	}{
		{
			name: "new server",
			args: args{
				tmbdClient: &tmdb.Client{
					Server: "my-tmdb-server",
				},
				prowlarrClient: &prowlarr.Client{
					Server: "my-prowlarr-server",
				},
				library: library.Library{},
				logger:  zap.NewNop().Sugar(),
			},
			want: Server{
				tmdb: &tmdb.Client{
					Server: "my-tmdb-server",
				},
				prowlarr: &prowlarr.Client{
					Server: "my-prowlarr-server",
				},
				library:    library.Library{},
				baseLogger: zap.NewNop().Sugar(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := New(tt.args.tmbdClient, tt.args.prowlarrClient, tt.args.library, tt.args.logger); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New() = %v, want %v", got, tt.want)
			}
		})
	}
}
