# S-012 Compose Image Tags

## Goal

Close GitHub issue #150 by replacing floating local Compose image tags with explicit versions and syncing the technology baseline and deploy documentation so local integration startup is reproducible.

## What I Already Know

- GitHub issue #150 is `[S-012] 本地 Compose 镜像版本固定`.
- The original PR #239 was closed unmerged, but replacement PR #268 was merged and added `deploy/docker-compose.yml`.
- Issue #150 is currently assigned to `Jackeyliu37`; I commented a takeover note as `ChenBaoZhuangdayun` and will keep the PR small to avoid duplicate work.
- Current `deploy/docker-compose.yml` contains floating tags for `qdrant/qdrant`, `minio/minio`, and `minio/mc`.
- Current `deploy/README.md` documents pulls using `latest` for those images.
- `docs/architecture/technology-decisions.md` still records Qdrant and MinIO server/client image versions as pending.

## Requirements

- Replace floating `latest` tags for local Compose infrastructure images owned by #150 scope.
- Update `docs/architecture/technology-decisions.md` so the fixed image versions match Compose.
- Update `deploy/README.md` pull commands and wording so local setup is reproducible and no longer says Qdrant/MinIO use `latest`.
- Keep the change limited to local Compose image/version documentation.
- Do not change service implementation code or introduce a broader deployment architecture.

## Acceptance Criteria

- [x] `deploy/docker-compose.yml` no longer uses `qdrant/qdrant:latest`, `minio/minio:latest`, or `minio/mc:latest`.
- [x] `docs/architecture/technology-decisions.md` records the same Qdrant, MinIO server, and MinIO client image tags used by Compose.
- [x] `deploy/README.md` pull commands use the fixed image tags.
- [x] Compose config validates for default and `ai` profile.
- [x] `git diff --check` passes.

## Definition Of Done

- Branch is based on latest `upstream/develop`.
- PR targets `Sakayori-Iroha-168/Software_Teamwork:develop`.
- Commit message follows Conventional Commits.
- Verification commands and any skipped runtime smoke are reported.

## Out Of Scope

- Changing production deployment strategy.
- Changing service Dockerfiles unrelated to image tag pinning.
- Adding new services, ports, secrets, or runtime dependencies.
- Fixing unrelated Compose health or readiness behavior unless config validation fails because of this task.

## Technical Notes

- Relevant files: `deploy/docker-compose.yml`, `deploy/README.md`, `docs/architecture/technology-decisions.md`.
- Relevant specs read: `CONTRIBUTING.md`, `.trellis/spec/cicd.md`, `.trellis/spec/backend/quality-guidelines.md`.
- Issue comment: https://github.com/Sakayori-Iroha-168/Software_Teamwork/issues/150#issuecomment-4839967768
