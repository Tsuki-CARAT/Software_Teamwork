# Auth and Gateway Service Test Audit

## Goal

Design and execute a focused but cross-service-aware test audit for `services/auth`,
`services/gateway`, and the gateway-facing capabilities that prove whether these
services can support the wider system. Persist the test plan, execution evidence,
results, gaps, and follow-up test ideas under `docs/tests/0701/`.

## Confirmed Facts

- `docs/testing/strategy.md` identifies backend cross-service smoke coverage as
  missing and names Auth -> Gateway -> Domain as a required future smoke target.
- `docs/services/auth/docs/implementation.md` says Auth unit and migration smoke
  had prior records, but Gateway/Auth/Redis end-to-end smoke was not run.
- `docs/services/gateway/docs/implementation.md` says Gateway has route matrix,
  auth proxy, Redis session cache, proxy header, binary/SSE proxy, middleware,
  and metrics tests, but lacks real Redis/downstream integration proof.
- `docs/services/gateway/docs/active-api-owner-map.md` lists 97 active
  operations, including 4 Auth-owned public operations and owner proxy routes for
  Knowledge, Document, QA, and AI Gateway.
- `deploy/docker-compose.yml` provides a local integration baseline with
  PostgreSQL, Redis, Auth, Gateway, Knowledge, File, Parser, QA, Document, and an
  optional `ai` profile for AI Gateway.
- `deploy/.env.example` declares local demo credentials, including
  `LOCAL_ADMIN_USERNAME=admin` and `LOCAL_ADMIN_PASSWORD=LocalDemoAdmin#12345`.

## Requirements

- R1: Create `docs/tests/0701/` and write a test report that starts from
  requirements documents, implementation documents, and machine-readable API
  contracts.
- R2: Classify tests by layer and risk: documentation/contract, unit/package,
  build, migration/config, Auth/Gateway integration, and system-facing
  cross-module smoke.
- R3: Execute available local checks instead of only proposing them. At minimum,
  run Auth and Gateway service-local tests and builds, Gateway active API
  verifier, and whitespace/doc sanity checks.
- R4: Attempt a real local integration smoke that covers login/session creation,
  Gateway Redis session behavior, `/api/v1/users/me`, logout/session deletion,
  and at least one authenticated owner-proxy route when the local environment can
  be brought up.
- R5: Record every command result in the same report with concrete pass/fail/skip
  status, timestamp or run context, and residual risk.
- R6: Add follow-up test ideas discovered during execution, especially when an
  environment limitation blocks a desired smoke.
- R7: Do not change Auth or Gateway production behavior unless test execution
  reveals a small, necessary test harness/reporting fix that directly supports
  this audit.

## Acceptance Criteria

- [x] `docs/tests/0701/auth-gateway-test-report.md` exists and contains source
      documents, test taxonomy, planned cases, command log, observed results,
      findings, and follow-up test ideas.
- [x] Auth and Gateway `go test ./...` results are recorded.
- [x] Auth and Gateway `go build ./cmd/server` results are recorded.
- [x] Gateway contract verification results are recorded.
- [x] Auth migration/apply or Compose migration smoke result is recorded, either
      as pass or explicitly skipped/failed with environment evidence.
- [x] Auth/Gateway/Redis/system smoke result is recorded, either as pass or
      explicitly blocked/skipped with evidence and residual risk.
- [x] `git diff --check` is run and recorded before final reporting.
