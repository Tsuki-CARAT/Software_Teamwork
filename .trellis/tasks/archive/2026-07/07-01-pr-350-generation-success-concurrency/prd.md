# Fix generated section success overwrite race

## Goal

Resolve the latest PR #350 review finding that a successful section generation write can overwrite a concurrent section edit or a newer generation job because it writes from a stale pre-AI-call section snapshot.

## Requirements

- `executeContentGeneration` must not persist generated content from a stale section snapshot after the AI call returns.
- The successful generated-section write path must re-read and lock the target section inside `WithinGenerationTx` before computing the next version and updating `report_sections`.
- The write must require the current section to still belong to the same report, still have `last_job_id == payload.JobID`, and still be in `running` generation status.
- The write must reject or skip stale writes when the section version or manual-edit state changed during generation, preserving the current section content, tables, version, source, and manual-edited state.
- Add regression tests that fail against the stale-snapshot overwrite behavior and pass after the fix.

## Acceptance Criteria

- [x] A concurrent manual edit before successful generated write is preserved; no AI section version is created from the stale generation result.
- [x] A superseded generation job is not overwritten by the older job's successful AI response.
- [x] Existing generation rollback/failure compensation behavior remains covered.
- [x] Document service tests pass.

## Notes

- Source review comment: PR #350 `github-actions` review on head `774b50a127ba`.
- This is a lightweight review-fix task; PRD-only planning is sufficient.
