# 报告生成 API 契约

## 1. 契约目标

本文定义报告生成模块的 HTTP API 契约，覆盖报告类型、模板管理、报告支撑材料引用、报告记录、大纲生成与编辑、章节内容流式生成、章节编辑、DOCX 导出和统计监控。

主责服务：

- `document`：报告模板、报告记录、大纲、章节内容、导出任务、报告统计。
- `knowledge`：报告支撑材料管理和检索能力，提供专业材料片段。
- `file`：模板文件、生成 DOCX、图片等对象存储和下载授权。
- `auth`：用户身份、角色权限。
- `gateway`：外部 API 入口和认证上下文转发。

边界原则：

- `document` 不直接读写 Qdrant，报告生成需要材料时通过 `knowledge` 检索或支撑材料接口获取。
- `document` 不直接暴露 MinIO object key，文件下载必须通过 `file` 或业务下载 URL 接口授权。
- 报告内容、章节、大纲、导出记录等业务真相保存在 PostgreSQL。
- DOCX 文件、模板文件和较大的图片/中间产物保存在 MinIO。
- Redis 可用于流式生成短期状态、任务进度缓存或轻量队列，但不作为长期业务真相。

## 2. 通用约定

### 2.1 基础路径

待确认：是否统一使用 `/api/v1`。本文暂按以下形式描述：

```text
/api/v1/reports/...
/api/v1/report-templates/...
```

### 2.2 RESTful + OpenAPI + Swagger UI 规范

本模块接口必须按 RESTful 风格设计，并以 OpenAPI 3.0+ 作为机器可读契约。Swagger UI 用于开发联调、验收演示和接口自测。

RESTful 约定：

- 报告、模板、章节、导出记录、生成任务均作为资源或子资源表达。
- 创建报告使用 `POST /reports`，查询报告详情使用 `GET /reports/{reportId}`。
- 大纲、章节、导出等属于报告子资源，例如 `/reports/{reportId}/outline`、`/reports/{reportId}/sections`、`/reports/{reportId}/exports`。
- 生成、重新导出、取消等动作型能力使用 `POST /resource:action`。
- 流式章节生成需在 OpenAPI 中声明为 `text/event-stream` 响应，并列出事件类型。
- 所有列表接口必须分页，时间字段统一使用 UTC ISO 8601 字符串。

OpenAPI 约定：

- `document` 服务维护服务内契约：`services/document/api/openapi.yaml`。
- 当前文档配套的 OpenAPI 草稿见 [reports.openapi.yaml](./openapi/reports.openapi.yaml)。
- 涉及模板文件和 DOCX 下载的公共文件接口由 `file` 服务维护：`services/file/api/openapi.yaml`。
- OpenAPI 必须声明 `securitySchemes`、通用错误响应、分页响应、SSE 响应、报告/模板/章节/导出 schema、状态枚举。
- 对 `knowledge` 支撑材料和检索 API 的依赖应通过外部契约引用或描述说明，不复制 Qdrant 内部结构。

Swagger UI 约定：

- 网关应暴露聚合入口，建议为 `/api/docs`。
- 服务级 OpenAPI JSON/YAML 建议暴露为 `/api/v1/reports/openapi.yaml` 或通过网关聚合。
- Swagger UI 只用于开发、测试和内网验收环境；生产环境是否开放需由部署配置控制。

### 2.3 认证与权限

待确认：认证方式采用 Cookie Session、Bearer Token/JWT，还是二者兼容。

权限要求：

| 能力 | 标准用户 | 管理员 | 超级管理员 |
| --- | --- | --- | --- |
| 创建和管理自己的报告 | 支持 | 支持 | 支持 |
| 下载自己的报告 | 支持 | 支持 | 支持 |
| 管理报告模板 | 不支持 | 支持 | 支持 |
| 管理报告支撑材料 | 待确认 | 支持 | 支持 |
| 查看全局报告统计 | 不支持 | 支持 | 支持 |

### 2.4 通用错误响应

```json
{
  "error": {
    "code": "validation_error",
    "message": "request validation failed",
    "requestId": "req_123",
    "fields": {
      "reportType": "required"
    }
  }
}
```

通用错误码：

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

### 2.5 报告类型

首期固定支持：

```text
summer_peak_inspection  迎峰度夏检查报告
coal_inventory_audit    煤库存审计报告
```

后续可扩展新的报告类型和模板。

### 2.6 报告状态

```text
draft
outline_generating
outline_ready
content_generating
content_ready
exporting
export_ready
failed
deleted
```

章节状态：

