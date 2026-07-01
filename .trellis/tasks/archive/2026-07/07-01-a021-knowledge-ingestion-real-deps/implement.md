# A-021 Implementation Plan

## Steps

1. Re-read backend specs and latest Knowledge/File/Parser/Qdrant implementation.
2. Add `TestKnowledgeIngestionRealDepsSmoke` under
   `services/knowledge/internal/integration`.
3. Add `TestGatewayKnowledgeOwnerRouteSmoke` under the same package.
4. Use a run id to isolate:
   - PostgreSQL schema: `knowledge_smoke_*`.
   - Qdrant collection: `knowledge_ingestion_smoke_*`.
   - request/document/test ids.
5. Create helpers for:
   - env loading and gate behavior;
   - applying Knowledge migrations in an isolated schema;
   - creating/deleting Qdrant collections;
   - retrieving Qdrant point payload;
   - capturing the ingestion queue payload.
6. Create Gateway owner route helpers for:
   - File/Parser/Knowledge `/readyz` probes;
   - PostgreSQL ping;
   - Redis `PING`;
   - spoofed `X-User-*` rejection without Bearer auth;
   - Gateway session creation;
   - Gateway `GET /api/v1/knowledge-bases` assertion.
7. Update docs:
   - `services/knowledge/README.md`;
   - `docs/runbooks/local-integration.md`;
   - `docs/testing/strategy.md`;
   - `docs/services/knowledge/docs/implementation.md`.
8. Update `.trellis/spec/backend/quality-guidelines.md` only if the smoke
   contract is not already represented clearly enough.

## Validation

- Default skip:
  `go test ./internal/integration -run TestKnowledgeIngestionRealDepsSmoke -count=1 -v`.
- Gateway owner route default skip:
  `go test ./internal/integration -run TestGatewayKnowledgeOwnerRouteSmoke -count=1 -v`.
- Knowledge full tests:
  `go test ./...`.
- Knowledge build:
  `go build ./cmd/server`.
- Compose config:
  `docker compose -f deploy/docker-compose.yml --env-file deploy/.env.example config --quiet`.
- Docker policy:
  `python3 scripts/check_docker_policy.py`.
- Enabled smoke when local dependencies are available:
  start PostgreSQL/File/Parser/Qdrant through latest `deploy` runbook, then run
  the gated test with `KNOWLEDGE_INGESTION_SMOKE=1`.
- Enabled Gateway owner route smoke when local dependencies are available:
  start Auth/Gateway/File/Parser/Knowledge/Redis/PostgreSQL through latest
  `deploy` runbook, then run the gated test with
  `GATEWAY_KNOWLEDGE_OWNER_SMOKE=1`.
- Whitespace:
  `git diff --check`.

## Risk Points

- Parser default backend may load OCR dependencies if the fixture type triggers
  OCR; keep the fixture as Markdown/text.
- Qdrant cleanup must run even when assertions fail.
- Do not leak tokens, DSNs, raw vectors, document text, or object keys in
  failures.
- Do not edit Docker build-source defaults as part of this task unless a new
  upstream regression is discovered and documented.
- Parser image absence blocks `docker compose up --no-build file parser knowledge`;
  document the pre-build/cache path instead of changing Docker defaults.
- The Gateway owner route smoke should not print credentials or access tokens
  when login or route calls fail.
