# Knowledge Service 实现说明

版本：v0.4
日期：2026-06-29
范围：`services/knowledge/` 当前实现、契约对齐、缺口和后续实现约束

## 1. 文档定位

本文档描述 `knowledge` 当前实现状态和后续实现约束。它只补充服务 README、OpenAPI、架构和技术选型文档，不覆盖这些上游契约。

权威来源：

| 类型 | 权威来源 | 本文档关系 |
| --- | --- | --- |
| 服务公开说明 | `docs/services/knowledge/README.md` | 只能补充，不能覆盖 |
| 服务 OpenAPI | `services/knowledge/api/openapi.yaml` | 只能跟随，不能另起契约 |
| Gateway 公开契约 | `docs/services/gateway/api/openapi.yaml` | 前端稳定契约以 gateway 为准 |
| 服务边界 | `docs/architecture/service-boundaries.md` | 必须遵守 |
| 技术基线 | `docs/architecture/technology-decisions.md` | 必须跟随 |
| 代码实现 | `services/knowledge/` | 本文档记录当前状态和差距 |

凡是本文档与上表文件冲突，以上游文件为准；发现冲突时，在“文档与实现出入”中记录并生成回写或实现任务。

## 2. 当前结论

| 项目 | 状态 | 说明 |
| --- | --- | --- |
| 文档状态 | active | README、公开草案、数据模型、内部 OpenAPI 和实现说明存在。 |
| 代码状态 | partial | Go service 已实现知识库 CRUD、文档列表/上传/详情、File Service handoff、PostgreSQL repository 和 asynq enqueue。 |
| 契约对齐 | drifted | Gateway OpenAPI 已声明 chunks、content、knowledge-queries、parser configs；gateway route matrix 将其中多个 Knowledge active path 标为 `NotImplemented`。 |
| 数据持久化 | postgres / Redis queue | runtime 使用 PostgreSQL；上传后投递 asynq ingestion task。Qdrant 和 chunk/vector 写入未落地。 |
| 测试状态 | partial | 单元和 handler tests 覆盖 CRUD、权限、上传补偿和 queue handoff；缺端到端入库、Qdrant、检索测试。 |
| 建议动作 | 补实现 / 回写文档 | 优先补文档内容/切片/检索链路，或回写公开契约为阶段性未实现。 |

## 3. 已实现

| 能力 | 代码位置 | 契约来源 | 验证方式 | 备注 |
| --- | --- | --- | --- | --- |
| 健康检查 | `services/knowledge/internal/http/server.go` | `services/knowledge/api/openapi.yaml` | `cd services/knowledge && go test ./...` | `GET /healthz`、`GET /readyz`。 |
| 知识库 CRUD | `services/knowledge/internal/http/server.go`、`internal/service/service.go` | `services/knowledge/api/openapi.yaml` | `TestKnowledgeBaseCRUDAndSoftDelete` | 支持列表、创建、详情、更新、软删除。 |
| 用户上下文和权限校验 | `services/knowledge/internal/service/service.go` | `docs/services/knowledge/README.md` | service tests | 依赖 gateway 注入的 user/permission context。 |
| 文档列表和详情 | `services/knowledge/internal/http/server.go` | `services/knowledge/api/openapi.yaml` | `TestDocumentListAndDetailExcludeDeletedKnowledgeBase` | 只覆盖文档元数据/状态。 |
| 文档上传 handoff | `services/knowledge/internal/platform/fileclient/client.go`、`internal/service/service.go` | `docs/services/knowledge/README.md`、`docs/services/file/README.md` | `TestUploadDocumentCreatesDocumentJobAndQueuesIngestion` | multipart 上传后调用 File `/internal/v1/files`，保存 `file_ref`，创建 processing job。 |
| asynq 入队 | `services/knowledge/internal/platform/queue` | `docs/architecture/technology-decisions.md` | service tests with fake queue | 只投递 ingestion task；worker 未落地。 |
| PostgreSQL migration/repository | `services/knowledge/migrations/0001_create_knowledge_core_tables.sql`、`internal/repository/postgres.go` | `docs/services/knowledge/docs/data-models.md` | `go test ./...` | runtime 使用 PostgreSQL。 |

## 4. 未实现

| 缺口 | 文档来源 | 影响范围 | 建议任务 |
| --- | --- | --- | --- |
| 文档处理 worker 未实现 | `docs/services/knowledge/README.md`、`docs/requirements-analysis/overall-requirements-analysis.md` | worker / DB / Redis | 待确认：实现 parsing/chunking/embedding 状态流转。 |
| chunks API 未实现 | `docs/services/gateway/api/openapi.yaml` | API / frontend / QA | 待确认：实现 `GET /internal/v1/documents/{documentId}/chunks` 并开放 gateway route。 |
| 原文 content API 未实现 | `docs/services/gateway/api/openapi.yaml` | API / file integration | 待确认：通过 file reference 读取原文件内容。 |
| `knowledge-queries` 检索未实现 | `docs/services/gateway/api/openapi.yaml`、`docs/services/knowledge/api/public.openapi.yaml` | API / QA / document | 待确认：实现 retrieval response，接入 Qdrant 或阶段性检索 adapter。 |
| Qdrant adapter / embedding / rerank 未实现 | `docs/architecture/service-boundaries.md`、`docs/services/knowledge/docs/data-models.md` | vector store / AI provider | 待确认：接入 AI Gateway embeddings/rerank 和 Qdrant。 |
| admin parser-configs 未实现 | `docs/services/gateway/api/openapi.yaml` | API / admin | 待确认：实现解析器配置资源或回写 active contract 状态。 |
| document PATCH/DELETE 未实现 | `docs/services/gateway/api/openapi.yaml` | API / frontend | 待确认：实现标签更新、删除和 file/index cleanup。 |

