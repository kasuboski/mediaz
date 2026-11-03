package manager

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateIndexerSource(t *testing.T) {
	key := "secret"
	valid := AddIndexerSourceRequest{Name: "native", Implementation: "newznab", Scheme: "https", Host: "indexer.example", APIPath: "/proxy/api"}
	for _, tc := range []struct {
		name string
		edit func(*AddIndexerSourceRequest)
	}{
		{"scheme", func(r *AddIndexerSourceRequest) { r.Scheme = "ftp" }},
		{"implementation", func(r *AddIndexerSourceRequest) { r.Implementation = "other" }},
		{"host path", func(r *AddIndexerSourceRequest) { r.Host = "indexer.example/api" }},
		{"host query", func(r *AddIndexerSourceRequest) { r.Host = "indexer.example?x=1" }},
		{"host credentials", func(r *AddIndexerSourceRequest) { r.Host = "user@indexer.example" }},
		{"host embedded port", func(r *AddIndexerSourceRequest) { r.Host = "indexer.example:443" }},
		{"port", func(r *AddIndexerSourceRequest) { p := int32(70000); r.Port = &p }},
		{"relative path", func(r *AddIndexerSourceRequest) { r.APIPath = "api" }},
		{"path query", func(r *AddIndexerSourceRequest) { r.APIPath = "/api?x=1" }},
		{"path fragment", func(r *AddIndexerSourceRequest) { r.APIPath = "/api#frag" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := valid
			tc.edit(&req)
			require.Error(t, validateIndexerSource(req))
		})
	}
	require.NoError(t, validateIndexerSource(valid))
	require.NoError(t, validateIndexerSource(AddIndexerSourceRequest{Name: "p", Implementation: "prowlarr", Scheme: "http", Host: "localhost", APIKey: &key}))
	require.Error(t, validateIndexerSource(AddIndexerSourceRequest{Name: "p", Implementation: "prowlarr", Scheme: "http", Host: "localhost"}))
	require.Equal(t, "/api", apiPathFor("newznab", ""))
	require.Equal(t, "", apiPathFor("prowlarr", ""))
}
