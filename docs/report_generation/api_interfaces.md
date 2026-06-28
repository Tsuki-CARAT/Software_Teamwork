# 报告生成接口文档

## 1. 文档说明

本文定义报告生成能力的公开 RESTful API 契约，用于约束 `document` 服务与 gateway 的前后端协作。

根据当前主仓库接口规范，稳定公开接口以 `docs/api/gateway.openapi.yaml` 为准。本文中的报告生成路径已经同步进入 gateway OpenAPI active paths，owner service 为 `document`。后续如果新增、删除或修改任一公开接口，必须同步更新：

- `docs/api/gateway.openapi.yaml`
- `docs/frontend-backend-contract.md`
- `docs/service-boundaries.md`
- 本文档

本文用于解释报告生成接口的业务语义和使用方式；字段、响应 envelope、错误结构与可依赖路径以 gateway OpenAPI 为最终准绳。

## 2. 接口设计原则

### 2.1 Base Path

公开接口统一挂在 gateway：

```text
/api/v1
```

前端、管理端、其他后端模块和 MCP 工具的 HTTP 调用都只调用 gateway，不直接调用 `document` 服务内部地址。

### 2.2 RESTful 资源路径

公开接口必须使用 RESTful 资源路径，由 HTTP method 表达动作。

禁止在稳定 path 中使用以下动作词：

- `generate`
- `regenerate`
- `export`
- `retry`
- `download`

报告生成中的“生成、重新生成、导出、重试”统一建模为资源创建：

| 业务动作 | RESTful 建模 |
|---|---|
| 生成或重新生成大纲 | `POST /api/v1/reports/{reportId}/jobs`，`jobType=outline_generation` 或 `outline_regeneration` |
| 生成或重新生成正文 | `POST /api/v1/reports/{reportId}/jobs`，`jobType=content_generation` 或 `content_regeneration` |
| 重新生成指定章节 | `POST /api/v1/reports/{reportId}/sections/{sectionId}/versions` |
| 重试失败任务 | `POST /api/v1/report-jobs/{jobId}/attempts` |
| 导出 DOCX | `POST /api/v1/report-files` |
| 获取导出文件内容 | `GET /api/v1/report-files/{reportFileId}/content` |

### 2.3 认证与上下文边界

本模块自身不实现用户认证、登录、角色权限系统。

公开 gateway 业务接口默认需要 `Authorization: Bearer <accessToken>`。Gateway 负责认证并向 `document` 服务传递上下文。

前端可传递：

| Header | 必填 | 说明 |
|---|---:|---|
| `Authorization` | 是 | gateway 认证凭据，本文档中的业务接口默认需要 |
| `X-Request-Id` | 否 | 调用方请求链路 ID，不传时由 gateway 生成 |

Gateway 调用 `document` 服务时应传递：

| Header | 说明 |
|---|---|
| `X-Request-Id` | 贯穿一次请求的 request id |
| `X-User-Id` | 已认证用户 ID |
| `X-User-Roles` | 逗号分隔角色 |
| `X-User-Permissions` | 逗号分隔权限 |
| `X-Forwarded-For` | 原始客户端地址链 |
| `X-Forwarded-Proto` | 原始请求协议 |

`document` 服务只消费上述上下文用于审计、权限判断和追踪，不负责登录态创建。

### 2.4 成功响应

单资源响应：

```json
{
  "data": {
    "id": "rep_123"
  },
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
    "total": 0
  },
  "requestId": "req_123"
}
```

### 2.5 错误响应

```json
{
  "error": {
    "code": "validation_error",
    "message": "request validation failed",
    "requestId": "req_123",
    "fields": {
      "reportType": "is required"
    }
  }
}
```

| HTTP status | code | 说明 |
|---:|---|---|
| 400 | `validation_error` | 请求参数不合法 |
| 401 | `unauthorized` | 未认证或认证失效 |
| 403 | `forbidden` | 已认证但权限不足 |
| 404 | `not_found` | 资源不存在或不可见 |
| 409 | `conflict` | 当前状态不允许执行该操作 |
| 429 | `rate_limited` | 频率或额度限制 |
| 502 | `dependency_error` | AI、数据库、file 服务或下游依赖失败 |
| 500 | `internal_error` | 未分类服务端错误 |

