# QA 服务接口文档

本文档定义 `qa` 服务的前端接口草案，用于把智能问答前端接口设计适配到当前 gateway 契约体系。本文档基于以下来源整理：

- 外部前端接口文档《智能问答系统 — 前端接口文档》。
- [`Gateway 服务规划`](gateway.md)、[`前后端集成契约`](../architecture/frontend-backend-contract.md)、[`服务边界矩阵`](../architecture/service-boundaries.md) 和 [`Gateway OpenAPI 契约`](../api/gateway.openapi.yaml)。
- 外部 QA 数据库设计：`qa_config_versions`、`llm_config_versions`、`conversations`、`messages`、`response_runs`、`message_content_blocks`、`response_process_steps`、`response_stream_events`、`citations`、`retrieval_test_runs`、`retrieval_test_results`、`admin_audit_logs`。

> 当前状态：本文档是 `qa` 服务接口草案。`docs/api/gateway.openapi.yaml` 仍将 QA 相关接口登记在 `x-missing-contracts` 中；前端和后端在把这些接口升级为稳定契约前，需要同步更新 OpenAPI。

## 职责边界

| 范围 | 说明 |
| --- | --- |
| 会话管理 | 维护用户的 QA 会话、会话标题、状态和消息顺序。 |
| 消息与回答运行 | 保存用户消息、助手消息、一次回答生成的运行状态、token 使用量、延迟和失败原因。 |
| Agent Loop | 负责意图识别、路由、RAG 调用、生成、后处理和可展示处理步骤。 |
| SSE 事件 | 产生并短期保存前端可消费的流式事件，用于断线恢复或调试。 |
| 引用快照 | 保存回答中的引用编号、文档/chunk 外部 ID、引用文本、上下文和分数。 |
| QA 配置 | 管理问答检索参数、默认知识库选择和配置版本。 |
| LLM 配置 | 管理 AI Gateway profile 引用、模型名称、超时和生成参数。 |
| 检索体验测试 | 记录管理员发起的检索测试及结果快照。 |
| QA 统计 | 基于 `response_runs` 聚合问答次数、延迟、意图分布和热门问题。 |

`qa` 不拥有用户、角色、权限、知识库主数据、文档原文件、文档解析、向量索引或文件下载。相关能力由 `auth`、`knowledge`、`file` 等服务拥有，gateway 只做公开入口、认证上下文和响应归一化。

## 接入模型

```text
frontend
   |
   v
gateway /api/v1/qa-sessions, /api/v1/qa-config-versions, /api/v1/retrieval-test-runs
   |
   v
qa service
   |
   +--> PostgreSQL conversations / messages / response_runs / citations
   +--> knowledge service for retrieval and citation source lookups
   +--> ai-gateway for OpenAI-compatible model calls
```

Gateway 调用 QA 服务时应传递：

| Header | 说明 |
| --- | --- |
| `X-Request-Id` | 贯穿一次前端请求的 request id。 |
| `X-User-Id` | 已认证用户 ID；映射到 QA 数据库中的 `external_user_id`。 |
| `X-User-Roles` | 逗号分隔的角色列表。 |
| `X-User-Permissions` | 逗号分隔的权限列表。 |
| `X-Forwarded-For` | 原始客户端地址链。 |
| `X-Forwarded-Proto` | 原始协议。 |

前端不得设置 `X-User-Id`、`X-User-Roles`、`X-User-Permissions`。QA 服务必须在自己的边界校验用户上下文和权限，不依赖前端传入身份字段。

## 与前端原始设计的映射

