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
- Generate gateway types from `docs/services/gateway/api/public.openapi.yaml` with
  `openapi-typescript@7.13.0`.
- Do not generate frontend clients from `docs/services/ai-gateway/api/internal.openapi.yaml`
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
- Generation source: `docs/services/gateway/api/public.openapi.yaml`.
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

## Scenario: Gateway Capability Error Presentation

### 1. Scope / Trigger

- Trigger: frontend pages call Gateway active paths whose backend workflow may
  still be staged, not implemented, or dependency-bound.
- Applies to Knowledge retrieval, document chunks/content, parser configs, and
  similar active contract paths under `apps/web/src/`.

### 2. Signatures

- Input error type: `ApiError` from `apps/web/src/api/client.ts`.
- Minimum fields used by UI classifiers: `status`, `code`, `message`,
  optional `requestId`.
- UI helper returns a typed issue with `kind`, `title`, `description`,
  `variant`, and `requestIdText`.

### 3. Contracts

- `501` or `error.code === "not_implemented"` means the route is active in
  Gateway but the backend workflow is not ready.
- `502` or `error.code === "dependency_error"` means a downstream service or
  infrastructure dependency failed.
- `403` or `error.code === "forbidden"` means permission denied and must not be
  presented as a readiness problem.
- User-visible notices must include `requestId` when present. If absent, say
  the response did not include one.
- Browser code must continue calling Gateway `/api/v1/**`; do not bypass
  Gateway to probe internal service readiness.

### 4. Validation & Error Matrix

| Condition | UI behavior |
| --- | --- |
| `501 not_implemented` | Show "capability not ready"; do not render empty data or fake success. |
| `502 dependency_error` | Show dependency failure with retry affordance when relevant. |
| `403 forbidden` | Show permission denied / forbidden state. |
| Gateway error with requestId | Include `requestId: <id>` in the notice detail. |
| Gateway error without requestId | State that no requestId was returned. |
| Non-Gateway error | State that requestId is unavailable because it was not a Gateway error. |

### 5. Good/Base/Bad Cases

- Good: Knowledge search clears stale results before mutation and shows a
  `not_implemented` warning if Gateway returns `501`.
- Base: A table query uses the shared classifier in its error state and keeps a
  retry button.
- Bad: Rendering `[]` when `/knowledge-queries` returns `501`.
- Bad: Matching localized error message text instead of `ApiError.code` or
  `ApiError.status`.

### 6. Tests Required

- Unit-test the classifier for `501/not_implemented`, `502/dependency_error`,
  `403/forbidden`, and missing requestId.
- For pages with stale mutable results, assert failed capability calls do not
  leave prior fake or stale success content visible.
- `bun run --cwd apps/web check` and `bun run --cwd apps/web build` must pass
  after changing shared API/error helpers.

### 7. Wrong vs Correct

#### Wrong

```ts
if (error instanceof Error) {
  setNotice(`加载失败: ${error.message}`)
}
setResults([])
```

#### Correct

```ts
const issue = getGatewayCapabilityIssue(error, '知识检索')
setNotice(`${issue.title}: ${issue.description}`)
setResults(null)
```

## Scenario: QA/LLM Config Version Forms

### 1. Scope / Trigger

- Trigger: frontend pages that display or save QA runtime settings, LLM
  generation settings, or LLM connection tests through gateway contracts.
- Scope: browser code under `apps/web/src/` only. The frontend does not own
  provider credentials, profile persistence, or backend validation semantics.

### 2. Signatures

- `GET /api/v1/qa-config-versions/current` -> `QAConfigVersion`.
- `POST /api/v1/qa-config-versions` with
  `CreateQAConfigVersionRequest` -> creates a new version.
- `GET /api/v1/llm-config-versions/current` -> `QALLMConfigVersion`.
- `POST /api/v1/llm-config-versions` with
  `CreateQALLMConfigVersionRequest` -> creates a new version.
- `POST /api/v1/llm-connection-tests` with
  `CreateQALLMConnectionTestRequest` -> creates a connection test record.

### 3. Contracts

- Import DTOs from `components['schemas']` in
  `apps/web/src/api/generated/gateway.ts`; do not hand-copy these response or
  request shapes.
- Normalize all requests through `gatewayRequest` so success uses
  `{ data, requestId }` and failures use the gateway error envelope.
- LLM request bodies must contain `provider: "ai-gateway"`, `profileId`, and
  `modelName`; optional fields may include generation or timeout parameters.
- Browser UI must not include provider API key fields, credential placeholders,
  secret refs, provider base URLs, or provider raw error details for QA-owned
  LLM config.

### 4. Validation & Error Matrix

- Empty `profileId` or `modelName` -> block the mutation and show a local form
  error.
- Non-numeric numeric field -> block the mutation and show a local form error.
- Integer-only field with decimal input -> block the mutation and show a local
  form error.
- Gateway `400`, `403`, or `502` -> show the sanitized gateway message and keep
  current form input unchanged.
- Missing backend/current config -> show a load error or empty metadata state;
  do not invent default server values.

### 5. Good/Base/Bad Cases

- Good: current config containing `0`, `false`, or `null` renders without being
  replaced by fallback defaults; use `??` and explicit formatting helpers.
- Base: save buttons create new config versions with `POST`; no frontend path
  should imply in-place update semantics.
- Bad: sending `apiKey`, masked key placeholders, provider base URLs, or raw
  provider errors from a QA config form.

### 6. Tests Required

- Typecheck assertion: request payloads satisfy generated OpenAPI schema types.
- Form assertion: `0`, `false`, and `null` values render distinctly and do not
  collapse through `||` defaults.
- Mutation assertion: LLM connection tests send only `provider`, `profileId`,
  `modelName`, and optional timeout.
- Failure assertion: failed test/save keeps user input and displays a sanitized
  error.

### 7. Wrong vs Correct

#### Wrong

```ts
const payload = {
  provider: 'openai',
  modelName,
  apiKey: maskedApiKey,
}
```

#### Correct

```ts
const payload: components['schemas']['CreateQALLMConnectionTestRequest'] = {
  provider: 'ai-gateway',
  profileId,
  modelName,
  timeoutSeconds,
}
```
