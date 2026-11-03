package newznab_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/newznab"
	"github.com/kasuboski/mediaz/pkg/newznab/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error { b.closed = true; return nil }

func TestHTTPTransportMock(t *testing.T) {
	for _, tc := range []struct {
		name, body   string
		status       int
		header       http.Header
		wantError    bool
		wantProtocol *newznab.ProtocolError
	}{
		{name: "empty results", body: `<rss><channel/></rss>`, status: 200},
		{name: "rate limited", body: "private response", status: 429, header: http.Header{"Retry-After": {"12"}}, wantError: true, wantProtocol: &newznab.ProtocolError{Status: 429, RetryAfter: 12 * time.Second}},
		{name: "HTTP failure", body: "private response", status: 503, wantError: true, wantProtocol: &newznab.ProtocolError{Status: 503}},
		{name: "XML error", body: `<?xml version="1.0"?><error code="100" description="PRIVATE_KEY"/>`, status: 200, wantError: true, wantProtocol: &newznab.ProtocolError{Code: "100"}},
		{name: "malformed", body: `<rss><channel>`, status: 200, wantError: true},
		{name: "trailing document", body: `<rss><channel/></rss><rss/>`, status: 200, wantError: true},
		{name: "oversized", body: strings.Repeat("x", (8<<20)+1), status: 200, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			transport := mocks.NewMockHTTPDoer(ctrl)
			body := &trackedBody{Reader: strings.NewReader(tc.body)}
			transport.EXPECT().Do(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, http.MethodGet, req.Method)
				require.Equal(t, "/prefix/api", req.URL.Path)
				require.Equal(t, url.Values{"apikey": {"PRIVATE_KEY"}, "t": {"search"}, "q": {"Title & more"}, "cat": {"2000,2040"}, "limit": {"100"}, "offset": {"0"}, "extended": {"1"}}, req.URL.Query())
				return &http.Response{StatusCode: tc.status, Header: tc.header, Body: body}, nil
			})
			client, err := newznab.NewWithHTTP("https://example.invalid/prefix/api", "PRIVATE_KEY", transport)
			require.NoError(t, err)
			got, err := client.Search(context.Background(), newznab.SearchRequest{Query: "Title & more", Categories: []string{"2000", "2040"}, Limit: 100})
			require.True(t, body.closed)
			if tc.wantError {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "PRIVATE_KEY")
				require.NotContains(t, err.Error(), "private response")
			} else {
				require.NoError(t, err)
				require.Equal(t, &newznab.Page{}, got)
			}
			if tc.wantProtocol != nil {
				var protocol *newznab.ProtocolError
				require.ErrorAs(t, err, &protocol)
				require.Equal(t, tc.wantProtocol, protocol)
			}
		})
	}
}

func TestTransportErrorDoesNotExposeRequestURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	transport := mocks.NewMockHTTPDoer(ctrl)
	transport.EXPECT().Do(gomock.Any()).Return(nil, &url.Error{Op: "Get", URL: "https://example.invalid/api?apikey=PRIVATE_KEY", Err: errors.New("PRIVATE_KEY")})
	client, err := newznab.NewWithHTTP("https://example.invalid/api", "PRIVATE_KEY", transport)
	require.NoError(t, err)
	_, err = client.Caps(context.Background())
	require.Error(t, err)
	require.NotContains(t, err.Error(), "PRIVATE_KEY")
	require.NotContains(t, err.Error(), "example.invalid")
}

func TestDefaultTransportDoesNotFollowRedirect(t *testing.T) {
	reached := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true; w.Write([]byte(`<caps/>`)) }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"?apikey=PRIVATE_KEY", http.StatusFound)
	}))
	defer source.Close()
	client, err := newznab.New(source.URL, "PRIVATE_KEY")
	require.NoError(t, err)
	_, err = client.Caps(context.Background())
	require.Error(t, err)
	require.False(t, reached)
	require.NotContains(t, err.Error(), "PRIVATE_KEY")
}
