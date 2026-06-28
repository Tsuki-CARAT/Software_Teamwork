# 智能问答 API 契约

## 1. 契约目标

本文定义智能问答模块的 HTTP API 契约，覆盖会话管理、多轮对话、SSE 流式回答、非流式回答、意图识别、RAG 检索增强、引用溯源、处理过程展示、问答配置、检索体验测试和统计监控。

主责服务：

- `qa`：会话、消息、意图识别、RAG 编排、答案生成、引用记录、问答统计。
- `knowledge`：知识检索能力，提供候选切片和引用元数据。
- `auth`：用户身份、角色权限。
- `gateway`：外部 API 入口和认证上下文转发。
- `ai-gateway`：统一提供 OpenAI-compatible LLM 调用入口。

边界原则：

- `qa` 不直接读写 Qdrant，不维护向量集合，不拥有文档切片索引。
- `qa` 通过 `knowledge` 的检索 API 获取上下文片段。
- `qa` 可保存回答、消息、引用快照和处理过程摘要，便于历史回放。
- 原文文件下载由知识管理或文件服务校验权限并生成下载 URL。
- 前端可展示“处理过程摘要”，不建议展示原始模型 chain-of-thought。
- 会话、消息和引用以服务端 PostgreSQL 为权威数据源；前端本地只缓存当前 `sessionId` 等恢复信息。

## 2. 通用约定

### 2.1 基础路径

外部 API 统一使用 `/api/v1` 作为网关前缀：

```text
/api/v1/qa/...
```

### 2.2 RESTful + OpenAPI + Swagger UI 规范

本模块接口必须按 RESTful 风格设计，并以 OpenAPI 3.0+ 作为机器可读契约。Swagger UI 用于开发联调、验收演示和接口自测。

RESTful 约定：

- 会话、消息、引用、配置、统计均作为资源或子资源表达。
- 创建会话使用 `POST /qa/conversations`，查询消息使用 `GET /qa/conversations/{conversationId}/messages`。
- 生成、取消、识别等动作型能力使用 `POST /resource:action`，例如 `POST /qa/conversations/{conversationId}/messages:stream`。
- 流式问答仍需在 OpenAPI 中声明为 `text/event-stream` 响应，并列出事件类型。
- 所有历史列表接口必须分页。
- 时间字段统一使用 UTC ISO 8601 字符串。

OpenAPI 约定：

- `qa` 服务维护服务内契约：`services/qa/api/openapi.yaml`。
- 当前文档配套的 OpenAPI 草稿见 [qa.openapi.yaml](./openapi/qa.openapi.yaml)。
- OpenAPI 必须声明 `securitySchemes`、通用错误响应、分页响应、SSE 响应、会话/消息/引用 schema、状态枚举。
- 对 `knowledge` 检索 API 的依赖应通过外部契约引用或在描述中标明，不复制 Qdrant 内部 payload 结构。

Swagger UI 约定：

- 网关应暴露聚合入口，建议为 `/api/docs`。
- 服务级 OpenAPI JSON/YAML 建议暴露为 `/api/v1/qa/openapi.yaml` 或通过网关聚合。
- Swagger UI 只用于开发、测试和内网验收环境；生产环境是否开放需由部署配置控制。

### 2.3 认证与权限

首期统一采用 Bearer Token/JWT：

```http
Authorization: Bearer <accessToken>
```

SSE 流式接口与普通 JSON 接口使用同一套 Bearer 鉴权，前端在请求头携带 `Authorization`；当前不为 QA 单独设计独立会话鉴权通道。

权限要求：

| 能力 | 标准用户 | 管理员 | 超级管理员 |
| --- | --- | --- | --- |
| 创建和使用自己的会话 | 支持 | 支持 | 支持 |
| 查看自己的历史会话 | 支持 | 支持 | 支持 |
| 删除自己的会话 | 支持 | 支持 | 支持 |
| 配置问答参数 | 不支持 | 支持 | 支持 |
| 检索体验测试 | 不支持 | 支持 | 支持 |
| 查看全局统计 | 不支持 | 支持 | 支持 |

首期只做角色级 RBAC。无权限知识库由 `knowledge` 检索接口按可见性过滤，QA 可在调试信息中返回 filtered 数量。

