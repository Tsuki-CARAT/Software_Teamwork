# 服务边界矩阵

本文档用于约束 `gateway`、`auth`、`file`、`knowledge`、`qa`、`document`、`ai-gateway` 的职责归属，避免早期并行开发时把业务规则堆进 gateway 或把 provider 细节泄露到领域服务外。

所有公开 gateway API 和服务间 HTTP API 必须使用 RESTful 资源路径，由 HTTP method 表达动作。除 `/healthz`、`/readyz` 外，不在稳定 path 中使用 `login`、`logout`、`register`、`download`、`search`、`generate`、`export`、`retry`、`revoke` 等动作词。

## 总览

| Service | Owns | Exposes to gateway | Must not own |
| --- | --- | --- | --- |
| `gateway` | Public API for frontend, admin, backend-module, and tool callers; routing; Redis-backed session cache; auth context propagation; response/error envelope; request id; lightweight aggregation. | `/api/v1/**`, `/healthz`, `/readyz`. | Durable user/role/permission persistence, document parsing, vector search, LLM workflows, report generation business logic. |
| `auth` | Users, credentials, roles, permissions, sessions or tokens, session identity issuing and revocation. | User creation, session creation/deletion, current user, permission checks, session identity for gateway caching. | File metadata, knowledge indexing, QA messages, report records. |
| `file` | Basic file upload/content APIs, original objects, object storage coordination, file-owned metadata lifecycle, MinIO middleware for backend services. | Knowledge document upload/content today; internal file-object APIs for other services once finalized. | Knowledge chunking, vector index, RAG, report generation, report material/template/report-file business state. |
| `knowledge` | Knowledge bases, document ingestion state, chunks, embeddings workflow, retrieval policies, retrieval queries, Qdrant index ownership. | Knowledge base CRUD, document processing details, chunk listing, and knowledge queries through gateway; calls AI Gateway internally for embeddings or rerank when needed. | User identity, raw object storage, LLM answer generation, DOCX export, provider API key storage. |
| `qa` | Chat sessions, messages, intent routing for QA, RAG answer generation, citations. | Missing/TBD: frontend-backend contract not finalized; calls AI Gateway internally for OpenAI-compatible chat completions when needed. | Knowledge base CRUD, file upload, report record management, provider API key storage. |
| `document` | Report templates, materials, report records, outlines, section content, report jobs, generated file metadata, statistics, and report operation logs. | Report generation routes under `/api/v1/report-*` and `/api/v1/reports/**`; uses file service for file-object storage/content and AI Gateway for model calls when files or model output are involved. | QA chat, knowledge indexing, auth persistence, provider API key storage, direct exposure of MinIO object keys or storage URLs. |
| `ai-gateway` | Model profiles, provider configuration, API key write state, OpenAI-compatible chat completions and embeddings, OpenAI-style rerankings, provider error normalization. | Internal `/internal/v1/model-profiles`, `/internal/v1/chat/completions`, `/internal/v1/embeddings`, `/internal/v1/rerankings`; health and readiness checks. | Frontend-facing APIs, QA sessions/messages, knowledge chunk persistence, Qdrant writes, report records, report export, domain permission decisions. |

## Workflow Ownership

| Workflow | Gateway role | Owner service | Notes |
| --- | --- | --- | --- |
| User and session creation | Public entrypoint, response normalization, Redis session cache write. | `auth` | Password validation and session/token issuing stay in auth; auth returns identity/session payload for gateway caching. |
| Current session deletion | Public entrypoint, response normalization, Redis session cache delete. | `auth` | Session/token invalidation stays in auth; gateway deletes the matching Redis cache entry. |
| Current user | Read Redis session cache and normalize response. | `auth` | Auth owns user/session source data; gateway owns runtime cache lookup and downstream context injection. |
| Knowledge base CRUD | Public entrypoint and response normalization. | `knowledge` | Active gateway contract. Gateway must not store knowledge-base business state. |
| Upload document to knowledge base | Public file upload entrypoint. | `file`; knowledge owns post-upload ingestion state. | File service owns raw upload; internal file -> knowledge handoff is an implementation detail. Gateway must not implement parsing or indexing. |
| Document processing status and chunks | Public read entrypoint and response normalization. | `knowledge` | Active gateway contract for document details and chunks. Gateway must not implement parsing, chunking, embedding, or Qdrant access. Knowledge may call AI Gateway for embedding generation but owns chunk and vector persistence. |
| Original document content | Route and enforce auth context. | `file` | File service owns object lookup and content authorization details. |
| Frontend knowledge queries | Public entrypoint and response normalization. | `knowledge` | Active gateway contract. Query execution is modeled as `knowledge-queries`, not as an action-style search path. Retrieval and rerank business rules stay in knowledge; model rerank calls can go through AI Gateway. |
| Chat answer generation | Missing public contract. | `qa` | Placeholder only. Streaming/non-streaming message and citation formats are not stable. QA owns conversation/message/citation state and may call AI Gateway for OpenAI-compatible chat completions. |
| Citation source lookup | Missing public contract. | `qa` or `knowledge`, depending on final citation model. | Placeholder only. The service storing citation references will own lookup. |
| Report template management | Public entrypoint and auth context propagation. | `document` | Document service owns template metadata, template structure, and template file references. |
| Report material management | Public entrypoint and auth context propagation. | `document` | Document service owns material metadata and material file references used by report jobs; raw file object storage should reuse file service instead of treating materials as knowledge-base documents. |
| Report record management | Public entrypoint and auth context propagation. | `document` | Document service owns report drafts, lifecycle state, outlines, sections, and soft deletion rules. |
| Report outline generation | Public job resource creation and status lookup. | `document` | Long-running outline generation and regeneration are represented as `ReportJob` resources. Document may call AI Gateway for model output but owns job state and outline versions. |
| Report section generation | Public job or section-version resource creation and status lookup. | `document` | Long-running content generation and section regeneration stay inside document service. Document may call AI Gateway for OpenAI-compatible streaming chunks but owns public event shape. |
| Report file creation and content | Public file resource creation, metadata lookup, and content stream. | `document` | Document service owns generated file metadata and should use file service for object storage/content access where possible; generated files are not knowledge documents. |
| Report statistics and operation logs | Public read entrypoint and auth context propagation. | `document` | Document service owns report-specific statistics and audit-friendly operation logs. |
| Runtime model profile management | Internal configuration API only. | `ai-gateway` | Model profiles, provider base URLs, model names, default parameters, timeout settings and API key write state stay behind `/internal/v1/model-profiles`. Public admin UI, if needed later, must go through gateway. |
| Provider model invocation | Internal model invocation API only. | `ai-gateway` | Chat and embedding APIs use OpenAI-compatible bodies. Rerank is OpenAI-style because OpenAI has no native rerank endpoint. Domain services own prompts, business context and persistence. |
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
- AI Gateway may store model provider configuration and encrypted or secret-backed API key material, but it must not own domain prompts, conversations, chunks, citations, reports, or generated files.
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
- Calling OpenAI-compatible, SiliconFlow-compatible, or local model providers directly from `gateway`, `qa`, `knowledge`, or `document` instead of routing model invocations through `ai-gateway`.
- Exposing AI Gateway `/internal/v1/**`, provider model names, provider base URLs, API key state, prompts, embeddings, rerank payloads, or raw provider errors to frontend contracts.
- Letting `document` duplicate file-service object storage semantics for report templates, materials, or generated files when a file-service internal resource can model the raw object.
- Creating shared Go packages before at least three services need the same stable abstraction.
