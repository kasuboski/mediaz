package download

//go:generate go run go.uber.org/mock/mockgen -package mocks -destination mocks/mock_download_client.go github.com/kasuboski/mediaz/pkg/download DownloadClient

//go:generate go run go.uber.org/mock/mockgen -package mocks -destination mocks/mock_factory.go github.com/kasuboski/mediaz/pkg/download Factory
