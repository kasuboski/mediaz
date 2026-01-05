package indexer

//go:generate go run go.uber.org/mock/mockgen -destination=mocks/mock_indexer_source.go -package=mocks github.com/kasuboski/mediaz/pkg/indexer IndexerSource,Factory
