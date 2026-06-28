# 知识管理 API 契约

## 1. 契约目标

本文定义知识管理模块的 HTTP API 契约，覆盖知识库、文档、文档处理、切片、检索、报告支撑材料和知识管理配置。该模块是智能问答和报告生成的数据底座。

主责服务：

- `knowledge`：知识库、文档业务元数据、解析/切片/向量化任务、Qdrant 索引、检索协调。
- `file`：文件上传、MinIO 对象存储、下载授权和预签名 URL。
- `auth`：用户、角色、权限和认证上下文。
- `gateway`：外部 API 入口。
- `ai-gateway`：统一提供 OpenAI-compatible 模型调用入口，供 embedding、rerank 和后续 LLM 能力使用。

边界原则：

- `knowledge` 是 Qdrant 的唯一业务写入方。
- `qa`、`document` 只能通过本契约的检索接口复用知识能力，不能直接写 Qdrant。
- 原始文件和较大解析产物存储在 MinIO，业务元数据存储在 PostgreSQL。
- Redis 用于异步任务队列、短期任务状态辅助和缓存；PostgreSQL 保存可追溯业务状态，不把 Redis 作为长期业务真相。

## 2. 通用约定

### 2.1 基础路径

外部 API 统一使用 `/api/v1` 作为网关前缀：

```text
/api/v1/knowledge/...
```

服务内部可映射到 `knowledge` 或 `file` 服务，但前端只感知网关路径。

### 2.2 RESTful + OpenAPI + Swagger UI 规范

本模块接口必须按 RESTful 风格设计，并以 OpenAPI 3.0+ 作为机器可读契约。Swagger UI 用于开发联调、验收演示和接口自测。

RESTful 约定：

- 资源使用复数名词，例如 `/knowledge/bases`、`/knowledge/documents`、`/knowledge/support-materials`。
- 创建资源使用 `POST /resources`。
- 查询列表使用 `GET /resources`，详情使用 `GET /resources/{id}`。
- 局部更新使用 `PATCH /resources/{id}`。
- 删除使用 `DELETE /resources/{id}`，批量或动作型能力使用 `POST /resources:action`。
- 异步任务必须返回任务资源 ID，例如 `processingJobId`，并提供任务查询接口。
- 所有列表接口必须分页，分页字段统一为 `page`、`pageSize`、`total`。
- 时间字段统一使用 UTC ISO 8601 字符串。

OpenAPI 约定：

- `knowledge` 服务维护服务内契约：`services/knowledge/api/openapi.yaml`。
- 当前文档配套的 OpenAPI 草稿见 [knowledge.openapi.yaml](./openapi/knowledge.openapi.yaml)。
- 涉及文件上传/下载的公共接口由 `file` 服务维护：`services/file/api/openapi.yaml`，知识管理契约只引用 `fileId` 和上传/下载 URL 流程。
- OpenAPI 必须声明 `securitySchemes`、通用错误响应、分页响应、资源 schema、状态枚举和 SSE/异步任务说明。
- API 文档中的 request/response 示例应与本文 Markdown 契约保持一致。

Swagger UI 约定：

- 网关应暴露聚合入口，建议为 `/api/docs`。
- 服务级 OpenAPI JSON/YAML 建议暴露为 `/api/v1/knowledge/openapi.yaml` 或通过网关聚合。
- Swagger UI 只用于开发、测试和内网验收环境；生产环境是否开放需由部署配置控制。

### 2.3 认证与权限

首期统一采用 Bearer Token/JWT：

```http
Authorization: Bearer <accessToken>
```

`gateway` 或服务侧鉴权中间件负责校验 token，并向下游传递用户 ID、角色和权限上下文。当前不为知识管理 API 单独设计独立会话鉴权通道。

权限要求：

| 能力 | 标准用户 | 管理员 | 超级管理员 |
| --- | --- | --- | --- |
| 查看有权限知识库 | 支持 | 支持 | 支持 |
| 创建/编辑/删除知识库 | 不支持 | 支持 | 支持 |
| 上传/删除文档 | 按角色级 RBAC 和知识库可见性控制 | 支持 | 支持 |
| 修改模型/解析配置 | 不支持 | 支持 | 支持 |
| 检索测试 | 不支持 | 支持 | 支持 |

