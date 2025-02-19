package http

//go:generate go run go.uber.org/mock/mockgen -package mocks -destination mocks/mock_http_client.go github.com/kasuboski/mediaz/pkg/http HTTPClient
