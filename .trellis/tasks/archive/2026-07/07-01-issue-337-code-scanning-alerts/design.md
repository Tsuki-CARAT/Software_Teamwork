# Code Scanning security alert convergence design

## Boundary Choices

- Keep fixes service-local. Shared helpers are acceptable inside a service when several packages in that service need the same safety check; do not create a cross-service Go module for two services.
- Prefer explicit validators close to the boundary that consumes untrusted input: config loading, platform client construction, repository parameter conversion, and service request normalization.
- Preserve existing public API behavior unless an input is unsafe. Unsafe inputs should fail as validation errors or dependency setup errors with stable, non-sensitive messages.

## Command Execution

QA local tools currently expose an opt-in bash tool but execute a caller-provided string through a shell. Replace that path with a restricted command request:

- Parse the `command` field as shell-like words only for backward-compatible simple invocations, then reject shell operators, redirects, pipes, expansion characters, NULs, and control characters.
- Execute with `exec.CommandContext(ctx, executable, args...)`, never `sh -c` or PowerShell `-Command`.
- Allow only a small, deterministic command set suitable for local diagnostics and tests, such as `echo`, `pwd`, `ls`, `cat`, `head`, `tail`, `grep`, `rg`, `wc`, and `sleep`/platform equivalent where needed by tests.
- Keep workspace directory, timeout, and output limits unchanged.

MCP stdio should remain capable of starting configured MCP servers, but the runtime boundary must reject unsafe startup values:

- Reject command strings containing whitespace, shell metacharacters, NULs, or control characters.
- Permit absolute or relative executable paths only after cleaning and checking they do not contain unsafe characters.
- Permit bare executable names from an allowlist plus the current test helper binary when the caller passes an absolute path.
- Reject arguments containing NULs or newlines. Arguments are still passed as an argument vector, not through a shell.

## Trusted Internal URLs

QA and Document call AI Gateway over internal HTTP. URL validation should prove the target is an expected AI Gateway endpoint, not only syntactically HTTP(S):

- Accept absolute `http` or `https` URLs with no userinfo, no fragments, and normalized paths.
- For QA modelclient, require the endpoint path to end in `/internal/v1/chat/completions`.
- For Document profile client, require the base URL to be a service base or `/internal/v1`, then build profile URLs with `url.JoinPath`.
- Trust loopback/local development hosts (`localhost`, `127.0.0.0/8`, `::1`) for local tests and Compose-style hostnames such as `ai-gateway`.
- Reject unspecified, multicast, link-local, and private IP literals unless loopback/local.
- Reject raw IP/host targets outside the trusted internal host rules.
- Apply validation in both config loading and client constructors where applicable so direct package users are protected.

## Allocation and Integer Boundaries

Knowledge retrieval already documents `topK` range 1-100. Enforce that cap before rerank allocation and allocate based on the effective limit, not raw result length or provider payload.

QA and Knowledge repositories should centralize safe `int` to `int32` conversion:

- Validate page, page size, offset, and stream cursor before building sqlc params.
- Return explicit errors on negative values or overflows.
- Avoid silent `MaxInt32` clamps because they hide invalid client input and can produce unexpected queries.

## Credential Fingerprints

AI Gateway API keys are encrypted for storage and use fingerprints for lookup/audit. Plain SHA-256 over a secret is not appropriate for a sensitive credential fingerprint because it is unsalted and reusable across environments.

Design:

- Derive fingerprints with HMAC-SHA-256 keyed by the configured 32-byte credential encryption key.
- Use a domain-separated key derivation string, for example `ai-gateway credential fingerprint v1`, to separate fingerprint use from AES-GCM encryption use.
- Keep the existing `FingerprintSHA256` storage field for compatibility, but document that the value is now an HMAC-SHA-256 fingerprint rather than raw SHA-256.
- Add tests for deterministic same-key fingerprints, different-key separation, non-equality to plain SHA-256, and decrypt compatibility.

## Workflow Permissions

Set `.github/workflows/check-api-types.yml` permissions to `contents: read`, because it checks out repository content and does not need write scopes.

## Documentation

Update docs only where the security rule is durable:

- `docs/architecture/technology-decisions.md`: AI Gateway credential fingerprinting and internal URL trust boundaries.
- `docs/testing/strategy.md`: Code scanning/security PR validation commands and workflow permission baseline.
- Service implementation docs for QA, Knowledge, Document, and AI Gateway if their current implementation notes would otherwise omit the new safety constraints.