首期只做角色级 RBAC，不引入组织、电厂、专业等多维数据权限。

### 2.4 通用响应结构

成功响应直接返回资源对象或列表对象。

分页响应：

```json
{
  "items": [],
  "page": 1,
  "pageSize": 20,
  "total": 0
}
```

错误响应：

```json
{
  "error": {
    "code": "validation_error",
    "message": "request validation failed",
    "requestId": "req_123",
    "fields": {
      "name": "required"
    }
  }
}
```

通用错误码：

| code | HTTP | 含义 |
| --- | --- | --- |
| `validation_error` | 400 | 请求字段不合法 |
| `unauthorized` | 401 | 未登录或认证失效 |
| `forbidden` | 403 | 无权限访问 |
| `not_found` | 404 | 资源不存在或不可见 |
| `conflict` | 409 | 当前状态不允许操作 |
| `rate_limited` | 429 | 请求频率超过限制 |
| `dependency_error` | 502 | 模型服务、Qdrant、MinIO 等依赖失败 |
| `internal_error` | 500 | 未预期错误 |

### 2.5 枚举

知识库文档类型：

```text
regulation        规程规范
technical_report  技术报告论文
term              术语条目
general           通用文档
support_material  报告支撑材料
```

分段策略：

```text
heading     基于标题层级智能分段
fixed_size  固定字符数分段
```

检索策略：

```text
vector        语义向量检索
vector_rerank 向量检索 + 重排序
```

文档状态：

```text
uploaded
parsing
chunking
embedding
ready
failed
deleted
```

处理任务状态：

```text
queued
running
succeeded
failed
cancelled
```

## 3. 数据对象

### 3.1 KnowledgeBase

```json
{
  "id": "kb_001",
  "name": "技术监督规程库",
  "description": "电厂技术监督相关规程和术语",
  "documentType": "regulation",
  "visibility": "private",
  "segmentation": {
    "strategy": "heading",
    "fixedSize": null
  },
  "retrieval": {
    "strategy": "vector_rerank",
    "topK": 8,
    "scoreThreshold": 0.35,
    "rerankThreshold": 0.5
  },
  "documentCount": 128,
  "createdBy": "user_001",
  "createdAt": "2026-06-28T10:00:00Z",
  "updatedAt": "2026-06-28T10:00:00Z"
}
```

权限说明：

- 首期按角色级 RBAC 和知识库可见性控制访问。
- `private`、`team`、`public` 可作为可见性枚举保留；组织、电厂、专业等多维权限不作为首期要求。
- `team` 首期可按角色级 RBAC 和创建人范围实现，不要求接入组织树。

### 3.2 KnowledgeDocument

```json
{
  "id": "doc_001",
  "knowledgeBaseId": "kb_001",
  "fileId": "file_001",
  "filename": "技术监督规程.pdf",
  "mimeType": "application/pdf",
  "sizeBytes": 1024000,
  "status": "ready",
  "tags": {
    "专业": "锅炉",
    "年份": "2026"
  },
  "chunkCount": 86,
  "errorMessage": null,
  "createdBy": "user_001",
  "createdAt": "2026-06-28T10:00:00Z",
  "updatedAt": "2026-06-28T10:10:00Z"
}
```

### 3.3 DocumentChunk

```json
{
  "id": "chunk_001",
  "documentId": "doc_001",
  "knowledgeBaseId": "kb_001",
  "index": 1,
  "sectionPath": "1. 总则 / 1.1 适用范围",
  "chunkType": "text",
  "contentPreview": "本规程适用于...",
  "tokenCount": 320,
  "metadata": {
    "page": 3
  },
  "createdAt": "2026-06-28T10:10:00Z"
}
```

### 3.4 RetrievalResult

```json
{
  "chunkId": "chunk_001",
  "documentId": "doc_001",
  "knowledgeBaseId": "kb_001",
  "documentName": "技术监督规程.pdf",
  "sectionPath": "1. 总则 / 1.1 适用范围",
  "score": 0.82,
  "rerankScore": 0.91,
  "content": "本规程适用于...",
  "chunkType": "text",
  "tags": {
    "专业": "锅炉"
  }
}
```

