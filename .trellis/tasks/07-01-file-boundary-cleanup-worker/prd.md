# File 基础对象边界与清理 worker 收口

## Goal

Complete GitHub issue #341 / S-035 by tightening the File Service base-object boundary, adding contract drift protection, and making deleted-object cleanup observable, retryable, and safe.

## Background

- Source issue: https://github.com/Sakayori-Iroha-168/Software_Teamwork/issues/341
- Affected module: `services/file`.
- Authoritative docs:
  - `docs/tests/0701/file-module-test-report.md`
  - `docs/services/file/docs/implementation.md`
  - `docs/services/file/README.md`
  - `docs/services/file/docs/data-models.md`
  - `docs/services/file/api/internal.openapi.yaml`
  - `docs/architecture/service-boundaries.md`
  - `docs/architecture/technology-decisions.md`
- Dependencies listed by issue: #79, #80, #154, #235, #286.
- Blocking downstream issue: #342.

## Requirements

- Preserve and extend the existing File PostgreSQL + MinIO smoke baseline; do not reimplement already-covered upload/read/delete paths.
- Add automated contract/schema drift checks between `docs/services/file/api/internal.openapi.yaml` and `services/file/api/openapi.yaml`.
- Cover service-token failure modes for missing token, wrong token, and owner-service direct calls that do not propagate the token.
- Remove, hide, or explicitly quarantine any legacy knowledge-document compatibility routes and repository methods. If short-term compatibility remains, document callers, exit conditions, and tests.
- Implement or converge `file:object:purge` cleanup so deleted metadata state, object cleanup, retry behavior, and sanitized failure summaries are traceable.
- Resolve `sqlc` drift by generating current query code with the project baseline or by explicitly removing unused sqlc entry points.
- Add File internal contract/integration tests for `/internal/v1/files/**`, PostgreSQL metadata behavior, MinIO/local storage, service token handling, read-after-delete behavior, and error mapping.
- Ensure responses, logs, and test output do not expose buckets, object keys, internal URLs, access keys, secret keys, local absolute storage paths, or sensitive file contents.

## Acceptance Criteria

- [ ] `docs/services/file/api/internal.openapi.yaml` and `services/file/api/openapi.yaml` have automated consistency coverage.
- [ ] The `FILE_MINIO_POSTGRES_SMOKE=1 ... TestFileMinIOPostgresSmoke` baseline is preserved and its result is recorded.
- [ ] Missing service token, wrong token, and owner-service calls without propagated token return `401 unauthorized` without leaking token or storage details.
- [ ] `cd services/file && go test ./...` passes.
- [ ] `/internal/v1/files/**` is the only new or recommended File internal base-object surface; legacy knowledge-document routes are removed or explicitly guarded.
- [ ] Delete and purge are idempotent, retryable after failure, and never re-expose deleted objects.
- [ ] PostgreSQL metadata, MinIO/local storage, service token, and error envelope behavior have automated tests or env-gated smoke notes.
- [ ] Responses, logs, and test output avoid object keys, internal URLs, storage credentials, and sensitive file content.
- [ ] `sqlc` commands or generated artifacts match the `docs/architecture/technology-decisions.md` version baseline.

## Out Of Scope

- Do not move knowledge documents, report templates, report materials, or report file business state into File Service.
- Do not implement Knowledge-owned delete side-effect orchestration; that belongs to #342.
- Do not duplicate the #286 MinIO + PostgreSQL smoke baseline beyond necessary extensions.