| 原始路径 | Gateway 草案路径 | Owner | 说明 |
| --- | --- | --- | --- |
| `POST /api/conversations` | `POST /api/v1/qa-sessions` | `qa` | 创建 QA 会话。 |
| `GET /api/conversations` | `GET /api/v1/qa-sessions` | `qa` | 查询当前用户会话列表。 |
| `GET /api/conversations/{conversation_id}` | `GET /api/v1/qa-sessions/{sessionId}` | `qa` | 查询会话详情。 |
| `PUT /api/conversations/{conversation_id}` | `PATCH /api/v1/qa-sessions/{sessionId}` | `qa` | 更新会话标题或状态。 |
| `DELETE /api/conversations/{conversation_id}` | `DELETE /api/v1/qa-sessions/{sessionId}` | `qa` | 归档或删除会话。 |
| `POST /api/chat/stream` | `POST /api/v1/qa-sessions/{sessionId}/messages` | `qa` | 创建用户消息并以 SSE 返回助手消息生成过程。 |
| `POST /api/rag/search` | `POST /api/v1/knowledge-queries` | `knowledge` | 正式检索查询由 `knowledge` 拥有；QA 只在 Agent Loop 或检索测试中调用。 |
| `POST /api/admin/rag/test` | `POST /api/v1/retrieval-test-runs` | `qa` | 管理员检索体验测试。 |
| `GET /api/citations/{chunk_id}` | `GET /api/v1/citations/{citationId}` | `qa` / `knowledge` | QA 返回回答引用快照；原文详情可由知识服务补齐。 |
| `POST /api/citations/batch` | `POST /api/v1/citation-lookups` | `qa` / `knowledge` | 批量引用详情查询草案。 |
| `GET/PUT /api/admin/knowledge-config` | `GET /api/v1/qa-config-versions/current`, `POST /api/v1/qa-config-versions` | `qa` | 配置采用版本化资源。 |
| `GET/PUT /api/admin/llm-config` | `GET /api/v1/llm-config-versions/current`, `POST /api/v1/llm-config-versions` | `qa` | LLM 配置采用版本化资源，只保存 AI Gateway profile 引用和生成参数。 |
| `POST /api/admin/llm-config/test` | `POST /api/v1/llm-connection-tests` | `qa` | 通过 AI Gateway profile 创建一次 LLM 连接测试记录。 |
| `GET /api/admin/stats/*` | `GET /api/v1/qa-metrics/*` | `qa` | QA 指标由 `response_runs` 聚合。 |

## 通用响应结构

JSON 成功响应遵循 gateway 统一 envelope：

```json
{
  "data": {},
  "requestId": "req_123"
}
```

分页响应：

```json
{
  "data": [],
  "page": {
    "page": 1,
    "pageSize": 20,
    "total": 100
  },
  "requestId": "req_123"
}
```

错误响应固定为：

```json
{
  "error": {
    "code": "validation_error",
    "message": "request validation failed",
    "requestId": "req_123",
    "fields": {
      "message": "is required"
    }
  }
}
```

错误码映射：

| 原始数字码 | 项目错误码 | HTTP status | 说明 |
| --- | --- | --- | --- |
| `40000` | `validation_error` | `400` | 请求参数错误。 |
| `40100` | `unauthorized` | `401` | 未登录或会话失效。 |
| `40300` | `forbidden` | `403` | 已登录但权限不足。 |
| `40400` | `not_found` | `404` | 会话、消息、引用或配置不存在。 |
| `50000` | `internal_error` | `500` | QA 服务未预期错误。 |
| `50100` | `dependency_error` | `502` | LLM 服务失败。 |
| `50200` | `dependency_error` | `502` | 知识检索或重排序依赖失败。 |
| `50300` | `dependency_error` | `502` | 文档处理或知识服务依赖失败。 |

## 公开接口总览

