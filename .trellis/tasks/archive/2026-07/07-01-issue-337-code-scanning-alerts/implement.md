# Code Scanning security alert convergence implementation plan

## Steps

1. Activate task after planning review.
2. Load `trellis-before-dev` and backend/CI specs before code edits.
3. Implement QA command execution hardening:
   - local tool command parser/allowlist,
   - MCP stdio executable/argument validator,
   - focused tests for allowed simple commands and rejected shell injection.
4. Implement trusted AI Gateway URL validation:
   - QA modelclient constructor and config tests,
   - Document profile client/config tests,
   - preserve `httptest.Server` and local development support.
5. Implement allocation and integer conversion checks:
   - Knowledge retrieval limit/rerank allocation guard,
   - QA pagination/event cursor conversion helpers,
   - Knowledge `limitOffset` explicit overflow errors,
   - focused boundary tests.
6. Replace AI Gateway credential fingerprinting with keyed HMAC-SHA-256:
   - derive fingerprint key from encryption key,
   - update tests and docs,
   - keep field compatibility.
7. Add workflow `permissions: contents: read`.
8. Update durable documentation for new safety boundaries.
9. Run validation:
   - `git diff --check`
   - `cd services/qa && go test ./... && go build ./cmd/server && go build ./cmd/agent`
   - `cd services/knowledge && go test ./... && go build ./cmd/server`
   - `cd services/document && go test ./... && go build ./cmd/server`
   - `cd services/ai-gateway && go test ./... && go build ./cmd/server`
   - YAML/static workflow check if available locally.
10. Prepare commit and PR body with alert mapping, commands, and residual risks.

## Risk Points

- Command hardening may break permissive local diagnostic commands; keep the change intentional and test covered because the issue explicitly requires no arbitrary command strings.
- URL validation must not reject existing `httptest.Server` tests or Compose-style `ai-gateway` hostnames.
- Repository conversion changes may require signature updates because functions that previously could not fail must now return errors.
- The AI Gateway fingerprint storage column name includes `sha256`; avoid migrations unless necessary by documenting the compatibility naming.

## Rollback

- Command and URL validators are local to platform/config packages and can be reverted independently if a compatibility issue appears.
- Integer conversion helpers can be adjusted to preserve signatures if tests show excessive blast radius.
- Credential fingerprint algorithm is security-sensitive; rollback must be avoided unless replaced by an equal or stronger keyed design.
