package download

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/kasuboski/mediaz/pkg/indexer"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/oapi-codegen/nullable"
	"github.com/stretchr/testify/require"
)

const factoryQueue = `{"queue":{"speed":"0 KB/s","slots":[{"nzo_id":"id-1","filename":"synthetic.nzb","mb":"4.00","percentage":"0","priority":"Normal"}]}}`
const factoryHistory = `{"history":{"slots":[]}}`

func sabFactoryConfig(host, scheme string, port int64, apiPath sql.NullString) sqlcdb.DownloadClient {
	return sqlcdb.DownloadClient{Implementation: "sabnzbd", Scheme: scheme, Host: host, Port: port,
		ApiKey: sql.NullString{String: "test-key", Valid: true}, ApiPath: apiPath}
}

func TestSabnzbdFactoryEndpointAndProtocol(t *testing.T) {
	var mu sync.Mutex
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.URL.Path+"|"+r.URL.Query().Get("mode")+"|"+r.URL.Query().Get("apikey"))
		mu.Unlock()
		if r.URL.Query().Get("mode") == "addurl" {
			_, _ = fmt.Fprint(w, `{"nzo_ids":["id-1"]}`)
		} else if r.URL.Query().Get("mode") == "history" {
			_, _ = fmt.Fprint(w, factoryHistory)
		} else {
			_, _ = fmt.Fprint(w, factoryQueue)
		}
	}))
	defer server.Close()
	hostport := strings.TrimPrefix(server.URL, "http://")
	_, portText, found := strings.Cut(hostport, ":")
	require.True(t, found)
	var port int64
	_, err := fmt.Sscan(portText, &port)
	require.NoError(t, err)

	for _, tc := range []struct {
		name, host string
		port       int64
		path       sql.NullString
		wantPath   string
	}{
		{"separate port and root path", "127.0.0.1", port, sql.NullString{String: "/api", Valid: true}, "/api"},
		{"legacy null path", "127.0.0.1", port, sql.NullString{}, "/sabnzbd/api"},
		{"custom prefix", "127.0.0.1", port, sql.NullString{String: "/prefix/custom", Valid: true}, "/prefix/custom"},
		{"inline port", hostport, 0, sql.NullString{String: "/inline/api", Valid: true}, "/inline/api"},
		{"matching explicit port", hostport, port, sql.NullString{String: "/same/api", Valid: true}, "/same/api"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mu.Lock()
			requests = nil
			mu.Unlock()
			client, err := NewDownloadClientFactory().NewDownloadClient(sabFactoryConfig(tc.host, "http", tc.port, tc.path))
			require.NoError(t, err)
			added, err := client.Add(context.Background(), AddRequest{Release: &indexer.Release{DownloadURL: nullable.NewNullableWithValue[string]("https://example.test/file.nzb")}})
			require.NoError(t, err)
			require.Equal(t, Status{ID: "id-1", Name: "synthetic.nzb", FilePaths: []string{""}, Progress: 0, Speed: 0, Size: 4}, added)
			listed, err := client.List(context.Background())
			require.NoError(t, err)
			require.Equal(t, []Status{{ID: "id-1", Name: "synthetic.nzb", FilePaths: []string{""}, Progress: 0, Speed: 0, Size: 4}}, listed)
			mu.Lock()
			got := append([]string(nil), requests...)
			mu.Unlock()
			require.Len(t, got, 5)
			modes := []string{"addurl", "queue", "history", "queue", "history"}
			for i, mode := range modes {
				parts := strings.Split(got[i], "|")
				require.Equal(t, tc.wantPath, parts[0])
				require.Equal(t, mode, parts[1])
				require.Equal(t, "test-key", parts[2])
			}
		})
	}
}

func TestSabnzbdFactoryRejectsInvalidConfiguration(t *testing.T) {
	for _, host := range []string{"", "evil/path", "//evil", "https://evil", "host?x", "host#x", `host\\name`} {
		_, err := NewDownloadClientFactory().NewDownloadClient(sabFactoryConfig(host, "http", 8080, sql.NullString{}))
		require.Error(t, err, host)
	}
	for _, port := range []int64{-1, 65536} {
		_, err := NewDownloadClientFactory().NewDownloadClient(sabFactoryConfig("localhost", "http", port, sql.NullString{}))
		require.Error(t, err)
	}
	_, err := NewDownloadClientFactory().NewDownloadClient(sabFactoryConfig("localhost:1234", "http", 1235, sql.NullString{}))
	require.Error(t, err, "conflicting port")
	for _, scheme := range []string{"", "ftp"} {
		_, err := NewDownloadClientFactory().NewDownloadClient(sabFactoryConfig("localhost", scheme, 0, sql.NullString{}))
		require.Error(t, err)
	}
	for _, path := range []string{"relative", "//evil", "https://evil/api", "/api?x", "/api#x", "/api\\x", "/api\n"} {
		_, err := NewDownloadClientFactory().NewDownloadClient(sabFactoryConfig("localhost", "http", 0, sql.NullString{String: path, Valid: true}))
		require.Error(t, err, path)
	}
	for _, key := range []sql.NullString{{}, {String: "", Valid: true}} {
		config := sabFactoryConfig("localhost", "http", 0, sql.NullString{})
		config.ApiKey = key
		_, err := NewDownloadClientFactory().NewDownloadClient(config)
		require.Error(t, err)
	}
	for _, config := range []sqlcdb.DownloadClient{
		sabFactoryConfig("localhost", "http", 0, sql.NullString{}),
		sabFactoryConfig("localhost", "http", 1, sql.NullString{}),
		sabFactoryConfig("localhost", "http", 65535, sql.NullString{}),
	} {
		_, err := NewDownloadClientFactory().NewDownloadClient(config)
		require.NoError(t, err)
	}
}

func TestSabnzbdFactoryCancellationDoesNotLeakAPIKey(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	host := strings.TrimPrefix(server.URL, "http://")
	client, err := NewDownloadClientFactory().NewDownloadClient(sabFactoryConfig(host, "http", 0, sql.NullString{}))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := client.List(ctx); done <- err }()
	<-started
	cancel()
	err = <-done
	require.Error(t, err)
	require.NotContains(t, err.Error(), "test-key")
}

func TestSabnzbdFactoryDoesNotFollowRedirect(t *testing.T) {
	var targetCalls int
	var mu sync.Mutex
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { mu.Lock(); targetCalls++; mu.Unlock() }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer redirect.Close()
	host := strings.TrimPrefix(redirect.URL, "http://")
	client, err := NewDownloadClientFactory().NewDownloadClient(sabFactoryConfig(host, "http", 0, sql.NullString{}))
	require.NoError(t, err)
	_, err = client.List(context.Background())
	require.Error(t, err)
	mu.Lock()
	defer mu.Unlock()
	require.Zero(t, targetCalls)
}

func TestValidateSABConfigIPv6(t *testing.T) {
	for _, host := range []string{"::1", "[::1]"} {
		require.NoError(t, ValidateSABConfig("http", host, 0, ""), host)
	}
	require.NoError(t, ValidateSABConfig("https", "sab.example", 443, "/sabnzbd/api"))
	listener, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("local IPv6 unavailable: %v", err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, factoryQueue) })}
	defer server.Close()
	go func() { _ = server.Serve(listener) }()
	port := int64(listener.Addr().(*net.TCPAddr).Port)
	client, err := NewDownloadClientFactory().NewDownloadClient(sabFactoryConfig("::1", "http", port, sql.NullString{}))
	require.NoError(t, err)
	_, err = client.List(context.Background())
	require.NoError(t, err)
}