| Method | Gateway Path | Auth | Owner | 说明 |
| --- | --- | --- | --- | --- |
| `POST` | `/api/v1/qa-sessions` | 需要 | `qa` | 创建 QA 会话。 |
| `GET` | `/api/v1/qa-sessions` | 需要 | `qa` | 查询当前用户 QA 会话列表。 |
| `GET` | `/api/v1/qa-sessions/{sessionId}` | 需要 | `qa` | 查询会话详情和消息摘要。 |
| `PATCH` | `/api/v1/qa-sessions/{sessionId}` | 需要 | `qa` | 更新会话标题或归档状态。 |
| `DELETE` | `/api/v1/qa-sessions/{sessionId}` | 需要 | `qa` | 删除或软删除会话。 |
| `GET` | `/api/v1/qa-sessions/{sessionId}/messages` | 需要 | `qa` | 查询会话消息列表。 |
| `POST` | `/api/v1/qa-sessions/{sessionId}/messages` | 需要 | `qa` | 创建用户消息并触发助手回答；可返回 SSE。 |
| `GET` | `/api/v1/qa-sessions/{sessionId}/events` | 需要 | `qa` | 按 `responseRunId` 读取短期保存的流式事件。 |
| `GET` | `/api/v1/messages/{messageId}/citations` | 需要 | `qa` | 查询某条助手消息的引用列表。 |
| `GET` | `/api/v1/citations/{citationId}` | 需要 | `qa` | 查询引用快照详情。 |
| `POST` | `/api/v1/citation-lookups` | 需要 | `qa` / `knowledge` | 批量查询引用详情草案。 |
| `GET` | `/api/v1/qa-config-versions/current` | 需要 | `qa` | 查询当前生效 QA 配置。 |
| `POST` | `/api/v1/qa-config-versions` | 需要 | `qa` | 创建新的 QA 配置版本并可设置为生效。 |
| `GET` | `/api/v1/llm-config-versions/current` | 需要 | `qa` | 查询当前生效 LLM 配置。 |
| `POST` | `/api/v1/llm-config-versions` | 需要 | `qa` | 创建新的 LLM 配置版本并可设置为生效。 |
| `POST` | `/api/v1/llm-connection-tests` | 需要 | `qa` | 测试一次 LLM 配置连接。 |
| `POST` | `/api/v1/retrieval-test-runs` | 需要 | `qa` | 创建一次检索体验测试。 |
| `GET` | `/api/v1/retrieval-test-runs/{testRunId}` | 需要 | `qa` | 查询检索体验测试结果。 |
| `GET` | `/api/v1/qa-metrics/overview` | 需要 | `qa` | 查询 QA 统计概览。 |
| `GET` | `/api/v1/qa-metrics/trend` | 需要 | `qa` | 查询问答趋势。 |
| `GET` | `/api/v1/qa-metrics/top-queries` | 需要 | `qa` | 查询热门问题。 |
| `GET` | `/api/v1/qa-metrics/intent-distribution` | 需要 | `qa` | 查询意图分布。 |

相关但非 QA-owned 的公开接口：

| Method | Gateway Path | Owner | 说明 |
| --- | --- | --- | --- |
| `POST` | `/api/v1/knowledge-queries` | `knowledge` | 语义检索、跨知识库联合检索和重排序结果。QA 可在内部调用，但不拥有知识索引。 |
| `GET/POST/PATCH/DELETE` | `/api/v1/knowledge-bases/**` | `knowledge` | 知识库和知识库内文档状态。 |
| `GET` | `/api/v1/documents/{documentId}/content` | `file` | 下载或读取原始文件内容。 |
| `GET/POST/PATCH/DELETE` | `/api/v1/users`, `/api/v1/sessions`, `/api/v1/users/me` | `auth` | 用户和会话。 |

## 会话与消息数据结构

### QASession

```json
{
  "id": "qa_sess_123",
  "title": "变压器巡检要点",
  "status": "active",
  "messageCount": 5,
  "lastMessagePreview": "根据知识库检索结果...",
  "createdAt": "2026-06-27T10:30:00Z",
  "updatedAt": "2026-06-27T11:00:00Z"
}
```

| 字段 | 类型 | 必填 | 来源 |
| --- | --- | --- | --- |
| `id` | `string` | 是 | `conversations.id` 的公开 ID。 |
| `title` | `string` | 否 | `conversations.title`。 |
| `status` | `active \| archived` | 是 | `conversations.status`。 |
| `messageCount` | `number` | 否 | 基于 `messages` 聚合。 |
| `lastMessagePreview` | `string` | 否 | 最近一条可展示消息摘要。 |
| `createdAt` | `string(date-time)` | 是 | `conversations.created_at`。 |
| `updatedAt` | `string(date-time)` | 是 | `conversations.updated_at`。 |

### QAMessage

```json
{
  "id": "msg_001",
  "sessionId": "qa_sess_123",
  "role": "assistant",
  "sequenceNo": 2,
  "status": "completed",
  "content": "根据知识库检索结果...",
  "thinking": [
    {
      "type": "retrieval",
      "label": "检索知识库",
      "status": "done",
      "detail": "命中 5 条结果"
    }
  ],
  "citations": [],
  "createdAt": "2026-06-27T10:30:15Z",
  "completedAt": "2026-06-27T10:30:18Z"
}
```

