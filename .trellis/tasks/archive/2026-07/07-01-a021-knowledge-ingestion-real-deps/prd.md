# A-021 Knowledge ingestion real dependency smoke

## Goal

Add an env-gated Knowledge ingestion real dependency smoke on top of latest
`develop@fb2e440`. The smoke verifies one fixture document through File-backed
source content, Parser parsing, Knowledge chunking, embedding, Qdrant indexing,
and Knowledge metadata/status updates. It must be opt-in so ordinary CI and
`go test ./...` remain offline-safe.

Scope update from Gateway/Auth preflight findings: also add an env-gated
Gateway -> Knowledge owner route smoke. This smoke verifies that local
Auth/Gateway/Redis/Knowledge wiring can authenticate through Gateway and proxy
`GET /api/v1/knowledge-bases` to the Knowledge owner service with trusted auth
context injected by Gateway.

## Requirements

- Provide one clear command to run the Knowledge ingestion real dependency smoke.
- Cover at least one fixture document from source ingestion through Parser,
  Knowledge chunks, embedding, Qdrant point write, and document/job status
  updates.
- Use env gating. With `KNOWLEDGE_INGESTION_SMOKE` unset, the test must skip
  before reading credentials or making network calls.
- With the gate enabled, missing required env must fail with actionable key
  names only.
- Reuse the refreshed Docker baseline instead of reintroducing old local
  workarounds:
  - `GO_DOCKER_GOPROXY` and `GO_DOCKER_GOSUMDB`.
  - `deploy/.env.china.example`.
  - `docs/runbooks/docker-build-environment.md`.
  - `scripts/check_docker_policy.py`.
- Do not make `GOSUMDB=off` part of this task. Latest Docker policy explicitly
  keeps checksum verification enabled and uses `sum.golang.google.cn` for the
  mainland Go checksum path.
- Implement cleanup for the run-scoped Qdrant collection, isolated Knowledge
  PostgreSQL schema, and uploaded File object.
- Update Knowledge/runbook/testing docs with prerequisites, env variables,
  running order, smoke command, cleanup behavior, and common failures.
- Document Parser image build/cache prerequisites. If
  `software-teamwork-local-parser:latest` is absent, `docker compose up
  --no-build file parser knowledge` cannot start Parser; rebuilding Parser may
  need `python:3.12-slim` metadata access or a pre-pulled/cached image.
- Add a Gateway -> Knowledge owner route smoke that prechecks Parser, File,
  Redis, and PostgreSQL readiness before calling the owner route.
- The Gateway owner route smoke must create a real Gateway session and call
  `GET /api/v1/knowledge-bases` through Gateway, proving Auth context reaches
  Knowledge instead of trusting caller-supplied `X-User-*` headers.
- The Gateway owner route smoke must also reject a caller that supplies spoofed
  `X-User-*` headers without a Bearer token before running the authenticated
  positive route assertion.

## Acceptance Criteria

- [x] A documented command runs the Knowledge ingestion real dependency smoke.
- [x] The smoke asserts at least one fixture document reaches `ready`/indexed
      state with chunks persisted.
- [x] The smoke asserts embedding metadata exists for at least one chunk.
- [x] The smoke asserts Qdrant has at least one point for the ingested fixture.
- [x] Missing dependencies are skipped by default or produce clear opt-in
      failure messages.
- [x] Cleanup strategy is implemented or documented with concrete identifiers.
- [x] Ordinary `go test ./...` remains usable without real dependencies.
- [x] Documentation records prerequisites, env vars, running order, cleanup, and
      common failures without conflicting with the refreshed Docker baseline.
- [x] A documented command runs the Gateway -> Knowledge owner route smoke.
- [x] The owner route smoke checks Parser/File/Redis/PostgreSQL readiness before
      the Gateway route assertion.
- [x] The owner route smoke authenticates through Gateway and validates
      `GET /api/v1/knowledge-bases` succeeds with a Gateway request id.
- [x] The owner route smoke verifies caller-supplied `X-User-*` headers do not
      authenticate the request.
- [x] Documentation calls out Parser image build/cache prerequisites and the
      Docker Hub `python:3.12-slim` metadata failure mode.

## Out Of Scope

- Docker image version, mirror, Go proxy, checksum, or container policy changes.
- Redis/asynq delivery reliability; the test may capture the queued ingestion
  payload and call the Knowledge worker handler directly.
- Full MCP/frontend E2E, retrieval/rerank E2E, and broad Gateway smoke matrix,
  which remain owned by S-008 and follow-up tasks.
