# File Service

`services/file` is the first runnable Go module for file-owned upload, metadata, deletion, and original content lookup. It is a domain service for gateway and other modules to call; it does not own knowledge ingestion, chunks, indexing, QA, or report workflows.

Public frontend routes remain owned by gateway and are documented in `docs/api/gateway.openapi.yaml`. This service exposes internal RESTful routes under `/internal/v1/**` for gateway and future backend integration.

The implemented MVP route is still knowledge-document shaped. Report templates, report materials, and generated report files are document-owned business resources; they should use a future generic file-object internal API rather than reusing `/internal/v1/knowledge-bases/{knowledgeBaseId}/documents`.

## Current Scope

Implemented now:

- `GET /healthz`
- `GET /readyz`
- `POST /internal/v1/knowledge-bases/{knowledgeBaseId}/documents`
- `GET /internal/v1/documents/{documentId}`
- `PATCH /internal/v1/documents/{documentId}`
- `DELETE /internal/v1/documents/{documentId}`
- `GET /internal/v1/documents/{documentId}/content`

Out of scope for this MVP:

- Local MinIO setup
- Production MinIO adapter
- PostgreSQL repository
- Knowledge ingestion handoff
- Generic file-object internal API for report templates, report materials, and generated report files
- Public knowledge-owned document list/detail/chunks contracts

## Local Run

```powershell
go test ./...
go build ./cmd/server
$env:FILE_HTTP_ADDR=':8082'; go run ./cmd/server
```

Business endpoints require gateway context headers for local testing:

```text
X-Request-Id: req_local
X-User-Id: usr_local
X-User-Roles: admin
X-User-Permissions: document:upload,document:read,document:update,document:delete
```

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `FILE_HTTP_ADDR` | `:8082` | HTTP listen address. |
| `FILE_MAX_UPLOAD_BYTES` | `33554432` | Multipart upload limit in bytes. |
| `FILE_STORAGE_BACKEND` | `memory` | Only `memory` is implemented in this MVP. |
| `FILE_SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown timeout. |

## Storage Port

Object storage is behind `service.ObjectStore`. The current `memory` adapter exists only for tests and early local integration. It is not a MinIO replacement and does not provide durability across process restarts.

A future MinIO adapter should be added under `internal/platform/storage/minio` and wired through `internal/config` without changing `internal/http` handlers or service use cases.

## Metadata Port

File metadata is behind `service.DocumentRepository`. The current memory repository supports handler tests and local smoke testing. A future PostgreSQL implementation should live under `internal/repository` and add real migrations under `migrations/`.

## Multipart Upload Shape

Upload uses `multipart/form-data`:

- `file`: required binary part
- `tags`: optional repeated fields, for example `tags=policy` and `tags=inspection`

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

Internal metadata responses include file-owned fields such as `contentType` and `sizeBytes` for gateway and module integration. They never expose bucket names, object keys, internal storage URLs, or storage credentials.

Content reads return raw bytes on success and the same JSON error envelope on failure.
