package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/mux"
	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/manager"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	tmdbMocks "github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestDiscoveryPaginationInvalidPages(t *testing.T) {
	for _, endpoint := range []string{"movie", "tv"} {
		for _, page := range []string{"", "0", "-1", "1.5", "abc", "2147483648", "1&page=2"} {
			t.Run(endpoint+"/page="+page, func(t *testing.T) {
				s := newTestServer() // A parse failure must return before touching the nil manager.
				req := httptest.NewRequest(http.MethodGet, "/api/v1/discover/"+endpoint+"?query=hello&page="+page, nil)
				rr := httptest.NewRecorder()
				handler := s.SearchMovie()
				if endpoint == "tv" {
					handler = s.SearchTV()
				}
				handler.ServeHTTP(rr, req)
				require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
			})
		}
	}
}

func TestDiscoveryPaginationPassthrough(t *testing.T) {
	for _, endpoint := range []string{"movie", "tv"} {
		for _, tc := range []struct {
			name, page string
			wantPage   int32
		}{{name: "default", wantPage: 1}, {name: "explicit page two", page: "2", wantPage: 2}} {
			t.Run(endpoint+"/"+tc.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				stub := tmdbMocks.NewMockITmdb(ctrl)
				store := newWireStore(t)
				cfg := config.Config{}
				mgr := manager.New(stub, nil, nil, store, nil, cfg.Manager, cfg)
				srv := New(zap.NewNop().Sugar(), mgr, cfg.Server)
				router := mux.NewRouter()
				srv.registerRoutes(router)

				query := "Dune & Spice+"
				page := tc.wantPage
				body := fmt.Sprintf(`{"page":%d,"total_pages":8,"total_results":42,"results":[{"id":123}]}`, page)
				response := httptestJSONResponse(http.StatusOK, body)
				if endpoint == "movie" {
					stub.EXPECT().SearchMovie(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *tmdb.SearchMovieParams, _ ...tmdb.RequestEditorFn) (*http.Response, error) {
						require.Equal(t, query, p.Query)
						require.NotNil(t, p.Page)
						require.Equal(t, tc.wantPage, *p.Page)
						return response, nil
					})
				} else {
					stub.EXPECT().SearchTv(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *tmdb.SearchTvParams, _ ...tmdb.RequestEditorFn) (*http.Response, error) {
						require.Equal(t, query, p.Query)
						require.NotNil(t, p.Page)
						require.Equal(t, tc.wantPage, *p.Page)
						return response, nil
					})
				}

				path := "/api/v1/discover/" + endpoint + "?query=" + url.QueryEscape(query)
				if tc.page != "" {
					path += "&page=" + tc.page
				}
				rr := httptest.NewRecorder()
				router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
				require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
				var got GenericResponse
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
				encoded, err := json.Marshal(got.Response)
				require.NoError(t, err)
				require.JSONEq(t, body, string(encoded))
			})
		}
	}
}
