# S-037 storage boundary docs cleanup

## Goal

Clean up outdated storage-boundary language in the Knowledge, Document, and File service documentation so owner services consistently treat File Service references as opaque `file_ref` values. Documentation should make clear that File Service exclusively owns bucket names, object keys, storage backends, and credentials.

Source issue: https://github.com/Sakayori-Iroha-168/Software_Teamwork/issues/354

## Background

- `docs/tests/0701/file-module-test-report.md` found that current File documentation and runtime behavior encapsulate bucket/object-key details inside File Service.
- Root local Compose currently uses the single bucket `software-teamwork-local`.
- Some existing docs still describe earlier bucket-category concepts such as `source-files`, `templates`, and `generated-reports`, which can incorrectly imply that owner services may depend on bucket classification.

## Requirements

- Remove or correct outdated references to owner-service-visible bucket categories, object keys, MinIO internals, MinIO URLs, signed URLs, or File internal IDs in Knowledge, Document, and File documentation.
- Use `file_ref` as the consistent term for opaque owner-service file references in Knowledge and Document documentation.
- State that File Service alone owns bucket selection, object key layout, storage backend details, and credentials.
- Where local Compose storage is mentioned, describe the `software-teamwork-local` bucket as a local implementation detail only.
- Ensure Gateway, Knowledge, and Document public API documentation does not expose File internal IDs, bucket names, object keys, MinIO URLs, or signed URLs as public contracts.

## Out of Scope

- Runtime code changes.
- API contract changes beyond documentation wording.
- Storage migration or MinIO configuration changes.

## Acceptance Criteria

- [ ] `rg "source-files|generated-reports|object key|bucket" docs/services/knowledge docs/services/document docs/services/file` returns only correct boundary statements or explicit prohibitions against exposing storage internals.
- [ ] Knowledge and Document docs consistently use `file_ref` for opaque file references owned by caller services.
- [ ] Documentation does not require owner services to depend on MinIO bucket classification.
- [ ] Any local Compose mention of `software-teamwork-local` makes clear that it is an implementation detail.
- [ ] `git diff --check` passes.