## 5. 文档与实现出入

| 出入点 | 文档要求 | 当前实现 | 风险 | 建议处理 |
| --- | --- | --- | --- | --- |
| Gateway active Knowledge paths | Gateway OpenAPI 将 18 个 knowledge operation 设为 active | `services/gateway/internal/http/routes.go` 中 `PATCH/DELETE /documents`、chunks、content、knowledge-queries、parser-configs 标为 `NotImplemented` | 前端生成 client 后调用会得到 501 | 要么补实现并取消 `NotImplemented`，要么把阶段性未实现状态回写到契约说明。 |
| 旧实现说明提到 Qdrant/local hashing | 早期文档描述 Qdrant HTTP adapter 和 local hashing embedding | 当前 `services/knowledge/` 无 Qdrant/embedding platform 代码 | 文档高估实现成熟度 | 已在本文改为未实现；同步更新技术选型状态。 |
| 公开 Knowledge 草案范围 | `docs/services/knowledge/api/public.openapi.yaml` 覆盖 deletion jobs、processing jobs、query tests、support materials、settings、statistics | runtime 只实现 KB CRUD 和文档 upload/list/detail | 公开草案可能被误读为已落地 | 保留为设计草案，在 implementation 中明确缺口。 |
| File handoff 边界 | Knowledge 拥有文档资源，File 只保存基础 file object | 当前已按 `/internal/v1/files` 保存 raw file，但 content/delete cleanup 未闭环 | 删除或内容读取时 file reference 可能残留 | 实现 document lifecycle cleanup。 |
| asynq 任务状态权威 | PostgreSQL 为 job 状态权威，Redis 只排队 | 已创建 job 并投递任务，但无 worker 更新状态 | 文档状态可能长期停留在 uploaded | 补 worker 或阶段性标记为 pending implementation。 |

## 6. MVP / mock / memory backend / 占位

| 项目 | 当前用途 | 退出条件 | 关联任务 |
| --- | --- | --- | --- |
| memory repository | 单元测试 | PostgreSQL integration tests 覆盖关键 CRUD 后仍可保留测试用 | 待确认 |
| fake file client / fake queue | 上传补偿和入队测试 | 真实 file/Redis 集成测试补齐 | 待确认 |
| gateway `NotImplemented` Knowledge routes | 暂时占住 active contract | 对应服务实现或契约降级 | 待确认 |

## 7. 运行与配置

| 项目 | 当前状态 | 缺口 |
| --- | --- | --- |
| 启动命令 | `cd services/knowledge && go run ./cmd/server` | 需要 PostgreSQL、File Service 和 Redis。 |
| 环境变量 | `DATABASE_URL`、`FILE_SERVICE_BASE_URL`、`KNOWLEDGE_REDIS_ADDR` 必填；另有 HTTP/version/env/max upload/service token/shutdown | 缺 Qdrant、embedding/rerank、parser 配置 runtime env。 |
| PostgreSQL / migration | `migrations/0001_create_knowledge_core_tables.sql`，runtime `pgx/v5` | 需要 migration apply CI/集成测试证据。 |
| Redis / queue | 使用 `asynq` client 投递 ingestion | worker 未实现。 |
| Object storage / vector store / AI provider | 通过 File Service 保存 raw file | Qdrant adapter 尚未落地；Knowledge 尚未接入 AI Gateway embedding/rerank 调用。 |

## 8. 测试与验证

| 验证项 | 命令或步骤 | 当前结果 | 缺口 |
| --- | --- | --- | --- |
| 单元测试 | `cd services/knowledge && go test ./...` | pass（本次执行） | 主要使用 memory/fake 依赖。 |
| 集成测试 | PostgreSQL + File + Redis end-to-end upload | missing | 需要真实依赖联调。 |
| 契约测试 | gateway route matrix + Knowledge handler tests | partial | active path 中多个仍 501。 |
| 手工 smoke | 启动 PostgreSQL、File、Redis 后上传文档 | not run | 需要可复现脚本或 Compose。 |

## 9. 建议任务

| 任务 | 类型 | 优先级 | 依据 | 说明 |
| --- | --- | --- | --- | --- |
| 实现 Knowledge 文档内容、删除和 chunks API | 新任务 | P0 | Gateway active contract | 解除 `/documents/**` 相关 501。 |
| 实现 ingestion worker 与状态流转 | 新任务 | P0 | 上传后必须进入处理闭环 | 处理 parsing/chunking/embedding/ready/failed。 |
| 实现 knowledge-queries 检索 | 新任务 | P0 | QA/Document 依赖检索 | 返回 chunk/document/source/score。 |
| 实现 parser configs 或回写契约状态 | 新任务 | P1 | Gateway active admin paths | 避免 active path 长期 501。 |

## 10. 最近检查记录

| 日期 | 检查人/工具 | 代码基准 | 结论 |
| --- | --- | --- | --- |
| 2026-06-29 | Codex goal | `eddf917` + working tree | Knowledge 已完成 KB CRUD 和文档上传 handoff，但入库 worker、chunks、content、retrieval、parser config 仍是关键缺口。 |
