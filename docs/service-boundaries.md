# 服务边界矩阵

本文档用于约束 `gateway`、`auth`、`file`、`knowledge`、`qa`、`document` 的职责归属，避免早期并行开发时把业务规则堆进 gateway。

## 总览

| Service | Owns | Exposes to gateway | Must not own |
| --- | --- | --- | --- |
| `gateway` | Public API, routing, Redis-backed session cache, auth context propagation, response/error envelope, request id, lightweight aggregation. | `/api/v1/**`, `/healthz`, `/readyz`. | Durable user/role/permission persistence, document parsing, vector search, LLM workflows, report generation business logic. |
| `auth` | Users, credentials, roles, permissions, sessions or tokens, session identity issuing and revocation. | Login, logout, current user, permission checks, session identity for gateway caching. | File metadata, knowledge indexing, QA messages, report records. |
| `file` | Uploads, original files, object storage coordination, file metadata lifecycle. | Upload, download, file metadata, file deletion. | Knowledge chunking, vector index, RAG, report generation. |
| `knowledge` | Knowledge bases, document ingestion state, chunks, embeddings, retrieval policies, search. | Knowledge base CRUD, document processing state, chunk list, search. | User identity, raw object storage, LLM answer generation, DOCX export. |
| `qa` | Chat sessions, messages, intent routing for QA, RAG answer generation, citations. | Chat session APIs, non-stream and stream answer APIs. | Knowledge base CRUD, file upload, report record management. |
| `document` | Report templates, report records, outlines, section content, DOCX export. | Report CRUD, outline generation, section generation, export/download. | QA chat, knowledge indexing, auth persistence. |

## Workflow Ownership

| Workflow | Gateway role | Owner service | Notes |
| --- | --- | --- | --- |
| Register / login | Public entrypoint, response normalization, Redis session cache write. | `auth` | Password validation and session/token issuing stay in auth; auth returns identity/session payload for gateway caching. |
| Current user | Read Redis session cache and normalize response. | `auth` | Auth owns user/session source data; gateway owns runtime cache lookup and downstream context injection. |
| Knowledge base CRUD | Route and normalize. | `knowledge` | Knowledge service owns metadata and retrieval strategy. |
| Upload document to knowledge base | Public workflow entrypoint. | `file` and `knowledge` with one explicit workflow owner to be finalized. | File service owns raw upload; knowledge service owns ingestion/indexing state. Gateway must not implement parsing or indexing. |
| Document processing retry | Route and normalize. | `knowledge` | Retry means re-run ingestion/indexing, not re-upload original file. |
| Download original document | Route and enforce auth context. | `file` | File service owns object lookup and download authorization details. |
| Frontend knowledge search | Route and normalize. | `knowledge` | Search includes metadata filters and retrieval settings. |
| Chat answer generation | Streaming entrypoint. | `qa` | QA service may call knowledge internally for RAG. Gateway should not orchestrate RAG steps. |
| Citation source lookup | Route and normalize. | `qa` or `knowledge`, depending on final citation model. | The service storing citation references owns lookup. |
| Report outline generation | Route and stream if needed. | `document` | Report templates and outline rules stay in document service. |
| Report section generation | Streaming entrypoint. | `document` | Gateway does not generate content. |
| Report DOCX export | Route and normalize. | `document` | Generated files may be stored through file service behind document service. |
| Admin overview | Read aggregation. | `gateway` aggregates; each service owns its metric. | Gateway can combine counts/trends but should not own source data. |

## Data Ownership Rules

- A service that owns a database table also owns the API that mutates that data.
- Gateway may expose a frontend-friendly path for that mutation, but must delegate business validation to the owner service.
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

## Anti-Patterns

- Adding SQL, MinIO, Qdrant, or LLM calls directly in gateway handlers.
- Duplicating permission logic in frontend, gateway, and domain service without a single owner.
- Letting gateway translate one frontend action into a long business workflow when one domain service should own the workflow.
- Returning downstream service internals directly to the frontend.
- Creating shared Go packages before at least three services need the same stable abstraction.
