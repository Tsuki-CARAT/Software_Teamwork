# QA System API Documentation

## Goal

Create a new QA service API interface document from:

- The provided WeChat frontend API markdown for the intelligent QA system.
- Existing project public contracts, especially `docs/services/gateway.md`,
  `docs/architecture/frontend-backend-contract.md`, `docs/architecture/service-boundaries.md`,
  and `docs/api/gateway.openapi.yaml`.
- The external QA database design under
  `D:\ACADEMIC\By_Course\3.大三下学期\软件项目综合实践\0628\qa-system-design`.

## Scope

- Add a new service-level document for the QA service at `docs/services/qa.md`.
- Add a QA database design document at `docs/services/qa-database.md`.
- Adapt the WeChat document to this project's contract-first gateway rules:
  - public gateway prefix `/api/v1`;
  - RESTful resource paths;
  - success envelope `{ data, requestId }`;
  - paginated envelope `{ data, page, requestId }`;
  - error envelope `{ error: { code, message, requestId, fields? } }`;
  - bearer-authenticated business APIs;
  - camelCase public JSON fields.
- Use the database model as the resource boundary for conversations, messages,
  response runs, stream events, citations, QA configuration, LLM configuration,
  retrieval test runs, retrieval test results, and admin audit logs.
- Update the docs index so the new documents are discoverable.
- Record the documentation-writing process in `.trellis/workspace/EmptyDust/journal-1.md`.

## Out of Scope

- Do not implement backend code.
- Do not generate frontend API clients.
- Do not activate these routes in `docs/api/gateway.openapi.yaml` in this task.
  The new document is a service-level draft; OpenAPI promotion should happen
  in a follow-up task after review.
- Do not change the external QA database design files.

## Contract Decisions

- The WeChat `/api/conversations` surface maps to `/api/v1/qa-sessions`.
- The WeChat `/api/chat/stream` surface maps to resource-oriented message
  creation under `/api/v1/qa-sessions/{sessionId}/messages`; replay or live
  event reading is represented by `/api/v1/qa-sessions/{sessionId}/events`.
- RAG debug search is represented as `POST /api/v1/knowledge-queries` when the
  owner is the knowledge service, and retrieval experience testing stays under
  QA-owned resources.
- Citation details are documented from the QA citation snapshot perspective;
  original file download remains file-owned through
  `/api/v1/documents/{documentId}/content`.
- Admin configuration endpoints are modeled as QA-owned active configuration
  resources and test resources, not action-style paths.

## Acceptance Criteria

- [ ] `docs/services/qa.md` exists and explains the QA service boundary.
- [ ] `docs/services/qa-database.md` exists and explains the QA PostgreSQL schema,
  table groups, relationships, write flows, indexes, local deployment, seed data,
  and migration rules.
- [ ] The document includes endpoint overview tables with method, gateway path,
  auth requirement, owner service, and behavior summary.
- [ ] The document documents request/response shapes for sessions, messages,
  SSE events, citations, configuration, retrieval tests, and statistics.
- [ ] The document calls out which routes are draft-only until OpenAPI is updated.
- [ ] The document references database-backed resources without exposing SQL
  internals or private implementation details to frontend consumers.
- [ ] `docs/README.md` links to the new QA service and QA database documents.
- [ ] Workspace journal records what was created and which source documents were used.

## Verification

- Parse/read the edited Markdown for broken relative links and obvious formatting issues.
- Verify `docs/services/qa.md` follows the backend API contract spec:
  RESTful paths, standard envelopes, auth context, and service ownership.
- Verify `docs/services/qa-database.md` matches the external QA schema files.
- Check git status and diff before reporting completion.
