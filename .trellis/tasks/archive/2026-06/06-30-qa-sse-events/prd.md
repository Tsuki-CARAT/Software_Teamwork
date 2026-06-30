# QA SSE Streaming, Replay, and Cancellation

## Background

Issue #92 (`B-006`) requires the QA service to provide the stable public SSE
contract for interactive QA messages. The branch is based on latest
`upstream/develop` after dependencies #89, #90, #91, and #75 are closed.

Authoritative references:

- `docs/services/qa/README.md`
- `docs/architecture/frontend-backend-contract.md`
- `docs/services/gateway/api/openapi.yaml`
- `docs/services/qa/docs/data-models.md`

## Scope

Implement and verify the QA-owned streaming path for:

- `POST /api/v1/qa-sessions/{sessionId}/messages` with
  `Accept: text/event-stream`.
- `GET /api/v1/qa-sessions/{sessionId}/events?responseRunId=...` replay.
- `PATCH /api/v1/response-runs/{responseRunId}` cancellation.
- Gateway proxy forwarding for QA SSE without changing event framing.

## Contract Requirements

The live stream must use the documented SSE frame shape:

```text
event: <event_type>
id: <event_seq>
data: <json_payload>
```

Supported public event types are:

- `message.created`
- `agent.iteration.started`
- `reasoning.step`
- `tool.started`
- `tool.completed`
- `tool.failed`
- `answer.delta`
- `citation.delta`
- `answer.completed`
- `error`
- `heartbeat`

`heartbeat` is a transport event and is not required to persist. Replayable
business events must be short-term persisted in `response_stream_events` with
monotonic `event_seq`.

## Privacy Requirements

SSE payloads and replay payloads must not expose full tool arguments, MCP raw
results, internal URLs, prompts, provider raw errors, object keys, full source
documents, or private chain-of-thought. Tool and reasoning events may expose only
safe summaries and public identifiers.

## Acceptance Criteria

- Streaming success emits contract event names and preserves SSE framing.
- Error paths emit a single `error` event with public error code/message.
- Cancellation stops the active run, persists cancelled run/message state, and
  leaves replayable public events.
- Replay returns events in increasing `eventSeq` order and honors
  `afterEventSeq`.
- Gateway forwards `text/event-stream` responses without JSON wrapping or frame
  corruption.
- Tests cover success, error event, cancel, replay, and gateway forwarding.

## Out Of Scope

- Report generation SSE.
- New public MCP raw schema or raw tool-result endpoints.
- Frontend UI changes.
