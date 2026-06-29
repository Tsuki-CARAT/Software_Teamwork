# Document Service

`document` owns report types, templates, materials, reports, outlines, sections,
report jobs, job attempts, events, generated file metadata, statistics, and
operation logs.

This first implementation slice is the service/data baseline for issue #97. It
does not implement AI generation, DOCX export execution, MCP tools, file-service
upload/download calls, or AI Gateway calls yet.

## Local Configuration

Required environment variables:

| Variable | Example | Purpose |
| --- | --- | --- |
| `DOCUMENT_DATABASE_URL` | `postgres://document:document@localhost:5432/document?sslmode=disable` | PostgreSQL connection string. |
| `DOCUMENT_REDIS_ADDR` | `localhost:6379` | Redis/asynq queue endpoint. Redis is not the durable job state authority. |
| `DOCUMENT_FILE_SERVICE_URL` | `http://localhost:8082` | Internal file service base URL for later template/material/report-file bytes. |
| `DOCUMENT_AI_GATEWAY_URL` | `http://localhost:8086` | Internal AI Gateway base URL for later generation calls. |
| `DOCUMENT_AI_GATEWAY_PROFILE_ID` | `default-chat` | AI Gateway profile reference used by report settings/default generation. |

Optional variables:

| Variable | Default | Purpose |
| --- | --- | --- |
| `DOCUMENT_HTTP_ADDR` | `:8085` | HTTP listen address. |
| `DOCUMENT_PANDOC_PATH` | `pandoc` | DOCX toolchain command path reserved for worker usage. |
| `DOCUMENT_LIBREOFFICE_PATH` | `soffice` | LibreOffice command path reserved for worker usage. |
| `DOCUMENT_SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown timeout. |

## Run

```powershell
$env:DOCUMENT_DATABASE_URL = "postgres://document:document@localhost:5432/document?sslmode=disable"
$env:DOCUMENT_REDIS_ADDR = "localhost:6379"
$env:DOCUMENT_FILE_SERVICE_URL = "http://localhost:8082"
$env:DOCUMENT_AI_GATEWAY_URL = "http://localhost:8086"
$env:DOCUMENT_AI_GATEWAY_PROFILE_ID = "default-chat"
go run ./cmd/server
```

Operational routes:

```text
GET /healthz
GET /readyz
```

Both JSON responses use the project envelope: `{ "data": ..., "requestId": "..." }`.
The service-local operational contract is documented in [`api/openapi.yaml`](api/openapi.yaml).

## Migrations

Migration files live in `migrations/` and are intended for `goose`.

```powershell
go install github.com/pressly/goose/v3/cmd/goose@v3.27.1
goose -dir migrations postgres "$env:DOCUMENT_DATABASE_URL" up
```

The first migration creates the report generation tables and seeds the initial
report types:

- `summer_peak_inspection`
- `coal_inventory_audit`

`report_jobs`, `report_job_attempts`, and `report_events` are PostgreSQL
business-state tables. Redis/asynq should only carry queue payloads and task
execution coordination.

## SQLC

SQL queries live under `internal/repository/queries/`, and generated code lives
under `internal/repository/sqlc/`.

```powershell
sqlc generate
```

## Tests

```powershell
go test ./...
go build ./cmd/server
```

Repository integration tests are skipped unless `DOCUMENT_TEST_DATABASE_URL` is
set:

```powershell
$env:DOCUMENT_TEST_DATABASE_URL = "postgres://document:document@localhost:5432/document_test?sslmode=disable"
go test ./internal/repository
```
