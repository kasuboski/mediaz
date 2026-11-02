package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/download"
	"github.com/kasuboski/mediaz/pkg/manager"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/stretchr/testify/require"
)

func TestDownloadClientAPIPathHTTPContract(t *testing.T) {
	ctx := context.Background()
	st, err := sqlite.New(ctx, ":memory:")
	require.NoError(t, err)
	mgr := manager.New(nil, nil, nil, st, download.NewDownloadClientFactory(), config.Manager{}, config.Config{})
	s := newTestServer(withManager(mgr))
	r := mux.NewRouter()
	r.HandleFunc("/api/v1/download/clients", s.CreateDownloadClient()).Methods("POST")
	r.HandleFunc("/api/v1/download/clients/{id}", s.GetDownloadClient()).Methods("GET")
	r.HandleFunc("/api/v1/download/clients/{id}", s.UpdateDownloadClient()).Methods("PUT")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		return rr
	}
	t.Run("create returns saved modern path and omits credential", func(t *testing.T) {
		rr := do("POST", "/api/v1/download/clients", `{"Type":"usenet","Implementation":"sabnzbd","Scheme":"https","Host":"sab.example","Port":8080,"APIKey":"topsecret"}`)
		require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
		var got map[string]any
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
		response := got["response"].(map[string]any)
		id := int64(response["ID"].(float64))
		require.Equal(t, map[string]any{"ID": float64(id), "Type": "usenet", "Implementation": "sabnzbd", "Scheme": "https", "Host": "sab.example", "Port": float64(8080), "APIPath": "/api"}, response)
		require.NotContains(t, rr.Body.String(), "topsecret")
		saved, err := st.GetDownloadClient(ctx, id)
		require.NoError(t, err)
		require.Equal(t, "/api", saved.ApiPath.String)
		get := do("GET", "/api/v1/download/clients/"+fmt.Sprint(id), "")
		require.Equal(t, http.StatusOK, get.Code)
		require.Contains(t, get.Body.String(), `"APIPath":"/api"`)
		updated := do("PUT", "/api/v1/download/clients/"+fmt.Sprint(id), `{"Type":"usenet","Implementation":"sabnzbd","Scheme":"https","Host":"sab.example","Port":8080,"apiPath":"/custom/api"}`)
		require.Equal(t, http.StatusOK, updated.Code, updated.Body.String())
		require.Contains(t, updated.Body.String(), `"APIPath":"/custom/api"`)
		saved, err = st.GetDownloadClient(ctx, id)
		require.NoError(t, err)
		require.Equal(t, "/custom/api", saved.ApiPath.String)
	})
	t.Run("legacy null path is omitted from response DTO", func(t *testing.T) {
		legacy := sqlcdb.DownloadClient{Type: "usenet", Implementation: "sabnzbd", Scheme: "https", Host: "legacy.example", Port: 8080, ApiKey: sql.NullString{String: "legacy-secret", Valid: true}}
		id, err := st.CreateDownloadClient(ctx, legacy)
		require.NoError(t, err)
		rr := do("GET", "/api/v1/download/clients/"+fmt.Sprint(id), "")
		require.Equal(t, http.StatusOK, rr.Code)
		var envelope map[string]any
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
		response := envelope["response"].(map[string]any)
		require.Equal(t, map[string]any{"ID": float64(id), "Type": "usenet", "Implementation": "sabnzbd", "Scheme": "https", "Host": "legacy.example", "Port": float64(8080)}, response)
		require.NotContains(t, rr.Body.String(), "APIPath")
		require.NotContains(t, rr.Body.String(), "legacy-secret")
	})

	t.Run("invalid create and update return 400 and preserve stored row", func(t *testing.T) {
		before := sqlcdb.DownloadClient{Type: "usenet", Implementation: "sabnzbd", Scheme: "https", Host: "sab.example", Port: 8080, ApiKey: sql.NullString{String: "secret", Valid: true}, ApiPath: sql.NullString{String: "/custom", Valid: true}}
		id, err := st.CreateDownloadClient(ctx, before)
		require.NoError(t, err)
		before.ID = id
		for _, tc := range []struct{ method, path, body, want string }{{"POST", "/api/v1/download/clients", `{"Type":"usenet","Implementation":"sabnzbd","Scheme":"ftp","Host":"sab.example","Port":80,"APIKey":"key"}`, "scheme must be http or https"}, {"PUT", "/api/v1/download/clients/" + fmt.Sprint(id), `{"Type":"usenet","Implementation":"sabnzbd","Scheme":"https","Host":"sab.example","Port":8080,"APIKey":"new","APIPath":"relative"}`, "invalid SABnzbd API path"}} {
			rr := do(tc.method, tc.path, tc.body)
			require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
			require.Contains(t, rr.Body.String(), tc.want)
			after, e := st.GetDownloadClient(ctx, id)
			require.NoError(t, e)
			require.Equal(t, before, after)
		}
	})
}