| 字段 | 类型 | 必填 | 来源 |
| --- | --- | --- | --- |
| `id` | `string` | 是 | `messages.id`。 |
| `sessionId` | `string` | 是 | `messages.conversation_id`。 |
| `role` | `user \| assistant \| system` | 是 | `messages.role`。 |
| `sequenceNo` | `number` | 是 | `messages.sequence_no`。 |
| `status` | `streaming \| completed \| stopped \| failed` | 是 | `messages.status`。 |
| `content` | `string` | 是 | 合并 `message_content_blocks` 中可展示文本块。 |
| `thinking` | `ThinkingStep[]` | 否 | `response_process_steps` 中可向用户展示的步骤。不得包含私有思维链。 |
| `citations` | `Citation[]` | 否 | `citations`。 |
| `createdAt` | `string(date-time)` | 是 | `messages.created_at`。 |
| `completedAt` | `string(date-time)` | 否 | `messages.completed_at`。 |

### ThinkingStep

```json
{
  "type": "retrieval",
  "label": "检索知识库",
  "status": "done",
  "detail": "命中 5 条结果，耗时 42ms"
}
```

允许的 `type` 初始值：`intent`、`retrieval`、`generation`、`verify`。

允许的 `status` 初始值：`pending`、`running`、`done`、`failed`。

## Endpoint 详情

### POST /api/v1/qa-sessions

创建当前用户的 QA 会话。

请求：

```json
{
  "title": "新对话"
}
```

响应 `201`：

```json
{
  "data": {
    "id": "qa_sess_123",
    "title": "新对话",
    "status": "active",
    "messageCount": 0,
    "createdAt": "2026-06-27T10:30:00Z",
    "updatedAt": "2026-06-27T10:30:00Z"
  },
  "requestId": "req_123"
}
```

### GET /api/v1/qa-sessions

查询当前用户的 QA 会话列表。

查询参数：

| 参数 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `page` | `number` | 否 | `1` | 页码，从 1 开始。 |
| `pageSize` | `number` | 否 | `20` | 每页数量。 |
| `status` | `string` | 否 | `active` | 可选：`active`、`archived`。 |
| `sort` | `string` | 否 | `-updatedAt` | 排序字段。 |

响应 `200`：

```json
{
  "data": [
    {
      "id": "qa_sess_123",
      "title": "变压器巡检要点",
      "status": "active",
      "messageCount": 5,
      "lastMessagePreview": "根据知识库检索结果...",
      "createdAt": "2026-06-27T10:30:00Z",
      "updatedAt": "2026-06-27T11:00:00Z"
    }
  ],
  "page": {
    "page": 1,
    "pageSize": 20,
    "total": 12
  },
  "requestId": "req_123"
}
```

### GET /api/v1/qa-sessions/{sessionId}

查询会话详情。默认返回最近消息摘要；完整消息列表由 `/messages` 子资源查询。

响应 `200`：

```json
{
  "data": {
    "id": "qa_sess_123",
    "title": "变压器巡检要点",
    "status": "active",
    "messageCount": 5,
    "createdAt": "2026-06-27T10:30:00Z",
    "updatedAt": "2026-06-27T11:00:00Z"
  },
  "requestId": "req_123"
}
```

### PATCH /api/v1/qa-sessions/{sessionId}

更新会话标题或状态。只允许当前用户更新自己的会话。

请求：

```json
{
  "title": "变压器巡检与维护要点",
  "status": "active"
}
```

响应 `200`：返回更新后的 `QASession`。

### DELETE /api/v1/qa-sessions/{sessionId}

删除或软删除会话。实现可将 `conversations.deleted_at` 置为当前时间，并隐藏该会话及其消息。

响应 `204`：无响应体。

### GET /api/v1/qa-sessions/{sessionId}/messages

查询会话消息列表。

查询参数：

