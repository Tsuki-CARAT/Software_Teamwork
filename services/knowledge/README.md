# Knowledge Service

Knowledge owns knowledge-base metadata, knowledge document metadata/status,
processing trace state, and future chunk/vector lifecycle coordination.

This implementation includes the A-09 foundation slice, the A-10 document
upload handoff, the A-11 ingestion worker path, A-12 knowledge-query
retrieval, and the A-14 active-operation contract surface. Knowledge accepts
the document upload, stores raw bytes through File Service, creates durable
document/job state, enqueues ingestion work, then consumes the A10 task payload
to read source bytes, parse, chunk, embed, index chunks, expose chunk/content
reads, and run retrieval over hydrated chunks.

## Runtime

- Go module: `go 1.25.0`
- HTTP: standard `net/http` `ServeMux`
- Logging: `log/slog`
- PostgreSQL access: `pgx` + generated `sqlc` query package
- Migrations: `goose`

All landed Go services use the repository Go 1.25 baseline. Knowledge keeps the
standard `net/http` / `http.ServeMux` service shape while leaving room for later
RAG MCP server work.

## Configuration

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `DATABASE_URL` | yes | - | PostgreSQL connection string. |
| `FILE_SERVICE_BASE_URL` | yes | - | Internal File Service base URL for `/internal/v1/files`. |
| `KNOWLEDGE_REDIS_ADDR` | yes | - | Redis/asynq endpoint for ingestion task handoff. |
| `KNOWLEDGE_HTTP_ADDR` | no | `:8083` | HTTP listen address. |
| `KNOWLEDGE_SERVICE_VERSION` | no | `dev` | Version returned by readiness checks. |
| `KNOWLEDGE_ENV` | no | `local` | Runtime environment label. |
| `KNOWLEDGE_MAX_UPLOAD_BYTES` | no | `33554432` | Multipart upload limit in bytes. |
| `KNOWLEDGE_SERVICE_TOKEN` | yes | - | Internal service token forwarded to File Service. |
| `KNOWLEDGE_SHUTDOWN_TIMEOUT` | no | `10s` | Graceful shutdown timeout. |
| `PARSER_SERVICE_BASE_URL` | yes | - | Internal Parser service base URL for document parsing. |
| `PARSER_SERVICE_TOKEN` | no | - | Optional Parser service token. |
| `PARSER_SERVICE_TIMEOUT` | no | `30s` | Parser request timeout. |
| `EMBEDDING_PROVIDER` | no | `local_hashing` | Embedding provider; `ai_gateway` uses AI Gateway. |
| `EMBEDDING_MODEL` | no | `local_hashing` | Embedding model/profile label. |
| `EMBEDDING_DIMENSION` | no | `384` | Embedding vector dimension. |
| `AI_GATEWAY_BASE_URL` | no | - | AI Gateway base URL when `EMBEDDING_PROVIDER=ai_gateway`. |
| `AI_GATEWAY_SERVICE_TOKEN` | no | - | Optional AI Gateway service token. |
| `AI_GATEWAY_EMBEDDING_PROFILE_ID` | no | - | Optional AI Gateway embedding profile ID. |
| `QDRANT_URL` | no | - | Optional Qdrant REST base URL; unset uses in-memory index. |
| `QDRANT_BASE_URL` | no | - | Alias for `QDRANT_URL`. |
| `QDRANT_API_KEY` | no | - | Optional Qdrant API key. |
| `QDRANT_COLLECTION` | no | `knowledge_chunks` | Qdrant collection name. |
| `RERANK_MODEL` | rerank | - | Optional AI Gateway rerank model. When unset, rerank requests use the local no-op fallback. |
| `RERANK_PROFILE_ID` | no | - | Optional AI Gateway rerank profile id. |

When `EMBEDDING_PROVIDER=ai_gateway`, `EMBEDDING_MODEL` must match the resolved AI Gateway embedding profile `model`. If `AI_GATEWAY_EMBEDDING_PROFILE_ID` or `EMBEDDING_PROFILE_ID` is unset, AI Gateway uses its default enabled embedding profile and still validates the `model` value before calling the provider. `RERANK_MODEL` is optional; when unset, query rerank keeps the vector order as a local no-op fallback.

## Implemented Routes

Operational routes:

- `GET /healthz`
- `GET /readyz`

Internal service routes:

- `GET /internal/v1/knowledge-bases`
- `POST /internal/v1/knowledge-bases`
- `GET /internal/v1/knowledge-bases/{knowledgeBaseId}`
- `PATCH /internal/v1/knowledge-bases/{knowledgeBaseId}`
- `DELETE /internal/v1/knowledge-bases/{knowledgeBaseId}`
- `GET /internal/v1/knowledge-bases/{knowledgeBaseId}/documents`
- `POST /internal/v1/knowledge-bases/{knowledgeBaseId}/documents`
- `GET /internal/v1/documents/{documentId}`
- `GET /internal/v1/documents/{documentId}/chunks`
- `GET /internal/v1/documents/{documentId}/content`
- `POST /internal/v1/knowledge-queries`

