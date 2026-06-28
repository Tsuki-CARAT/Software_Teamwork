# AI 网关 API 契约

## 1. 契约目标

本文定义 `ai-gateway` 的内部 HTTP API 契约，覆盖模型配置、Chat Completion、Embedding 和 Rerank 能力。当前任务只定义接口文档和 OpenAPI 草稿，不创建 `services/ai-gateway/` 代码。

主责服务：

- `ai-gateway`：统一维护模型 provider 配置、API key 写入状态、模型调用适配、错误归一化和请求追踪。
- `qa`：问答会话、消息、RAG 编排、引用记录和答案持久化。
- `knowledge`：知识库、文档解析、切片、向量索引、检索和重排序业务流程。
- `document`：报告模板、报告记录、大纲、章节内容、导出和报告生成任务。
- `gateway`：外部 API 入口；不直接面向 AI provider，也不暴露 AI Gateway 内部接口给前端。

边界原则：

- `ai-gateway` 是内部服务，不直接面向前端、管理端或公网调用方。
- 前端仍只调用 public `gateway` 的 `/api/v1/**`。
- `qa`、`knowledge`、`document` 需要模型能力时，通过内部 HTTP API 调用 `ai-gateway`。
- Chat Completions 和 Embeddings 使用 OpenAI-compatible 请求/响应体。
- Rerankings 是 OpenAI-style 扩展；OpenAI 官方没有原生 rerank endpoint。
- `ai-gateway` 不保存 QA 会话、知识库切片、报告记录、引用格式或业务任务状态。
- API key、provider bearer token、完整 prompt、内部 provider URL、原始 provider 错误体和向量 payload 不得出现在响应、普通日志或错误文案中。

## 2. 通用约定

### 2.1 基础路径

健康检查使用服务根路径：

```text
GET /healthz
GET /readyz
```

内部业务 API 使用：

```text
/internal/v1
```

当前文档配套的 OpenAPI 草稿见 [ai-gateway.openapi.yaml](./openapi/ai-gateway.openapi.yaml)。

### 2.2 OpenAI-Compatible 约定

模型调用接口遵循 OpenAI-compatible 格式：

- Chat：`POST /internal/v1/chat/completions`
- Embeddings：`POST /internal/v1/embeddings`
- Streaming Chat：复用 Chat endpoint，`stream: true` 时返回 `text/event-stream`

这些接口的成功响应不使用项目统一 `data/requestId` envelope，而是返回 OpenAI-compatible response body。调用方从响应头 `X-Request-Id` 获取请求追踪 ID。

配置、健康检查等项目自有接口仍使用项目统一 envelope。

### 2.3 内部认证与上下文

调用方必须使用内部服务凭据：

```http
X-Service-Token: <internal-service-token>
X-Caller-Service: qa
```

建议同时传递请求和用户上下文：

| Header | 必填 | 说明 |
| --- | --- | --- |
| `X-Request-Id` | 建议 | 贯穿 public gateway、领域服务、AI Gateway 和 provider 调用日志。缺失时由 AI Gateway 生成。 |
| `X-Service-Token` | 是 | 内部服务认证凭据。签发、轮换和校验策略由部署安全策略定义。 |
| `X-Caller-Service` | 是 | 调用方服务名，例如 `qa`、`knowledge`、`document`。 |
| `X-User-Id` | 建议 | 触发本次 AI 调用的用户 ID；后台任务可缺省。 |
| `X-User-Roles` | 否 | 逗号分隔角色列表，用于审计和后续配额策略。 |
| `X-User-Permissions` | 否 | 逗号分隔权限列表，用于审计和后续配额策略。 |

AI Gateway 使用这些上下文做审计、配额预留和问题排查，但不拥有领域权限判断。知识库权限、问答权限和报告权限仍由对应领域服务负责。

### 2.4 项目自有响应结构

配置和健康检查成功响应使用：

