package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/manager"
	tmdbMocks "github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newQualityManager(t *testing.T) manager.MediaManager {
	t.Helper()
	ctrl := gomockController(t)
	tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
	store := newInMemoryStore(t)
	return manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})
}

// itoa converts an int32 or int to string for URL paths.
func itoa[T ~int32 | ~int | ~int64](v T) string {
	return strconv.FormatInt(int64(v), 10)
}

// --- Quality Definition tests ---

func TestServer_ListQualityDefinitions(t *testing.T) {
	t.Run("success - returns default definitions", func(t *testing.T) {
		mgr := newQualityManager(t)

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("GET", "/quality/definitions", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.ListQualityDefinitions()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		defList, ok := response.Response.([]any)
		require.True(t, ok, "Response should be an array")
		// Default schema seeds quality definitions
		assert.NotEmpty(t, defList)

		// Verify first definition has all expected fields
		first := defList[0].(map[string]any)
		assert.Contains(t, first, "ID")
		assert.Contains(t, first, "Name")
		assert.Contains(t, first, "MediaType")
		assert.Contains(t, first, "PreferredSize")
		assert.Contains(t, first, "MinSize")
		assert.Contains(t, first, "MaxSize")
	})

	t.Run("success - returns created definition", func(t *testing.T) {
		mgr := newQualityManager(t)
		ctx := context.Background()

		_, err := mgr.AddQualityDefinition(ctx, manager.AddQualityDefinitionRequest{
			Name: "TestCustom1080p", Type: "movie",
			PreferredSize: 40, MinSize: 20, MaxSize: 60,
		})
		require.NoError(t, err)

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("GET", "/quality/definitions", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.ListQualityDefinitions()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		defList, ok := response.Response.([]any)
		require.True(t, ok, "Response should be an array")

		// Find our created definition by name (unique to avoid colliding with seeded defaults)
		var found map[string]any
		for _, item := range defList {
			m := item.(map[string]any)
			if m["Name"] == "TestCustom1080p" {
				found = m
				break
			}
		}
		require.NotNil(t, found, "Should find created definition")
		assert.Equal(t, "movie", found["MediaType"])
		assert.Equal(t, float64(40), found["PreferredSize"])
		assert.Equal(t, float64(20), found["MinSize"])
		assert.Equal(t, float64(60), found["MaxSize"])
	})
}

func TestServer_GetQualityDefinition(t *testing.T) {
	t.Run("success - returns definition by id", func(t *testing.T) {
		mgr := newQualityManager(t)
		ctx := context.Background()

		created, err := mgr.AddQualityDefinition(ctx, manager.AddQualityDefinitionRequest{
			Name: "TestCustom1080p", Type: "movie",
			PreferredSize: 40, MinSize: 20, MaxSize: 60,
		})
		require.NoError(t, err)

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("GET", "/quality/definitions/"+itoa(created.ID), nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/definitions/{id}", s.GetQualityDefinition()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		def, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")
		assert.Equal(t, float64(created.ID), def["ID"])
		assert.Equal(t, "TestCustom1080p", def["Name"])
		assert.Equal(t, "movie", def["MediaType"])
		assert.Equal(t, float64(40), def["PreferredSize"])
		assert.Equal(t, float64(20), def["MinSize"])
		assert.Equal(t, float64(60), def["MaxSize"])
	})

	t.Run("invalid id format", func(t *testing.T) {
		s := newTestServer()

		req, err := http.NewRequest("GET", "/quality/definitions/invalid", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/definitions/{id}", s.GetQualityDefinition()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid id")
	})

	t.Run("error - nonexistent id", func(t *testing.T) {
		mgr := newQualityManager(t)

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("GET", "/quality/definitions/9999", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/definitions/{id}", s.GetQualityDefinition()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestServer_CreateQualityDefinition(t *testing.T) {
	t.Run("success - creates a definition", func(t *testing.T) {
		mgr := newQualityManager(t)

		s := newTestServer(withManager(mgr))

		requestBody := `{"name":"TestCustom1080p","type":"movie","preferredSize":40,"minSize":20,"maxSize":60}`
		req, err := http.NewRequest("POST", "/quality/definitions", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.CreateQualityDefinition()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		def, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")
		assert.NotZero(t, def["ID"])
		assert.Equal(t, "TestCustom1080p", def["Name"])
		assert.Equal(t, "movie", def["MediaType"])
		assert.Equal(t, float64(40), def["PreferredSize"])
		assert.Equal(t, float64(20), def["MinSize"])
		assert.Equal(t, float64(60), def["MaxSize"])
	})

	t.Run("invalid request body - malformed json", func(t *testing.T) {
		s := newTestServer()

		requestBody := `{"invalid json`
		req, err := http.NewRequest("POST", "/quality/definitions", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.CreateQualityDefinition()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid request body")
	})
}

func TestServer_UpdateQualityDefinition(t *testing.T) {
	t.Run("success - updates a definition", func(t *testing.T) {
		mgr := newQualityManager(t)
		ctx := context.Background()

		created, err := mgr.AddQualityDefinition(ctx, manager.AddQualityDefinitionRequest{
			Name: "TestCustom1080p", Type: "movie",
			PreferredSize: 40, MinSize: 20, MaxSize: 60,
		})
		require.NoError(t, err)

		s := newTestServer(withManager(mgr))

		requestBody := `{"name":"Bluray-720p","type":"movie","preferredSize":25,"minSize":10,"maxSize":40}`
		req, err := http.NewRequest("PUT", "/quality/definitions/"+itoa(created.ID), strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/definitions/{id}", s.UpdateQualityDefinition()).Methods("PUT")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		def, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")
		assert.Equal(t, float64(created.ID), def["ID"])
		assert.Equal(t, "Bluray-720p", def["Name"])
		assert.Equal(t, "movie", def["MediaType"])
		assert.Equal(t, float64(25), def["PreferredSize"])
		assert.Equal(t, float64(10), def["MinSize"])
		assert.Equal(t, float64(40), def["MaxSize"])
	})

	t.Run("invalid id format", func(t *testing.T) {
		s := newTestServer()

		requestBody := `{"name":"Bluray-720p","type":"movie"}`
		req, err := http.NewRequest("PUT", "/quality/definitions/invalid", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/definitions/{id}", s.UpdateQualityDefinition()).Methods("PUT")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid id")
	})

	t.Run("invalid request body - malformed json", func(t *testing.T) {
		s := newTestServer()

		requestBody := `{"invalid json`
		req, err := http.NewRequest("PUT", "/quality/definitions/1", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/definitions/{id}", s.UpdateQualityDefinition()).Methods("PUT")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid request body")
	})
}

func TestServer_DeleteQualityDefinition(t *testing.T) {
	t.Run("success - deletes a definition", func(t *testing.T) {
		mgr := newQualityManager(t)
		ctx := context.Background()

		created, err := mgr.AddQualityDefinition(ctx, manager.AddQualityDefinitionRequest{
			Name: "TestCustom1080p", Type: "movie",
			PreferredSize: 40, MinSize: 20, MaxSize: 60,
		})
		require.NoError(t, err)

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("DELETE", "/quality/definitions/"+itoa(created.ID), nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/definitions/{id}", s.DeleteQualityDefinition()).Methods("DELETE")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		respMap, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")
		assert.Equal(t, float64(created.ID), respMap["id"])
	})

	t.Run("invalid id format", func(t *testing.T) {
		s := newTestServer()

		req, err := http.NewRequest("DELETE", "/quality/definitions/invalid", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/definitions/{id}", s.DeleteQualityDefinition()).Methods("DELETE")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid id")
	})
}

// --- Quality Profile tests ---

func TestServer_GetQualityProfile(t *testing.T) {
	t.Run("success - returns profile by id", func(t *testing.T) {
		mgr := newQualityManager(t)
		ctx := context.Background()

		// Create quality definitions first
		qd1, err := mgr.AddQualityDefinition(ctx, manager.AddQualityDefinitionRequest{
			Name: "TestHDTV720p", Type: "movie",
			PreferredSize: 10, MinSize: 5, MaxSize: 15,
		})
		require.NoError(t, err)

		qd2, err := mgr.AddQualityDefinition(ctx, manager.AddQualityDefinitionRequest{
			Name: "TestCustom1080p", Type: "movie",
			PreferredSize: 40, MinSize: 20, MaxSize: 60,
		})
		require.NoError(t, err)

		cutoffID := qd2.ID
		profile, err := mgr.AddQualityProfile(ctx, manager.AddQualityProfileRequest{
			Name:            "HD Profile",
			CutoffQualityID: &cutoffID,
			UpgradeAllowed:  true,
			QualityIDs:      []int32{qd1.ID, qd2.ID},
		})
		require.NoError(t, err)

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("GET", "/quality/profiles/"+itoa(profile.ID), nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/profiles/{id}", s.GetQualityProfile()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		prof, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")
		assert.Equal(t, float64(profile.ID), prof["id"])
		assert.Equal(t, "HD Profile", prof["name"])
		assert.Equal(t, true, prof["upgradeAllowed"])

		// Check cutoff quality ID is present
		cutoff, ok := prof["cutoff_quality_id"]
		require.True(t, ok)
		assert.Equal(t, float64(cutoffID), cutoff)

		// Check qualities array
		qualities, ok := prof["qualities"].([]any)
		require.True(t, ok, "qualities should be an array")
		assert.Len(t, qualities, 2)
	})

	t.Run("invalid id format", func(t *testing.T) {
		s := newTestServer()

		req, err := http.NewRequest("GET", "/quality/profiles/invalid", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/profiles/{id}", s.GetQualityProfile()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid id")
	})

	t.Run("error - nonexistent id", func(t *testing.T) {
		mgr := newQualityManager(t)

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("GET", "/quality/profiles/9999", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/profiles/{id}", s.GetQualityProfile()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestServer_ListQualityProfiles(t *testing.T) {
	t.Run("success - returns profiles", func(t *testing.T) {
		mgr := newQualityManager(t)
		ctx := context.Background()

		// Create quality definitions and a profile
		qd1, err := mgr.AddQualityDefinition(ctx, manager.AddQualityDefinitionRequest{
			Name: "TestHDTV720p", Type: "movie",
			PreferredSize: 10, MinSize: 5, MaxSize: 15,
		})
		require.NoError(t, err)

		_, err = mgr.AddQualityProfile(ctx, manager.AddQualityProfileRequest{
			Name:       "Movie HD",
			QualityIDs: []int32{qd1.ID},
		})
		require.NoError(t, err)

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("GET", "/quality/profiles", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.ListQualityProfiles()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		profileList, ok := response.Response.([]any)
		require.True(t, ok, "Response should be an array")
		assert.NotEmpty(t, profileList)

		first := profileList[0].(map[string]any)
		assert.Contains(t, first, "id")
		assert.Contains(t, first, "name")
		assert.Contains(t, first, "upgradeAllowed")
		assert.Contains(t, first, "qualities")
	})

	t.Run("success - filters by movie type", func(t *testing.T) {
		mgr := newQualityManager(t)
		ctx := context.Background()

		qd, err := mgr.AddQualityDefinition(ctx, manager.AddQualityDefinitionRequest{
			Name: "TestHDTV720p", Type: "movie",
			PreferredSize: 10, MinSize: 5, MaxSize: 15,
		})
		require.NoError(t, err)

		_, err = mgr.AddQualityProfile(ctx, manager.AddQualityProfileRequest{
			Name:       "Movie Profile",
			QualityIDs: []int32{qd.ID},
		})
		require.NoError(t, err)

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("GET", "/quality/profiles?type=movie", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.ListQualityProfiles()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("success - filters by series type", func(t *testing.T) {
		mgr := newQualityManager(t)
		ctx := context.Background()

		qd, err := mgr.AddQualityDefinition(ctx, manager.AddQualityDefinitionRequest{
			Name: "TestHDTV720p", Type: "episode",
			PreferredSize: 10, MinSize: 5, MaxSize: 15,
		})
		require.NoError(t, err)

		_, err = mgr.AddQualityProfile(ctx, manager.AddQualityProfileRequest{
			Name:       "Series Profile",
			QualityIDs: []int32{qd.ID},
		})
		require.NoError(t, err)

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("GET", "/quality/profiles?type=series", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.ListQualityProfiles()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestServer_CreateQualityProfile(t *testing.T) {
	t.Run("success - creates a profile", func(t *testing.T) {
		mgr := newQualityManager(t)
		ctx := context.Background()

		qd1, err := mgr.AddQualityDefinition(ctx, manager.AddQualityDefinitionRequest{
			Name: "TestHDTV720p", Type: "movie",
			PreferredSize: 10, MinSize: 5, MaxSize: 15,
		})
		require.NoError(t, err)

		qd2, err := mgr.AddQualityDefinition(ctx, manager.AddQualityDefinitionRequest{
			Name: "TestCustom1080p", Type: "movie",
			PreferredSize: 40, MinSize: 20, MaxSize: 60,
		})
		require.NoError(t, err)

		s := newTestServer(withManager(mgr))

		cutoffID := int(qd2.ID)
		requestBody := `{"name":"Test Profile","cutoffQualityId":` + itoa(int32(cutoffID)) + `,"upgradeAllowed":true,"qualityIds":[` + itoa(qd1.ID) + `,` + itoa(qd2.ID) + `]}`
		req, err := http.NewRequest("POST", "/quality/profiles", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.CreateQualityProfile()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		prof, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")
		assert.NotZero(t, prof["id"])
		assert.Equal(t, "Test Profile", prof["name"])
		assert.Equal(t, true, prof["upgradeAllowed"])

		qualities, ok := prof["qualities"].([]any)
		require.True(t, ok, "qualities should be an array")
		assert.Len(t, qualities, 2)
	})

	t.Run("invalid request body - malformed json", func(t *testing.T) {
		s := newTestServer()

		requestBody := `{"invalid json`
		req, err := http.NewRequest("POST", "/quality/profiles", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.CreateQualityProfile()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid request body")
	})
}

func TestServer_UpdateQualityProfile(t *testing.T) {
	t.Run("success - updates a profile", func(t *testing.T) {
		mgr := newQualityManager(t)
		ctx := context.Background()

		qd1, err := mgr.AddQualityDefinition(ctx, manager.AddQualityDefinitionRequest{
			Name: "TestHDTV720p", Type: "movie",
			PreferredSize: 10, MinSize: 5, MaxSize: 15,
		})
		require.NoError(t, err)

		qd2, err := mgr.AddQualityDefinition(ctx, manager.AddQualityDefinitionRequest{
			Name: "TestCustom1080p", Type: "movie",
			PreferredSize: 40, MinSize: 20, MaxSize: 60,
		})
		require.NoError(t, err)

		cutoffID := qd1.ID
		profile, err := mgr.AddQualityProfile(ctx, manager.AddQualityProfileRequest{
			Name:            "Old Name",
			CutoffQualityID: &cutoffID,
			UpgradeAllowed:  false,
			QualityIDs:      []int32{qd1.ID},
		})
		require.NoError(t, err)

		s := newTestServer(withManager(mgr))

		newCutoffID := int(qd2.ID)
		requestBody := `{"name":"Updated Profile","cutoffQualityId":` + itoa(int32(newCutoffID)) + `,"upgradeAllowed":true,"qualityIds":[` + itoa(qd1.ID) + `,` + itoa(qd2.ID) + `]}`
		req, err := http.NewRequest("PUT", "/quality/profiles/"+itoa(profile.ID), strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/profiles/{id}", s.UpdateQualityProfile()).Methods("PUT")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		prof, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")
		assert.Equal(t, float64(profile.ID), prof["id"])
		assert.Equal(t, "Updated Profile", prof["name"])
		assert.Equal(t, true, prof["upgradeAllowed"])

		qualities, ok := prof["qualities"].([]any)
		require.True(t, ok, "qualities should be an array")
		assert.Len(t, qualities, 2)
	})

	t.Run("invalid id format", func(t *testing.T) {
		s := newTestServer()

		requestBody := `{"name":"Updated Profile","qualityIds":[1]}`
		req, err := http.NewRequest("PUT", "/quality/profiles/invalid", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/profiles/{id}", s.UpdateQualityProfile()).Methods("PUT")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid id")
	})

	t.Run("invalid request body - malformed json", func(t *testing.T) {
		s := newTestServer()

		requestBody := `{"invalid json`
		req, err := http.NewRequest("PUT", "/quality/profiles/1", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/profiles/{id}", s.UpdateQualityProfile()).Methods("PUT")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid request body")
	})
}

func TestServer_DeleteQualityProfile(t *testing.T) {
	t.Run("success - deletes a profile", func(t *testing.T) {
		mgr := newQualityManager(t)
		ctx := context.Background()

		qd, err := mgr.AddQualityDefinition(ctx, manager.AddQualityDefinitionRequest{
			Name: "TestHDTV720p", Type: "movie",
			PreferredSize: 10, MinSize: 5, MaxSize: 15,
		})
		require.NoError(t, err)

		profile, err := mgr.AddQualityProfile(ctx, manager.AddQualityProfileRequest{
			Name:       "To Delete",
			QualityIDs: []int32{qd.ID},
		})
		require.NoError(t, err)

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("DELETE", "/quality/profiles/"+itoa(profile.ID), nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/profiles/{id}", s.DeleteQualityProfile()).Methods("DELETE")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))
	})

	t.Run("invalid id format", func(t *testing.T) {
		s := newTestServer()

		req, err := http.NewRequest("DELETE", "/quality/profiles/invalid", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/profiles/{id}", s.DeleteQualityProfile()).Methods("DELETE")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid id")
	})
}
