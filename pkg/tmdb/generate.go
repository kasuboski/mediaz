package tmdb

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=config.yaml ./schema/tmdb.schema.json
//go:generate go run go.uber.org/mock/mockgen -package mocks -destination mocks/mock_tmdb_client.go github.com/kasuboski/mediaz/pkg/tmdb ClientInterface
//go:generate go run go.uber.org/mock/mockgen -package mocks -destination mocks/mock_itmdb.go github.com/kasuboski/mediaz/pkg/tmdb ITmdb