## 4. 知识库 API

### 4.1 创建知识库

```http
POST /api/v1/knowledge/bases
```

请求：

```json
{
  "name": "技术监督规程库",
  "description": "电厂技术监督相关规程和术语",
  "documentType": "regulation",
  "visibility": "private",
  "segmentation": {
    "strategy": "heading",
    "fixedSize": {
      "chunkSize": 1200,
      "overlapSize": 200,
      "recursiveMerge": true,
      "separators": ["\n\n", "\n", "。"]
    }
  },
  "retrieval": {
    "strategy": "vector_rerank",
    "topK": 8,
    "scoreThreshold": 0.35,
    "rerankThreshold": 0.5
  }
}
```

响应：`201 Created`

```json
{
  "id": "kb_001",
  "name": "技术监督规程库",
  "documentType": "regulation",
  "visibility": "private",
  "documentCount": 0,
  "createdAt": "2026-06-28T10:00:00Z"
}
```

校验规则：

- `name` 必填，建议同一可见范围内唯一。
- `documentType` 必须是允许枚举。
- `segmentation.strategy=fixed_size` 时必须提供 `chunkSize` 和 `overlapSize`。
- `retrieval.strategy=vector_rerank` 时需要存在可用重排序模型配置。

### 4.2 查询知识库列表

```http
GET /api/v1/knowledge/bases?page=1&pageSize=20&type=regulation&q=技术监督
```

响应：

```json
{
  "items": [
    {
      "id": "kb_001",
      "name": "技术监督规程库",
      "documentType": "regulation",
      "visibility": "private",
      "documentCount": 128,
      "createdBy": "user_001",
      "createdAt": "2026-06-28T10:00:00Z"
    }
  ],
  "page": 1,
  "pageSize": 20,
  "total": 1
}
```

### 4.3 获取知识库详情

```http
GET /api/v1/knowledge/bases/{knowledgeBaseId}
```

响应：`KnowledgeBase`

### 4.4 更新知识库

```http
PATCH /api/v1/knowledge/bases/{knowledgeBaseId}
```

请求：

```json
{
  "name": "技术监督规程库",
  "description": "更新后的描述",
  "segmentation": {
    "strategy": "fixed_size",
    "fixedSize": {
      "chunkSize": 1200,
      "overlapSize": 200,
      "recursiveMerge": true,
      "separators": ["\n\n", "\n", "。"]
    }
  },
  "retrieval": {
    "strategy": "vector_rerank",
    "topK": 10,
    "scoreThreshold": 0.35,
    "rerankThreshold": 0.5
  }
}
```

响应：`KnowledgeBase`

状态影响：

- 分段策略变更后，所有 `ready` 文档需要进入后台重处理。
- 检索策略变更不一定需要重建向量，但如果影响 embedding 模型或向量维度，则必须重建索引。

### 4.5 删除知识库

```http
DELETE /api/v1/knowledge/bases/{knowledgeBaseId}
```

响应：`204 No Content`

规则：

- 删除前必须校验权限。
- 首期采用软删除；Qdrant 向量、MinIO 文件引用和后台清理由生命周期任务处理。
- 删除知识库时应处理 PostgreSQL 元数据、Qdrant 向量、MinIO 文件引用的生命周期。

### 4.6 批量删除知识库

```http
POST /api/v1/knowledge/bases:batch-delete
```

请求：

```json
{
  "ids": ["kb_001", "kb_002"]
}
```

响应：

```json
{
  "deleted": ["kb_001"],
  "failed": [
    {
      "id": "kb_002",
      "code": "forbidden",
      "message": "no permission"
    }
  ]
}
```

## 5. 文档 API

### 5.1 上传文档

建议采用两步上传，避免业务服务直接接收大文件：

1. 向 `file` 服务申请上传 URL。
2. 上传完成后向 `knowledge` 创建文档记录并启动处理。

#### 5.1.1 申请上传 URL

```http
POST /api/v1/files/uploads
```

请求：

```json
{
  "filename": "技术监督规程.pdf",
  "mimeType": "application/pdf",
  "sizeBytes": 1024000,
  "purpose": "knowledge_document"
}
```

响应：