### 2.4 通用错误响应

```json
{
  "error": {
    "code": "validation_error",
    "message": "request validation failed",
    "requestId": "req_123",
    "fields": {
      "message": "required"
    }
  }
}
```

通用错误码同知识管理契约：

```text
validation_error
unauthorized
forbidden
not_found
conflict
rate_limited
dependency_error
internal_error
```

### 2.5 SSE 事件约定

流式问答接口使用 Server-Sent Events。

响应头：

```http
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

事件类型：

| event | 含义 |
| --- | --- |
| `message.created` | 消息记录已创建 |
| `intent.detected` | 意图识别完成 |
| `retrieval.started` | 开始检索 |
| `retrieval.completed` | 检索完成 |
| `answer.delta` | 回答增量文本 |
| `citation.delta` | 引用增量或引用预告 |
| `reasoning.step` | 处理过程摘要步骤 |
| `answer.completed` | 回答完成 |
| `error` | 流式过程失败 |

示例：

```text
event: answer.delta
data: {"messageId":"msg_002","delta":"根据技术监督规程，"}

event: answer.completed
data: {"messageId":"msg_002","status":"completed"}
```

## 3. 数据对象

### 3.1 Conversation

```json
{
  "id": "conv_001",
  "title": "锅炉技术监督咨询",
  "ownerUserId": "user_001",
  "status": "active",
  "createdAt": "2026-06-28T10:00:00Z",
  "updatedAt": "2026-06-28T10:05:00Z",
  "lastMessageAt": "2026-06-28T10:05:00Z"
}
```

状态：

```text
active
deleted
```

会话删除采用软删除，便于误删恢复和统计口径稳定；完整审计服务首期暂缓。

### 3.2 Message

```json
{
  "id": "msg_001",
  "conversationId": "conv_001",
  "role": "user",
  "content": "锅炉技术监督有哪些检查要求？",
  "intent": "knowledge_qa",
  "status": "completed",
  "createdAt": "2026-06-28T10:00:00Z"
}
```

角色：

```text
user
assistant
system
```

消息状态：

```text
queued
generating
completed
failed
cancelled
```

意图：

```text
knowledge_qa
general_chat
report_generation
data_analysis
unknown
```

`data_analysis` 仅作为后续扩展意图预留。首期统计指标 API 仍然实现；Excel/表格类数据分析意图本期不实现，命中时返回 `unsupported_intent`，不创建数据分析任务。

### 3.3 Citation

```json
{
  "id": "cit_001",
  "messageId": "msg_002",
  "chunkId": "chunk_001",
  "documentId": "doc_001",
  "knowledgeBaseId": "kb_001",
  "documentName": "技术监督规程.pdf",
  "sectionPath": "1. 总则 / 1.1 适用范围",
  "contentPreview": "本规程适用于...",
  "score": 0.82,
  "rerankScore": 0.91,
  "chunkType": "text",
  "isSourceAvailable": true
}
```

### 3.4 ReasoningStep

```json
{
  "id": "step_001",
  "messageId": "msg_002",
  "type": "retrieval",
  "title": "检索技术监督知识库",
  "summary": "命中 8 个相关片段，最高相关度 0.91",
  "status": "completed",
  "createdAt": "2026-06-28T10:00:01Z"
}
```

说明：

- 该对象用于展示处理链路摘要，例如“识别意图”“检索知识库”“组织引用”“生成回答”。
- 不要求，也不建议，暴露模型内部原始推理链。

## 4. 会话 API

### 4.1 创建会话

```http
POST /api/v1/qa/conversations
```

请求：

```json
{
  "title": "锅炉技术监督咨询"
}
```

响应：`201 Created`

```json
{
  "id": "conv_001",
  "title": "锅炉技术监督咨询",
  "status": "active",
  "createdAt": "2026-06-28T10:00:00Z"
}
```

规则：

- `title` 可选；未传时可使用首条问题自动生成标题。
- 会话归属于当前登录用户。

### 4.2 查询会话列表

```http
GET /api/v1/qa/conversations?page=1&pageSize=20&q=锅炉
```

响应：

```json
{
  "items": [
    {
      "id": "conv_001",
      "title": "锅炉技术监督咨询",
      "lastMessageAt": "2026-06-28T10:05:00Z",
      "createdAt": "2026-06-28T10:00:00Z"
    }
  ],
  "page": 1,
  "pageSize": 20,
  "total": 1
}
```

### 4.3 获取会话详情

```http
GET /api/v1/qa/conversations/{conversationId}
```

响应：`Conversation`

### 4.4 更新会话标题

```http
PATCH /api/v1/qa/conversations/{conversationId}
```

请求：

```json
{
  "title": "锅炉监督检查要求"
}
```

### 4.5 删除会话

```http
DELETE /api/v1/qa/conversations/{conversationId}
```

响应：`204 No Content`

规则：

- 用户只能删除自己的会话；管理员和超级管理员可按角色级 RBAC 查看、软删除全站会话。
- 会话删除采用软删除，历史统计不受影响。

## 5. 消息 API

### 5.1 查询会话消息

```http
GET /api/v1/qa/conversations/{conversationId}/messages?page=1&pageSize=50
```

响应：

```json
{
  "items": [
    {
      "id": "msg_001",
      "role": "user",
      "content": "锅炉技术监督有哪些检查要求？",
      "intent": "knowledge_qa",
      "status": "completed",
      "createdAt": "2026-06-28T10:00:00Z"
    },
    {
      "id": "msg_002",
      "role": "assistant",
      "content": "根据技术监督规程...",
      "status": "completed",
      "citationCount": 3,
      "createdAt": "2026-06-28T10:00:10Z"
    }
  ],
  "page": 1,
  "pageSize": 50,
  "total": 2
}
```

### 5.2 非流式问答

```http
POST /api/v1/qa/conversations/{conversationId}/messages
```

请求：

```json
{
  "message": "锅炉技术监督有哪些检查要求？",
  "mode": "knowledge_qa",
  "knowledgeBaseIds": ["kb_001", "kb_002"],
  "retrieval": {
    "topK": 8,
    "scoreThreshold": 0.35,
    "rerankThreshold": 0.5,
    "enableRerank": true,
    "tagFilters": {
      "专业": ["锅炉"]
    }
  }
}
```

响应：

```json
{
  "userMessage": {
    "id": "msg_001",
    "role": "user",
    "content": "锅炉技术监督有哪些检查要求？"
  },
  "assistantMessage": {
    "id": "msg_002",
    "role": "assistant",
    "content": "根据技术监督规程...",
    "intent": "knowledge_qa",
    "status": "completed"
  },
  "citations": [
    {
      "id": "cit_001",
      "documentId": "doc_001",
      "documentName": "技术监督规程.pdf",
      "sectionPath": "1. 总则",
      "score": 0.82
    }
  ],
  "reasoningSteps": [
    {
      "type": "intent",
      "summary": "识别为知识问答"
    }
  ]
}
```

规则：

- `mode` 可选；不传时系统自动识别意图。
- 用户未指定知识库时，系统使用问答配置中的默认术语知识库和技术监督知识库。
- 对无权限知识库按角色级 RBAC 和知识库可见性过滤；请求显式指定且完全无权限时可返回 `forbidden`。
- 识别为 `data_analysis` 时返回 `unsupported_intent`，响应中保留用户消息和处理摘要，不调用数据分析执行链路。

### 5.3 流式问答

```http
POST /api/v1/qa/conversations/{conversationId}/messages:stream
```

请求同非流式问答。

响应：SSE。

关键事件示例：

```text
event: message.created
data: {"userMessageId":"msg_001","assistantMessageId":"msg_002"}

