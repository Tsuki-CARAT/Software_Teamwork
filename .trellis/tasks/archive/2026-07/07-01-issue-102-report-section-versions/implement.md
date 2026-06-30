# Implementation Plan

## Order

1. Add failing service tests for `CreateSectionVersion`:
   - rejects running sections with conflict
   - creates version and switches current section in one transaction
   - rolls back inserted version when current section update fails
   - lists historical manual and AI versions
2. Add failing tests for manual edit snapshot creation through `UpdateSection` and `SaveSections`.
3. Add/adjust generation tests:
   - default preservation still skips manual sections
   - `preserveUserEdits=false` overwrites manual sections
   - generated section update and version insert are transactional
   - single-section regeneration leaves report base and unrelated sections unchanged
4. Implement service helpers for next section version, content-source/manual flag updates, and manual edit snapshotting.
5. Wrap section version creation and current-section switch in `ReportRepository.WithinTx`.
6. Wrap generation section update plus version insert in `WithinGenerationTx`.
7. Align Document public OpenAPI section version schemas with Gateway public OpenAPI.
8. Run formatting and verification commands.

## Validation Commands

- `C:/Users/liu/codex_tools/go1.25.11/go/bin/go.exe test ./internal/service -count=1`
- `C:/Users/liu/codex_tools/go1.25.11/go/bin/go.exe test ./... -count=1`
- `C:/Users/liu/codex_tools/go1.25.11/go/bin/go.exe build ./cmd/server`
- `C:/Users/liu/codex_tools/go1.25.11/go/bin/go.exe run golang.org/x/vuln/cmd/govulncheck@latest ./...`
- `git diff --check`

Run from `services/document` for Go commands.

## Security Self-Check

- Verify section version reads/writes keep the existing `GetSection -> GetReport -> CanAccessReport` ownership path.
- Verify operation logs record only IDs, version numbers, source values, and booleans, not section content, requirements, tables, prompts, tokens, or downstream raw errors.
- Run secret-pattern scans over changed code/docs/task artifacts.
- Run debug-residue scans for accidental `fmt.Print`, `log`, `slog`, `TODO`, `FIXME`, and `panic` additions.
- Run `govulncheck`; upgrade vulnerable dependencies when the fix is low-risk and tests remain green.

## Review Gates

- Confirm no `.local/` machine notes are staged.
- Confirm no generated files or unrelated docs are staged.
- Confirm API docs use UTF-8 text and no mojibake in PR-visible content.
- Confirm history/version behavior is covered at service level before committing.

## Risk Points

- Existing fake repositories need real rollback behavior for transaction tests.
- `SaveSections` updates multiple sections in one transaction; helper code must not create version snapshots outside that transaction.
- Public docs currently disagree on section version source enum; update only the Document-owned schema needed for this issue.
