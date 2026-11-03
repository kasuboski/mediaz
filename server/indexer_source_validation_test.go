package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/manager"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	tmdbMocks "github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestIndexerSourceValidationIsBadRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	tmdb := tmdbMocks.NewMockITmdb(ctrl)
	store, err := mediaSqlite.New(context.Background(), ":memory:")
	require.NoError(t, err)
	var storageInterface storage.Storage = store
	mgr := manager.New(tmdb, nil, nil, storageInterface, nil, config.Manager{}, config.Config{})
	s := newTestServer(withManager(mgr))

	for _, tc := range []struct {
		name, method, path, body string
	}{
		{"create", http.MethodPost, "/indexer-sources", `{"name":"bad","implementation":"newznab","scheme":"ftp","host":"example.com"}`},
		{"update", http.MethodPut, "/indexer-sources/1", `{"name":"bad","implementation":"newznab","scheme":"ftp","host":"example.com"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			rr := httptest.NewRecorder()
			if tc.method == http.MethodPost {
				s.CreateIndexerSource().ServeHTTP(rr, req)
			} else {
				seed := sqlcdb.IndexerSource{Name: "existing", Implementation: "newznab", Scheme: "http", Host: "example.com", Enabled: true}
				id, err := store.CreateIndexerSource(context.Background(), seed)
				require.NoError(t, err)
				before, err := store.GetIndexerSource(context.Background(), id)
				require.NoError(t, err)
				req = mux.SetURLVars(req, map[string]string{"id": strconv.FormatInt(id, 10)})
				s.UpdateIndexerSource().ServeHTTP(rr, req)
				after, err := store.GetIndexerSource(context.Background(), id)
				require.NoError(t, err)
				require.Equal(t, before, after)
			}
			require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
			require.Contains(t, rr.Body.String(), "scheme must be http or https")
		})
	}
}
