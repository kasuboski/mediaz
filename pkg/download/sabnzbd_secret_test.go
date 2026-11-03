package download

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/kasuboski/mediaz/pkg/http/mocks"
	"github.com/kasuboski/mediaz/pkg/indexer"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/oapi-codegen/nullable"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestSabnzbdAddDoesNotExposeSecrets(t *testing.T) {
	const (
		apiKey = "sab-api-secret-7fd"
		uri    = "https://provider.invalid/get?key=provider-secret-91&token=download-token"
	)
	encodedURI := "https%3A%2F%2Fprovider.invalid%2Fget%3Fkey%3Dprovider-secret-91%26token%3Ddownload-token"
	secrets := []string{apiKey, "provider-secret-91", "download-token", uri, encodedURI}

	tests := []struct {
		name      string
		transport error
		cancel    bool
	}{
		{name: "transport error"},
		{name: "canceled", cancel: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			client := mocks.NewMockHTTPClient(ctrl)
			core, logs := observer.New(zapcore.DebugLevel)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx = logger.WithCtx(ctx, zap.New(core).Sugar())

			if tt.cancel {
				cancel()
				tt.transport = fmt.Errorf("transport failed for %s?apikey=%s", uri, apiKey)
			}
			client.EXPECT().Do(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, "/sabnzbd/api", req.URL.Path)
				require.Equal(t, "addurl", req.URL.Query().Get("mode"))
				return nil, fmt.Errorf("transport failed for %s?apikey=%s", uri, apiKey)
			})

			_, err := NewSabnzbdClient(client, "https", "sab.invalid", "", apiKey).Add(ctx, AddRequest{
				Release: &indexer.Release{DownloadURL: nullable.NewNullableWithValue[string](uri)},
			})
			require.Error(t, err)
			if tt.cancel {
				require.ErrorIs(t, err, context.Canceled)
			}
			captured := err.Error()
			for _, entry := range logs.All() {
				captured += entry.Message + fmt.Sprint(entry.Context)
			}
			for _, secret := range secrets {
				require.NotContains(t, captured, secret)
			}
			require.NotContains(t, strings.ToLower(captured), "sab-api-secret")
		})
	}
}