event: intent.detected
data: {"intent":"knowledge_qa","confidence":0.92}

event: retrieval.completed
data: {"resultCount":8,"topScore":0.91}

event: answer.delta
data: {"messageId":"msg_002","delta":"根据检索到的规程，"}

event: citation.delta
data: {"citationId":"cit_001","documentName":"技术监督规程.pdf","sectionPath":"1. 总则"}

event: answer.completed
data: {"messageId":"msg_002","status":"completed"}
```

错误事件：

```text
event: error
data: {"code":"dependency_error","message":"llm provider unavailable","requestId":"req_123"}
```

规则：

- 流式接口必须在连接断开时保存已生成内容；客户端断开标记为 `cancelled`，依赖错误标记为 `failed`。
- 前端刷新后应能通过消息列表恢复已保存历史。
- SSE 与普通 JSON 接口一样使用 `Authorization: Bearer <accessToken>` 鉴权。

### 5.4 取消生成

```http
POST /api/v1/qa/messages/{messageId}:cancel
```

响应：

```json
{
  "id": "msg_002",
  "status": "cancelled"
}
```

规则：

- 仅 `queued` 或 `generating` 状态可取消。
- 是否能真正取消下游 LLM 请求取决于模型供应商能力。

## 6. 引用 API

### 6.1 查询消息引用

```http
GET /api/v1/qa/messages/{messageId}/citations
```

响应：

```json
{
  "items": [
    {
      "id": "cit_001",
      "chunkId": "chunk_001",
      "documentId": "doc_001",
      "documentName": "技术监督规程.pdf",
      "sectionPath": "1. 总则",
      "contentPreview": "本规程适用于...",
      "score": 0.82,
      "rerankScore": 0.91,
      "isSourceAvailable": true
    }
  ]
}
```

### 6.2 获取引用详情

```http
GET /api/v1/qa/citations/{citationId}
```

响应：

```json
{
  "id": "cit_001",
  "documentId": "doc_001",
  "documentName": "技术监督规程.pdf",
  "sectionPath": "1. 总则",
  "content": "完整引用片段文本...",
  "score": 0.82,
  "rerankScore": 0.91,
  "chunkType": "text",
  "source": {
    "available": true,
    "downloadEndpoint": "/api/v1/knowledge/documents/doc_001:download-url"
  }
}
```

文档已删除或无权限时：

```json
{
  "id": "cit_001",
  "source": {
    "available": false,
    "reason": "source_deleted_or_forbidden"
  },
  "content": "历史回答保留的引用片段快照..."
}
```

规则：

- 引用详情可以展示回答生成时保存的快照，避免源文档删除后完全不可读。
- 原始文件下载必须调用知识管理/文件服务校验权限。

## 7. 处理过程 API

### 7.1 查询处理过程摘要

```http
GET /api/v1/qa/messages/{messageId}/reasoning-steps
```

响应：

```json
{
  "items": [
    {
      "id": "step_001",
      "type": "intent",
      "title": "识别问题类型",
      "summary": "识别为知识问答",
      "status": "completed"
    },
    {
      "id": "step_002",
      "type": "retrieval",
      "title": "检索知识库",
      "summary": "检索 2 个知识库，命中 8 个片段",
      "status": "completed"
    }
  ]
}
```

展示规则：

- 支持折叠和展开。
- 默认在流式完成后折叠。
- 可区分文本摘要和代码内容，但首期建议只展示文本摘要。

## 8. 意图识别 API

### 8.1 单独识别意图

管理员或调试接口：

```http
POST /api/v1/qa/intents:detect
```

请求：

```json
{
  "message": "帮我生成迎峰度夏检查报告",
  "conversationId": "conv_001"
}
```

响应：

```json
{
  "intent": "report_generation",
  "confidence": 0.89,
  "allowed": true,
  "route": {
    "module": "document",
    "suggestedEndpoint": "/api/v1/reports"
  }
}
```

规则：

- `allowed=false` 时必须返回原因，例如用户无报告生成权限。
- 首期作为管理员调试接口开放；普通用户不直接调用。

## 9. 问答配置 API

### 9.1 获取问答配置

管理员接口：

```http
GET /api/v1/qa/settings
```

响应：

```json
{
  "defaultKnowledgeBaseIds": ["kb_terms", "kb_supervision"],
  "retrieval": {
    "topK": 8,
    "scoreThreshold": 0.35,
    "rerankThreshold": 0.5,
    "enableRerank": true
  },
  "llm": {
    "provider": "ai-gateway",
    "profileId": "mp_chat_default",
    "model": "llm-model-name",
    "timeoutSeconds": 60
  }
}
```

### 9.2 更新问答配置

```http
PATCH /api/v1/qa/settings
```

请求：

```json
{
  "defaultKnowledgeBaseIds": ["kb_terms", "kb_supervision"],
  "retrieval": {
    "topK": 8,
    "scoreThreshold": 0.35,
    "rerankThreshold": 0.5,
    "enableRerank": true
  },
  "llm": {
    "provider": "ai-gateway",
    "profileId": "mp_chat_default",
    "model": "llm-model-name",
    "timeoutSeconds": 60
  }
}
```

响应：

```json
{
  "updatedAt": "2026-06-28T10:00:00Z"
}
```

规则：

- `profileId` 指向 AI Gateway 中的 chat profile；QA 不保存 provider `baseUrl` 或 `apiKey`，也不实现供应商适配层。
- 配置变更应记录操作人和时间。
- 默认知识库必须存在且管理员有配置权限。

## 10. 检索体验测试 API

智能问答需求里提出管理员可测试知识库召回效果。该能力实际由 `knowledge` 执行检索，`qa` 可提供面向问答场景的包装接口。

### 10.1 创建问答检索测试

```http
POST /api/v1/qa/retrieval-tests
```

请求：

```json
{
  "question": "锅炉技术监督有哪些检查要求？",
  "knowledgeBaseIds": ["kb_001"],
  "retrieval": {
    "topK": 8,
    "enableRerank": true,
    "scoreThreshold": 0.35,
    "rerankThreshold": 0.5,
    "tagFilters": {
      "专业": ["锅炉"]
    }
  }
}
```

响应：

```json
{
  "id": "qrt_001",
  "results": [
    {
      "documentName": "技术监督规程.pdf",
      "sectionPath": "1. 总则",
      "score": 0.82,
      "rerankScore": 0.91,
      "contentPreview": "本规程适用于..."
    }
  ],
  "createdAt": "2026-06-28T10:00:00Z"
}
```

## 11. 统计 API

```http
GET /api/v1/qa/stats/overview
```

响应：

```json
{
  "totalQuestionCount": 1200,
  "conversationCount": 320,
  "trend30d": [
    {
      "date": "2026-06-28",
      "questionCount": 42
    }
  ]
}
```

可选增强：

- 按知识库统计命中率。
- 按意图统计问答分布。
- 按用户或部门统计使用量，需结合权限策略。

## 12. 存储与数据归属

| 数据 | 存储 | 所有者 |
| --- | --- | --- |
| 会话、消息、引用、处理过程摘要 | PostgreSQL | `qa` |
| 问答配置 | PostgreSQL 保存业务默认参数和 AI Gateway profile 引用；provider 密钥由 AI Gateway 管理 | `qa` / `ai-gateway` |
| 检索候选片段 | 来自 `knowledge` API | `knowledge` |
| 原始文档与下载 URL | MinIO + file metadata | `file` / `knowledge` |
| 流式生成短期状态 | Redis 可缓存 | `qa` |
| LLM 调用结果 | PostgreSQL 保存最终消息，流式中间态可短期缓存 | `qa` / `ai-gateway` |

## 13. 已确认决策与后续跟踪

| 编号 | 结论 |
| --- | --- |
| Q1 | 会话历史服务端 PostgreSQL 持久化；前端本地仅缓存 `sessionId` 等恢复信息。 |
| Q2 | SSE 鉴权使用 `Authorization: Bearer <accessToken>`，与普通 JSON 接口一致。 |
| Q3 | 流式生成中断时保存已生成部分；客户端断开标记为 `cancelled`，依赖错误标记为 `failed`。 |
| Q4 | “思考过程展示”只展示处理摘要，不返回原始模型推理链。 |
| Q5 | 统计指标本期实现；Excel/表格类数据分析意图本期不实现，命中后返回 `unsupported_intent`。 |
| Q6 | 首期按角色级 RBAC 控制管理员能力；管理员和超级管理员可查看、软删除全站会话。 |
| Q7 | LLM 通过 `ai-gateway` 的 OpenAI-compatible API 调用；业务服务通过配置引用 `profileId`、模型名和业务默认参数，provider `baseURL` 与 `apiKey` 由 AI Gateway 管理。 |
| Q8 | 回答引用保存片段快照，同时保留 chunkId/documentId 便于追溯。 |