```text
pending
generating
ready
failed
cancelled
```

章节类型：

```text
text
table
image
mixed
```

## 3. 数据对象

### 3.1 ReportTemplate

```json
{
  "id": "tpl_001",
  "name": "迎峰度夏检查报告模板",
  "reportType": "summer_peak_inspection",
  "fileId": "file_001",
  "status": "active",
  "styleProfile": {
    "font": "SimSun",
    "titleLevels": 3,
    "tableStyle": "default"
  },
  "createdBy": "user_001",
  "createdAt": "2026-06-28T10:00:00Z",
  "updatedAt": "2026-06-28T10:00:00Z"
}
```

### 3.2 Report

```json
{
  "id": "rpt_001",
  "title": "A电厂迎峰度夏检查报告",
  "reportType": "summer_peak_inspection",
  "specialty": "锅炉",
  "plantName": "A电厂",
  "year": 2026,
  "status": "content_ready",
  "templateId": "tpl_001",
  "ownerUserId": "user_001",
  "createdAt": "2026-06-28T10:00:00Z",
  "updatedAt": "2026-06-28T10:20:00Z"
}
```

### 3.3 ReportOutlineNode

```json
{
  "id": "node_001",
  "reportId": "rpt_001",
  "parentId": null,
  "number": "1",
  "title": "总体情况",
  "sectionType": "text",
  "sortOrder": 1,
  "children": []
}
```

### 3.4 ReportSection

```json
{
  "id": "sec_001",
  "reportId": "rpt_001",
  "outlineNodeId": "node_001",
  "number": "1",
  "title": "总体情况",
  "sectionType": "text",
  "status": "ready",
  "content": "本章节正文...",
  "tableData": null,
  "imageFileIds": [],
  "citations": [
    {
      "materialId": "mat_001",
      "chunkId": "chunk_001",
      "documentName": "检查材料.pdf",
      "sectionPath": "1. 检查范围"
    }
  ],
  "updatedAt": "2026-06-28T10:20:00Z"
}
```

### 3.5 ReportExport

```json
{
  "id": "exp_001",
  "reportId": "rpt_001",
  "fileId": "file_100",
  "format": "docx",
  "status": "ready",
  "styleProfileVersion": "sp_001",
  "createdAt": "2026-06-28T10:30:00Z"
}
```

## 4. 报告模板 API

### 4.1 上传模板文件

建议复用 `file` 服务两步上传。

```http
POST /api/v1/files/uploads
```

请求：