Public gateway equivalents are documented in
`docs/services/gateway/api/public.openapi.yaml`.

## Access Context

Business routes require gateway-injected `X-User-Id`.

Supported permission strings follow the current auth docs:

- `knowledge:read`
- `knowledge:write`

Rules:

- Callers can read resources they created.
- `knowledge:read`, `knowledge:write`, `admin`, or `super_admin` can read
  broader resources.
- Create, update, and delete require `knowledge:write`, `admin`, or
  `super_admin`.
- Hidden or deleted resources return `404 not_found`.
- Authenticated callers without mutation rights receive `403 forbidden`.

## Data Model

The first migration creates:

- `knowledge_bases`
- `knowledge_documents`
- `processing_jobs`
- `document_chunks`

Document upload stores the File Service object ID only in
`knowledge_documents.file_ref`. Public document responses expose `jobId` and
document status, but never `fileRef`, File Service internal IDs, object keys, or
internal URLs.

`document_chunks` is now written by the ingestion worker. Qdrant payloads are
limited to `knowledge_base_id`, `document_id`, `chunk_id`, `chunk_index`,
`chunk_type`, `section_path`, `tags`, `metadata`, `job_id`,
`job_attempt`, and ingestion attempt markers. Knowledge query retrieval uses the
configured vector index and PostgreSQL chunk hydration; tests can still inject
fake embedder/vector adapters without real AI Gateway or Qdrant.

Knowledge base deletion is soft-delete-first:

- mark `knowledge_bases.deleted_at`;
- mark owned `knowledge_documents.deleted_at` in the same transaction for the
  PostgreSQL runtime repository;
- leave chunk/index cleanup for a future lifecycle job instead of hard-deleting
  chunks or vectors in this metadata route.

## Local Integration Notes

The default local service path uses PostgreSQL, File Service, Parser Service,
Redis/asynq, local hashing embeddings, and an in-memory vector index. This is
enough for upload handoff, worker processing, chunk listing, original content
reads, and seeded/fake-backed contract tests.

Real Qdrant and AI Gateway integration is optional:

- Leave `QDRANT_URL` empty to use the in-memory vector index.
- Set `QDRANT_URL=http://qdrant:6333` and `QDRANT_COLLECTION=knowledge_chunks`
  only after the collection exists and Qdrant is healthy.
- Leave `EMBEDDING_PROVIDER=local_hashing` for deterministic local runs.
- Set `EMBEDDING_PROVIDER=ai_gateway`, `AI_GATEWAY_BASE_URL`, and
  `AI_GATEWAY_SERVICE_TOKEN` only when the optional AI Gateway profile is
  running with a real provider credential.
- Set `RERANK_MODEL` only when AI Gateway rerank should be called. Otherwise
  `rerank=true` requests keep vector order as a local no-op fallback.

### Knowledge Ingestion Real Dependency Smoke

`TestKnowledgeIngestionRealDepsSmoke` is an opt-in integration smoke under
`internal/integration`. With `KNOWLEDGE_INGESTION_SMOKE` unset it skips before
reading dependency configuration, so ordinary `go test ./...` remains
offline-safe. When enabled, it verifies one Markdown fixture through:

- Knowledge PostgreSQL metadata in an isolated `knowledge_smoke_*` schema.
- File Service upload and content read.
- Parser Service `/internal/v1/parsed-documents`.
- Knowledge chunking, embedding, worker handler state transitions, and chunk
  persistence.
- Qdrant collection creation, point upsert, payload lookup, and cleanup.

Start the local dependency subset from the root Compose baseline:

```bash
cd deploy
cp .env.example .env
# For mainland China Docker builds, append the project-provided mirror overlay:
# cat .env.china.example >> .env
DOCKER_BUILDKIT=1 docker compose --env-file .env up -d --build postgres migrate-file file parser qdrant
```

Then run the smoke from `services/knowledge`:

```bash
KNOWLEDGE_INGESTION_SMOKE=1 \
KNOWLEDGE_TEST_DATABASE_URL='postgres://knowledge_app:knowledge_app_dev@127.0.0.1:5432/knowledge_system?sslmode=disable' \
FILE_SERVICE_BASE_URL='http://127.0.0.1:8082' \
KNOWLEDGE_SERVICE_TOKEN='local-dev-internal-service-token-change-me' \
PARSER_SERVICE_BASE_URL='http://127.0.0.1:8087' \
PARSER_SERVICE_TOKEN='local-dev-internal-service-token-change-me' \
QDRANT_URL='http://127.0.0.1:6333' \
EMBEDDING_PROVIDER=local_hashing \
EMBEDDING_MODEL=local_hashing \
EMBEDDING_DIMENSION=384 \
go test ./internal/integration -run '^TestKnowledgeIngestionRealDepsSmoke$' -count=1 -v
```

