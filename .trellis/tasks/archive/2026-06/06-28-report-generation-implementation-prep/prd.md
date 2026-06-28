# report generation implementation preparation

## Goal

Prepare the report generation feature for backend implementation by closing documentation inconsistencies, aligning the gateway OpenAPI contract with the report generation documents, and producing an implementation plan for `services/document`.

## What I Already Know

- Report generation belongs to the `document` service and implementation should live under `services/document`.
- All public HTTP calls, including frontend, admin, backend module callers, and MCP tool internal HTTP calls, must go through gateway `/api/v1`.
- Gateway is the public entrypoint and contract owner for public API shape; business state and report workflows are owned by `document`.
- The module does not implement user authentication, frontend pages, model service configuration, or admin UI.
- Stable public API paths must be RESTful resource paths and avoid action verbs such as `generate`, `regenerate`, `export`, `retry`, and `download`.
- Report generation has active gateway OpenAPI paths in `docs/api/gateway.openapi.yaml`.
- The user does not want automatic PR creation or pushing.

## Requirements

- Finalize documentation consistency across:
  - `docs/api/gateway.openapi.yaml`
  - `docs/gateway.md`
  - `docs/frontend-backend-contract.md`
  - `docs/service-boundaries.md`
  - `docs/report_generation/api_interfaces.md`
  - `docs/report_generation/data_models.md`
  - `docs/report_generation/report_generation.md`
- Ensure report generation routes are modeled as gateway `/api/v1` resources owned by `document`.
- Ensure data models distinguish database/internal fields from public API fields.
- Ensure MinIO object keys remain internal and are not exposed through public API schemas.
- Decide and apply the OperationLog public contract gap for MCP/audit fields.
- Add an implementation planning document under `docs/report_generation/`.
- The implementation plan must place backend code under `services/document` and keep gateway business-logic-free.

## Acceptance Criteria

- [ ] Documentation no longer contains stale report-generation action paths as stable public routes.
- [ ] `docs/api/gateway.openapi.yaml` parses successfully and all `$ref` targets resolve.
- [ ] Every active `/api/v1/**` report operation has `operationId`, `tags`, `summary`, responses, and `x-owner-service: document`.
- [ ] Report generation OpenAPI paths do not contain forbidden action verbs.
- [ ] Report data models cover records, outlines, sections, section versions, templates, materials, jobs, attempts, events, files, logs, and statistics.
- [ ] OperationLog public response fields match the audit/MCP requirements selected for this phase.
- [ ] `docs/report_generation/implementation_plan.md` describes service layout, persistence, workflow, MCP mapping, testing, and phased delivery for `services/document`.
- [ ] No commit, push, or PR is performed without explicit user approval.

## Definition of Done

- Documentation changes are applied.
- Contract validation scripts pass.
- `git diff --check` has no content errors; CRLF warnings are acceptable on this Windows workspace.
- Remaining risks or open questions are summarized for the user.

## Out of Scope

- Implementing Go code in `services/document`.
- Creating database migrations.
- Wiring Docker Compose services.
- Creating frontend pages.
- Creating or pushing commits.
- Creating a PR.

## Technical Approach

- Treat `docs/api/gateway.openapi.yaml` as the source of truth for public gateway API.
- Use `docs/report_generation/api_interfaces.md` as business-facing API explanation, not a competing contract.
- Use `docs/report_generation/data_models.md` for logical persistence models; database fields use snake_case and public API fields use camelCase.
- Use `docs/report_generation/implementation_plan.md` to bridge docs into the first backend implementation pass.
- Keep service implementation under `services/document`:
  - `cmd/server/` for entrypoint and dependency wiring
  - `internal/http/` for handlers and DTO mapping
  - `internal/service/` for workflows and state transitions
  - `internal/repository/` for PostgreSQL persistence
  - `internal/platform/` for MinIO, model/MCP integration clients, and other infrastructure
  - `migrations/` for document-owned tables
  - `api/` for service-local internal API notes if needed

## Technical Notes

- Relevant Trellis specs:
  - `.trellis/spec/backend/directory-structure.md`
  - `.trellis/spec/backend/api-contracts.md`
  - `.trellis/spec/backend/database-guidelines.md`
  - `.trellis/spec/backend/error-handling.md`
  - `.trellis/spec/backend/logging-guidelines.md`
  - `.trellis/spec/backend/quality-guidelines.md`
  - `.trellis/spec/guides/cross-layer-thinking-guide.md`
- Current branch has existing uncommitted documentation changes from the report generation contract work.
