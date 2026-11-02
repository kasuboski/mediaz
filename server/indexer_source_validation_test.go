package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/manager"
	"github.com/kasuboski/mediaz/pkg/storage"
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
				s.UpdateIndexerSource().ServeHTTP(rr, req)
			}
			require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		})
	}
}
