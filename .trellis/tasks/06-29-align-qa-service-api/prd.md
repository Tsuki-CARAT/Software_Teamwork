# Align Gateway and QA Service APIs with Documentation

## Goal

Align the implemented Gateway-to-QA HTTP path end to end with the authoritative
contracts under `docs/`, so frontend callers can use the documented public
`/api/v1/**` resources while Gateway authenticates and forwards trusted context
to QA-owned internal endpoints.

## What I Already Know

- The public source of truth is `docs/services/gateway/api/openapi.yaml`.
- QA behavior and data semantics are documented by `docs/services/qa/README.md`,
  `docs/services/qa/api/openapi.yaml`, and `docs/services/qa/docs/data-models.md`.
- Existing Gateway and QA code still implements the legacy
  `/api/v1/qa/conversations/**`, `/messages:stream`, and `/api/v1/qa/settings`
  surface.
- The documented public contract uses `/api/v1/qa-sessions/**`; JSON and SSE
  share `POST /api/v1/qa-sessions/{sessionId}/messages` via `Accept` content
  negotiation, and event replay is a separate resource.
- Gateway owns public Bearer authentication, permission checks, request IDs,
  public envelopes, error normalization, and SSE-safe proxying. QA owns Agent
  execution, conversations/messages/runs/events/citations/config versions,
  retrieval tests, and metrics.
- QA PostgreSQL migrations already define the core tables required by the
  documented contract, but repositories and handlers currently cover only a
  subset.
- The worktree contains substantial unrelated user changes. This task must
  preserve them and only edit Gateway/QA implementation plus task artifacts.

## Requirements

- Treat every active QA-owned operation (`x-owner-service: qa`) in
  `docs/services/gateway/api/openapi.yaml` as the public contract to implement.
- Align Gateway QA route registration, permission policy, downstream path
  rewriting, trusted headers, request ID propagation, standard JSON envelopes,
  downstream error handling, and SSE passthrough.
- Align QA internal routes and its executable OpenAPI with the public resources,
  using internal paths and `X-User-Id`/service authentication rather than
  accepting frontend Bearer credentials directly.
- Implement documented session, message, SSE/event replay, response-run,
  citation, configuration-version, LLM connection-test, retrieval-test, and QA
  metrics behavior backed by the existing PostgreSQL schema.
- Preserve existing Agent/MCP runtime behavior behind the new message resource.
- Do not expose provider secrets, raw MCP schemas/results, private chain of
  thought, SQL errors, internal URLs, or service tokens.
- Remove reliance on undocumented legacy QA public paths; do not advertise
  compatibility aliases as active contract endpoints.
- Keep public JSON field names camelCase and documented enum/status values.
- Add or update tests at both Gateway and QA boundaries, including SSE and
  authorization/permission routing.
- Update `services/gateway/api/openapi.yaml`, `services/qa/api/openapi.yaml`, and
  service README examples when their implemented surface changes.

## Acceptance Criteria

- [x] All active QA-owned Gateway OpenAPI operations have registered Gateway
      routes and matching permission policy.
- [x] Gateway rewrites those public paths to QA internal paths without leaking
      caller-supplied trusted headers.
- [x] Public JSON success responses follow `{ data, requestId }`; paginated
      responses follow `{ data, page, requestId }`; errors retain the standard
      error envelope.
- [x] `POST .../messages` returns JSON normally and streams documented SSE when
      `Accept: text/event-stream` is requested.
- [x] QA implements the corresponding internal operations with ownership checks
      based on `X-User-Id`.
- [x] Event replay, response-run reads/cancellation, citations, active/config
      version creation, retrieval tests, and metric reads use existing QA-owned
      database tables.
- [x] Executable service OpenAPI files match implemented routes and DTOs.
- [x] Gateway and QA `go test ./...` pass; formatting and available static checks
      pass.
- [x] No unrelated dirty worktree files are overwritten or included in this
      task's reported change set.

## Technical Approach

- Keep Gateway thin: authenticate once, enforce route permission, map public
  path to `/internal/v1/...`, proxy JSON/SSE, and normalize public response
  envelopes at the boundary.
- Keep domain DTOs and queries in QA. Extend the current service/repository
  interfaces in coherent resource groups rather than embedding SQL in handlers.
- Use `Accept` negotiation on the single message-creation route. Persist replay
  events and response-run state as part of answer execution.
- Implement the documented gateway surface first; QA-only diagnostic resources
  that are not active in the gateway contract remain internal unless required
  by that flow.

## Decision (ADR-lite)

**Context:** The repository currently has two incompatible API generations.

**Decision:** Make the active Gateway OpenAPI the public authority, use the QA
documentation for business semantics, and make QA expose matching internal
resource routes. Legacy public paths are not retained as advertised aliases.

**Consequences:** Existing frontend code still calling `/qa/conversations` must
be migrated separately or in a follow-up frontend task. Gateway and QA become
contract-consistent, and public/internal ownership is explicit.

## Out of Scope

- Frontend API-client migration or UI changes.
- Non-QA Gateway route families.
- New provider-secret storage in QA; AI Gateway remains the owner.
- Exposing raw MCP management/tool payloads through the documented frontend API.
- Database redesign beyond a narrowly required additive migration.

## Technical Notes

- Public contract: `docs/services/gateway/api/openapi.yaml`.
- QA semantics: `docs/services/qa/README.md`.
- QA design OpenAPI: `docs/services/qa/api/openapi.yaml`.
- Data contract: `docs/services/qa/docs/data-models.md`.
- Relevant specs: `.trellis/spec/backend/index.md`, `api-contracts.md`,
  `directory-structure.md`, `database-guidelines.md`, `error-handling.md`,
  `quality-guidelines.md`, `logging-guidelines.md`, `gateway-auth-rbac.md`, and
  `mcp-agent-runtime.md`.
