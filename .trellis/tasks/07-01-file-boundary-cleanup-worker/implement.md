# Implementation Plan

## Checklist

- [x] Inspect File service docs, implementation notes, OpenAPI files, service code, tests, and current sqlc setup.
- [x] Identify current upload/read/delete/purge behavior and any legacy knowledge-document routes or repository methods.
- [x] Add OpenAPI drift test for File internal contracts.
- [x] Add or strengthen service-token negative tests.
- [x] Remove or explicitly quarantine legacy knowledge-document File surface.
- [x] Implement or converge `file:object:purge` cleanup semantics with idempotency, retryability, and sanitized failure summaries.
- [x] Resolve sqlc drift through pinned regeneration or removal of unused sqlc entry points.
- [x] Update File README/implementation docs only where behavior or verification instructions change.
- [x] Run service-local tests and required repository checks.

## Validation Results

- `cd services/file && go test ./...`: pass.
- `cd services/file && go build ./cmd/server`: pass.
- `cd services/file && $env:CGO_ENABLED='0'; go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate`: pass. Plain `go run` hit a local cgo compiler limitation (`64-bit mode not compiled in`), so generation was rerun with cgo disabled.
- `git diff --check`: pass, with CRLF/LF conversion warnings for existing touched files.
- `FILE_MINIO_POSTGRES_SMOKE=1 ... TestFileMinIOPostgresSmoke`: blocked locally because PostgreSQL is not listening on `localhost:5432`.

## Validation Commands

Run from the repository root unless noted:

```bash
cd services/file
go test ./...
go build ./cmd/server
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
cd ../..
git diff --check
```

If local dependencies are available, also preserve the issue smoke command:

```bash
cd services/file
FILE_MINIO_POSTGRES_SMOKE=1 go test ./internal/integration -run '^TestFileMinIOPostgresSmoke$' -count=1 -v
```

If the smoke cannot run locally, record the blocker and keep ordinary tests passing.

## Risk Points

- Purge must never make deleted content readable again.
- Storage failure summaries must be sanitized.
- Contract comparison should avoid brittle formatting-only diffs.
- Do not introduce Redis/asynq unless the existing File design already requires it; a service-local cleanup boundary is acceptable if it satisfies #341.
- Do not change Knowledge or Document business deletion orchestration for #342.
