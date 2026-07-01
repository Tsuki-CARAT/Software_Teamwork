# File 模块测试报告

日期：2026-07-01
范围：`services/file`、File Service 所依赖的 PostgreSQL / MinIO 本地工作区，以及验证 File 能否服务 `knowledge`、`document` 等 owner service 的相关边界。

## 1. 测试目标

本轮测试不只验证 file 模块能否编译，而是验证它是否能作为系统里的基础文件能力可靠服务其他模块。

核心问题：

- File 是否按需求/架构文档只负责基础 file object、元数据、对象存储协调和内容流。
- File 是否不直接拥有前端公开 API，不保存 knowledge/document 的业务字段，不泄露 bucket、object key、内部 URL、access key、secret key。
- `/internal/v1/files/**` 是否能覆盖创建、读取元数据、读取原始内容、删除和删除后不可读。
- PostgreSQL metadata、MinIO object store、service token、readyz 和默认 memory/local adapter 是否与实现说明一致。
- 现有测试是否足以证明 File 可以服务系统，哪些跨服务 smoke 仍需后续补齐。

## 2. 测试工作区

| 工作区 | 用途 | 当前使用方式 |
| --- | --- | --- |
| `services/file` | File 服务本地 Go module，包含代码、OpenAPI、migration、unit/integration tests。 | 执行 `go test ./...`、`go build ./cmd/server`、env-gated tests。 |
| `deploy/docker-compose.yml` + `deploy/.env.example` | 根级本地依赖基线，提供 PostgreSQL、MinIO、`minio-init` bucket 初始化和 file migration job。 | 用于 `docker compose ... config` 和 File PostgreSQL + MinIO smoke 依赖。 |
| `docs/services/file/**` | File 需求、内部契约、实现状态和数据模型的权威文档入口。 | 分类提取测试要求和预期行为。 |
| `.trellis/spec/backend/**`、`docs/testing/strategy.md` | 后端服务测试、错误、数据库、API 和质量规范。 | 确定必跑检查、env-gated 风险和报告口径。 |

## 3. 依据文档和实现来源

| 分类 | 文件 | 本轮使用方式 |
| --- | --- | --- |
| 需求 / 架构 | `docs/architecture/service-boundaries.md` | 定义 File 职责：基础文件上传/内容 API、原始对象、对象存储协调、最小 file 元数据生命周期；不得负责知识库、报告业务状态。 |
| 需求 / 技术基线 | `docs/architecture/technology-decisions.md` | 确认 Go 1.25、PostgreSQL 16、`pgx/v5@v5.9.2`、goose、MinIO SDK、MinIO server/mc tag、Go testing。 |
| 测试策略 | `docs/testing/strategy.md` | File 应跑 `cd services/file && go test ./...`、`go build ./cmd/server`；env-gated integration tests 需区分默认跳过和真实依赖结果。 |
| File 预期 | `docs/services/file/README.md` | 定义职责边界、内部 `/internal/v1/files/**` 目标契约、权限上下文、服务 token、错误码、安全和日志要求。 |
| File 实现说明 | `docs/services/file/docs/implementation.md` | 当前状态为 partial：Go service、base file API、memory/local/MinIO object store、PostgreSQL metadata runtime、service-token 校验已存在；异步清理 worker、兼容路由退出、sqlc 生成仍缺。 |
| File 数据模型 | `docs/services/file/docs/data-models.md` | 定义 `file_objects` 字段、状态、禁止保存业务字段、对象存储内部字段不得对外返回。 |
| File OpenAPI | `docs/services/file/api/internal.openapi.yaml`、`services/file/api/openapi.yaml` | 机器可读内部契约；公开前端契约不在 File 直接暴露。 |
| 本地联调 | `docs/runbooks/local-integration.md`、`deploy/docker-compose.yml` | File PostgreSQL + MinIO smoke 的运行步骤和限制。 |
| 实现代码 | `services/file/**` | 核对 handler、service、repository、storage adapters、migration、测试覆盖和 env-gated 条件。 |

## 4. 测试分类设计

