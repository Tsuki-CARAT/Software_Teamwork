# A-021 Design

## Architecture

Add a service-local Go integration test under `services/knowledge/internal/integration`.
The test lives inside the Knowledge module so it can instantiate Knowledge's
repository, File client, Parser client, embedding adapter, Qdrant adapter,
service, and worker handler without importing another service's `internal`
packages.

Add a second service-local black-box smoke under the same package for the
Gateway -> Knowledge owner route. It uses only HTTP/TCP/PostgreSQL boundaries:
Gateway session creation, Gateway public Knowledge route, File/Parser/Knowledge
readiness endpoints, Redis PING, and PostgreSQL ping.

External dependencies stay behind documented HTTP or database boundaries:

- Knowledge PostgreSQL uses `KNOWLEDGE_TEST_DATABASE_URL`.
- File Service is called through `FILE_SERVICE_BASE_URL` and
  `KNOWLEDGE_SERVICE_TOKEN`.
- Parser Service is called through `PARSER_SERVICE_BASE_URL` and optional
  `PARSER_SERVICE_TOKEN`.
- Qdrant is called through `QDRANT_URL` or `QDRANT_BASE_URL`.
- AI Gateway embedding is optional and selected only when
  `EMBEDDING_PROVIDER=ai_gateway`.
- Gateway owner route smoke uses `GATEWAY_BASE_URL`, `GATEWAY_SMOKE_USERNAME`,
  `GATEWAY_SMOKE_PASSWORD`, `KNOWLEDGE_SERVICE_BASE_URL`,
  `FILE_SERVICE_BASE_URL`, `PARSER_SERVICE_BASE_URL`,
  `KNOWLEDGE_TEST_DATABASE_URL`, and `KNOWLEDGE_REDIS_ADDR`.

## Data Flow

1. Gate on `KNOWLEDGE_INGESTION_SMOKE=1`; skip before reading env when unset.
2. Load required env and fail with missing key names only.
3. Create a run-scoped Qdrant collection.
4. Create an isolated PostgreSQL schema and apply Knowledge migrations in that
   schema.
5. Construct the production repository, File client, Parser client, embedder,
   Qdrant vector index, Knowledge service, and worker handler.
6. Create a test knowledge base and upload a small Markdown fixture through
   Knowledge `UploadDocument`.
7. Capture the queued ingestion payload and pass it directly to the worker
   handler. This proves the ingestion data chain without making Redis/asynq
   delivery reliability part of A-021.
8. Assert document `ready`, current job `succeeded`, chunks persisted, embedding
   metadata present, Qdrant point id present, and Qdrant payload contains the
   expected knowledge base/document/chunk ids.
9. Cleanup File object, Qdrant collection, and PostgreSQL schema.

## Gateway Owner Route Data Flow

1. Gate on `GATEWAY_KNOWLEDGE_OWNER_SMOKE=1`; skip before reading env when unset.
2. Load required env and fail with missing key names only.
3. Precheck readiness before the owner route assertion:
   - `GET <FILE_SERVICE_BASE_URL>/readyz`;
   - `GET <PARSER_SERVICE_BASE_URL>/readyz`;
   - `GET <KNOWLEDGE_SERVICE_BASE_URL>/readyz`;
   - PostgreSQL ping using `KNOWLEDGE_TEST_DATABASE_URL`;
   - Redis `PING` using `KNOWLEDGE_REDIS_ADDR`.
4. Before authenticated access, call
   `GET <GATEWAY_BASE_URL>/api/v1/knowledge-bases` with spoofed `X-User-*`
   headers and no Bearer token, and assert Gateway returns `401 unauthorized`.
5. Call `POST <GATEWAY_BASE_URL>/api/v1/sessions` with seeded local credentials.
6. Call `GET <GATEWAY_BASE_URL>/api/v1/knowledge-bases` with the returned Bearer
   token, a request id, and a spoofed inbound `X-User-Id` header.
7. Assert `200`, JSON envelope `requestId`, and an array `data` field. Knowledge
   rejects missing trusted user context, so this black-box path proves Gateway
   authenticated through Auth/session cache and injected owner-service context.

## Compatibility

- No public or internal Knowledge API semantics change.
- HTTP handlers do not import pgx, generated sqlc, Qdrant, Parser internals, or
  File internals.
- Dockerfile and Compose build-source behavior stays with the refreshed upstream
  Docker baseline. This task only documents how to start dependencies for the
  smoke.
- The Gateway route smoke does not import Gateway/Auth internals and does not
  expand into a broad Gateway route matrix.

## Trade-Offs

- Direct worker handler invocation is intentional. It keeps the smoke
  deterministic and scoped to Knowledge ingestion data correctness. Redis/asynq
  delivery remains covered by queue handoff tests and future S-008 E2E work.
- The fixture is Markdown/text. Parser owns PaddleOCR model quality and resource
  smoke; Knowledge only needs proof that it calls the Parser HTTP boundary and
  consumes normalized parsed content.
- Local hashing embedding is the default to avoid real provider credentials.
  AI Gateway embedding remains opt-in for controlled environments.
- The owner route smoke uses `GET /api/v1/knowledge-bases` rather than upload or
  query routes. It is the smallest stable Knowledge owner route that verifies
  Auth/Gateway context propagation without requiring Parser/OCR work during the
  route assertion itself.

## Operational Notes

- Use latest Docker runbook defaults. For mainland local builds, use
  `deploy/.env.china.example`; do not add `GOSUMDB=off`.
- Parser image availability is a precondition for `--no-build` smoke startup.
  If `software-teamwork-local-parser:latest` is absent, pre-build/pull Parser
  with the documented Docker mirror path; otherwise Docker may block on
  `python:3.12-slim` metadata from Docker Hub.
- If enabled smoke is not runnable locally, record the exact dependency blocker
  and residual risk in PR verification.