| 参数 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `page` | `number` | 否 | `1` | 页码。 |
| `pageSize` | `number` | 否 | `50` | 每页消息数。 |
| `includeThinking` | `boolean` | 否 | `true` | 是否返回可展示处理步骤。 |
| `includeCitations` | `boolean` | 否 | `true` | 是否返回引用摘要。 |

响应 `200`：分页返回 `QAMessage[]`。

### POST /api/v1/qa-sessions/{sessionId}/messages

创建一条用户消息并触发 Agent Loop。请求头 `Accept: text/event-stream` 时，gateway 以 SSE 流式返回回答过程；请求完成后，用户消息、助手消息、回答运行、处理步骤和引用快照均应持久化。

请求：

```json
{
  "message": "变压器巡检有哪些要点？",
  "knowledgeBaseIds": ["kb_power_standard", "kb_device_manual"],
  "params": {
    "topK": 5,
    "similarityThreshold": 0.7,
    "useRerank": true,
    "rerankThreshold": 0.5,
    "rerankTopN": 3
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `message` | `string` | 是 | 用户输入内容。 |
| `knowledgeBaseIds` | `string[]` | 否 | 指定知识库 ID；为空时使用当前 QA 配置。 |
| `params.topK` | `number` | 否 | 覆盖当前配置的检索返回数。 |
| `params.similarityThreshold` | `number` | 否 | 覆盖当前配置的相似度阈值。 |
| `params.useRerank` | `boolean` | 否 | 是否启用重排序。 |
| `params.rerankThreshold` | `number` | 否 | 重排序阈值。 |
| `params.rerankTopN` | `number` | 否 | 重排序后保留数。 |

非流式响应 `202` 草案：

```json
{
  "data": {
    "responseRunId": "run_123",
    "userMessageId": "msg_user_001",
    "assistantMessageId": "msg_assistant_002",
    "status": "running"
  },
  "requestId": "req_123"
}
```

流式响应使用 `Content-Type: text/event-stream`，事件结构见下一节。

## SSE 事件协议

SSE 通用格式：

```text
event: <event_type>
id: <event_seq>
data: <json_payload>
```

事件类型：

| 事件类型 | 触发时机 | 用途 | 数据库来源 |
| --- | --- | --- | --- |
| `intent` | 意图识别开始或完成 | 展示意图识别状态。 | `response_runs.intent_type`、`response_process_steps`。 |
| `step` | 每个可展示步骤变化 | 更新前端思考步骤列表。 | `response_process_steps`。 |
| `token` | LLM 生成文本增量 | 流式渲染回答。 | `response_stream_events`，最终合并入 `message_content_blocks`。 |
| `citation` | 引用产生或确认 | 展示引用标注。 | `citations`。 |
| `done` | 回答完成 | 关闭流式状态。 | `response_runs`、`messages`。 |
| `error` | 任何环节失败 | 展示错误并决定是否终止流。 | `response_runs.error`、`messages.error_code`。 |
| `heartbeat` | 空闲保活 | 防止代理超时。 | 可不持久化。 |

> 数据库当前 `response_stream_events.event_type` 约束为 `intent`、`step`、`token`、`citation`、`done`、`error`。`heartbeat` 是传输层事件，不要求持久化。

示例：

```text
event: intent
id: 1
data: {"status":"started","label":"正在分析问题..."}

event: intent
id: 2
data: {"status":"done","label":"识别为：知识问答","intent":"knowledge_qa","confidence":0.95}

event: step
id: 3
data: {"step":{"type":"retrieval","label":"检索知识库","status":"running"}}

event: token
id: 4
data: {"text":"根","index":0}

event: citation
id: 5
data: {"citation":{"id":"cit_1","citationNo":1,"docId":"DOC-001","docName":"电力变压器巡检手册.pdf","chunkId":"chunk_abc","text":"变压器外壳应保持清洁...","score":0.96}}