```json
{
  "filename": "迎峰度夏检查报告模板.docx",
  "mimeType": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  "sizeBytes": 204800,
  "purpose": "report_template"
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

### 4.2 创建模板记录

```http
POST /api/v1/report-templates
```

请求：

```json
{
  "name": "迎峰度夏检查报告模板",
  "reportType": "summer_peak_inspection",
  "fileId": "file_001",
  "styleProfile": {
    "font": "SimSun",
    "titleLevels": 3,
    "paragraphSpacing": "standard",
    "tableStyle": "default",
    "headerFooter": "default"
  }
}
```

响应：`201 Created`

```json
{
  "id": "tpl_001",
  "name": "迎峰度夏检查报告模板",
  "reportType": "summer_peak_inspection",
  "status": "active",
  "createdAt": "2026-06-28T10:00:00Z"
}
```

### 4.3 查询模板列表

```http
GET /api/v1/report-templates?page=1&pageSize=20&reportType=summer_peak_inspection
```

响应：

```json
{
  "items": [
    {
      "id": "tpl_001",
      "name": "迎峰度夏检查报告模板",
      "reportType": "summer_peak_inspection",
      "status": "active",
      "createdAt": "2026-06-28T10:00:00Z"
    }
  ],
  "page": 1,
  "pageSize": 20,
  "total": 1
}
```

### 4.4 获取模板详情

```http
GET /api/v1/report-templates/{templateId}
```

响应：`ReportTemplate`

### 4.5 更新模板

```http
PATCH /api/v1/report-templates/{templateId}
```

请求：

```json
{
  "name": "迎峰度夏检查报告模板 v2",
  "styleProfile": {
    "font": "SimSun",
    "titleLevels": 3,
    "tableStyle": "grid"
  }
}
```

响应：`ReportTemplate`

### 4.6 删除模板

```http
DELETE /api/v1/report-templates/{templateId}
```

响应：`204 No Content`

规则：

- 已被历史报告使用的模板是否允许删除待确认。建议软删除或停用。
- 模板文件在 MinIO 中的清理策略待确认。

### 4.7 模板可视化配置

加分项接口，首期可暂缓。

```http
PATCH /api/v1/report-templates/{templateId}/structure
```

请求：

```json
{
  "outlineSchema": [
    {
      "title": "总体情况",
      "sectionType": "text",
      "required": true
    }
  ],
  "materialMappings": [
    {
      "sectionTitle": "总体情况",
      "tagFilters": {
        "专业": ["锅炉"]
      }
    }
  ]
}
```

## 5. 报告记录 API

### 5.1 创建报告

```http
POST /api/v1/reports
```

请求：

```json
{
  "title": "A电厂迎峰度夏检查报告",
  "reportType": "summer_peak_inspection",
  "specialty": "锅炉",
  "plantName": "A电厂",
  "year": 2026,
  "templateId": "tpl_001",
  "topic": "A电厂 2026 年迎峰度夏检查"
}
```

响应：`201 Created`

```json
{
  "id": "rpt_001",
  "title": "A电厂迎峰度夏检查报告",
  "reportType": "summer_peak_inspection",
  "status": "draft",
  "createdAt": "2026-06-28T10:00:00Z"
}
```

规则：

- `reportType` 首期必须为两种固定类型之一。
- `templateId` 可选；不传时使用对应报告类型的默认模板，是否允许待确认。

### 5.2 查询报告列表

```http
GET /api/v1/reports?page=1&pageSize=20&reportType=summer_peak_inspection&year=2026
```

响应：

```json
{
  "items": [
    {
      "id": "rpt_001",
      "title": "A电厂迎峰度夏检查报告",
      "reportType": "summer_peak_inspection",
      "specialty": "锅炉",
      "plantName": "A电厂",
      "year": 2026,
      "status": "content_ready",
      "createdAt": "2026-06-28T10:00:00Z"
    }
  ],
  "page": 1,
  "pageSize": 20,
  "total": 1
}
```

### 5.3 获取报告详情

```http
GET /api/v1/reports/{reportId}
```

响应：`Report`

### 5.4 更新报告基础信息

```http
PATCH /api/v1/reports/{reportId}
```

请求：

```json
{
  "title": "A电厂迎峰度夏检查报告",
  "specialty": "锅炉",
  "plantName": "A电厂",
  "year": 2026
}
```

### 5.5 删除报告

```http
DELETE /api/v1/reports/{reportId}
```

响应：`204 No Content`

规则：

- 建议软删除，保留必要审计信息。
- 用户只能删除自己的报告，管理员全局删除策略待确认。

## 6. 大纲 API

### 6.1 生成大纲

```http
POST /api/v1/reports/{reportId}/outline:generate
```

请求：

```json
{
  "topic": "A电厂 2026 年迎峰度夏检查",
  "mode": "template",
  "materialFilters": {
    "专业": ["锅炉"],
    "电厂": ["A电厂"],
    "年份": ["2026"]
  }
}
```

响应：

```json
{
  "reportId": "rpt_001",
  "status": "outline_ready",
  "outline": [
    {
      "id": "node_001",
      "number": "1",
      "title": "总体情况",
      "sectionType": "text",
      "children": []
    }
  ]
}
```

规则：

- 固定报告类型优先基于预定义模板生成固定大纲结构。
- `mode=ai` 可作为后续增强，允许 AI 根据主题生成大纲。

### 6.2 查询大纲

```http
GET /api/v1/reports/{reportId}/outline
```

响应：

```json
{
  "items": [
    {
      "id": "node_001",
      "number": "1",
      "title": "总体情况",
      "sectionType": "text",
      "sortOrder": 1,
      "children": []
    }
  ]
}
```

### 6.3 更新大纲

```http
PUT /api/v1/reports/{reportId}/outline
```

请求：

```json
{
  "items": [
    {
      "id": "node_001",
      "title": "总体情况",
      "sectionType": "text",
      "sortOrder": 1,
      "children": []
    }
  ],
  "autoRenumber": true
}
```

响应：

```json
{
  "items": [
    {
      "id": "node_001",
      "number": "1",
      "title": "总体情况",
      "sectionType": "text",
      "sortOrder": 1
    }
  ]
}
```

规则：

- 支持删除章节、调整顺序。
- `autoRenumber=true` 时服务端重新生成章节编号。
- 大纲更新可能影响已有章节内容，处理策略待确认。

## 7. 章节内容 API

### 7.1 生成全部章节

非流式任务启动：

```http
POST /api/v1/reports/{reportId}/sections:generate
```

请求：

```json
{
  "sectionIds": [],
  "materialFilters": {
    "专业": ["锅炉"],
    "电厂": ["A电厂"],
    "年份": ["2026"]
  },
  "retrieval": {
    "topK": 8,
    "scoreThreshold": 0.35,
    "rerankThreshold": 0.5,
    "enableRerank": true
  }
}
```

响应：

```json
{
  "jobId": "rjob_001",
  "status": "queued"
}
```

规则：

- `document` 通过 `knowledge` 检索接口获取报告支撑材料片段。
- 空 `sectionIds` 表示生成全部未完成章节。

### 7.2 流式生成章节

```http
POST /api/v1/reports/{reportId}/sections:stream
```

请求同生成全部章节。

响应：SSE。

事件：

| event | 含义 |
| --- | --- |
| `job.created` | 生成任务创建 |
| `section.started` | 章节开始生成 |
| `section.delta` | 章节内容增量 |
| `section.completed` | 章节生成完成 |
| `progress.updated` | 总体进度更新 |
| `error` | 生成失败 |

示例：

```text
event: section.delta
data: {"sectionId":"sec_001","delta":"本次检查覆盖..."}

