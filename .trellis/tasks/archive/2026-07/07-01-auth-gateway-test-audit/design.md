# Test Audit Design

## Scope

The audit focuses on `services/auth`, `services/gateway`, and gateway-facing
system behavior that depends on Auth identity/session data. It may touch
Knowledge, QA, Document, File, Parser, AI Gateway, PostgreSQL, and Redis only as
integration dependencies needed to prove the Auth/Gateway boundary.

## Evidence Sources

- Expected behavior:
  - `docs/services/auth/README.md`
  - `docs/services/gateway/README.md`
  - `docs/architecture/service-boundaries.md`
  - `docs/architecture/frontend-backend-contract.md`
  - `docs/testing/strategy.md`
- Implementation status:
  - `docs/services/auth/docs/implementation.md`
  - `docs/services/gateway/docs/implementation.md`
  - service code under `services/auth` and `services/gateway`
- Contracts:
  - `docs/services/gateway/api/public.openapi.yaml`
  - `docs/services/gateway/docs/active-api-owner-map.md`
  - `docs/services/auth/api/internal.openapi.yaml`
  - service-local `api/openapi.yaml` copies where present
- Local runtime:
  - `deploy/docker-compose.yml`
  - `deploy/.env.example`
  - `deploy/seeds/*.sql`

## Test Layers

1. Documentation and contract consistency:
   - Gateway active path coverage and REST/resource naming.
   - Auth/Gateway implementation documents match current code and test status.
2. Service-local correctness:
   - Auth package tests for validation, argon2id, token hash, sessions, service
     token, and repository mapping.
   - Gateway package tests for route matrix, auth proxy/cache behavior, request
     ID, error envelope, proxy headers, admin authorization, binary/SSE proxy,
     and middleware.
3. Build and runtime wiring:
   - `go build ./cmd/server` for both services.
   - Docker Compose config validation for deploy wiring.
4. Persistence and migration:
   - Auth migrations apply through local Compose or direct goose command.
5. Cross-service smoke:
   - Bring up the local deploy stack when possible.
   - Authenticate through Gateway using seeded admin credentials or by creating a
     user through Gateway.
   - Verify `/api/v1/users/me` returns the cached/authority-refreshed user.
   - Verify at least one authenticated owner-proxy route receives Auth context.
   - Verify logout invalidates the current session.

## Reporting Contract

The report under `docs/tests/0701/` is the durable deliverable. It must include:

- tested commit/branch and environment notes;
- planned test matrix before execution;
- command log with pass/fail/skip;
- observations and risks;
- follow-up test backlog.

No production behavior changes are expected.
