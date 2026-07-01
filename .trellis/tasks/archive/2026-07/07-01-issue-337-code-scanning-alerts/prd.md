# Code Scanning security alert convergence

## Goal

Resolve GitHub issue #337 / S-034 by fixing or defensibly constraining the 12 open GitHub Code Scanning alerts listed in the issue body, without weakening CodeQL, workflow security, or service boundaries.

## Background

- Issue: <https://github.com/Sakayori-Iroha-168/Software_Teamwork/issues/337>
- Branch: `Special/fix/code-scanning-alerts`
- Target PR base: `develop`
- Scope spans `services/qa`, `services/knowledge`, `services/document`, `services/ai-gateway`, `.github/workflows/check-api-types.yml`, and security-related docs.
- GitHub Code Scanning REST access returned `404` for this local auth context, so alert IDs and URLs must be taken from the issue body unless browser access becomes available.
- Repository docs are authoritative when local code or older drafts conflict with public `docs/` guidance.

## Alert Requirements

| Alert | Rule | Location | Requirement |
| --- | --- | --- | --- |
| #15 | `go/uncontrolled-allocation-size` | `services/knowledge/internal/service/retrieval.go:249` | Bound rerank result map allocation by a validated maximum derived from `topK` / `rerankTopN`, not by untrusted or inflated provider/user input. |
| #14 | `go/uncontrolled-allocation-size` | `services/knowledge/internal/service/retrieval.go:248` | Bound rerank result slice allocation by the same service-level maximum and preserve existing no-reranker fallback semantics. |
| #13 | `go/command-injection` | `services/qa/internal/platform/localtools/client.go:384` | Remove shell interpretation of arbitrary command strings; execute only approved command specs with fixed executables and argument vectors. |
| #12 | `go/request-forgery` | `services/qa/internal/platform/modelclient/openai.go:144` | Ensure QA AI Gateway calls use a trusted internal endpoint: absolute HTTP(S), no credentials, path-constrained to AI Gateway chat completions, and no private/untrusted host escape unless explicitly allowed for local development/test. |
| #11 | `go/request-forgery` | `services/document/internal/platform/aigateway/profile_client.go:75` | Ensure Document AI Gateway profile calls use a trusted internal base URL with the same URL safety constraints. |
| #10 | `go/incorrect-integer-conversion` | `services/qa/internal/repository/postgres.go:144` | Convert pagination values to `int32` only after explicit range checks or 32-bit parsing, including offset overflow. |
| #9 | `go/weak-sensitive-data-hashing` | `services/ai-gateway/internal/service/crypto.go:49` | Treat provider API key fingerprinting as a keyed non-password lookup/audit fingerprint: replace plain SHA-256 with keyed HMAC-SHA-256 derived from the encryption key or document a stronger equivalent boundary in code and docs. |
| #8 | `actions/missing-workflow-permissions` | `.github/workflows/check-api-types.yml:13` | Add minimum `permissions` for the workflow. |
| #7 | `go/command-injection` | `services/qa/internal/platform/mcpclient/client.go:58` | Validate MCP stdio executable and arguments at the runtime boundary; reject shell metacharacters, non-allowlisted executables, and malformed arguments before `exec.Command`. |
| #6 | `go/incorrect-integer-conversion` | `services/qa/internal/repository/sqlc_map.go:31` | Convert conversation pagination values to `int32` only after explicit range checks, including offset overflow. |
| #4 | `go/incorrect-integer-conversion` | `services/qa/internal/repository/resources_postgres.go:20` | Convert stream event cursor to `int32` only after explicit range checks and reject negative / overflowed cursors. |
| #3 | `go/incorrect-integer-conversion` | `services/knowledge/internal/repository/postgres.go:938` | Replace clamp-to-`MaxInt32` pagination behavior with explicit range validation that reports invalid input rather than silently changing caller intent. |

## Constraints

- Do not disable CodeQL, broaden ignore rules, or remove the checked functionality to hide alerts.
- Do not introduce new business capabilities in QA, Knowledge, Document, or AI Gateway.
- Keep service-local package boundaries; do not import another service's `internal/` package.
- Do not log or document real provider credentials, service tokens, internal private URLs, full prompts, raw provider bodies, object keys, or sensitive payloads.
- Preserve existing local test behavior where it is security-compatible; test-only endpoints may remain valid through explicit local/test allowances.
- Any new security configuration must be documented in the relevant service docs or technology/testing baseline.

## Acceptance Criteria

- [ ] All 12 issue-listed alerts have code, workflow, or auditable security-boundary fixes mapped in the PR description.
- [ ] QA local command tool no longer invokes `/bin/sh -c`, PowerShell command strings, or equivalent shell interpretation for user-controlled command input.
- [ ] MCP stdio startup rejects unsafe executable names/paths and arguments, and only starts allowlisted commands or explicitly configured safe test binaries.
- [ ] QA and Document AI Gateway clients reject malformed URLs, credentialed URLs, unexpected paths, untrusted hostnames, and private network targets unless they are loopback/local development targets.
- [ ] Knowledge rerank allocations are capped by validated retrieval limits and covered by boundary tests.
- [ ] QA and Knowledge repository `int` to `int32` conversions fail explicitly on negative or over-`int32` values instead of overflowing or silently clamping.
- [ ] AI Gateway provider credential fingerprinting no longer uses plain SHA-256 over the API key; tests prove deterministic fingerprints, key separation, and no plaintext recovery.
- [ ] `.github/workflows/check-api-types.yml` declares minimum `permissions`.
- [ ] Documentation records new security boundaries for command execution, trusted internal AI Gateway URLs, pagination conversion, credential fingerprinting, and workflow permissions where appropriate.
- [ ] Required local validation is attempted and results are recorded: `cd services/qa && go test ./... && go build ./cmd/server && go build ./cmd/agent`; `cd services/knowledge && go test ./... && go build ./cmd/server`; `cd services/document && go test ./... && go build ./cmd/server`; `cd services/ai-gateway && go test ./... && go build ./cmd/server`; `git diff --check`; workflow YAML parse/static check where tooling exists.
