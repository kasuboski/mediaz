package indexer_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kasuboski/mediaz/pkg/indexer"
	"github.com/kasuboski/mediaz/pkg/newznab"
	"github.com/stretchr/testify/require"
)

func TestNewznabXMLQuotaCodesActivateCooldown(t *testing.T) {
	for _, code := range []string{"500", "501"} {
		t.Run(code, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Query().Get("t") == "caps" {
					_, _ = w.Write([]byte(`<caps><searching><search available="yes" supportedParams="q"/></searching></caps>`))
					return
				}
				_, _ = w.Write([]byte(`<error code="` + code + `" description="quota"/>`))
			}))
			defer server.Close()
			c, err := newznab.New(server.URL, "test-key")
			require.NoError(t, err)
			src := indexer.NewNewznabIndexerSourceWithClient(c, config())
			_, err = src.Search(context.Background(), 1, nil, indexer.SearchOptions{})
			require.Error(t, err)
			_, err = src.Search(context.Background(), 1, nil, indexer.SearchOptions{})
			var cooldown *indexer.RetryableCooldownError
			require.ErrorAs(t, err, &cooldown)
			require.Equal(t, 2, calls)
		})
	}
}