```json
{
  "data": {},
  "requestId": "req_123"
}
```

项目自有错误响应使用：

```json
{
  "error": {
    "code": "validation_error",
    "message": "request validation failed",
    "requestId": "req_123",
    "fields": {
      "model": "is required"
    }
  }
}
```

模型调用错误响应使用 OpenAI-compatible 结构：

```json
{
  "error": {
    "message": "request validation failed",
    "type": "invalid_request_error",
    "param": "model",
    "code": "validation_error"
  }
}
```

### 2.5 通用错误码

| code | HTTP | 场景 |
| --- | --- | --- |
| `validation_error` | 400 | 请求字段缺失、类型错误、参数越界或 profile 不支持该能力。 |
| `unauthorized` | 401 | 缺少或无法校验内部服务凭据。 |
| `forbidden` | 403 | 调用方服务无权访问配置写接口或指定模型能力。 |
| `not_found` | 404 | 指定 profile 不存在或已隐藏。 |
| `conflict` | 409 | 配置版本冲突、默认 profile 约束冲突或 profile 正在被使用不能删除。 |
| `rate_limited` | 429 | 调用方、用户、provider 或模型配额超限。 |
| `dependency_error` | 502 | Provider 超时、不可用、返回非契约响应或网络失败。 |
| `internal_error` | 500 | AI Gateway 内部未预期错误。 |

Provider 的 `401`、`403`、`429` 和 `5xx` 不应原样透传给上游服务。AI Gateway 应归一化为自己的错误码；模型调用接口再映射为 OpenAI-style `error.type`，例如 `invalid_request_error`、`authentication_error`、`permission_error`、`rate_limit_error`、`upstream_error`。

## 3. 接口总览