| 分类 | 目的 | 代表测试 |
| --- | --- | --- |
| 文档 / 契约一致性 | 确认 File 只声明内部基础文件 API，公开能力通过 gateway/owner service 复用，不直接暴露前端 `/api/v1/files/**`。 | OpenAPI 路径审计、README/implementation/data-models 对照。 |
| 服务单元 / handler 测试 | 验证 envelope、request id、multipart、checksum、上传大小、service token、readyz、内容流和删除后不可读。 | `cd services/file && go test ./...`。 |
| 构建测试 | 确认 File 作为独立 Go module 可构建 server。 | `cd services/file && go build ./cmd/server`。 |
| 配置 / adapter 测试 | 验证 memory/local/MinIO storage adapter、MinIO 错误脱敏、local path traversal、防泄露规则和配置校验。 | `go test` 中的 `internal/config`、`internal/platform/storage` tests。 |
| PostgreSQL repository smoke | 验证 migration 后 repository metadata lifecycle 行为。 | `FILE_TEST_DATABASE_URL=... go test ./internal/repository -count=1 -v`。 |
| PostgreSQL + MinIO 联合 smoke | 验证真实依赖下 File 可上传、写 metadata、写 object、读取内容、删除并清理对象。 | `FILE_MINIO_POSTGRES_SMOKE=1 ... go test ./internal/integration -run TestFileMinIOPostgresSmoke -count=1 -v`。 |
| Compose 配置测试 | 验证本地依赖工作区能解析，PostgreSQL/MinIO/migration/minio-init 配置未明显损坏。 | `docker compose -f deploy/docker-compose.yml --env-file deploy/.env.example config --quiet`。 |
| 安全负向测试 | 验证错误响应不泄露 token、object key、bucket、MinIO endpoint、access key、secret key 或 DB 细节。 | handler/storage/integration tests + 报告人工审计。 |
| 系统服务能力评估 | 判断 File 是否足以服务 Knowledge/Document，记录跨服务 smoke 缺口。 | 文档对照 + File 联合 smoke + Knowledge/Document fileclient 测试现状。 |

## 5. 计划用例矩阵

| ID | 分类 | 用例 | 预期 | 结果 |
| --- | --- | --- | --- | --- |
| FILE-001 | 文档 / 契约 | File 内部 OpenAPI 路径审计 | 只声明 `/healthz`、`/readyz`、`/internal/v1/files/**`；不声明前端公开 `/api/v1/files/**`。 | pass：`docs/services/file/api/internal.openapi.yaml` 和 `services/file/api/openapi.yaml` 路径一致；`docs/services/file/api/public.openapi.yaml` 无 paths。 |
| FILE-002 | 文档 / 边界 | README / data-model / implementation 边界审计 | 明确 File 不保存 `knowledgeBaseId`、`reportId`、`templateId`、`materialId`、ACL 等业务字段；兼容 document 路由只作为短期兼容。 | pass with risk：File 文档一致；另见第 8 节，Knowledge 旧 API contract 仍有 bucket 拆分口径需要跟进。 |
| FILE-003 | 单元测试 | File 全包测试 | `go test ./...` 通过；默认跳过 env-gated PostgreSQL 和 MinIO smoke。 | pass：非缓存 `go test ./... -count=1` 全包通过。 |
| FILE-004 | 构建测试 | File server build | `go build ./cmd/server` 成功。 | pass；生成的 `services/file/server` 构建产物已清理。 |
| FILE-005 | Compose 配置 | 根级本地依赖配置解析 | `docker compose ... config --quiet` 成功。 | pass。 |
| FILE-006 | PostgreSQL repository | Env-gated repository smoke | 有 PostgreSQL 时通过；无依赖时记录 skipped/blocked 原因。 | pass：真实 PostgreSQL 依赖下 `TestPostgresRepositoryFileObjectSmoke` 通过。 |
| FILE-007 | PostgreSQL + MinIO | Env-gated 联合 smoke | 有 PostgreSQL + MinIO + bucket 时通过上传/读取/删除/清理；无依赖时记录阻塞和残余风险。 | pass：真实 PostgreSQL + MinIO 依赖下 `TestFileMinIOPostgresSmoke` 通过。 |
| FILE-008 | Migration | File goose migration apply | 空库 apply 成功，或由 compose migration / env-gated smoke 间接验证；如果未跑真实 DB，记录风险。 | pass：`migrate-file` 日志显示 `0001_create_file_objects.sql` applied，goose migrated to version 1。 |
| FILE-009 | 安全 / 泄露 | 响应字段和错误脱敏审计 | JSON 响应不含 `bucket`、`objectKey`、`files/`、`minio`、`accessKey`、`secretKey`、service token。 | pass：handler/storage/integration tests 均包含敏感字段断言；Knowledge/Document fileclient 下游错误脱敏测试通过。 |
| FILE-010 | 内容流 | 原始内容读取行为 | 成功时 raw bytes，不包 JSON；返回 `Content-Type`、安全 `Content-Disposition`、`Content-Length` 和 `X-Request-Id`。 | pass：handler test 与真实 smoke 均覆盖 content read；Knowledge/Document fileclient content tests 通过。 |
| FILE-011 | 删除生命周期 | 删除后 metadata/content 不可读 | DELETE 后读取 metadata/content 都返回 `404 not_found`；真实 MinIO smoke 还应验证对象不可读。 | pass：unit test 和真实 MinIO smoke 均覆盖；真实 smoke 还验证 MinIO object delete 后不可读。 |
| FILE-012 | 系统适配 | Owner service 复用能力评估 | File base API 可被 Knowledge/Document 内部 client 复用；跨服务一键 E2E 仍作为缺口登记。 | pass with gap：Knowledge/Document fileclient tests 通过；Gateway -> owner service -> File 真实 E2E 仍缺。 |
| FILE-013 | 收尾检查 | `git diff --check` | 无 whitespace error。 | pass。 |

