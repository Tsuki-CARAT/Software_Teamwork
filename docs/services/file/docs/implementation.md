# File 服务实现说明

版本：v0.3
日期：2026-06-30
范围：`services/file/` 当前实现、契约对齐、缺口和后续实现约束

## 1. 文档定位

本文档描述 `file` 当前实现状态和后续实现约束。它只补充服务 README、OpenAPI、架构和技术选型文档，不覆盖这些上游契约。

权威来源：

| 类型 | 权威来源 | 本文档关系 |
| --- | --- | --- |
| 服务公开说明 | `docs/services/file/README.md` | 只能补充，不能覆盖 |
| 服务 OpenAPI | `services/file/api/openapi.yaml` | 只能跟随，不能另起契约 |
| Gateway 公开契约 | `docs/services/gateway/api/openapi.yaml` | 前端稳定契约以 gateway 为准 |
| 服务边界 | `docs/architecture/service-boundaries.md` | 必须遵守 |
| 技术基线 | `docs/architecture/technology-decisions.md` | 必须跟随 |
| 代码实现 | `services/file/` | 本文档记录当前状态和差距 |

凡是本文档与上表文件冲突，以上游文件为准；发现冲突时，在“文档与实现出入”中记录并生成回写或实现任务。

## 2. 当前结论

| 项目 | 状态 | 说明 |
| --- | --- | --- |
| 文档状态 | active | README、数据模型和内部 OpenAPI 已存在。 |
| 代码状态 | partial | Go service、`/internal/v1/files/**`、兼容 document 路由、memory/local/MinIO object store、file_objects migration、PostgreSQL metadata runtime 和 service-token 校验已存在。 |
| 契约对齐 | partial | 内部 OpenAPI 声明 `/internal/v1/files/**`；代码仍保留 knowledge-document 兼容路由。PostgreSQL metadata runtime 已按 `FILE_DATABASE_URL` 接入；MinIO adapter 已落地但缺本地 server/mc smoke。 |
| 数据持久化 | memory / postgres metadata + memory/local/MinIO objects | `FILE_DATABASE_URL` 为空时 metadata 使用 memory repository；配置后使用 PostgreSQL repository。object bytes 可用 memory、local 或 MinIO adapter。 |
| 测试状态 | partial | `go test ./...` 覆盖 service、handler、local storage、MinIO adapter、config 和默认跳过的 env-gated PostgreSQL repository smoke；缺少真实 PostgreSQL/MinIO 联合 smoke。 |
| 建议动作 | 联调 / 回写文档 | 明确兼容路由退出计划、补 MinIO server/mc Compose 或部署说明，并在可用 PostgreSQL 环境中运行 env-gated smoke。 |

## 3. 已实现

| 能力 | 代码位置 | 契约来源 | 验证方式 | 备注 |
| --- | --- | --- | --- | --- |
| 健康检查 | `services/file/internal/http/server.go` | `services/file/api/openapi.yaml` | `cd services/file && go test ./...` | `GET /healthz`、`GET /readyz`。 |
| 基础文件创建 | `services/file/internal/http/server.go`、`services/file/internal/service/service.go` | `services/file/api/openapi.yaml` | `TestFileCreateGetContentDeleteFlow` | `POST /internal/v1/files` 接收 multipart，计算或校验 SHA-256。 |
| 基础文件元数据读取 | `services/file/internal/service/service.go` | `services/file/api/openapi.yaml` | `TestFileCreateGetContentDeleteFlow` | 返回 file ID、文件名、content type、大小、checksum，不返回 object key。 |
| 基础文件内容读取 | `services/file/internal/http/server.go` | `services/file/api/openapi.yaml` | `TestFileCreateGetContentDeleteFlow` | 成功时返回二进制流，不包 JSON envelope。 |
| 基础文件删除 | `services/file/internal/service/service.go` | `docs/services/file/README.md` | `TestDeleteFileHidesMetadataAndContent` | 删除后 metadata/content 读取返回 not found。 |
| 上传校验 | `services/file/internal/service/service.go`、`services/file/internal/http/server_test.go` | `docs/services/file/README.md` | handler/service tests | 覆盖缺少文件、空文件、超限、checksum 格式和 mismatch。 |
| local object store | `services/file/internal/platform/storage/local.go` | `docs/services/file/README.md` | `TestLocalStorePutGetDelete` | 可用于本地持久化 smoke；metadata 仍非持久。 |
| MinIO object store | `services/file/internal/platform/storage/minio.go` | `docs/services/file/README.md`、`docs/architecture/technology-decisions.md` | `TestMinIOStorePutSendsContentTypeChecksumAndSize` 等 adapter tests | 使用官方 `github.com/minio/minio-go/v7@v7.2.1` SDK；隐藏 bucket、object key、内部 URL 和凭据。 |
| PostgreSQL metadata runtime | `services/file/cmd/server/main.go`、`services/file/internal/repository/postgres.go` | 数据模型 / 技术选型 | `go test ./...`、`FILE_TEST_DATABASE_URL` smoke（可选） | `FILE_DATABASE_URL` 配置后使用 PostgreSQL repository；未配置时保留 memory 模式。 |
| service token 校验 | `services/file/internal/config/config.go`、`services/file/internal/http/server.go` | File README / 内部服务契约 | handler/config tests | DB 模式要求 `FILE_INTERNAL_SERVICE_TOKEN` 或 `INTERNAL_SERVICE_TOKEN`；保护 `/internal/v1/files/**`。 |
| readyz runtime 状态 | `services/file/internal/http/server.go` | `services/file/api/openapi.yaml` | handler tests | 返回 metadata/storage backend、service token 配置状态和 PostgreSQL dependency 状态。 |
| knowledge-document 兼容路由 | `services/file/internal/http/server.go`、`services/file/internal/service/document.go` | 历史联调兼容，不是目标内部 OpenAPI | `TestUploadGetPatchAndContent` | 仍暴露 `/internal/v1/knowledge-bases/{knowledgeBaseId}/documents` 等。 |
| file_objects migration | `services/file/migrations/0001_create_file_objects.sql` | `docs/services/file/docs/data-models.md` | 需手工 goose apply；env-gated test 会应用 migration | PostgreSQL runtime 使用该 schema。 |