```json
{
  "fileId": "file_001",
  "uploadUrl": "https://minio-presigned-url",
  "expiresAt": "2026-06-28T10:10:00Z"
}
```

#### 5.1.2 创建知识库文档

```http
POST /api/v1/knowledge/bases/{knowledgeBaseId}/documents
```

请求：

```json
{
  "fileId": "file_001",
  "filename": "技术监督规程.pdf",
  "tags": {
    "专业": "锅炉",
    "年份": "2026"
  }
}
```

响应：`201 Created`

```json
{
  "id": "doc_001",
  "knowledgeBaseId": "kb_001",
  "fileId": "file_001",
  "status": "uploaded",
  "processingJobId": "job_001",
  "createdAt": "2026-06-28T10:00:00Z"
}
```

### 5.2 查询文档列表

```http
GET /api/v1/knowledge/bases/{knowledgeBaseId}/documents?page=1&pageSize=20&status=ready&q=规程
```

响应：

```json
{
  "items": [
    {
      "id": "doc_001",
      "filename": "技术监督规程.pdf",
      "status": "ready",
      "tags": {
        "专业": "锅炉"
      },
      "chunkCount": 86,
      "createdAt": "2026-06-28T10:00:00Z"
    }
  ],
  "page": 1,
  "pageSize": 20,
  "total": 1
}
```

### 5.3 获取文档详情

```http
GET /api/v1/knowledge/documents/{documentId}
```

响应：`KnowledgeDocument`

### 5.4 更新文档标签

```http
PATCH /api/v1/knowledge/documents/{documentId}
```

请求：

```json
{
  "tags": {
    "专业": "锅炉",
    "年份": "2026"
  }
}
```

响应：`KnowledgeDocument`

### 5.5 删除文档

```http
DELETE /api/v1/knowledge/documents/{documentId}
```

响应：`204 No Content`

规则：

- 删除文档必须同步标记 Qdrant 向量生命周期。
- 如果历史问答引用了该文档，引用详情应返回“原文已删除或无权限访问”的 fallback。
- 首期不立即硬删原始 MinIO 对象；通过 `file` service 生命周期和后台清理策略处理。
- 首期删除只做软删除，原始文件对象保留到后台生命周期任务确认无引用后清理。

### 5.6 批量删除文档

```http
POST /api/v1/knowledge/documents:batch-delete
```

请求：

```json
{
  "ids": ["doc_001", "doc_002"]
}
```

响应结构同知识库批量删除。

### 5.7 重试文档处理

```http
POST /api/v1/knowledge/documents/{documentId}:retry
```

响应：

```json
{
  "processingJobId": "job_002",
  "status": "queued"
}
```

规则：

- 仅 `failed` 或管理员允许重处理的状态可重试。
- 重试需要保留上一次失败原因供排查。
- 自动重试已满 3 次后仍允许管理员通过该接口手动重排队。

### 5.8 获取文档切片

```http
GET /api/v1/knowledge/documents/{documentId}/chunks?page=1&pageSize=50
```

响应：

```json
{
  "items": [
    {
      "id": "chunk_001",
      "index": 1,
      "sectionPath": "1. 总则",
      "chunkType": "text",
      "contentPreview": "本规程适用于...",
      "tokenCount": 320
    }
  ],
  "page": 1,
  "pageSize": 50,
  "total": 86
}
```

### 5.9 获取原文下载 URL

```http
POST /api/v1/knowledge/documents/{documentId}:download-url
```

响应：

```json
{
  "downloadUrl": "https://minio-presigned-url",
  "expiresAt": "2026-06-28T10:10:00Z"
}
```

规则：

- 必须先校验文档访问权限。
- 不得返回内部 MinIO object key。
- 审计日志首期暂缓，后续可接入独立审计服务。

## 6. 文档处理任务 API

### 6.1 获取处理任务

```http
GET /api/v1/knowledge/processing-jobs/{jobId}
```

响应：

```json
{
  "id": "job_001",
  "documentId": "doc_001",
  "status": "running",
  "stage": "embedding",
  "attemptCount": 1,
  "maxAttempts": 3,
  "progress": {
    "current": 42,
    "total": 86
  },
  "errorMessage": null,
  "attempts": [
    {
      "attempt": 1,
      "stage": "embedding",
      "status": "running",
      "startedAt": "2026-06-28T10:04:00Z"
    }
  ],
  "createdAt": "2026-06-28T10:00:00Z",
  "updatedAt": "2026-06-28T10:05:00Z"
}
```

