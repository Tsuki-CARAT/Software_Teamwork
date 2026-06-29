# Document service baseline

## Goal

Implement issue #97 (`C-01`) by creating the initial `services/document` Go service baseline for report generation. The first slice must make the service independently runnable, add the report-generation database schema, provide configuration and health checks, and establish repository tests without implementing the actual AI/DOCX generation workflow.

## Requirements

- Create an independent Go module under `services/document`.
- Add `cmd/server`, `internal/config`, `internal/http`, `internal/service`, `internal/repository`, `internal/worker`, `migrations`, and `sqlc.yaml`.
- Use the repository's backend baseline: Go service-local module, `net/http` / `http.ServeMux`, `log/slog`, `pgx/v5`, `sqlc`, and `goose` migrations.
- Add startup configuration validation for HTTP address, PostgreSQL DSN, Redis/asynq address, file service base URL, AI Gateway base URL/profile reference, and DOCX toolchain commands.
- Provide `GET /healthz` and `GET /readyz`.
- Create report-generation schema migrations for:
  - `report_types`
  - `report_templates`
  - `report_template_materials`
  - `report_materials`
  - `reports`
  - `report_outlines`
  - `report_sections`
  - `report_section_versions`
  - `report_jobs`
  - `report_job_attempts`
  - `report_events`
  - `report_files`
  - `report_operation_logs`
- PostgreSQL must be the durable authority for `ReportJob`, `ReportJobAttempt`, and `ReportEvent`; Redis/asynq may only be represented as queue/dependency configuration and task identifiers.
- Add initial repository methods/tests for baseline persistence, prioritizing creation and lookup of report types, reports, jobs, attempts, and events.
- Update `services/document/README.md` with local startup, configuration, migration, and test commands.

## Acceptance Criteria

- [ ] `services/document/go test ./...` passes.
- [ ] `services/document/go build ./cmd/server` passes.
- [ ] Document service can start with valid local configuration.
- [ ] Health endpoint returns a stable JSON envelope.
- [ ] Readiness endpoint validates configured dependencies enough for the baseline slice.
- [ ] Goose migration SQL can be applied to an empty PostgreSQL database.
- [ ] Repository tests cover baseline report/job persistence behavior.
- [ ] Redis/asynq is not treated as the business source of truth for report jobs or events.
- [ ] README documents local startup and migration workflow.

## Definition Of Done

- Implementation stays inside `services/document` except for necessary repo-level or documentation updates.
- No public gateway route changes unless implementation discovers a contract mismatch that must be fixed first.
- No actual outline generation, content generation, section regeneration, file export, MCP tool, or model call workflow is implemented in this issue.
- No MinIO object key, file internal ID, prompt, provider raw error, or secret is exposed in responses or logs.
- PR targets the main repository `develop` branch and references issue #97 with an automatic closing keyword.

## Technical Approach

- Follow the existing service-local layout used by `services/file`, `services/knowledge`, and `services/qa`.
- Prefer `pgx/v5` + `pgxpool` for new code, consistent with the newer QA repository implementation.
- Keep HTTP operational routes small and project-envelope compatible.
- Keep domain types in `internal/service`; keep PostgreSQL access in `internal/repository`.
- Add `sqlc.yaml` and query files as the contract scaffold. If generated code is not required for the first minimal repository methods, still keep the layout ready for future sqlc generation.
- Use forward migration SQL under `services/document/migrations/0001_create_report_generation_tables.sql`.
- Use integration-style repository tests gated by `DOCUMENT_TEST_DATABASE_URL`; tests skip when the variable is not set, matching existing service patterns.

## Decision

**Context**: Issue #97 asks for a service/data baseline rather than the full generation workflow.

**Decision**: Build the minimal runnable service plus PostgreSQL schema and repository contract first, with dependency configuration validation and operational endpoints. Keep generation, file creation, AI calls, and worker execution as stubs or out of scope.

**Consequences**: The PR gives later document tasks a stable database and service module to build on, while avoiding a risky one-shot implementation of generation behavior.

## Out Of Scope

- Actual AI outline/content generation.
- DOCX rendering/export execution.
- File service upload/download integration.
- AI Gateway model invocation.
- asynq worker task handlers beyond baseline package/placeholder structure.
- Gateway proxy implementation changes.
- Frontend work.

## Technical Notes

- GitHub issue: https://github.com/Sakayori-Iroha-168/Software_Teamwork/issues/97
- Recommended branch from issue: `PrimeTeam/feat/document-service-baseline`.
- Issue references `docs/services/document/docs/implementation-plan.md`, but that file is absent on current `upstream/develop`; current implementation follows existing `docs/services/document/README.md`, `docs/services/document/docs/data-models.md`, and `docs/architecture/technology-decisions.md`.
- `services/document` currently contains only `.gitkeep`.
- Relevant docs read:
  - `docs/services/document/README.md`
  - `docs/services/document/docs/data-models.md`
  - `docs/architecture/technology-decisions.md`
  - `docs/architecture/service-boundaries.md`
  - `docs/architecture/frontend-backend-contract.md`
  - `.trellis/spec/backend/*`
- Existing patterns inspected:
  - `services/file`
  - `services/knowledge`
  - `services/qa`
