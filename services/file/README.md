# File Service

`services/file` is the first runnable Go module for base file-object upload, metadata, deletion, and original content lookup. It is an internal foundation service for owner services to call; it does not own knowledge ingestion, knowledge document state, chunks, indexing, QA, report templates, report materials, report files, or report workflows.

Public frontend routes remain owned by gateway and are documented in `docs/services/gateway/api/openapi.yaml`. Frontend callers must not call this service directly. Stable file capability must be reached through gateway `/api/v1/**` resources owned by `knowledge` or `document`, while those owner services reuse this service's internal base file APIs.

The implemented internal contract is generic file-object shaped (`/internal/v1/files/**`). The knowledge-document routes remain available only for compatibility and should not be extended for report templates, report materials, generated report files, or new knowledge business metadata.

## Current Scope

Implemented now:

- `GET /healthz`
- `GET /readyz`
- `POST /internal/v1/files`
- `GET /internal/v1/files/{fileId}`
- `DELETE /internal/v1/files/{fileId}`
- `GET /internal/v1/files/{fileId}/content`
- `POST /internal/v1/knowledge-bases/{knowledgeBaseId}/documents`
- `GET /internal/v1/documents/{documentId}`
- `PATCH /internal/v1/documents/{documentId}`
- `DELETE /internal/v1/documents/{documentId}`
- `GET /internal/v1/documents/{documentId}/content`
- Memory, local, and MinIO object-store adapters behind `service.ObjectStore`


Out of scope for this MVP:

- Local MinIO server / `mc` setup
- Async object cleanup worker
- Knowledge ingestion handoff and knowledge document state
- Report template, report material, and generated report file business state
- Public knowledge-owned document list/detail/chunks/content contracts

## Local Run

```powershell
go test ./...
go build ./cmd/server
$env:FILE_HTTP_ADDR=':8082'; go run ./cmd/server
```

By default, metadata uses the in-memory repository for tests and local development.
To run with durable PostgreSQL metadata, apply the migration, set `FILE_DATABASE_URL`,
and configure a service token:

```powershell
cd services/file
$env:FILE_DATABASE_URL = "postgres://file:file@localhost:5432/file?sslmode=disable"
$env:FILE_INTERNAL_SERVICE_TOKEN = "local-file-service-token"
go run ./cmd/server
```

Business endpoints require gateway context headers for local testing:

```text
X-Request-Id: req_local
X-User-Id: usr_local
X-User-Roles: admin
X-User-Permissions: document:upload,document:read,document:update,document:delete
```

The service enforces the permission header for business routes. Missing user
context returns `401 unauthorized`; missing operation permission returns
`403 forbidden`.

When `FILE_INTERNAL_SERVICE_TOKEN` or `INTERNAL_SERVICE_TOKEN` is configured,
base file routes under `/internal/v1/files/**` also require `X-Service-Token`.
Invalid or missing service tokens return `401 unauthorized`.

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `FILE_HTTP_ADDR` | `:8082` | HTTP listen address. |
| `FILE_MAX_UPLOAD_BYTES` | `33554432` | Multipart upload limit in bytes. |
| `FILE_STORAGE_BACKEND` | `memory` | Supported values: `memory`, `local`, `minio`. |
| `FILE_LOCAL_STORAGE_DIR` | `.file-storage` | Local object-store root when `FILE_STORAGE_BACKEND=local`. |
| `FILE_MINIO_ENDPOINT` | empty | Required when `FILE_STORAGE_BACKEND=minio`; host and port without scheme. |
| `FILE_MINIO_ACCESS_KEY` | empty | Required when `FILE_STORAGE_BACKEND=minio`; never returned in responses. |
| `FILE_MINIO_SECRET_KEY` | empty | Required when `FILE_STORAGE_BACKEND=minio`; never returned in responses or logs. |
| `FILE_MINIO_BUCKET` | empty | Required when `FILE_STORAGE_BACKEND=minio`; internal storage detail. |
| `FILE_MINIO_USE_SSL` | `false` | Whether the MinIO endpoint uses TLS. |
| `FILE_MINIO_REGION` | empty | Optional MinIO/S3 region. |
| `FILE_MINIO_TIMEOUT` | `10s` | Per-request MinIO client timeout. |
| `FILE_DATABASE_URL` | empty | Enables PostgreSQL metadata repository when set; empty keeps memory metadata mode. |
| `FILE_INTERNAL_SERVICE_TOKEN` | empty | Required when `FILE_DATABASE_URL` is set. Validates `X-Service-Token` for `/internal/v1/files/**`. |
| `INTERNAL_SERVICE_TOKEN` | empty | Shared fallback token when `FILE_INTERNAL_SERVICE_TOKEN` is empty. |
| `FILE_SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown timeout. |

## Storage Port

Object storage is behind `service.ObjectStore`. The `memory` adapter exists only for tests and early local integration. The `local` adapter stores objects under `FILE_LOCAL_STORAGE_DIR` for local durable smoke tests. The `minio` adapter uses the official `github.com/minio/minio-go/v7@v7.2.1` SDK and expects an existing MinIO or S3-compatible endpoint.

Storage adapters do not expose object keys, bucket names, storage paths, internal URLs, access keys, or secret keys through API responses. MinIO SDK usage stays inside `internal/platform/storage` and `cmd/server` wiring; `internal/http` handlers and service use cases continue to depend on the `service.ObjectStore` port.

## Metadata Port

File metadata is behind the service repository port. The runtime uses the memory repository when `FILE_DATABASE_URL` is empty and switches to the PostgreSQL repository when `FILE_DATABASE_URL` is configured. PostgreSQL stores only base file metadata such as file id, display filename, content type, size, checksum, storage reference, created timestamp, delete request timestamp, purge timestamp, and safe failure summary. Knowledge-base IDs, report IDs, template IDs, material IDs, business tags, processing status, and ACLs belong to their owner services.


## Migrations

The contract migration under `migrations/` is applied with the project-pinned `goose@v3.27.1` command:

```powershell
cd services/file
$env:FILE_DATABASE_URL = "postgres://file:file@localhost:5432/file?sslmode=disable"
go run github.com/pressly/goose/v3/cmd/goose@v3.27.1 -dir migrations postgres "$env:FILE_DATABASE_URL" up
```

Repository smoke tests are env-gated and use an isolated schema:

```powershell
cd services/file
$env:FILE_TEST_DATABASE_URL = "postgres://file:file@localhost:5432/file?sslmode=disable"
go test ./internal/repository
```
## Multipart Upload Shape

Upload uses `multipart/form-data`:

- `file`: required binary part
- `checksumSha256`: optional SHA-256 checksum for `/internal/v1/files`; when omitted, the service computes it
- `tags`: optional repeated fields for compatibility document uploads, for example `tags=policy` and `tags=inspection`

## Response Shape

JSON success responses use:

```json
{
  "data": {},
  "requestId": "req_123"
}
```

JSON errors use:

```json
{
  "error": {
    "code": "validation_error",
    "message": "request validation failed",
    "requestId": "req_123"
  }
}
```

Internal metadata responses include base file fields such as `contentType`, `sizeBytes`, and checksum for owner-service integration. They never expose bucket names, object keys, internal storage URLs, or storage credentials.

Content reads return raw bytes on success and the same JSON error envelope on failure.
