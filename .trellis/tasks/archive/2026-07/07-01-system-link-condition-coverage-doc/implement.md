# Implementation Plan

## Checklist

1. [x] Read project documentation context.
   - `docs/README.md`
   - `docs/collaboration/documentation-workflow.md`
   - `docs/requirements-analysis/overall-requirements-analysis.md`
   - `docs/architecture/service-boundaries.md`
   - `docs/architecture/current-capability-matrix.md`
   - `docs/architecture/frontend-backend-contract.md`
   - `docs/runbooks/local-integration.md`
   - relevant `docs/services/**/README.md` and `docs/services/**/docs/implementation.md`
2. [x] Read frontend/backend integration and OpenAPI owner context as needed.
   - `docs/services/gateway/docs/active-api-owner-map.md`
   - `docs/services/gateway/api/public.openapi.yaml`
3. [x] Create `docs/architecture/system-link-condition-coverage.md`.
4. [x] Update `docs/README.md` architecture table with the new document.
5. [x] Validate.
   - `git diff --check`
   - Check links and paths manually through `rg`/`git grep`.
6. [x] Run Trellis check skill before finishing implementation.

## Validation Commands

```bash
git diff --check
rg -n "system-link-condition-coverage|链路|条件覆盖" docs
```

## Risk Notes

- Do not accidentally claim target/pending capabilities are already implemented.
- Do not copy large OpenAPI operation tables into the new document.
- Do not update OpenAPI contracts.
- Use `develop` docs as the factual baseline; current working branch may contain unrelated untracked user files.

## Files Expected To Change

- `docs/architecture/system-link-condition-coverage.md`
- `docs/README.md`

## Rollback

Documentation-only change. Roll back by removing the new doc and the README link if scope is rejected.
