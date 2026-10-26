package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/manager"
	storeMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestHTTPMovieQualityIDsAboveMaxInt32(t *testing.T) {
	// Exercise URL parsing, scalar JSON input, the real manager service, storage
	// call arguments, DTO mapping and JSON encoding without narrowing either ID.
	const movieID int64 = 1<<32 + 17
	const profileID int64 = 1<<32 + 23
	ctrl := gomock.NewController(t)
	store := storeMocks.NewMockStorage(ctrl)
	store.EXPECT().UpdateMovieQualityProfile(gomock.Any(), movieID, profileID).Return(nil)
	store.EXPECT().GetMovie(gomock.Any(), movieID).Return(sqlcdb.GetMovieByIDRow{Movie: sqlcdb.Movie{ID: movieID, QualityProfileID: profileID}, State: "missing"}, nil)
	mgr := manager.New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
	s := newTestServer(withManager(mgr))
	router := mux.NewRouter()
	s.registerRoutes(router)
	request := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/library/movies/%d/quality", movieID), strings.NewReader(fmt.Sprintf(`{"qualityProfileId":%d}`, profileID)))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var body struct {
		Response movieResponse `json:"response"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Equal(t, movieResponse{ID: movieID, QualityProfileID: profileID, State: "missing"}, body.Response)
	assertNoSQLNullLeak(t, response.Body.String())
}