event: done
id: 6
data: {"responseRunId":"run_123","messageId":"msg_assistant_002","totalTokens":256,"latencyMs":3200}
```

### GET /api/v1/qa-sessions/{sessionId}/events

读取短期保存的 SSE 事件，主要用于断线后恢复 UI 或调试。

查询参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `responseRunId` | `string` | 是 | 回答运行 ID。 |
| `afterEventSeq` | `number` | 否 | 只返回指定序号之后的事件。 |

响应 `200`：

```json
{
  "data": [
    {
      "eventSeq": 4,
      "eventType": "token",
      "payload": {
        "text": "根",
        "index": 0
      },
      "createdAt": "2026-06-27T10:30:16Z"
    }
  ],
  "requestId": "req_123"
}
```

## 引用溯源

### Citation

```json
{
  "id": "cit_123",
  "messageId": "msg_assistant_002",
  "citationNo": 1,
  "docId": "DOC-001",
  "docName": "电力变压器巡检手册.pdf",
  "chunkId": "chunk_abc",
  "text": "变压器外壳应保持清洁，无渗漏油现象...",
  "context": "第三章 巡检项目\n3.1 外观检查\n...",
  "pageNumber": 12,
  "score": 0.96,
  "metadata": {}
}
```

QA 返回的是回答时保存的引用快照，字段对应 `citations.external_doc_id`、`external_chunk_id`、`quote_text`、`context`、`page_number` 和 `score`。如果前端需要原文件内容，应使用 file-owned `GET /api/v1/documents/{documentId}/content`。

### GET /api/v1/messages/{messageId}/citations

查询助手消息的引用列表。

响应 `200`：

```json
{
  "data": [
    {
      "id": "cit_123",
      "messageId": "msg_assistant_002",
      "citationNo": 1,
      "docId": "DOC-001",
      "docName": "电力变压器巡检手册.pdf",
      "chunkId": "chunk_abc",
      "text": "变压器外壳应保持清洁...",
      "pageNumber": 12,
      "score": 0.96
    }
  ],
  "requestId": "req_123"
}
```

### GET /api/v1/citations/{citationId}

查询单条引用快照详情。

响应 `200`：返回 `Citation`。

### POST /api/v1/citation-lookups

批量查询引用详情草案。若需要实时从知识服务补齐 chunk 上下文，QA 应通过内部客户端调用 `knowledge`，并由 gateway 保持对前端的统一 envelope。

请求：

```json
{
  "citationIds": ["cit_123", "cit_456"]
}
```

响应 `200`：

```json
{
  "data": [
    {
      "id": "cit_123",
      "messageId": "msg_assistant_002",
      "citationNo": 1,
      "docId": "DOC-001",
      "docName": "电力变压器巡检手册.pdf",
      "chunkId": "chunk_abc",
      "text": "变压器外壳应保持清洁...",
      "context": "第三章 巡检项目\n3.1 外观检查\n...",
      "pageNumber": 12,
      "score": 0.96
    }
  ],
  "requestId": "req_123"
}
```

## QA 配置接口

### QAConfigVersion

```json
{
  "id": "qa_cfg_1",
  "versionNo": 1,
  "topK": 5,
  "similarityThreshold": 0.7,
  "useRerank": false,
  "rerankThreshold": null,
  "rerankTopN": null,
  "knowledgeBases": [
    {
      "id": "kb_power_standard",
      "type": "technical_supervision",
      "displayName": "电力标准规范库",
      "sortOrder": 0
    }
  ],
  "isActive": true,
  "createdAt": "2026-06-27T09:00:00Z"
}
```

### GET /api/v1/qa-config-versions/current

查询当前生效 QA 配置。

响应 `200`：返回 `QAConfigVersion`。

### POST /api/v1/qa-config-versions

创建新的 QA 配置版本。为避免原地修改配置导致历史回答不可追溯，更新配置时创建新版本；`activate` 为 `true` 时将新版本设为当前生效版本。

请求：

```json
{
  "topK": 5,
  "similarityThreshold": 0.65,
  "useRerank": true,
  "rerankThreshold": 0.5,
  "rerankTopN": 3,
  "knowledgeBases": [
    {
      "id": "kb_power_standard",
      "type": "technical_supervision",
      "displayName": "电力标准规范库",
      "sortOrder": 0
    }
  ],
  "activate": true
}
```

响应 `201`：返回新建的 `QAConfigVersion`。

## LLM 配置接口

### LLMConfigVersion

```json
{
  "id": "llm_cfg_1",
  "versionNo": 1,
  "provider": "ai-gateway",
  "profileId": "mp_chat_default",
  "modelName": "gpt-4o-mini",
  "timeoutSeconds": 60,
  "temperature": 0.7,
  "maxTokens": 4096,
  "isActive": true,
  "createdAt": "2026-06-27T09:00:00Z"
}
```

QA 不得保存或向前端返回 provider API key。Provider `baseUrl` 和密钥由 AI Gateway 的 model profile 管理，QA 只保存 `profileId`、模型名、超时和生成参数。

### GET /api/v1/llm-config-versions/current

查询当前生效 LLM 配置。

响应 `200`：返回 `LLMConfigVersion`。

### POST /api/v1/llm-config-versions

创建新的 LLM 配置版本。

请求：

```json
{
  "provider": "ai-gateway",
  "profileId": "mp_chat_default",
  "modelName": "Qwen/Qwen2.5-7B-Instruct",
  "timeoutSeconds": 30,
  "temperature": 0.1,
  "maxTokens": 2048,
  "activate": true
}
```

响应 `201`：返回脱敏后的 `LLMConfigVersion`。

### POST /api/v1/llm-connection-tests

通过 AI Gateway profile 创建一次 LLM 连接测试记录。该接口不接收 provider API key，也不应把 prompt、provider 原始错误或内部 URL 写入日志和响应。

请求：

```json
{
  "provider": "ai-gateway",
  "profileId": "mp_chat_default",
  "modelName": "Qwen/Qwen2.5-7B-Instruct",
  "timeoutSeconds": 30
}
```

响应 `201`：

```json
{
  "data": {
    "id": "llm_test_123",
    "success": true,
    "latencyMs": 520,
    "modelName": "Qwen/Qwen2.5-7B-Instruct",
    "testedAt": "2026-06-27T11:00:00Z"
  },
  "requestId": "req_123"
}
```

## 检索体验测试

正式知识检索由 `knowledge` 服务拥有。QA 的检索体验测试只记录管理员使用当前 QA 配置发起的一次测试运行及结果快照，对应 `retrieval_test_runs` 和 `retrieval_test_results`。

### POST /api/v1/retrieval-test-runs

请求：

```json
{
  "query": "变压器巡检要点",
  "knowledgeBaseIds": ["kb_power_standard"],
  "overrides": {
    "topK": 5,
    "similarityThreshold": 0.7,
    "useRerank": true,
    "rerankThreshold": 0.5,
    "rerankTopN": 3
  }
}
```

响应 `201`：

```json
{
  "data": {
    "id": "retrieval_run_123",
    "query": "变压器巡检要点",
    "status": "completed",
    "resultCount": 2,
    "latencyMs": 42,
    "results": [
      {
        "rankNo": 1,
        "knowledgeBaseId": "kb_power_standard",
        "docId": "DOC-001",
        "docName": "电力变压器巡检手册.pdf",
        "chunkId": "chunk_abc",
        "text": "变压器外壳应保持清洁，无渗漏油现象...",
        "vectorScore": 0.92,
        "rerankScore": 0.96,
        "metadata": {
          "pageNumber": 12,
          "chunkIndex": 5
        }
      }
    ],
    "createdAt": "2026-06-27T11:00:00Z",
    "finishedAt": "2026-06-27T11:00:01Z"
  },
  "requestId": "req_123"
}
```

### GET /api/v1/retrieval-test-runs/{testRunId}

查询检索体验测试结果。

响应 `200`：返回与创建接口一致的测试运行详情。

## QA 统计接口

统计接口基于 `response_runs`、`messages` 和 `citations` 聚合，不新增重复统计事实表。

### GET /api/v1/qa-metrics/overview

查询参数：

| 参数 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `days` | `number` | 否 | `1` | 统计窗口。 |

响应 `200`：

```json
{
  "data": {
    "totalQaCount": 12850,
    "todayQaCount": 127,
    "avgLatencyMs": 3200,
    "activeUsersToday": 23,
    "knowledgeBaseCount": 5,
    "documentCount": 78
  },
  "requestId": "req_123"
}
```

`knowledgeBaseCount` 和 `documentCount` 来源于 `knowledge` 服务或聚合缓存；QA 服务不能把知识库主数据复制为自己的源数据。

### GET /api/v1/qa-metrics/trend

查询参数：`days`，默认 `30`。

响应 `200`：

```json
{
  "data": {
    "days": 30,
    "points": [
      {
        "date": "2026-06-27",
        "count": 127
      }
    ]
  },
  "requestId": "req_123"
}
```

### GET /api/v1/qa-metrics/top-queries

查询参数：

| 参数 | 类型 | 必填 | 默认值 |
| --- | --- | --- | --- |
| `limit` | `number` | 否 | `10` |
| `days` | `number` | 否 | `7` |

响应 `200`：

```json
{
  "data": [
    {
      "query": "变压器巡检有哪些要点",
      "count": 86,
      "avgLatencyMs": 3100,
      "lastAskedAt": "2026-06-27T10:30:00Z"
    }
  ],
  "requestId": "req_123"
}
```

### GET /api/v1/qa-metrics/intent-distribution

查询参数：`days`，默认 `7`。

响应 `200`：

```json
{
  "data": [
    {
      "intent": "knowledge_qa",
      "label": "知识问答",
      "count": 650,
      "percent": 72.2
    },
    {
      "intent": "general_chat",
      "label": "一般对话",
      "count": 180,
      "percent": 20.0
    }
  ],
  "requestId": "req_123"
}
```

## Agent Loop 处理约定

前端只需要创建消息并消费 SSE 事件。QA 服务内部按以下顺序执行：

```text
user message
  -> intent classification
  -> route selection
  -> retrieval through knowledge service when intent requires RAG
  -> optional rerank
  -> LLM generation
  -> citation post-processing
  -> persist assistant message, content blocks, process steps, citations, and response run
