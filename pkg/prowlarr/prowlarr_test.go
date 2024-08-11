package prowlarr

import (
	"net/http"
	"net/url"
	"testing"
)

func TestSetRequestAPIKey(t *testing.T) {
	type args struct {
		apiKey string
		req    *http.Request
	}
	tests := []struct {
		name    string
		args    args
		wantReq *http.Request
	}{
		{
			name: "set api key",
			args: args{
				apiKey: "my-api-key",
				req: &http.Request{
					URL: &url.URL{
						RawQuery: url.Values{
							"my-qp": []string{"value-1", "value-2"},
						}.Encode(),
					},
				},
			},
			wantReq: &http.Request{
				URL: &url.URL{
					RawQuery: url.Values{
						"apiKey": []string{"my-api-key"},
						"my-qp":  []string{"value-1", "value-2"},
					}.Encode(),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetRequestAPIKey(tt.args.apiKey, tt.args.req)
		})
	}
}
