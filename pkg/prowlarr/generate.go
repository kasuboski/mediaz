package prowlarr

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=config.yaml ../../prowlarr.schema.json
//go:generate go run go.uber.org/mock/mockgen -package mocks -destination mocks/mock_prowlarr_client.go github.com/kasuboski/mediaz/pkg/prowlarr ClientInterface