规则：

- PostgreSQL 中的 processing job 是权威状态；Redis 只用于队列投递、短期进度和并发协调。
- 自动重试最多 3 次，超过后进入 `failed`；手动 retry 会创建新的排队尝试并递增 `attemptCount`。
- `attempts` 最多返回最近 10 次尝试摘要，包含阶段、状态、错误信息和时间字段。

### 6.2 触发知识库重处理

```http
POST /api/v1/knowledge/bases/{knowledgeBaseId}:reprocess
```

请求：

```json
{
  "documentIds": ["doc_001"],
  "reason": "segmentation_changed"
}
```

响应：

```json
{
  "jobIds": ["job_010", "job_011"]
}
```

## 7. 检索 API

### 7.1 知识检索

```http
POST /api/v1/knowledge/retrieval/search
```

请求：

```json
{
  "query": "锅炉技术监督有哪些检查要求？",
  "knowledgeBaseIds": ["kb_001", "kb_002"],
  "mode": "vector_rerank",
  "topK": 8,
  "scoreThreshold": 0.35,
  "rerankThreshold": 0.5,
  "tagFilters": {
    "专业": ["锅炉"],
    "年份": ["2026"]
  },
  "includeContent": true
}
```

响应：

```json
{
  "query": "锅炉技术监督有哪些检查要求？",
  "results": [
    {
      "chunkId": "chunk_001",
      "documentId": "doc_001",
      "knowledgeBaseId": "kb_001",
      "documentName": "技术监督规程.pdf",
      "sectionPath": "1. 总则 / 1.1 适用范围",
      "score": 0.82,
      "rerankScore": 0.91,
      "content": "本规程适用于...",
      "chunkType": "text",
      "tags": {
        "专业": "锅炉"
      }
    }
  ]
}
```

规则：

- 必须过滤用户无权限访问的知识库和文档。
- `includeContent=false` 时仅返回预览和元数据，供轻量列表使用。
- `qa` 和 `document` 应通过该接口复用检索能力。

### 7.2 检索测试

管理员接口：

```http
POST /api/v1/knowledge/retrieval/tests
```

请求同 `retrieval/search`，可额外包含：

```json
{
  "name": "锅炉召回测试"
}
```

响应：

```json
{
  "id": "rt_001",
  "results": [],
  "createdAt": "2026-06-28T10:00:00Z"
}
```

## 8. 报告支撑材料 API

报告支撑材料指报告生成复用的专业业务文档，例如厂级专业报告、技术文档、检查报告。它不是 UI 素材，也不是普通附件。

报告支撑材料首期作为独立业务资源建模，复用 `file` service 的上传、下载授权和 MinIO 能力；需要检索时复用 `knowledge` 的处理和检索能力，避免和普通知识库文档混淆。

### 8.1 创建报告支撑材料

```http
POST /api/v1/knowledge/support-materials
```

请求：

```json
{
  "fileId": "file_001",
  "name": "某电厂迎峰度夏检查材料",
  "materialType": "plant_report",
  "tags": {
    "电厂": "A电厂",
    "专业": "锅炉",
    "年份": "2026"
  }
}
```

响应：

```json
{
  "id": "mat_001",
  "name": "某电厂迎峰度夏检查材料",
  "materialType": "plant_report",
  "status": "uploaded",
  "processingJobId": "job_020"
}
```

### 8.2 查询报告支撑材料

```http
GET /api/v1/knowledge/support-materials?page=1&pageSize=20&type=plant_report&tag.专业=锅炉
```

响应：分页列表。

### 8.3 更新标签

```http
PATCH /api/v1/knowledge/support-materials/{materialId}
```

请求：

```json
{
  "tags": {
    "电厂": "A电厂",
    "专业": "锅炉",
    "年份": "2026"
  }
}
```

### 8.4 删除材料

```http
DELETE /api/v1/knowledge/support-materials/{materialId}
```

响应：`204 No Content`

## 9. 配置 API

### 9.1 获取知识管理配置