| Method | Path | 响应风格 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/healthz` | 项目 envelope | 进程存活检查。 |
| `GET` | `/readyz` | 项目 envelope | 配置和关键依赖就绪检查。 |
| `GET` | `/internal/v1/model-profiles` | 项目 envelope | 查询模型配置列表。 |
| `POST` | `/internal/v1/model-profiles` | 项目 envelope | 创建模型配置。 |
| `GET` | `/internal/v1/model-profiles/{profileId}` | 项目 envelope | 查询单个模型配置。 |
| `PATCH` | `/internal/v1/model-profiles/{profileId}` | 项目 envelope | 更新模型配置，包括 API key 轮换。 |
| `DELETE` | `/internal/v1/model-profiles/{profileId}` | 空响应或项目错误 | 删除或停用模型配置。 |
| `POST` | `/internal/v1/chat/completions` | OpenAI-compatible | 创建非流式或流式 chat completion。 |
| `POST` | `/internal/v1/embeddings` | OpenAI-compatible | 创建 embedding 向量。 |
| `POST` | `/internal/v1/rerankings` | OpenAI-style | 对候选文本重排序。 |

## 4. 数据对象

### 4.1 ModelProfile

```json
{
  "id": "mp_chat_default",
  "name": "default-chat",
  "purpose": "chat",
  "provider": "siliconflow",
  "baseUrl": "https://api.siliconflow.cn/v1",
  "model": "Qwen/Qwen2.5-72B-Instruct",
  "enabled": true,
  "isDefault": true,
  "timeoutMs": 60000,
  "apiKeyConfigured": true,
  "supportsStreaming": true,
  "defaultParameters": {
    "temperature": 0.2,
    "top_p": 0.9
  },
  "createdAt": "2026-06-28T10:00:00Z",
  "updatedAt": "2026-06-28T10:00:00Z"
}
```

用途枚举：

```text
chat
embedding
rerank
```

Provider 枚举：

```text
openai_compatible
siliconflow
local_compatible
```

创建和更新请求允许写入 `apiKey`，但任何响应都只能返回 `apiKeyConfigured`，不得返回明文 key、key hash 或 provider token。

### 4.2 ChatCompletion

请求示例：

```json
{
  "model": "default-chat",
  "profile_id": "mp_chat_default",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "锅炉技术监督有哪些检查要求？"
    }
  ],
  "temperature": 0.2,
  "stream": false,
  "metadata": {
    "conversation_id": "conv_001"
  }
}
```

非流式响应示例：

```json
{
  "id": "chatcmpl_123",
  "object": "chat.completion",
  "created": 1782631200,
  "model": "Qwen/Qwen2.5-72B-Instruct",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "锅炉技术监督通常包括运行参数、受热面、保护装置和检修记录检查。"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 120,
    "completion_tokens": 40,
    "total_tokens": 160
  }
}
```

流式响应使用 SSE：

```text
data: {"id":"chatcmpl_123","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"锅炉"},"finish_reason":null}]}

data: [DONE]
```

AI Gateway 不保存会话历史。`qa` 和 `document` 必须自己管理业务上下文、消息持久化、引用和重试恢复。

### 4.3 Embeddings

请求示例：

```json
{
  "model": "default-embedding",
  "profile_id": "mp_embedding_default",
  "input": [
    "锅炉水冷壁监督检查要求",
    "汽轮机振动监测标准"
  ],
  "encoding_format": "float"
}
```

响应示例：

```json
{
  "object": "list",
  "data": [
    {
      "object": "embedding",
      "embedding": [0.012, -0.034, 0.056],
      "index": 0
    }
  ],
  "model": "BAAI/bge-large-zh-v1.5",
  "usage": {
    "prompt_tokens": 32,
    "total_tokens": 32
  }
}
```

AI Gateway 只负责向量生成和错误归一化，不写入 Qdrant。`knowledge` 负责 chunk、向量索引、检索策略和数据持久化。

### 4.4 Rerankings

请求示例：

```json
{
  "model": "default-rerank",
  "profile_id": "mp_rerank_default",
  "query": "锅炉技术监督检查要求",
  "documents": [
    {
      "id": "chunk_001",
      "text": "锅炉受热面监督检查包括..."
    }
  ],
  "top_n": 5
}
```

响应示例：

```json
{
  "object": "list",
  "model": "BAAI/bge-reranker-large",
  "data": [
    {
      "index": 0,
      "document_id": "chunk_001",
      "relevance_score": 0.91
    }
  ],
  "usage": {
    "prompt_tokens": 80,
    "total_tokens": 80
  }
}
```

Rerank 接口不负责召回候选集，也不决定 RAG 引用格式。`knowledge` 或 `qa` 负责业务过滤、召回、引用和展示字段。

## 5. 服务协作

| 调用方 | 调用场景 | AI Gateway 角色 | 调用方仍然负责 |
| --- | --- | --- | --- |
| `knowledge` | 文档向量化、检索结果重排序 | 生成 embedding 或 rerank score | 文档解析、chunk、Qdrant 写入、检索策略和知识库权限。 |
| `qa` | 问答意图处理后的答案生成 | Chat completion 或 streaming chunk | 会话、消息、RAG 编排、引用、回答保存和用户权限。 |
| `document` | 报告大纲、章节内容、改写和总结 | Chat completion 或 streaming chunk | 报告记录、任务状态、章节版本、导出和模板业务规则。 |
| `gateway` | 后续管理员模型配置入口 | 仅通过内部 API 提供配置能力 | public API 认证、权限和响应 envelope。 |

## 6. 验收要求

文档变更必须至少验证：

- `docs/接口契约/openapi/ai-gateway.openapi.yaml` 可以被 YAML 解析。
- OpenAPI 中所有 `$ref` 均可解析。
- 除 `/healthz`、`/readyz` 外，AI Gateway 内部业务路径都以 `/internal/v1/**` 开头。
- 模型调用接口的成功和错误响应为 OpenAI-compatible，不套项目 envelope。
- README、服务边界、前后端集成契约和 gateway 服务说明均明确前端不得直连 AI Gateway。