AI 生成失败、导出失败等长任务失败优先体现在 `ReportJob.status=failed` 和 `ReportJob.error` 中；只有同步创建任务失败时才直接返回错误响应。

## 3. 接口总览表

下表为报告生成公开接口草案，路径均相对于 gateway `/api/v1`。

| 分组 | 方法 | 路径 | Auth | Owner | 说明 |
|---|---|---|---|---|---|
| 报告类型 | `GET` | `/report-types` | 需要 | `document` | 查询支持的报告类型 |
| 报告模板 | `GET` | `/report-templates` | 需要 | `document` | 查询模板列表 |
| 报告模板 | `POST` | `/report-templates` | 需要 | `document` | 上传模板 |
| 报告模板 | `GET` | `/report-templates/{reportTemplateId}` | 需要 | `document` | 查询模板详情 |
| 报告模板 | `PATCH` | `/report-templates/{reportTemplateId}` | 需要 | `document` | 更新模板元数据 |
| 报告模板 | `DELETE` | `/report-templates/{reportTemplateId}` | 需要 | `document` | 删除或停用模板 |
| 报告模板 | `GET` | `/report-templates/{reportTemplateId}/structure` | 需要 | `document` | 查询模板结构 |
| 报告模板 | `PATCH` | `/report-templates/{reportTemplateId}/structure` | 需要 | `document` | 保存模板结构 |
| 报告素材 | `GET` | `/report-materials` | 需要 | `document` | 查询素材列表 |
| 报告素材 | `POST` | `/report-materials` | 需要 | `document` | 上传素材 |
| 报告素材 | `GET` | `/report-materials/{materialId}` | 需要 | `document` | 查询素材详情 |
| 报告素材 | `DELETE` | `/report-materials/{materialId}` | 需要 | `document` | 删除素材 |
| 报告 | `GET` | `/reports` | 需要 | `document` | 分页查询报告记录 |
| 报告 | `POST` | `/reports` | 需要 | `document` | 创建报告草稿 |
| 报告 | `GET` | `/reports/{reportId}` | 需要 | `document` | 查询报告详情 |
| 报告 | `PATCH` | `/reports/{reportId}` | 需要 | `document` | 更新报告基础信息 |
| 报告 | `DELETE` | `/reports/{reportId}` | 需要 | `document` | 删除报告记录 |
| 大纲 | `GET` | `/reports/{reportId}/outlines` | 需要 | `document` | 查询报告大纲版本 |
| 大纲 | `POST` | `/reports/{reportId}/outlines` | 需要 | `document` | 创建或保存大纲版本 |
| 大纲 | `GET` | `/reports/{reportId}/outlines/{outlineId}` | 需要 | `document` | 查询指定大纲 |
| 大纲 | `PATCH` | `/reports/{reportId}/outlines/{outlineId}` | 需要 | `document` | 编辑大纲内容、排序、编号 |
| 大纲 | `DELETE` | `/reports/{reportId}/outlines/{outlineId}/sections/{sectionId}` | 需要 | `document` | 删除大纲章节 |
| 正文 | `GET` | `/reports/{reportId}/sections` | 需要 | `document` | 查询章节内容 |
| 正文 | `POST` | `/reports/{reportId}/sections` | 需要 | `document` | 创建章节草稿或批量保存章节 |
| 正文 | `GET` | `/reports/{reportId}/sections/{sectionId}` | 需要 | `document` | 查询指定章节 |
| 正文 | `PATCH` | `/reports/{reportId}/sections/{sectionId}` | 需要 | `document` | 更新章节正文或表格 |
| 正文版本 | `GET` | `/reports/{reportId}/sections/{sectionId}/versions` | 需要 | `document` | 查询章节版本 |
| 正文版本 | `POST` | `/reports/{reportId}/sections/{sectionId}/versions` | 需要 | `document` | 创建章节新版本，可用于 AI 重新生成 |
| 生成任务 | `GET` | `/report-jobs/{jobId}` | 需要 | `document` | 查询任务状态 |
| 生成任务 | `GET` | `/reports/{reportId}/jobs` | 需要 | `document` | 查询报告任务列表 |
| 生成任务 | `POST` | `/reports/{reportId}/jobs` | 需要 | `document` | 创建生成或重新生成任务 |
| 任务尝试 | `GET` | `/report-jobs/{jobId}/attempts` | 需要 | `document` | 查询任务尝试记录 |
| 任务尝试 | `POST` | `/report-jobs/{jobId}/attempts` | 需要 | `document` | 创建新的任务尝试，用于重试 |
| 事件 | `GET` | `/reports/{reportId}/events` | 需要 | `document` | 查询或订阅报告生成事件 |
| 报告文件 | `GET` | `/report-files` | 需要 | `document` | 查询报告文件列表 |
| 报告文件 | `POST` | `/report-files` | 需要 | `document` | 创建报告文件，可用于 DOCX 导出 |
| 报告文件 | `GET` | `/report-files/{reportFileId}` | 需要 | `document` | 查询报告文件元数据 |
| 报告文件 | `GET` | `/report-files/{reportFileId}/content` | 需要 | `document` | 获取报告文件内容 |
| 统计 | `GET` | `/report-statistics/overview` | 需要 | `document` | 查询报告统计概览 |
| 统计 | `GET` | `/report-statistics/daily` | 需要 | `document` | 查询每日生成趋势 |
| 日志 | `GET` | `/report-operation-logs` | 需要 | `document` | 查询报告相关操作日志 |