## 6. 命令执行记录

> 本节随测试执行实时更新。

| 时间 | ID | 命令 | 结果 | 证据 / 备注 |
| --- | --- | --- | --- | --- |
| 2026-07-01 02:10 +0800 | FILE-003 | `cd services/file && go test ./...` | pass | 初次执行通过，部分包来自 test cache；随后用 `-count=1` 重跑。 |
| 2026-07-01 02:14 +0800 | FILE-003 | `cd services/file && go test ./... -count=1` | pass | `cmd/server` no test files；`internal/config` 0.522s、`internal/http` 0.850s、`internal/integration` 1.149s、`internal/platform/storage` 1.595s、`internal/repository` 2.053s、`internal/service` 2.431s。 |
| 2026-07-01 02:10 +0800 | FILE-004 | `cd services/file && go build ./cmd/server` | pass | 构建成功；生成的 `services/file/server` 本轮已删除，未保留构建产物。 |
| 2026-07-01 02:10 +0800 | FILE-005 | `docker compose -f deploy/docker-compose.yml --env-file deploy/.env.example config --quiet` | pass | 无输出，退出码 0。 |
| 2026-07-01 02:11 +0800 | FILE-007 / FILE-008 | `cd deploy && docker compose --env-file .env.example up -d postgres minio minio-init migrate-file` | pass | `postgres`、`minio` healthy；`migrate-file` 执行 `0001_create_file_objects.sql`；`minio-init` 创建 `software-teamwork-local` bucket。 |
| 2026-07-01 02:12 +0800 | FILE-006 | `cd services/file && FILE_TEST_DATABASE_URL='postgres://file_app:file_app_dev@localhost:5432/file_system?sslmode=disable' go test ./internal/repository -count=1 -v` | pass | `=== RUN TestPostgresRepositoryFileObjectSmoke`，PASS，包耗时 0.595s。 |
| 2026-07-01 02:12 +0800 | FILE-007 | `cd services/file && FILE_MINIO_POSTGRES_SMOKE=1 FILE_TEST_DATABASE_URL='postgres://file_app:file_app_dev@localhost:5432/file_system?sslmode=disable' FILE_MINIO_ENDPOINT='localhost:9000' FILE_MINIO_ACCESS_KEY='minio_local_demo' FILE_MINIO_SECRET_KEY='minio-local-demo-password' FILE_MINIO_BUCKET='software-teamwork-local' go test ./internal/integration -run TestFileMinIOPostgresSmoke -count=1 -v` | pass | `=== RUN TestFileMinIOPostgresSmoke`，PASS，包耗时 1.000s。 |
| 2026-07-01 02:13 +0800 | FILE-001 | Python YAML path audit + `cmp -s docs/services/file/api/internal.openapi.yaml services/file/api/openapi.yaml` | pass | 两个 internal OpenAPI 副本路径相同且文件内容一致；File public OpenAPI 没有 paths。 |
| 2026-07-01 02:13 +0800 | FILE-012 | `cd services/knowledge && go test ./internal/platform/fileclient -count=1 -v` | pass | 覆盖 multipart create、context/service-token headers、content read、redirect 防护和下游错误脱敏。 |
| 2026-07-01 02:13 +0800 | FILE-012 | `cd services/document && go test ./internal/platform/fileclient -count=1 -v` | pass | 覆盖 multipart create、service-token headers、delete cleanup、content read 和 missing file 映射。 |
| 2026-07-01 02:15 +0800 | FILE-013 | `git diff --check` | pass | 最终复跑无输出，退出码 0。 |

