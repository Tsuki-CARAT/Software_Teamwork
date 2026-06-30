# Progress

## 2026-06-30

- Updated local `develop` from `upstream/develop` before implementation.
- Created branch `Frontend/test/frontend-test-baseline`.
- Created Trellis task `06-30-issue-163-frontend-test-baseline`.
- Read issue #163 and dependency issue #161.
- Confirmed #161 is closed and `openapi-typescript@7.13.0` is already fixed.
- Read current frontend workflow, quality, directory, component, hook, state, type-safety, technology, and testing strategy docs.
- Confirmed `private/doc-update-tasks-20260629.md` is not present in current checkout; using public `docs/` and `.trellis/spec/` as authority.
- Confirmed current gap: no frontend test dependencies, no `test` scripts, and no frontend test/check/build CI workflow.
- Added fixed frontend test dependencies: Vitest, React Testing Library, jest-dom, user-event, jsdom, and Playwright.
- Added `test`, `test:unit`, `test:unit:watch`, `test:e2e`, and `test:e2e:ui` scripts under `apps/web`.
- Added Vitest config and shared RTL setup for jsdom component/unit tests.
- Added API client error-envelope and unauthorized-token regression coverage.
- Added a Button component accessibility/render smoke test.
- Added Playwright config and route smoke coverage for `/login` and anonymous protected-route redirect.
- Added frontend CI for PR/push to `develop` when frontend/package/workflow files change. CI installs Bun `1.3.12`, runs install/check/build/unit tests, installs Playwright Chromium with system deps, then runs e2e smoke tests.
- Added Prettier ignore entries for Playwright output directories so local test artifacts do not break `format:check`.
- Fixed the local non-interactive Bun command environment outside the repo by adding `~/.local/bin/bun` and `~/.local/bin/bunx` wrappers that set `BUN_INSTALL=~/.bun` and `BUN_TMPDIR=~/.bun/tmp`.
- Fetched latest `upstream/develop` again after remote advanced from `88a7420` to `8f294ec`, rebased `Frontend/test/frontend-test-baseline`, and reapplied the implementation without conflicts.
- Synced Playwright smoke assertions with the latest upstream login page, which now exposes the page title through the `电力行业知识助手` SVG image label and uses spaced `登 录` button text.
- Completed Trellis spec-update review: no `.trellis/spec/` change was made because the frontend test stack was already documented as the target baseline, user instructions said not to modify public/remote docs for this alignment task, and task-specific version/environment notes are captured here.
- Resolved the local Playwright Chromium dependency gap without sudo by downloading the Debian packages reported by `playwright install-deps --dry-run`, extracting them to `~/.local/playwright-deps`, and extending the local `bun`/`bunx` wrappers with the required `LD_LIBRARY_PATH`, `XKB_CONFIG_ROOT`, and `FONTCONFIG_PATH`.

## Verification

- `bun install --frozen-lockfile` -> passed.
- `bun run --cwd apps/web test:unit` -> passed: 2 test files, 3 tests.
- `bun run --cwd apps/web check` -> passed after ignoring Playwright output directories in `.prettierignore`.
- `bun run --cwd apps/web build` -> passed with upstream frontend bundle warnings for ineffective dynamic import and large chunks.
- `git diff --check` -> passed.
- `bunx playwright install-deps chromium --dry-run` -> initially confirmed missing local packages: `at-spi2-common`, `fonts-freefont-ttf`, `fonts-ipafont-gothic`, `fonts-liberation`, `fonts-noto-color-emoji`, `fonts-tlwg-loma-otf`, `fonts-unifont`, `fonts-wqy-zenhei`, `libasound2-data`, `libasound2t64`, `libatk-bridge2.0-0t64`, `libatk1.0-0t64`, `libatspi2.0-0t64`, `libunwind8`, `libxdamage1`, `libxfont2`, `libxkbcommon0`, `x11-xkb-utils`, `xfonts-scalable`, `xkb-data`, `xserver-common`, and `xvfb`.
- `bun run --cwd apps/web test:e2e` -> passed after the local user-space Chromium dependency extraction and the exact login button assertion fix: 2 Playwright smoke tests passed.
- CI still installs Playwright Chromium with system dependencies through `bunx playwright install --with-deps chromium`.

## Completion Checklist

- [x] Baseline dependencies added.
- [x] Unit/component test config added.
- [x] Playwright config added.
- [x] API client tests added.
- [x] Component/page tests added.
- [x] E2E smoke added.
- [x] Frontend CI added.
- [x] Verification commands run and recorded.
- [x] Meaningful progress committed.