## 4. 报告类型与模板

### 4.1 查询报告类型

```http
GET /api/v1/report-types
```

响应：

```json
{
  "data": [
    {
      "code": "summer_peak_inspection",
      "name": "迎峰度夏检查报告",
      "description": "用于迎峰度夏检查场景",
      "enabled": true
    },
    {
      "code": "coal_inventory_audit",
      "name": "煤库存审计报告",
      "description": "用于煤库存审计场景",
      "enabled": true
    }
  ],
  "requestId": "req_123"
}
```

### 4.2 查询模板列表

```http
GET /api/v1/report-templates?page=1&pageSize=20&reportType=summer_peak_inspection&enabled=true
```

响应使用分页 envelope。

### 4.3 上传模板

```http
POST /api/v1/report-templates
```

请求类型：`multipart/form-data`

字段：

- `file`
- `templateName`
- `reportType`
- `description`

### 4.4 查询、更新、删除模板

```http
GET /api/v1/report-templates/{reportTemplateId}
PATCH /api/v1/report-templates/{reportTemplateId}
DELETE /api/v1/report-templates/{reportTemplateId}
```

### 4.5 查询和保存模板结构

```http
GET /api/v1/report-templates/{reportTemplateId}/structure
PATCH /api/v1/report-templates/{reportTemplateId}/structure
```

模板结构请求体：

```json
{
  "outlineSchema": [],
  "styleConfig": {}
}
```

## 5. 报告记录

### 5.1 创建报告草稿

```http
POST /api/v1/reports
```

请求体：

```json
{
  "name": "2026年迎峰度夏检查报告",
  "reportType": "summer_peak_inspection",
  "templateId": "tpl_123",
  "topic": "2026年迎峰度夏检查",
  "specialty": "电气",
  "businessObject": "某电厂",
  "year": 2026,
  "extraContext": {
    "region": "华东",
    "notes": "重点关注设备隐患"
  }
}
```

响应：

```json
{
  "data": {
    "id": "rep_123",
    "status": "draft"
  },
  "requestId": "req_123"
}
```

### 5.2 查询报告记录

```http
GET /api/v1/reports?page=1&pageSize=20&reportType=summer_peak_inspection&status=generated
GET /api/v1/reports/{reportId}
```

列表响应使用分页 envelope。详情响应应包含报告基础信息、当前大纲、章节摘要、最新任务和最新报告文件引用。

### 5.3 更新或删除报告

```http
PATCH /api/v1/reports/{reportId}
DELETE /api/v1/reports/{reportId}
```

删除建议默认软删除。物理删除应由后续实现单独确认。

## 6. 大纲与正文资源

### 6.1 创建或保存大纲版本

```http
POST /api/v1/reports/{reportId}/outlines
```

请求体：

```json
{
  "source": "manual",
  "sections": [
    {
      "clientSectionId": "sec_client_1",
      "title": "概述",
      "level": 1,
      "numbering": "1",
      "children": []
    }
  ]
}
```