```http
GET /api/v1/knowledge/settings
```

响应：

```json
{
  "embeddingModel": {
    "provider": "ai-gateway",
    "profileId": "mp_embedding_default",
    "model": "embedding-model-name",
    "dimension": 1024
  },
  "rerankModel": {
    "provider": "ai-gateway",
    "profileId": "mp_rerank_default",
    "model": "rerank-model-name",
    "topN": 20
  },
  "parser": {
    "backend": "external_api",
    "baseUrl": "https://parser.example.com",
    "maxConcurrency": 4
  }
}
```

### 9.2 更新知识管理配置

```http
PATCH /api/v1/knowledge/settings
```

请求：

```json
{
  "embeddingModel": {
    "provider": "ai-gateway",
    "profileId": "mp_embedding_default",
    "model": "embedding-model-name",
    "dimension": 1024
  },
  "rerankModel": {
    "provider": "ai-gateway",
    "profileId": "mp_rerank_default",
    "model": "rerank-model-name",
    "topN": 20
  },
  "parser": {
    "backend": "external_api",
    "baseUrl": "https://parser.example.com",
    "apiKey": "secret",
    "timeoutSeconds": 120,
    "maxConcurrency": 4
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

- 模型配置中的 `profileId` 指向 AI Gateway 中的 embedding 或 rerank profile；`knowledge` 不保存 provider `baseUrl` 或 `apiKey`，也不直接适配多个模型供应商。
- 配置变更应记录变更人和时间。
- `parser.backend` 首期固定为 `external_api`；`parser.apiKey` 只允许写入，不允许明文读取。
- embedding 维度或模型族变化时创建新的 Qdrant collection 版本，并通过后台任务重建索引；旧 collection 保留到切换完成后清理。

## 10. 统计 API

```http
GET /api/v1/knowledge/stats/overview
```

响应：

```json
{
  "knowledgeBaseCount": 12,
  "documentCount": 128,
  "chunkCount": 9800,
  "uploadTrend30d": [
    {
      "date": "2026-06-28",
      "count": 6
    }
  ]
}
```

## 11. 存储与数据归属

| 数据 | 存储 | 所有者 |
| --- | --- | --- |
| 知识库元数据 | PostgreSQL | `knowledge` |
| 文档元数据和状态 | PostgreSQL | `knowledge` |
| 文件对象和生成下载 URL | MinIO + PostgreSQL 元数据；bucket 首期分为 `source-files`、`templates`、`generated-reports` | `file` |
| 切片元数据 | PostgreSQL | `knowledge` |
| 向量和检索 payload | Qdrant | `knowledge` |
| 处理任务状态 | PostgreSQL 持久化，Redis 队列和短期状态辅助；自动重试最多 3 次 | `knowledge` |
| 模型配置 | PostgreSQL 保存业务默认参数和 AI Gateway profile 引用；provider 密钥由 AI Gateway 管理 | `knowledge` / `ai-gateway` |

## 12. 已确认决策与后续跟踪

| 编号 | 结论 |
| --- | --- |
| K1 | 首期采用角色级 RBAC 和知识库可见性，不做组织/电厂/专业多维权限。 |
| K2 | 报告支撑材料是独立资源，复用 `file` service 和必要的 `knowledge` 检索能力。 |
| K3 | 文档删除首期按软删除设计；Qdrant 和 MinIO 清理在后台生命周期任务中处理。 |
| K4 | 文档解析/OCR 首期使用外部 HTTP 解析服务，通过 `parser.baseUrl`、`apiKey`、`timeoutSeconds`、`maxConcurrency` 配置。 |
| K5 | 异步任务采用 Redis 队列 + PostgreSQL 持久化状态。 |
| K6 | embedding 维度或模型族变化后创建版本化 Qdrant collection，并通过后台任务重建索引。 |
| K7 | 任务自动重试最多 3 次，PostgreSQL job 保留最近 10 次尝试摘要。 |
| K8 | MinIO bucket 首期拆为 `source-files`、`templates`、`generated-reports` 三类，实际名称由部署配置决定。 |
| K9 | 审计日志首期暂缓，不作为知识管理 API 的强制验收项；首期保留配置变更、任务失败和删除结果等排查字段。 |
