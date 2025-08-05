# CRUSH.md

## Commands
- Build: `go build -o mediaz main.go`
- Run server: `./mediaz serve`
- Generate code/clients: `make generate` or `go generate ./...`
- Format: `go fmt ./...`
- Lint: `golangci-lint run` (if installed); otherwise `go vet ./...`
- Frontend lint: `npm run lint --prefix frontend`
- Test all: `go test ./...`
- Test with race: `go test -race ./...`
- Test with coverage: `go test -cover ./...`
- Test single package (verbose): `go test -v ./pkg/manager`
- Test single file: `go test -run TestName ./pkg/manager`
- Test single test: `go test ./pkg/manager -run '^TestName$'`
- Generate DB schema: `./mediaz generate schema`

## Code Style
- Go version from go.mod; use standard library first-party tooling. Keep imports grouped: stdlib, third-party, local (module path), each group alphabetized.
- Use go fmt defaults; no custom formatters. Run `go vet` before commits.
- Types: prefer explicit types; use pointers for large structs or when mutation/nullable semantics are needed.
- Naming: Exported identifiers are PascalCase with clear domain terms (TMDB, Prowlarr). Unexported are lowerCamelCase. Interfaces use behavior names (e.g., Storage, Downloader). Files use snake_case where conventional.
- Errors: return `error` as last result; wrap with context using `fmt.Errorf("...: %w", err)`; no panics in library code. Compare with `errors.Is/As`.
- Context: thread `context.Context` as first arg in public methods; honor cancellation/timeouts.
- Logging: use `pkg/logger` (Zap) with structured fields; avoid logging secrets/keys.
- Concurrency: prefer channels/contexts over globals; guard shared state; use `-race` in CI.
- Testing: use `gomock` mocks in `mocks/`; prefer in-memory SQLite for storage tests; keep tests deterministic; table-driven tests where practical.
- Generated code: never edit files under generated paths; regenerate via `make generate`.

## Agent Notes
- CLI entry: `main.go` → `cmd/root.go`; HTTP server in `server/server.go` (Gorilla Mux) under `/api/v1`.
- Config via Viper: flags → env (MEDIAZ_*) → file → defaults.
- Respect existing patterns in `pkg/manager`, `pkg/storage/sqlite`, `pkg/download`, `pkg/tmdb`, `pkg/prowlarr`.
- Include Cursor/Copilot rules if present; follow `.windsurfrules`.
