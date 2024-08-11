package server

//go:generate mockgen -package mocks -destination mocks/mock_tmdb_client.go github.com/kasuboski/mediaz/server TMDBClientInterface
//go:generate mockgen -package mocks -destination mocks/mock_prowlarr_client.go github.com/kasuboski/mediaz/server ProwlarrClientInterface
