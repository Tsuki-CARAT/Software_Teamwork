# Design: System Link Condition Coverage Document

## Document Placement

Create `docs/architecture/system-link-condition-coverage.md`.

Rationale:

- The content is cross-service and workflow-level, so it belongs under `docs/architecture/`.
- Service README files should keep service-local semantics and link to architecture docs rather than duplicate cross-service standards.
- `docs/README.md` should link this document from the architecture table.

## Source Documents

Primary evidence:

- `docs/requirements-analysis/overall-requirements-analysis.md`
- `docs/architecture/service-boundaries.md`
- `docs/architecture/current-capability-matrix.md`
- `docs/architecture/frontend-backend-contract.md`
- `docs/runbooks/local-integration.md`
- `docs/services/*/README.md`
- `docs/services/*/docs/implementation.md`
- `docs/services/gateway/docs/active-api-owner-map.md`

The new document should not override OpenAPI. It should link to Gateway OpenAPI and owner map for schema/operation details.

## Structure

Recommended sections:

1. Purpose and Reading Rules
   - Explain "condition coverage" as major workflow families + key branch conditions.
   - State that it is not a per-operation OpenAPI matrix.
2. Global Chain Invariants
   - Gateway-only frontend entry.
   - Owner service owns business state.
   - PostgreSQL as business truth, Redis/asynq for queues/cache, File for object bytes, Qdrant for vectors, AI Gateway for provider calls.
   - No sensitive leakage.
3. Condition Taxonomy
   - Auth, permission, resource state, request validation, dependency, async state, streaming, configuration, implementation status.
4. Chain Catalog
   - Authentication/session lifecycle.
   - Gateway proxy/context propagation.
   - File object lifecycle.
   - Parser internal parse.
   - Knowledge base CRUD and document lifecycle.
   - Knowledge ingestion.
   - Knowledge retrieval.
   - AI Gateway profile and invocation.
   - QA session/message/response/SSE/tool/citation/config/retrieval test/metrics.
   - Document template/material/report/outline/section/job/file/settings/statistics/logs.
   - Local integration/readiness/smoke.
5. Coverage Checklist
   - Compact matrix that testers/reviewers can use to verify each condition category is represented.
6. Current Gaps
   - Clearly mark documented target paths that are not fully implemented.
7. Maintenance Rules
   - When to update this doc.

## Chain Entry Template

Each chain should use a compact repeated shape:

- Owner:
- Trigger:
- Participants:
- Normal path:
- Branch conditions:
- Outputs/state:
- Status/gaps:
- Non-leakage rules:

This keeps entries testable without copying full API schemas.

## Important Trade-Off

The selected scope intentionally avoids enumerating every Gateway active operation. This keeps the document useful for architecture and test planning while avoiding drift with OpenAPI.

## Compatibility

No code or contract behavior changes. This is a documentation-only addition.
