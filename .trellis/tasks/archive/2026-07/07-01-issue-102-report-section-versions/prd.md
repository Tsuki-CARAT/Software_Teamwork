# Document section versions and edit protection

## Goal

Complete issue #102 / C-006 for the Document service: section version creation must behave as the backend resource used by frontends and jobs for single-section regeneration, while preserving manual edits by default and keeping historical versions readable.

## Confirmed Facts

- Issue #102 depends on #99, #100, and #101; all dependencies are closed on upstream `develop`.
- The authoritative scope is in `docs/services/document/README.md`, `docs/services/document/docs/data-models.md`, `docs/services/document/docs/requirements.md`, and the Gateway public OpenAPI document under `docs/services/gateway/api/public.openapi.yaml`.
- Existing Document code already exposes `GET/POST /reports/{reportId}/sections/{sectionId}/versions` through `services/document/internal/http/reports.go`.
- Existing service/repository code already has `ListSectionVersions`, `CreateSectionVersion`, `CreateReportSectionVersion`, and `ListReportSectionVersions`.
- Current `CreateSectionVersion` only inserts a `ReportSectionVersion`; it does not switch the current `ReportSection` content/tables/version in the same transaction.
- The data model explicitly allows the current version reference to be represented by `ReportSection.version`; adding `current_version_id` is optional future work.
- Existing generation jobs already respect manual edit preservation by default through `preserveManualEdits`, and can overwrite only when explicitly set false.
- Document public OpenAPI currently still shows section version `source` as `manual|job`, while Gateway public OpenAPI and service models use `manual|ai`.

## Requirements

- R1: `POST /reports/{reportId}/sections/{sectionId}/versions` must create a historical section version and update the current `ReportSection` content, tables, source, manual edit flag, version, timestamps, and generated metadata as one transactional operation.
- R2: Version numbers must advance monotonically from both existing `ReportSection.version` and stored `ReportSectionVersion.version`, so the current section version and created historical version remain aligned.
- R3: Creating a section version must return conflict when the target section is in `running` generation state, matching existing section update protection.
- R4: Manual section edits through `UpdateSection` and bulk `SaveSections` must persist user-edited content/table versions as `ReportSectionVersion` snapshots.
- R5: Manual edits must remain protected by default during AI generation; only an explicit false preserve option may overwrite them.
- R6: Single-section regeneration must only mutate the target section/version records; report base fields and unrelated sections must not be deleted or overwritten.
- R7: Historical section version listing must continue to return stored versions for the requested report section and reject cross-report access through the existing `GetSection` ownership check.
- R8: Public API documentation for the Document service must align section version request/response fields with the implemented `manual|ai` source and `tables` shape already used by Gateway OpenAPI.

## Acceptance Criteria

- [x] Creating a section version while the section `generationStatus` is `running` returns `conflict`.
- [x] Creating a section version inserts the new `ReportSectionVersion` and switches the current `ReportSection.version` to that version inside one transaction.
- [x] If current-section update fails after version insertion, the created version is rolled back and the original section remains unchanged.
- [x] `UpdateSection` and `SaveSections` save user-edited content/table snapshots that are visible through `ListSectionVersions`.
- [x] Tests cover preserve-default and explicit-overwrite behavior for generation jobs.
- [x] Tests cover historical version reads after manual and AI version creation.
- [x] Tests verify single-section regeneration does not mutate other sections or report base data.
- [x] Document and Gateway-facing API schemas do not disagree on section version source values or table field names.

## Out of Scope

- No `current_version_id` database migration in this task; `ReportSection.version` is the current-version reference.
- No complex version diff UI.
- No frontend implementation beyond keeping the backend contract ready for frontend use.
- No new async job endpoint; existing report job flow remains the async generation path.

## Open Questions

- None blocking. Repository and docs answer the current-version reference decision.
