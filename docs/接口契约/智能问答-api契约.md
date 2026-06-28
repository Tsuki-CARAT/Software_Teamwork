# 智能问答 API 契约

## 1. 契约目标

本文定义智能问答模块的 HTTP API 契约，覆盖会话管理、多轮对话、SSE 流式回答、非流式回答、意图识别、RAG 检索增强、引用溯源、处理过程展示、问答配置、检索体验测试和统计监控。

主责服务：

- `qa`：会话、消息、意图识别、RAG 编排、答案生成、引用记录、问答统计。
- `knowledge`：知识检索能力，提供候选切片和引用元数据。
- `auth`：用户身份、角色权限。
- `gateway`：外部 API 入口和认证上下文转发。

边界原则：

- `qa` 不直接读写 Qdrant，不维护向量集合，不拥有文档切片索引。
- `qa` 通过 `knowledge` 的检索 API 获取上下文片段。
- `qa` 可保存回答、消息、引用快照和处理过程摘要，便于历史回放。
- 原文文件下载由知识管理或文件服务校验权限并生成下载 URL。
- 前端可展示“处理过程摘要”，不建议展示原始模型 chain-of-thought。

## 2. 通用约定

### 2.1 基础路径

待确认：是否统一使用 `/api/v1`。本文暂按以下形式描述：

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

待确认：认证方式采用 Cookie Session、Bearer Token/JWT，还是二者兼容。

权限要求：

| 能力 | 标准用户 | 管理员 | 超级管理员 |
| --- | --- | --- | --- |
| 创建和使用自己的会话 | 支持 | 支持 | 支持 |
| 查看自己的历史会话 | 支持 | 支持 | 支持 |
| 删除自己的会话 | 支持 | 支持 | 支持 |
| 配置问答参数 | 不支持 | 支持 | 支持 |
| 检索体验测试 | 不支持 | 支持 | 支持 |
| 查看全局统计 | 不支持 | 支持 | 支持 |

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

待确认：会话删除是软删除还是硬删除。建议软删除，便于审计和误删恢复。

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

待确认：`data_analysis` 是否属于本期范围。

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

- 用户只能删除自己的会话，管理员是否可删除他人会话待确认。
- 建议软删除，历史统计不受影响。

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
- 对无权限知识库必须过滤或返回 `forbidden`，具体策略待确认。

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

- 流式接口必须在连接断开时保存已生成内容或标记为 `cancelled`，具体策略待确认。
- 前端刷新后应能通过消息列表恢复已保存历史。
- SSE 在 Cookie/JWT 认证下的具体传递方式待确认。

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
- 是否作为前端直接调用接口待确认；也可以只作为内部能力。

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
    "provider": "siliconflow",
    "model": "llm-model-name",
    "baseUrl": "https://api.example.com",
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
    "provider": "siliconflow",
    "model": "llm-model-name",
    "baseUrl": "https://api.example.com",
    "apiKey": "secret",
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

- `apiKey` 只允许写入，不允许明文读取。
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
| 问答配置 | PostgreSQL 或安全配置存储，密钥需加密 | `qa` |
| 检索候选片段 | 来自 `knowledge` API | `knowledge` |
| 原始文档与下载 URL | MinIO + file metadata | `file` / `knowledge` |
| 流式生成短期状态 | Redis 可缓存 | `qa` |
| LLM 调用结果 | PostgreSQL 保存最终消息，流式中间态可短期缓存 | `qa` |

## 13. 待确认问题

| 编号 | 问题 |
| --- | --- |
| Q1 | 会话历史是否必须服务端 PostgreSQL 持久化？原始需求提到本地持久化，但服务端持久化更利于多端和审计。 |
| Q2 | SSE 鉴权采用 Cookie 还是 Authorization header？ |
| Q3 | 流式生成中断时，部分回答是否保存？状态标为 `failed` 还是 `cancelled`？ |
| Q4 | “思考过程展示”是否只展示处理摘要，而不是原始模型推理链？ |
| Q5 | 数据分析意图是否本期实现？若实现，需要独立数据分析契约。 |
| Q6 | 管理员是否可查看或删除普通用户会话？ |
| Q7 | LLM 默认供应商、模型名、超时和限流策略是什么？ |
| Q8 | 回答引用是否保存全文快照，还是只保存 chunkId 后续实时查询？建议至少保存引用片段快照。 |