```

支持的意图初始值：

| 意图 | 标签 | 处理链路 |
| --- | --- | --- |
| `knowledge_qa` | 知识问答 | RAG 检索、重排序、生成、引用。 |
| `general_chat` | 一般对话 | 直接 LLM 生成，不调用知识检索。 |
| `document_query` | 文档查询 | 查询文档元信息，具体 owner 需与 `knowledge` 确认。 |
| `system_command` | 系统指令 | 解析指令并执行允许的系统操作，需单独权限控制。 |

QA 服务只能保存可向用户展示的处理步骤。不得保存或返回模型私有思维链、完整 prompt、内部工具参数、向量 payload、下游内部 URL、原始 token 或完整 API key。

## 内部服务接口草案

以下接口为 gateway 到 QA 的内部草案，不直接暴露给前端；公开路径仍以 `/api/v1/**` 为准。

| Method | Internal Path | 说明 |
| --- | --- | --- |
| `POST` | `/internal/qa-sessions` | 创建会话。 |
| `GET` | `/internal/qa-sessions` | 按 `X-User-Id` 查询会话列表。 |
| `POST` | `/internal/qa-sessions/{sessionId}/messages` | 创建消息并触发回答。 |
| `GET` | `/internal/qa-sessions/{sessionId}/events` | 查询短期流式事件。 |
| `GET` | `/internal/messages/{messageId}/citations` | 查询消息引用。 |
| `GET` | `/internal/qa-config-versions/current` | 查询当前 QA 配置。 |
| `POST` | `/internal/qa-config-versions` | 创建配置版本。 |
| `POST` | `/internal/retrieval-test-runs` | 创建检索体验测试。 |

内部服务错误也必须被 gateway 归一化为公开错误 envelope，不能把 SQL 错误、AI Gateway 或 provider 原始错误、密钥引用、prompt 或内部 URL 直接返回给前端。

## OpenAPI 升级清单

当团队决定把本文档升级为稳定公开契约时，需要同步：

- 在 `docs/api/gateway.openapi.yaml` 添加对应 active `paths`、schemas、tags、`operationId`、错误响应和 `x-owner-service`。
- 从 `x-missing-contracts` 中移除或收窄已稳定的 QA placeholder。
- 更新 [`前后端集成契约`](../architecture/frontend-backend-contract.md) 中 SSE 和 QA 路径说明。
- 更新 [`服务边界矩阵`](../architecture/service-boundaries.md) 中 QA 缺失状态。
- 前端只从 OpenAPI active paths 生成或实现可调用 client。