说明：

- 手工保存大纲时使用 `source=manual`。
- AI 生成大纲不使用动作路径，应通过 `POST /reports/{reportId}/jobs` 创建 `outline_generation` 或 `outline_regeneration` 任务，任务完成后产生新的大纲版本。

### 6.2 查询和编辑大纲

```http
GET /api/v1/reports/{reportId}/outlines
GET /api/v1/reports/{reportId}/outlines/{outlineId}
PATCH /api/v1/reports/{reportId}/outlines/{outlineId}
DELETE /api/v1/reports/{reportId}/outlines/{outlineId}/sections/{sectionId}
```

`PATCH` 可用于修改标题、层级、排序、编号和章节树。

### 6.3 查询和编辑章节

```http
GET /api/v1/reports/{reportId}/sections
GET /api/v1/reports/{reportId}/sections/{sectionId}
PATCH /api/v1/reports/{reportId}/sections/{sectionId}
```

章节更新请求体：

```json
{
  "content": "更新后的章节正文",
  "tables": []
}
```

### 6.4 创建章节版本

```http
POST /api/v1/reports/{reportId}/sections/{sectionId}/versions
```

请求体：

```json
{
  "source": "ai",
  "requirements": "请强化该章节的风险分析内容",
  "materialIds": ["mat_123"],
  "preserveManualEdits": false
}
```

该接口用于创建指定章节的新版本。若处理耗时较长，应先创建 `ReportJob`，再由任务结果产出新版本。

## 7. 生成任务

### 7.1 创建任务

```http
POST /api/v1/reports/{reportId}/jobs
```

请求体：

```json
{
  "jobType": "outline_generation",
  "target": {
    "scope": "report",
    "sectionId": null
  },
  "requirements": "请生成适合迎峰度夏检查场景的专业报告大纲",
  "materialIds": ["mat_123"],
  "options": {
    "preserveManualEdits": false,
    "saveResult": true
  }
}
```

`jobType` 建议枚举：

| jobType | 说明 |
|---|---|
| `outline_generation` | 首次生成大纲 |
| `outline_regeneration` | 重新生成大纲 |
| `content_generation` | 生成完整正文 |
| `content_regeneration` | 重新生成完整正文 |
| `section_regeneration` | 重新生成指定章节 |
| `report_file_creation` | 创建报告文件 |

响应：

```json
{
  "data": {
    "id": "job_123",
    "reportId": "rep_123",
    "jobType": "outline_generation",
    "status": "pending"
  },
  "requestId": "req_123"
}
```

### 7.2 查询任务

```http
GET /api/v1/report-jobs/{jobId}
GET /api/v1/reports/{reportId}/jobs
```

任务状态：

| status | 说明 |
|---|---|
| `pending` | 等待执行 |
| `running` | 执行中 |
| `succeeded` | 成功 |
| `partial_succeeded` | 部分成功 |
| `failed` | 失败 |
| `canceled` | 已取消 |

### 7.3 创建任务尝试

```http
POST /api/v1/report-jobs/{jobId}/attempts
```

说明：

- 该接口创建新的任务尝试，用于替代旧的 `retry` 动作路径。
- 服务应保留原任务和历史尝试记录。

### 7.4 报告事件

```http
GET /api/v1/reports/{reportId}/events
```

可用于轮询事件列表，后续也可升级为 SSE。稳定 SSE 契约需要先进入 gateway OpenAPI。

## 8. 报告文件

### 8.1 创建报告文件

```http
POST /api/v1/report-files
```

请求体：

```json
{
  "reportId": "rep_123",
  "format": "docx",
  "templateId": "tpl_123",
  "styleOptions": {
    "numberingMode": "global"
  }
}
```

说明：

- 该接口用于替代旧的 `export` 动作路径。
- 文件生成耗时较长时，响应可以返回 `reportFile.status=pending` 和关联 `jobId`。
- 文件内容不暴露 MinIO object key、file 内部 ID 或内部 URL，前端只通过 content 接口获取。

### 8.2 查询报告文件

```http
GET /api/v1/report-files?page=1&pageSize=20&reportId=rep_123
GET /api/v1/report-files/{reportFileId}
GET /api/v1/report-files/{reportFileId}/content
```