## 4. 未实现

| 缺口 | 文档来源 | 影响范围 | 建议任务 |
| --- | --- | --- | --- |
| PostgreSQL repository 的 legacy document 方法仍返回 `ErrConflict` / `ErrNotFound` | `services/file/internal/repository/postgres.go` | API / DB | 删除兼容业务方法或显式迁移到 owner service。 |
| MinIO server / mc 本地初始化尚未提供 | `docs/architecture/technology-decisions.md`、`docs/services/file/README.md` | Object storage / deploy | 若后续需要 Compose smoke，固定 MinIO server 和 `mc` 镜像版本并补 bucket 初始化。 |
| 对象清理 worker / asynq 未实现 | `docs/services/file/README.md`、`docs/services/file/docs/data-models.md` | worker / Redis | 按 `file:object:purge` 增加清理任务。 |
| `sqlc` 生成代码未落地 | `docs/architecture/technology-decisions.md` | DB / CI | 当前 `queries/file_objects.sql` 与 hand-written SQL 保持对齐；后续补 sqlc CLI 版本和生成产物。 |

## 5. 文档与实现出入

| 出入点 | 文档要求 | 当前实现 | 风险 | 建议处理 |
| --- | --- | --- | --- | --- |
| 生产对象存储 | 文档要求 File Service 封装 MinIO | 已有 MinIO adapter；MinIO server/mc Compose 尚未落地 | 可通过既有 MinIO/S3 兼容端点运行；本地一键 MinIO smoke 仍缺失 | 后续补固定镜像和 bucket 初始化。 |
| 服务 OpenAPI 与代码路由 | `services/file/api/openapi.yaml` 只声明 `/internal/v1/files/**` | 代码还暴露 knowledge-document 兼容路由 | 调用方可能继续依赖错误 owner 边界 | 保留短期兼容但在 implementation 中登记退出条件；不要扩展兼容路由。 |
| 配置说明 | 早期实现说明列出 `FILE_DATABASE_URL`、MinIO、Redis 等 | runtime 已解析 `FILE_DATABASE_URL`、service token 和 MinIO 配置；Redis/异步清理仍未落地 | 部署若只配置 Redis 仍不会启用异步清理 | README 和 implementation 只把已接入项写成 runtime 能力，未落地项放在缺口。 |
| file reference 边界 | File 不保存业务字段 | 兼容 document 路由仍保存 `knowledgeBaseId` 和 tags | 与 owner service 边界冲突 | 迁移 knowledge 上传链路到 `/internal/v1/files`，逐步删除兼容业务字段。 |

## 6. MVP / mock / memory backend / 占位

| 项目 | 当前用途 | 退出条件 | 关联任务 |
| --- | --- | --- | --- |
| memory repository | handler tests 和无 DB 本地运行 | 部署/真实联调统一配置 `FILE_DATABASE_URL` 后，memory 仅保留测试和早期本地用途 | PostgreSQL runtime 已接入；仍需部署联调 |
| memory object store | 单元测试和无文件系统本地运行 | local/MinIO smoke 成为默认验证路径 | MinIO adapter / smoke 任务 |
| local object store | 本地持久化 smoke | MinIO adapter 可在 Compose/部署中使用 | MinIO adapter / smoke 任务 |
| MinIO object store | 持久对象存储 adapter，可接已有 MinIO/S3 兼容端点 | 固定 MinIO server/mc Compose 或部署脚本用于一键 smoke | MinIO server/mc smoke 任务 |
| knowledge-document 兼容路由 | 兼容早期 knowledge/file 联调 | Knowledge 只调用 `/internal/v1/files/**` 且无调用方依赖旧路由 | 兼容路由退出任务 |

