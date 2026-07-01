# Design

## Scope And Boundary

The File Service owns base object upload, metadata, content reads, soft delete state, and object-store cleanup. Owner services such as Knowledge and Document own their domain resources and may only reference File through `/internal/v1/files/**` with the required service token.

Gateway-facing or domain-specific behavior must not be added to File. File responses remain limited to safe metadata and content streams; storage implementation details stay internal.

## Contract Drift Check

The service keeps two File internal OpenAPI copies:

- `docs/services/file/api/internal.openapi.yaml`
- `services/file/api/openapi.yaml`

The implementation should add a deterministic test that parses and compares the two files. If the repository already has OpenAPI comparison helpers, reuse them; otherwise use a small service-local test helper that normalizes YAML into generic structures before comparing.

## Service Token Coverage

Service-token enforcement should stay in File HTTP middleware or route setup. Tests should cover:

- missing token,
- wrong token,
- owner-style request with context headers but no valid File service token.

Expected result is the standard `{ error: { code: "unauthorized", ... } }` envelope with no credential echo.

## Legacy Knowledge-Document Surface

Any File route or repository method that models knowledge documents directly should be removed if no active callers depend on it. If removal is too broad for this task, the route must be hidden from recommended docs and protected by tests/comments documenting the temporary caller and exit condition.

## Purge Cleanup

Deleted File metadata must not expose content even if object cleanup has not succeeded. Purge should be idempotent:

- repeated purge of an already-purged or missing object must not resurrect metadata,
- storage not-found should converge to a safe purged/deleted state,
- dependency errors should keep enough sanitized failure information for retry.

If the current code already has a synchronous cleanup path, converge it behind a worker-like service boundary named for `file:object:purge` semantics rather than adding unnecessary infrastructure.

## sqlc

If `services/file/sqlc.yaml` and `internal/repository/queries/file_objects.sql` are active, regenerate code with:

```bash
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
```

If the service intentionally uses hand-written SQL only, remove or quarantine unused sqlc inputs and document the reason so query definitions do not drift.

## Safety

Tests and user-facing errors should assert or inspect that responses avoid bucket names, object keys, internal URLs, local filesystem paths, and secrets. Logs should follow `slog` structured logging guidance and avoid request bodies or storage identifiers.