`GET /api/v1/report-files/{reportFileId}/content` 可返回二进制文件流；错误响应仍使用统一错误结构。

## 9. 素材、统计与日志

### 9.1 报告素材

```http
GET /api/v1/report-materials
POST /api/v1/report-materials
GET /api/v1/report-materials/{materialId}
DELETE /api/v1/report-materials/{materialId}
```

上传使用 `multipart/form-data`。素材可作为报告生成任务的 `materialIds` 输入。报告素材是 `document` 拥有的独立业务资源；底层原文件由 `document` 通过 file 服务保存和读取，不复用知识库文档上传路径。

### 9.2 统计

```http
GET /api/v1/report-statistics/overview
GET /api/v1/report-statistics/daily?days=30
```

统计接口应返回报告数量、模板数量、素材数量、任务状态分布、近 30 天趋势等。

### 9.3 操作日志

```http
GET /api/v1/report-operation-logs?targetType=report&targetId=rep_123
GET /api/v1/report-operation-logs?requestId=req_123&toolName=generate_report_outline
```

操作日志用于追溯报告创建、任务创建、内容编辑、模板变更、素材变更、文件创建和 MCP 工具调用。

公开响应可包含：

| 字段 | 说明 |
|---|---|
| `id` | 操作日志 ID |
| `operatorId` | 操作者 ID，来自 gateway 或 MCP 上下文 |
| `operatorName` | 操作者名称 |
| `operationType` | 操作类型 |
| `targetType` | 目标资源类型 |
| `targetId` | 目标资源 ID |
| `requestId` | 请求链路 ID |
| `requestSource` | 请求来源，例如 `frontend`、`admin`、`mcp`、`backend` |
| `toolName` | MCP 工具名，可空 |
| `parameterSummary` | 已脱敏的参数摘要，不包含 prompt、密钥、MinIO object key 或完整文档内容 |
| `operationResult` | 操作结果 |
| `errorMessage` | 错误信息，可空 |
| `metadata` | 已脱敏审计扩展信息 |
| `createdAt` | 创建时间 |

## 10. MCP 工具映射

MCP 工具可以继续使用动词型工具名。工具内部调用 HTTP 接口时，必须映射到 gateway `/api/v1` 下的 RESTful 资源接口，不得直接调用 `document` 服务内部地址。

| MCP 工具 | 建议 HTTP 能力 |
|---|---|
| `generate_report_outline` | `POST /reports/{reportId}/jobs`，`jobType=outline_generation` |
| `regenerate_report_outline` | `POST /reports/{reportId}/jobs`，`jobType=outline_regeneration` |
| `generate_report_text` | `POST /reports/{reportId}/jobs`，`jobType=content_generation` |
| `regenerate_report_text` | `POST /reports/{reportId}/jobs`，`jobType=content_regeneration` |
| `regenerate_report_section` | `POST /reports/{reportId}/jobs`，`jobType=section_regeneration` |
| `get_generation_status` | `GET /report-jobs/{jobId}` |
| `get_report_result` | `GET /reports/{reportId}` |
| `export_report_docx` | `POST /report-files` |
| `get_template_schema` | `GET /report-templates/{reportTemplateId}/structure` |

## 11. 接口验收要求

- 公开路径必须已进入 `docs/api/gateway.openapi.yaml` active paths，才能作为稳定前端契约。
- 路径必须使用资源名，不出现 `generate`、`regenerate`、`export`、`retry`、`download` 等动作词。
- 所有公开响应使用 gateway 统一 envelope。
- 所有公开 ID 使用 string，公开字段使用 camelCase。
- 业务接口默认通过 gateway bearer token 认证；`document` 服务不实现登录认证，前端、管理端、其他后端模块和 MCP 工具不得绕过 gateway 直连 `document`。
- Gateway 向 `document` 服务传递 `X-Request-Id`、`X-User-Id`、`X-User-Roles`、`X-User-Permissions`。
- AI 生成、重新生成、文件创建和任务重试都必须创建可追踪资源记录。
- 文件内容接口不得暴露 MinIO object key、file 内部 ID、内部 URL 或敏感存储路径。
- MCP 工具调用应记录 request id、工具名、参数摘要、执行状态和错误信息。