event: progress.updated
data: {"completed":3,"total":8}
```

### 7.3 查询章节列表

```http
GET /api/v1/reports/{reportId}/sections
```

响应：

```json
{
  "items": [
    {
      "id": "sec_001",
      "outlineNodeId": "node_001",
      "number": "1",
      "title": "总体情况",
      "sectionType": "text",
      "status": "ready",
      "content": "本章节正文..."
    }
  ]
}
```

### 7.4 获取章节详情

```http
GET /api/v1/reports/{reportId}/sections/{sectionId}
```

响应：`ReportSection`

### 7.5 编辑章节内容

```http
PATCH /api/v1/reports/{reportId}/sections/{sectionId}
```

请求：

```json
{
  "content": "用户编辑后的章节正文...",
  "tableData": {
    "headers": ["项目", "结果"],
    "rows": [["锅炉检查", "正常"]]
  },
  "imageFileIds": ["file_img_001"]
}
```

响应：`ReportSection`

规则：

- 用户编辑内容必须覆盖后续导出结果。
- 编辑后不应自动重新调用 AI，除非用户明确重新生成章节。

### 7.6 重新生成单章

加分项：

```http
POST /api/v1/reports/{reportId}/sections/{sectionId}:regenerate
```

请求：

```json
{
  "instruction": "补充煤库存风险分析",
  "preserveUserEdits": false
}
```

响应：

```json
{
  "jobId": "rjob_002",
  "status": "queued"
}
```

待确认：重新生成是否覆盖用户编辑内容，建议要求用户明确选择。

## 8. 报告导出 API

### 8.1 导出 DOCX

```http
POST /api/v1/reports/{reportId}/exports
```

请求：

```json
{
  "format": "docx",
  "styleProfileId": "sp_001",
  "numberingMode": "global"
}
```

响应：

```json
{
  "exportId": "exp_001",
  "status": "queued"
}
```

规则：

- 首期必须支持 DOCX。
- 导出必须使用已保存报告数据，不重新执行 AI 生成。
- 导出必须保留用户编辑后的章节内容。
- 统一样式体系属于加分项，但契约预留 `styleProfileId`。

编号模式：

```text
global      全局编号
by_chapter  按章编号
```

图表编号模式是否首期实现待确认。

### 8.2 查询导出记录

```http
GET /api/v1/reports/{reportId}/exports
```

响应：

```json
{
  "items": [
    {
      "id": "exp_001",
      "format": "docx",
      "status": "ready",
      "createdAt": "2026-06-28T10:30:00Z"
    }
  ]
}
```

### 8.3 获取下载 URL

```http
POST /api/v1/reports/{reportId}/exports/{exportId}:download-url
```

响应：

```json
{
  "downloadUrl": "https://minio-presigned-url",
  "expiresAt": "2026-06-28T10:40:00Z"
}
```

规则：

- 必须校验报告访问权限。
- 不返回内部 MinIO object key。
- 建议记录下载审计日志。

### 8.4 基于保存数据重新导出 Word

```http
POST /api/v1/reports/{reportId}:reexport
```

请求：

```json
{
  "format": "docx",
  "styleProfileId": "sp_001"
}
```

响应同创建导出。

## 9. 生成任务 API

### 9.1 查询任务状态

```http
GET /api/v1/reports/jobs/{jobId}
```

响应：

```json
{
  "id": "rjob_001",
  "reportId": "rpt_001",
  "type": "section_generation",
  "status": "running",
  "progress": {
    "completed": 3,
    "total": 8
  },
  "currentSectionId": "sec_004",
  "errorMessage": null,
  "createdAt": "2026-06-28T10:00:00Z",
  "updatedAt": "2026-06-28T10:05:00Z"
}
```

### 9.2 取消任务

```http
POST /api/v1/reports/jobs/{jobId}:cancel
```

响应：

```json
{
  "id": "rjob_001",
  "status": "cancelled"
}
```

## 10. 报告支撑材料引用

报告支撑材料由知识管理模块管理，报告生成模块通过以下方式引用：

- 根据模板映射或用户筛选条件调用 `knowledge` 检索接口。
- 保存生成章节引用的材料片段快照，便于后续追溯。
- 下载原始支撑材料时跳转到知识管理/文件服务授权下载接口。

报告生成侧可提供只读引用查询：

```http
GET /api/v1/reports/{reportId}/materials
```

响应：

```json
{
  "items": [
    {
      "materialId": "mat_001",
      "name": "A电厂迎峰度夏检查材料",
      "materialType": "plant_report",
      "tags": {
        "电厂": "A电厂",
        "专业": "锅炉"
      },
      "usedBySections": ["sec_001", "sec_002"]
    }
  ]
}
```

## 11. 配置 API

### 11.1 获取报告生成配置

管理员接口：

```http
GET /api/v1/reports/settings
```

响应：

```json
{
  "llm": {
    "provider": "siliconflow",
    "model": "llm-model-name",
    "baseUrl": "https://api.example.com",
    "timeoutSeconds": 120
  },
  "defaultTemplates": {
    "summer_peak_inspection": "tpl_001",
    "coal_inventory_audit": "tpl_002"
  },
  "export": {
    "defaultFormat": "docx",
    "defaultNumberingMode": "global"
  }
}
```

### 11.2 更新报告生成配置

```http
PATCH /api/v1/reports/settings
```

请求：

```json
{
  "llm": {
    "provider": "siliconflow",
    "model": "llm-model-name",
    "baseUrl": "https://api.example.com",
    "apiKey": "secret",
    "timeoutSeconds": 120
  },
  "defaultTemplates": {
    "summer_peak_inspection": "tpl_001",
    "coal_inventory_audit": "tpl_002"
  },
  "export": {
    "defaultFormat": "docx",
    "defaultNumberingMode": "global"
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
- 默认模板必须存在且状态可用。

## 12. 统计 API

```http
GET /api/v1/reports/stats/overview
```

响应：

```json
{
  "templateCount": 6,
  "reportCount": 120,
  "trend30d": [
    {
      "date": "2026-06-28",
      "generatedCount": 5
    }
  ]
}
```

可选增强：

- 按报告类型统计。
- 按电厂、专业、年份统计。
- 统计导出成功率和平均生成耗时。

## 13. 存储与数据归属

| 数据 | 存储 | 所有者 |
| --- | --- | --- |
| 报告模板元数据 | PostgreSQL | `document` |
| 模板文件 | MinIO + file metadata | `file` / `document` |
| 报告记录、大纲、章节内容 | PostgreSQL | `document` |
| 章节引用快照 | PostgreSQL | `document` |
| 生成 DOCX | MinIO + export metadata | `file` / `document` |
| 生成任务状态 | PostgreSQL，Redis 可缓存 | `document` |
| 报告支撑材料 | PostgreSQL/Qdrant/MinIO，主责在 `knowledge` 和 `file` | `knowledge` / `file` |
| LLM 配置 | PostgreSQL 或安全配置存储，密钥需加密 | `document` |

## 14. 待确认问题

| 编号 | 问题 |
| --- | --- |
| R1 | 模板是否必须上传 DOCX，还是允许 JSON/Markdown 模板？ |
| R2 | 固定报告类型的大纲结构由代码内置、数据库模板配置，还是模板文件解析生成？ |
| R3 | 报告支撑材料归属知识管理还是报告生成？建议由知识管理主责，报告生成只引用。 |
| R4 | 章节流式生成中断后是否保存部分内容？ |
| R5 | 重新生成单章是否覆盖用户编辑，是否需要版本历史？ |
| R6 | DOCX 样式统一配置是否首期必须实现，还是加分项？ |
| R7 | 图/表编号模式切换是否首期必须实现？ |
| R8 | 报告删除是否软删除？生成文件何时清理？ |
| R9 | 报告记录是否支持管理员查看所有用户报告？ |
| R10 | 生成报告是否需要完整审计日志和模型调用成本统计？ |
