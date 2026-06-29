# Type Safety

> TypeScript, OpenAPI, Zod, and runtime validation rules.

## Core Rules

- Use TypeScript for all frontend code.
- Prefer generated API types from OpenAPI when backend contracts exist.
- Validate user input and untrusted runtime data with Zod.
- Keep route params and search params typed through TanStack Router.
- Avoid `any`; use `unknown` plus validation when the shape is not known.

## API Types

- Store generated clients/types under `apps/web/src/api/generated/`.
- Generate gateway types from `docs/services/gateway/api/openapi.yaml`.
- Do not generate frontend clients from `docs/services/ai-gateway/api/openapi.yaml`
  or any internal `/internal/v1/**` service contract.
- Do not manually edit generated files.
- Wrap generated calls in feature-level functions when UI needs domain naming,
  query keys, or response normalization.
- Keep frontend DTO mapping explicit when backend response shape is not UI-ready.
- Gateway project JSON responses use `{ data, requestId }` for success,
  `{ data, page, requestId }` for paginated lists, and `{ error }` for failures.
  Do not extend the old `{ code, message, data }` client shape.
- Access tokens returned by auth/session responses are opaque Bearer tokens.
  Type them as strings and never decode them as JWT payloads.

## Zod Schemas

Use Zod for:

- Login and registration forms.
- Knowledge base create/edit forms.
- Retrieval parameter forms: Top K, similarity threshold, rerank threshold, selected knowledge bases.
- Model configuration forms: API URL, model name, timeout, credentials placeholders.
- Report generation parameters.
- Report outline and section save payloads when edited client-side.

Infer form value types from schemas:

```ts
const retrievalSettingsSchema = z.object({
  topK: z.number().int().min(1).max(100),
  similarityThreshold: z.number().min(0).max(1),
  rerankThreshold: z.number().min(0).max(1).optional(),
})

type RetrievalSettingsForm = z.infer<typeof retrievalSettingsSchema>
```

## Domain Types

Define domain types for important client-side structures:

```ts
type Citation = {
  documentId: string
  documentName: string
  chunkId: string
  content: string
  score: number
  sectionPath?: string
}

type ReportOutlineNode = {
  id: string
  title: string
  level: number
  kind: 'text' | 'table' | 'image'
  children?: ReportOutlineNode[]
}
```

Prefer generated backend types for persisted entities and explicit frontend types for UI-only state.

## Discriminated Unions

Use discriminated unions for status-heavy UI:

- Document processing status.
- Upload item status.
- Chat message status.
- Report section generation status.
- Long task status.

Example:

```ts
type UploadItemState =
  | { status: 'queued'; file: File }
  | { status: 'uploading'; file: File; progress: number }
  | { status: 'done'; documentId: string }
  | { status: 'failed'; file: File; message: string }
```

## Forbidden Patterns

- `any` for API responses, form values, route params, or event payloads.
- Blind `as` assertions to force types through compile errors.
- Duplicating backend DTO types by hand when generated types exist.
- Duplicating gateway OpenAPI types by hand or importing internal AI Gateway
  types into browser code.
- Allowing untyped search params into query keys.
- Treating streamed JSON chunks as trusted without parsing and validation.

## Scenario: Gateway Typed Transport Wrapper

### 1. Scope / Trigger
- Trigger: frontend API infrastructure must use the public gateway OpenAPI contract and normalize gateway transport behavior in one place.
- Applies to `apps/web/src/api/client.ts`, `apps/web/src/api/generated/gateway.ts`, and feature API wrappers under `apps/web/src/api/`.

### 2. Signatures
- Type generation command: `bun run --cwd apps/web api:generate`.
- Generation source: `docs/services/gateway/api/openapi.yaml`.
- Generated output: `apps/web/src/api/generated/gateway.ts`.
- Transport helpers must remain hand-written outside `api/generated/`:
  - `requestJson<T>(path, options): Promise<T>` unwraps `{ data, requestId }`.
  - `requestPaginated<T>(path, options): Promise<{ data: T[]; page; requestId }>` preserves pagination metadata.
  - `requestVoid(path, options): Promise<void>` handles empty success responses.
  - `requestBinary(path, options): Promise<Blob>` handles file downloads.
  - `streamGateway(path, options): { abort; signal }` uses `fetch` stream readers plus `AbortController`.

### 3. Contracts
- Base URL defaults to `/api/v1`; Vite may override it with `VITE_API_BASE_URL`.
- Auth uses `Authorization: Bearer <accessToken>`; tokens are opaque strings and must not be decoded as JWTs.
- Request id uses optional `X-Request-Id`.
- JSON success envelope: `{ data, requestId }`.
- Paginated envelope: `{ data, page: { page, pageSize, total }, requestId }`.
- Error envelope: `{ error: { code, message, requestId, fields? } }` mapped to `ApiError`.
- Upload requests use `FormData`; the wrapper must not force `Content-Type: application/json` for `FormData` bodies.
- SSE requests use `Accept: text/event-stream`; POST streaming must not use native `EventSource`.
- Mock handlers may only target active `paths` entries from generated gateway types. Top-level OpenAPI `x-missing-contracts` entries must not become callable methods or mocks.

### 4. Validation & Error Matrix
- Non-2xx JSON error envelope -> throw `ApiError` with gateway `code`, `message`, `requestId`, and `fields`.
- Non-2xx non-JSON response -> throw `ApiError` with `http_<status>` and response text/status text.
- Expected SSE response without `text/event-stream` -> throw `ApiError` code `invalid_stream_response`.
- Readable stream missing -> throw `ApiError` code `empty_stream_response`.
- Mock path not present in active OpenAPI `paths` -> throw before registering the mock route.

### 5. Good/Base/Bad Cases
- Good: feature wrapper imports generated schema types, calls `requestJson` or `requestPaginated`, and maps backend DTOs to UI DTOs explicitly.
- Base: generated `gateway.ts` is replaced wholesale by the generation command; no manual edits are made under `api/generated/`.
- Bad: feature code calls `/rag/search`, `/admin/stats/*`, `/admin/users`, or other inactive legacy paths directly.
- Bad: code assumes the legacy `{ code, message, data }` envelope or parses `message` text instead of `error.code`.

### 6. Tests Required
- `bun run --cwd apps/web check` must pass after API wrapper changes.
- `bun run --cwd apps/web build` must pass after generated type changes.
- `git diff --check` must pass.
- For future unit tests, assert envelope unwrapping, `ApiError` mapping, FormData header behavior, SSE event parsing, abort behavior, and active-path mock rejection.

### 7. Wrong vs Correct

#### Wrong
```ts
const res = await fetch('/api/v1/rag/search', { method: 'POST' })
const json: { code: number; message: string; data: Result } = await res.json()
if (json.code !== 0) throw new Error(json.message)
return json.data
```

#### Correct
```ts
const data = await requestJson<KnowledgeQuerySummary>('/knowledge-queries', {
  method: 'POST',
  body: { query, topK: 10, scoreThreshold: 0, rerank: false },
})
return data.results
```