## 7. 当前结果摘要

本轮 File 模块测试结论：File 自身基础能力可以服务系统内 owner service 的文件对象存储/读取场景。

已被当前证据覆盖的能力：

- `services/file` 独立 Go module 可测试、可构建。
- 内部 `/internal/v1/files/**` 能创建基础 file object、返回安全 metadata、读取 raw content、删除并在删除后隐藏 metadata/content。
- PostgreSQL metadata repository 和 MinIO object store 在真实本地依赖下通过联合 smoke。
- `migrate-file` 能把 `file_objects` migration 应用到本地 PostgreSQL。
- `X-Service-Token`、caller context、request id、multipart checksum、大小限制、storage adapter 错误脱敏和内容响应头均有自动化覆盖。
- Knowledge 和 Document 的 File client 已按 `/internal/v1/files/**` 调用，并有 header 传递、内容流和下游错误脱敏测试。

仍不能由本轮证明的能力：

- 统一 Gateway -> Knowledge/Document -> File 的真实跨服务 E2E 还没有固定脚本或 CI job。
- 异步对象清理 worker 尚未实现，本轮只能证明当前同步删除路径和 MinIO object 删除。
- 兼容 knowledge-document 路由退出尚未完成，本轮只确认其被登记为短期兼容而非目标扩展面。

## 8. 初步风险与观察

- File 文档和实现都明确当前状态是 `partial`，不是完整生产文件平台；异步对象清理 worker、兼容 knowledge-document 路由退出和 sqlc 生成代码仍未落地。
- 当前 File 自身已有 PostgreSQL + MinIO env-gated 联合 smoke，但普通 `go test ./...` 默认跳过真实依赖，不能把单元测试通过解读为真实对象存储链路已持续验证。
- `FILE_DATABASE_URL` 配置后 `/internal/v1/files/**` 需要 `X-Service-Token`；如果 owner service 或 worker 直连 File，必须配置并传递相同 token。
- 兼容 document routes 仍使用 knowledge-document 形态并保存 `knowledgeBaseId` / tags；这与最终 owner service 边界不一致，但当前实现说明已登记为短期兼容。
- 当前仓库测试策略仍说明缺少统一后端跨服务 E2E smoke；File 自身 smoke 不能证明 Gateway -> Knowledge/Document -> File 的完整链路。
- `docs/services/knowledge/docs/api-contract.md` 的“存储与数据归属”仍写到 bucket 首期拆为 `source-files`、`templates`、`generated-reports`。当前 File 文档和 runtime 口径是 File 内部封装 bucket/object key，根级本地 Compose 使用单 bucket `software-teamwork-local`；该旧表述容易让 owner service 误以为可以依赖 bucket 分类，建议后续文档清理。

## 9. 后续可追加测试想法

- 把 File PostgreSQL + MinIO smoke 固化到可选 CI job 或 runbook 脚本，避免每次手动拼环境变量。
- 增加 OpenAPI schema 级 contract test，自动比对 `docs/services/file/api/internal.openapi.yaml` 与 `services/file/api/openapi.yaml`。
- 增加 Knowledge -> File content handoff 和 Document -> File report file content 的跨服务 smoke，证明 owner service 不泄露 file ID、bucket 或 object key。
- 增加 service-token 轮换/缺失配置 smoke，覆盖 worker 直连 File 的失败形态。
- 增加异步清理 worker 后的失败重试测试：对象删除失败时 metadata 状态、`last_error_*`、attempt 计数和后续重试。
- 清理 Knowledge 历史 API contract 中关于 bucket 拆分的旧说法，统一到 File Service 内部封装 bucket/object key、owner service 只保存不透明 `file_ref` 的口径。
