# Design

## Current Framework

Latest `develop` already provides the QA service scaffold, internal QA routes,
PostgreSQL-backed `response_stream_events`, `response_runs` cancellation, a
ReAct-style agent loop, and gateway active proxy routes.

## Implementation Choices

- Keep business event generation in `services/qa/internal/service/qa.go`.
- Keep SSE transport framing in `services/qa/internal/http/server.go`.
- Keep replay ownership in PostgreSQL through `response_stream_events`.
- Treat `heartbeat` as an HTTP transport event emitted by the handler. It has no
  `id` and is not written to `response_stream_events`.
- Protect live SSE writes with a mutex so heartbeat and business events cannot
  interleave frame bytes.
- Do not add new event payload fields that expose prompts, tool args, raw MCP
  results, provider details, or internal URLs.

## Validation Plan

- QA HTTP handler tests:
  - stream success and documented event names;
  - stream error de-duplication;
  - heartbeat frame emitted while Ask is idle.
- QA service tests:
  - cancellation persists cancelled state and replayable error event;
  - model failures persist sanitized error event.
- QA repository tests:
  - replay order and `afterEventSeq` behavior.
- Gateway tests:
  - stream response is proxied as SSE without envelope or fixed timeout.

## Risks

- Full `go test ./...` may hit local Windows symlink permission issues in
  unrelated local tool packages. If that happens, run targeted service packages
  and report the residual local-environment limitation.
