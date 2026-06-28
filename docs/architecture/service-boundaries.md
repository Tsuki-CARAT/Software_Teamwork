# 服务边界矩阵

本文档用于约束 `gateway`、`auth`、`file`、`knowledge`、`qa`、`document` 的职责归属，避免早期并行开发时把业务规则堆进 gateway。

所有公开 gateway API 和服务间 HTTP API 必须使用 RESTful 资源路径，由 HTTP method 表达动作。除 `/healthz`、`/readyz` 外，不在稳定 path 中使用 `login`、`logout`、`register`、`download`、`search`、`generate`、`export`、`retry`、`revoke` 等动作词。

## 总览

| Service | Owns | Exposes to gateway | Must not own |
| --- | --- | --- | --- |
| `gateway` | Public API for frontend, admin, backend-module, and tool callers; routing; Redis-backed session cache; auth context propagation; response/error envelope; request id; lightweight aggregation. | `/api/v1/**`, `/healthz`, `/readyz`. | Durable user/role/permission persistence, document parsing, vector search, LLM workflows, report generation business logic. |
| `auth` | Users, credentials, roles, permissions, sessions or tokens, session identity issuing and revocation. | User creation, session creation/deletion, current user, permission checks, session identity for gateway caching. | File metadata, knowledge indexing, QA messages, report records. |
| `file` | Basic file upload/content APIs, original objects, object storage coordination, file-owned metadata lifecycle, MinIO middleware for backend services. | Knowledge document upload/content today; internal file-object APIs for other services once finalized. | Knowledge chunking, vector index, RAG, report generation, report material/template/report-file business state. |
| `knowledge` | Knowledge bases, document ingestion state, chunks, embeddings, retrieval policies, retrieval queries. | Knowledge base CRUD, document processing details, chunk listing, and knowledge queries through gateway. | User identity, raw object storage, LLM answer generation, DOCX export. |
| `qa` | Chat sessions, messages, intent routing for QA, RAG answer generation, citations. | Missing/TBD: frontend-backend contract not finalized. | Knowledge base CRUD, file upload, report record management. |
| `document` | Report templates, materials, report records, outlines, section content, report jobs, generated file metadata, statistics, and report operation logs. | Report generation routes under `/api/v1/report-*` and `/api/v1/reports/**`; uses file service for file-object storage and content when files are involved. | QA chat, knowledge indexing, auth persistence, direct exposure of MinIO object keys or storage URLs. |

## Workflow Ownership

| Workflow | Gateway role | Owner service | Notes |
| --- | --- | --- | --- |
| User and session creation | Public entrypoint, response normalization, Redis session cache write. | `auth` | Password validation and session/token issuing stay in auth; auth returns identity/session payload for gateway caching. |
| Current session deletion | Public entrypoint, response normalization, Redis session cache delete. | `auth` | Session/token invalidation stays in auth; gateway deletes the matching Redis cache entry. |
| Current user | Read Redis session cache and normalize response. | `auth` | Auth owns user/session source data; gateway owns runtime cache lookup and downstream context injection. |
| Knowledge base CRUD | Public entrypoint and response normalization. | `knowledge` | Active gateway contract. Gateway must not store knowledge-base business state. |
| Upload document to knowledge base | Public file upload entrypoint. | `file`; knowledge owns post-upload ingestion state. | File service owns raw upload; internal file -> knowledge handoff is an implementation detail. Gateway must not implement parsing or indexing. |
| Document processing status and chunks | Public read entrypoint and response normalization. | `knowledge` | Active gateway contract for document details and chunks. Gateway must not implement parsing, chunking, embedding, or Qdrant access. |
| Original document content | Route and enforce auth context. | `file` | File service owns object lookup and content authorization details. |
| Frontend knowledge queries | Public entrypoint and response normalization. | `knowledge` | Active gateway contract. Query execution is modeled as `knowledge-queries`, not as an action-style search path. |
| Chat answer generation | Missing public contract. | `qa` | Placeholder only. Streaming/non-streaming message and citation formats are not stable. |
| Citation source lookup | Missing public contract. | `qa` or `knowledge`, depending on final citation model. | Placeholder only. The service storing citation references will own lookup. |
| Report template management | Public entrypoint and auth context propagation. | `document` | Document service owns template metadata, template structure, and template file references. |
| Report material management | Public entrypoint and auth context propagation. | `document` | Document service owns material metadata and material file references used by report jobs; raw file object storage should reuse file service instead of treating materials as knowledge-base documents. |
| Report record management | Public entrypoint and auth context propagation. | `document` | Document service owns report drafts, lifecycle state, outlines, sections, and soft deletion rules. |
| Report outline generation | Public job resource creation and status lookup. | `document` | Long-running outline generation and regeneration are represented as `ReportJob` resources. |
| Report section generation | Public job or section-version resource creation and status lookup. | `document` | Long-running content generation and section regeneration stay inside document service. |
| Report file creation and content | Public file resource creation, metadata lookup, and content stream. | `document` | Document service owns generated file metadata and should use file service for object storage/content access where possible; generated files are not knowledge documents. |
| Report statistics and operation logs | Public read entrypoint and auth context propagation. | `document` | Document service owns report-specific statistics and audit-friendly operation logs. |
| Admin overview | Missing public contract. | `gateway` aggregates; each service owns its metric. | Placeholder only. Metrics and aggregation shape are not stable. |

