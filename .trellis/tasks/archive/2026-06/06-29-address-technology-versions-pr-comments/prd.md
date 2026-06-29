# Address technology versions PR review comments

## Goal

Resolve review comments on PR #132 by updating the technology selection document to use clearer fixed versions, while asking the user for any baseline version choice that changes project direction.

## What I Already Know

- PR #132 updates `docs/architecture/technology-decisions.md`.
- Review comment 1 asks to change the Go version to 1.25.
- Review comment 2 asks to select a concrete option instead of leaving a decision open around the three-choice decision record.
- The second comment is most relevant to the migration tool row where `goose` is selected but the version is still marked as pending.
- Current repository service modules still declare `go 1.22` in `services/knowledge/go.mod`, `services/file/go.mod`, and local WIP `services/auth/go.mod` / `services/gateway/go.mod`.
- `services/knowledge/Dockerfile` currently uses `golang:1.22-alpine`.

## Research Notes

- Official Go downloads currently list `go1.26.4` as the latest stable line and `go1.25.10` as the current Go 1.25 patch line.
- `pressly/goose` latest release is `v3.27.1`.
- `pressly/goose v3.27.0` release notes say the minimum Go version is now 1.25, so selecting latest goose aligns with a Go 1.25+ baseline.

## Requirements

- Address both PR review comments.
- Keep PR #132 limited to the technology decision document unless the user explicitly asks to broaden scope.
- Do not include unrelated local WIP files in the PR.
- User selected option A: update the documented Go baseline to 1.25 and fix `goose` to `v3.27.1`.
- Keep this PR documentation-only; service `go.mod` and Dockerfile migration can happen in a later PR.

## Acceptance Criteria

- [x] `docs/architecture/technology-decisions.md` reflects Go 1.25 and `goose v3.27.1`.
- [x] PR #132 is updated with only the intended documentation file.
- [x] Review comments are replied to after the PR branch is updated.
- [x] `git diff --check` or equivalent whitespace validation passes for the changed Markdown.

## Out of Scope

- Migrating every existing Go module and Dockerfile unless the user chooses to expand the PR scope.
- Including auth/gateway local WIP in PR #132.
