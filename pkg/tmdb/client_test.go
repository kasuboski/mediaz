package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	client "github.com/kasuboski/mediaz/pkg/tmdb"
)

func TestClient_CanCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Write([]byte("{}"))
	}))
	defer server.Close()
	hc := server.Client()

	c, err := client.NewClientWithResponses(server.URL, client.WithHTTPClient(hc))
	if err != nil {
		t.Fatalf("couldn't create client: %v", err)
	}

	resp, err := c.ConfigurationDetailsWithResponse(context.TODO())
	if err != nil {
		t.Fatalf("failed to get config: %v", err)
	}

	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode())
	}
}