## Missing Contract Register

The following downstream frontend/backend interfaces remain intentionally blank in
`docs/api/gateway.openapi.yaml` until the teams finalize their request and
response shapes:

| Area | Placeholder paths | Owner |
| --- | --- | --- |
| QA chat and RAG | `GET/POST /api/v1/qa-sessions`, `GET/DELETE /api/v1/qa-sessions/{sessionId}`, `GET/POST /api/v1/qa-sessions/{sessionId}/messages`, `GET /api/v1/qa-sessions/{sessionId}/events` | `qa` |
| Administration aggregation | `GET /api/v1/admin-overview`, `GET /api/v1/admin-metrics` | `gateway` plus domain services |

Do not generate frontend API clients or backend handlers for these placeholder
paths until the corresponding OpenAPI operations are added.

## Data Ownership Rules

- A service that owns a database table also owns the API that mutates that data.
- Gateway may expose caller-friendly public paths for frontend, admin, backend-module, and tool callers, but must delegate business validation to the owner service.
- Cross-service IDs should be strings in public API contracts. Each service can decide internal ID representation.
- Timestamps in public contracts use RFC 3339 / OpenAPI `date-time`.
- Delete operations must be owned by the service that owns the resource lifecycle.

## Boundary Checks For New Endpoints

Before adding a gateway endpoint, answer these questions in the endpoint doc or OpenAPI description:

1. Which service owns the resource state?
2. Does the endpoint only route, or does it aggregate multiple services?
3. If it aggregates, what frontend screen needs this shape?
4. Which service validates domain rules?
5. Which error codes can the frontend rely on?
6. Does the endpoint expose raw object keys, credentials, prompts, vector payloads, or internal URLs? It should not.
7. Is the path modeled as a resource or collection, with the HTTP method carrying the action?

## Anti-Patterns

- Adding SQL, MinIO, Qdrant, or LLM calls directly in gateway handlers.
- Adding action-style paths such as `/login`, `/logout`, `/download`, `/search`, `/generate`, `/export`, `/retry`, or `/revoke` instead of modeling users, sessions, content, queries, jobs, files, messages, or events as resources.
- Duplicating permission logic in frontend, gateway, and domain service without a single owner.
- Letting gateway translate one frontend action into a long business workflow when one domain service should own the workflow.
- Returning downstream service internals directly to the frontend.
- Letting `document` duplicate file-service object storage semantics for report templates, materials, or generated files when a file-service internal resource can model the raw object.
- Creating shared Go packages before at least three services need the same stable abstraction.