The test creates a run-scoped Qdrant collection named
`knowledge_ingestion_smoke_*`, uploads one File Service object, and creates a
PostgreSQL schema named `knowledge_smoke_*`. Test cleanup deletes all three. If
the process is interrupted, remove leftover Qdrant collections with the same
prefix and drop leftover PostgreSQL schemas after checking no other local smoke
is using them.

Optional AI Gateway embedding can be tested by also starting the `ai` profile,
creating a usable embedding profile/provider credential, and setting
`EMBEDDING_PROVIDER=ai_gateway`, `AI_GATEWAY_BASE_URL`,
`AI_GATEWAY_SERVICE_TOKEN`, `AI_GATEWAY_EMBEDDING_PROFILE_ID` or
`EMBEDDING_PROFILE_ID`, `EMBEDDING_MODEL`, and `EMBEDDING_DIMENSION`.

### Gateway -> Knowledge Owner Route Smoke

`TestGatewayKnowledgeOwnerRouteSmoke` is a separate opt-in smoke for the
Gateway owner route boundary. With `GATEWAY_KNOWLEDGE_OWNER_SMOKE` unset it
skips before reading env. When enabled, it prechecks File, Parser, Knowledge,
PostgreSQL, and Redis readiness, verifies that unauthenticated caller-supplied
`X-User-*` headers are rejected, then creates a real Gateway session and calls
`GET /api/v1/knowledge-bases` through Gateway. Knowledge rejects missing trusted
user context, so the spoofed-header `401` plus authenticated `200` response with
the supplied request id proves Gateway authenticated through Auth/session cache
and injected owner-service context into Knowledge instead of trusting inbound
`X-User-*` headers.

Parser image availability is a precondition when starting the local stack with
`--no-build`. If `software-teamwork-local-parser:latest` is absent, Docker
cannot start `parser`; rebuilding it may need `python:3.12-slim` metadata from
Docker Hub or the mirror/registry rewrite documented in
`docs/runbooks/docker-build-environment.md`. Prefer one of these before running
the owner smoke:

```bash
cd deploy
cp .env.example .env
# For mainland China Docker builds, append the project-provided mirror overlay:
# cat .env.china.example >> .env
DOCKER_BUILDKIT=1 docker compose --env-file .env build parser
DOCKER_BUILDKIT=1 docker compose --env-file .env up -d --build gateway
```

Then run:

```bash
GATEWAY_KNOWLEDGE_OWNER_SMOKE=1 \
GATEWAY_BASE_URL='http://127.0.0.1:8080' \
KNOWLEDGE_SERVICE_BASE_URL='http://127.0.0.1:8083' \
FILE_SERVICE_BASE_URL='http://127.0.0.1:8082' \
PARSER_SERVICE_BASE_URL='http://127.0.0.1:8087' \
KNOWLEDGE_TEST_DATABASE_URL='postgres://knowledge_app:knowledge_app_dev@127.0.0.1:5432/knowledge_system?sslmode=disable' \
KNOWLEDGE_REDIS_ADDR='127.0.0.1:6379' \
GATEWAY_SMOKE_USERNAME='admin' \
GATEWAY_SMOKE_PASSWORD='LocalDemoAdmin#12345' \
go test ./internal/integration -run '^TestGatewayKnowledgeOwnerRouteSmoke$' -count=1 -v
```

`GET /internal/v1/documents/{documentId}/content` validates the Knowledge-owned
document first, then reads the raw bytes from File Service internally. It never
exposes `file_ref`, object keys, File Service IDs, or storage URLs in JSON
responses.

## Migrations

Apply the service-owned migration with the project-pinned `goose@v3.27.1` command:

```bash
go run github.com/pressly/goose/v3/cmd/goose@v3.27.1 -dir migrations postgres "$DATABASE_URL" up
```
## Development

```bash
go test ./...
go build ./cmd/server
```

Contract tests under `internal/http` use seeded repositories and fake file,
vector, and embedding adapters, matching the decoupling rule in
`docs/services/knowledge/docs/api-contract.md` section 2.6. The env-gated
Knowledge ingestion smoke above covers full upload -> File -> Parser ->
worker -> embedding -> Qdrant indexing for one fixture document. The Gateway
owner route smoke covers Auth/Gateway context injection into Knowledge for
`GET /api/v1/knowledge-bases`; retrieval, rerank, MCP, and frontend end-to-end
checks remain separate follow-up scopes.

Regenerate the query package from `sqlc.yaml` after changing SQL files:

```bash
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
```