## 7. 运行与配置

| 项目 | 当前状态 | 缺口 |
| --- | --- | --- |
| 启动命令 | `cd services/file && go run ./cmd/server` | 无 Dockerfile / Compose。 |
| 环境变量 | `FILE_HTTP_ADDR`、`FILE_MAX_UPLOAD_BYTES`、`FILE_STORAGE_BACKEND`、`FILE_LOCAL_STORAGE_DIR`、`FILE_MINIO_ENDPOINT`、`FILE_MINIO_ACCESS_KEY`、`FILE_MINIO_SECRET_KEY`、`FILE_MINIO_BUCKET`、`FILE_MINIO_USE_SSL`、`FILE_MINIO_REGION`、`FILE_MINIO_TIMEOUT`、`FILE_DATABASE_URL`、`FILE_INTERNAL_SERVICE_TOKEN`、`INTERNAL_SERVICE_TOKEN`、`FILE_SHUTDOWN_TIMEOUT` | Redis 尚未被 runtime 使用。 |
| PostgreSQL / migration | `migrations/0001_create_file_objects.sql`、`sqlc.yaml`、`queries/file_objects.sql`、runtime repository | sqlc 生成代码未落地；真实 PostgreSQL smoke 需要本地 DB。 |
| Redis / queue | 未使用 | 清理 worker 未实现。 |
| Object storage / vector store / AI provider | memory/local/MinIO object store | MinIO server/mc 本地初始化尚未提供；File 不涉及 vector/AI provider。 |

## 8. 测试与验证

| 验证项 | 命令或步骤 | 当前结果 | 缺口 |
| --- | --- | --- | --- |
| 单元测试 | `cd services/file && go test ./...` | pass（2026-06-30，本次执行） | 不覆盖真实 MinIO server；env-gated DB smoke 默认跳过。 |
| 集成测试 | `FILE_TEST_DATABASE_URL=... go test ./internal/repository` | available / not run by default | 需要真实 DB；测试会创建独立 schema、应用 migration 并验证 repository metadata lifecycle。 |
| 契约测试 | handler tests against `/internal/v1/files/**` | partial | 未从 OpenAPI 自动校验。 |
| 手工 smoke | `FILE_STORAGE_BACKEND=local go run ./cmd/server` 后上传/读取/删除 | pass（本次执行：healthz、上传、读取、删除、删除后 404） | 仍需补脚本化 smoke 或 README smoke。 |

## 9. 建议任务

| 任务 | 类型 | 优先级 | 依据 | 说明 |
| --- | --- | --- | --- | --- |
| 补 MinIO server/mc 本地 smoke | 新任务 | P1 | MinIO adapter 已落地，但缺一键本地对象存储依赖 | 固定 MinIO server 和 `mc` 镜像版本，补 bucket 初始化和 smoke 命令。 |
| 清理 knowledge-document 兼容路由 | 修改既有任务 | P1 | owner service 边界 | 等 knowledge 已完全使用 `/internal/v1/files/**` 后删除或隐藏旧路由。 |
| 将 PostgreSQL smoke 纳入稳定 CI 或 runbook | 新任务 | P1 | env-gated 测试已存在但默认跳过 | 在服务级 Compose 或 CI DB 可用后固定执行方式。 |
| 增加 File 契约/集成测试 | 新任务 | P1 | 当前测试未自动校验 OpenAPI/MinIO | 覆盖 OpenAPI、MinIO 错误映射和跨服务 smoke。 |

## 10. 最近检查记录

| 日期 | 检查人/工具 | 代码基准 | 结论 |
| --- | --- | --- | --- |
| 2026-06-29 | Codex goal | `eddf917` + working tree | File 已有可用基础文件 API 和本地存储雏形，但生产持久化、MinIO、service token 和兼容路由退出仍未完成。 |
| 2026-06-30 | Codex task #154 | `upstream/develop` + `L1nggTeam/feat/file-minio-storage` | File 已接入官方 MinIO Go SDK adapter 和 runtime 配置；metadata runtime、异步清理 worker、service token 和 MinIO Compose smoke 仍未完成。 |
| 2026-06-30 | Codex goal #235 | working tree | PostgreSQL metadata runtime、`X-Service-Token` 校验、readyz backend/dependency 状态和 env-gated repository smoke 已补齐；异步清理、MinIO Compose smoke 和兼容路由退出仍未完成。 |
