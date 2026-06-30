# Design

## Boundaries

This task stays inside the Document service plus public contract docs. The current storage schema already has `report_sections.version` and `report_section_versions`, so the implementation will not add a new current-version column.

Primary code boundaries:

- `services/document/internal/service/report_service.go`
- `services/document/internal/service/report_generation_service.go`
- `services/document/internal/repository/reports.go`
- `services/document/internal/http/reports.go`
- `docs/services/document/api/public.openapi.yaml`

## Current-Version Model

The current active section version is represented by `ReportSection.version`. A created `ReportSectionVersion` with version N becomes current when the associated `ReportSection.version` is updated to N in the same transaction.

This matches `docs/services/document/docs/data-models.md`, which permits either `ReportSection.version` or a future `current_version_id`.

## Create Section Version Flow

1. Resolve the report and section through `GetSection` to keep access control and cross-report validation.
2. Reject deleted reports through the existing `GetReport` path, and reject sections in `JobStatusRunning` with `CodeConflict`.
3. Normalize source:
   - `manual`: version content represents a user-authored snapshot and current section becomes manual/mixed according to source semantics.
   - `ai`: version content represents regenerated AI content and current section becomes AI, `ManualEdited=false`, and `GeneratedAt` is set.
4. Choose content/tables from request overrides, falling back to the current section values.
5. Compute the next version using both the current section version and stored version history.
6. Inside `ReportRepository.WithinTx`:
   - Insert `ReportSectionVersion`.
   - Update the current `ReportSection` with the same version/content/tables/source flags.
7. Record operation logs after the transaction succeeds.

## Manual Edit Snapshots

Manual edits already flow through `UpdateSection` and `SaveSections`. When either changes content or tables, the service must create a matching `ReportSectionVersion` snapshot in the same repository transaction as the section update. Metadata-only edits do not create a version.

For `SaveSections`, each changed section can create its own snapshot. The enclosing transaction already protects the batch.

## Generation Preservation

The generation service already skips manual edited sections by default and overwrites only when `preserveManualEdits=false`. To avoid source contract drift, it should also accept `preserveUserEdits=false` as the public option alias while keeping `preserveManualEdits` backward-compatible.

Generated content update and version insertion should be transactional per generated section, so a version insert failure cannot leave current section content switched without history.

## API Compatibility

Gateway OpenAPI is already aligned to `source: manual|ai` and `tables`. Document public OpenAPI should be updated to match. The HTTP handler already uses `source`, `content`, `tables`, and `requirements`.

## Rollback Shape

No schema migrations are planned. Code rollback is limited to reverting service behavior changes and OpenAPI/doc adjustments. Transaction tests must catch partial-write regressions.
