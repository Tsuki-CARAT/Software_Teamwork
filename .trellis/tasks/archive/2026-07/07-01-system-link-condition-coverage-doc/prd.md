# 制作系统链路条件覆盖文档

## Goal

Create a cross-service link/flow document based on the system requirements and current `develop` docs. The document should record the possible service chains users and administrators can trigger, with condition-level coverage for success paths, authorization branches, dependency failures, async states, and not-yet-implemented boundaries.

The user value is to make the microservice collaboration model inspectable: developers, testers, and reviewers should be able to see which services participate in each workflow, what each dependency provides, what branches must be handled, and which paths are current gaps rather than implemented capabilities.

## Confirmed Facts

- `origin/develop` was checked during implementation and is at `fb2e440`.
- The architecture docs define eight service boundaries: `gateway`, `auth`, `file`, `knowledge`, `parser`, `qa`, `document`, and `ai-gateway`.
- Frontend-facing calls must enter `gateway` through `/api/v1/**`; internal service APIs live under `/internal/v1/**` or service-local paths.
- Cross-service standards and service boundaries belong in `docs/architecture/`; service README files should not duplicate cross-service rules.
- Current stable or draft contracts include Auth, File, Knowledge, Parser, Document, QA, and AI Gateway capabilities. The remaining explicitly missing public contract is management overview / cross-service metrics aggregation.
- Current capability status is mixed:
  - `auth` and core `ai-gateway` model/profile capabilities are implemented.
  - `gateway`, `file`, `knowledge`, `parser`, `qa`, and `document` have partial implementations with important smoke/E2E gaps.
  - Document `summer_peak_inspection` basic AI outline/content generation is implemented; Document MCP tools, more report-type generation strategies, full QA RAG/citation smoke, true provider smoke, true Parser OCR smoke, and one-click cross-service E2E remain gaps.

## Requirements

- Add a new architecture document under `docs/architecture/` that records system chains at condition-coverage level.
- Use the agreed breadth: cover major user/admin/system workflow families and key branch conditions. Do not create an exhaustive condition matrix for every Gateway active API operation.
- Update `docs/README.md` so the new chain document is discoverable from the docs entry point.
- Base the document on current `develop` docs, especially:
  - `docs/requirements-analysis/overall-requirements-analysis.md`
  - `docs/architecture/service-boundaries.md`
  - `docs/architecture/current-capability-matrix.md`
  - `docs/runbooks/local-integration.md`
  - service README / implementation docs under `docs/services/**`
- Cover at minimum these chain families:
  - Authentication and session lifecycle.
  - Knowledge base and document lifecycle.
  - Knowledge ingestion: upload, file handoff, parsing, chunking, embedding, vector index, status.
  - Knowledge retrieval / `knowledge-queries`.
  - QA session, message, response run, SSE/non-SSE answer, tool calls, citations, retrieval tests, metrics/config.
  - AI Gateway model profile and model invocation paths.
  - Document report template/material/report/outline/section/job/file/settings/statistics/log workflows.
  - Parser runtime internal parse path.
  - File service object storage/content path.
  - Cross-service readiness/smoke gaps.
- For each chain family, document:
  - Triggering actor/API.
  - Primary owner service.
  - Participating services/infrastructure.
  - Normal path.
  - Conditional branches needed for condition coverage.
  - Expected outputs/state changes.
  - Current implementation status or gap, using current capability matrix terminology.
  - Sensitive data that must not leak.
- Conditions should include at least:
  - Authenticated vs unauthenticated.
  - Authorized vs forbidden.
  - Resource exists vs not found/deleted/not ready.
  - Valid vs invalid request.
  - Dependency available vs dependency error.
  - Async queued/running/succeeded/failed/cancelled/retry states where applicable.
  - Streaming vs non-streaming where applicable.
  - Config/profile present vs missing/disabled/mismatched where applicable.
  - Current implemented path vs documented target/gap.
- Do not invent new contracts or claim missing capabilities are implemented.
- Use Chinese prose consistent with existing docs.
- Keep the document maintainable: avoid copying full OpenAPI operation tables; link to Gateway OpenAPI, owner map, service boundaries, and service docs for detailed schemas.

## Proposed Artifact

- `docs/architecture/system-link-condition-coverage.md`
- `docs/README.md` link update in the architecture table.

## Acceptance Criteria

- [x] New architecture document exists and is linked from `docs/README.md`.
- [x] Document covers the required chain families listed above.
- [x] Each covered chain has enough branches to guide condition-level tests/reviews rather than only happy-path sequence descriptions.
- [x] Document is scoped to major workflow families and does not attempt a per-operation Gateway API matrix.
- [x] Document clearly marks implemented, partial, missing, target, and smoke-gap paths.
- [x] Document identifies which service owns business state and which dependency only provides supporting capability.
- [x] Document does not duplicate full OpenAPI schemas or contradict service boundary rules.
- [x] Markdown links point to existing docs paths.
- [x] `git diff --check` passes.

## Out of Scope

- Changing OpenAPI contracts.
- Changing service code.
- Creating automated tests.
- Proving runtime behavior with Docker Compose or provider smoke tests.
- Exhaustively listing every single Gateway active operation if it does not add a distinct service chain or condition branch.

## Notes

- User selected the recommended scope: cover the major user/admin/system workflow families and their branch conditions, not a formal MC/DC matrix over every OpenAPI field.
