# Issue 163: Frontend Test Baseline

## Goal

Land the frontend test baseline requested by GitHub issue #163 on top of the latest `upstream/develop`: fixed test dependencies, runnable frontend test scripts, initial unit/component/e2e coverage, and a CI workflow that runs frontend checks for frontend PRs.

## What I Already Know

- Issue: https://github.com/Sakayori-Iroha-168/Software_Teamwork/issues/163.
- Issue title: `[F-20260629-03] 前端测试基线落地`.
- The issue is assigned to `EIR9264` and already has the required claim comment.
- Required stack from current docs: Vitest + React Testing Library + Playwright.
- Current repository baseline has `openapi-typescript@7.13.0` fixed by #161; #161 is closed.
- Current `apps/web/package.json` has `check`, `build`, `typecheck`, `lint`, and `format:check`, but no `test` scripts or test dependencies.
- Current CI has Go, migration, gateway contract, API type drift, commitlint, and PR guard workflows; there is no frontend test/check/build workflow yet.
- `private/doc-update-tasks-20260629.md` is referenced by the issue but is not present in the current checkout; public `docs/` and `.trellis/spec/` are the available authority.
- User instruction: update from upstream first, align with latest docs, do not change upstream/public docs as part of the remote-doc alignment, report progress frequently, and commit meaningful progress.

## Requirements

- Add fixed frontend test dev dependencies under `apps/web`.
- Add unit/component test scripts using Vitest.
- Add Playwright e2e/smoke test script.
- Cover API client error handling with unit tests.
- Add at least one component test.
- Add at least one key workflow smoke test for frontend routing or pages.
- Add CI so frontend PRs run install, check, build, and tests.
- Keep generated API files untouched unless a generation command requires them.
- Use Bun commands from repo root: `bun run --cwd apps/web <script>`.
- Do not modify the remote/public documentation files for this task unless an implementation change makes a local task note necessary; record task-specific notes under this Trellis task directory.

## Acceptance Criteria

- [ ] `bun run --cwd apps/web test` passes.
- [ ] `bun run --cwd apps/web test:unit` passes.
- [ ] `bun run --cwd apps/web test:e2e` or an equivalent Playwright smoke command passes, or is documented with an environment limitation.
- [ ] `bun run --cwd apps/web check` passes.
- [ ] `bun run --cwd apps/web build` passes.
- [ ] `git diff --check` passes.
- [ ] At least one API client error-handling unit test exists.
- [ ] At least one component test exists.
- [ ] At least one route/page/workflow smoke test exists.
- [ ] Frontend CI runs on PRs to `develop` for `apps/web`, relevant root package/lock files, and the workflow file.

## Definition Of Done

- Dependencies are fixed in `apps/web/package.json` and `bun.lock`.
- Test configuration files live under `apps/web/` and follow existing Vite/TypeScript alias conventions.
- Tests mock network at the API boundary and avoid generated OpenAPI internals.
- CI uses `oven-sh/setup-bun@v2` with `bun@1.3.12` and `bun install --frozen-lockfile`.
- Progress and verification notes are maintained in `progress.md`.
- Commits are made for meaningful progress using Conventional Commits.

## Technical Approach

Use Vitest as the unit/component runner with React Testing Library and jsdom for browser-like component tests. Use Playwright for a minimal route/page smoke test against a local Vite dev server. Keep the first coverage intentionally narrow: API transport error mapping, one component/page render, and one workflow smoke that proves the app boots and important route content renders.

## Out Of Scope

- Broad E2E coverage for login, document upload, full chat streaming, and report generation.
- Changing public documentation under `docs/` or project specs unless a later quality check proves it is required.
- Modifying backend services or gateway contracts.
- Manually editing `apps/web/src/api/generated/gateway.ts`.

## Technical Notes

- Latest upstream update completed with `git fetch upstream --prune` and `git merge --ff-only upstream/develop`.
- Working branch: `Frontend/test/frontend-test-baseline`.
- Relevant docs read:
  - `CONTRIBUTING.md`
  - `docs/collaboration/frontend-workflow.md`
  - `docs/architecture/technology-decisions.md`
  - `docs/testing/strategy.md`
  - `.trellis/spec/frontend/index.md`
  - `.trellis/spec/frontend/quality-guidelines.md`
  - `.trellis/spec/frontend/directory-structure.md`
  - `.trellis/spec/frontend/component-guidelines.md`
  - `.trellis/spec/frontend/hook-guidelines.md`
  - `.trellis/spec/frontend/state-management.md`
  - `.trellis/spec/frontend/type-safety.md`

