# Auth / Gateway 模块测试报告

日期：2026-07-01
范围：`services/auth`、`services/gateway`、以及验证这两个模块服务系统能力所需的相关链路。

## 1. 测试目标

本轮测试不是只看单个模块是否能编译，而是验证 Auth 与 Gateway 是否能可靠支撑系统入口、认证上下文、会话缓存、公开契约和下游服务代理。

核心问题：

- Auth 是否按文档提供用户、凭证、角色、权限、会话和 token hash 能力。
- Gateway 是否按文档只作为统一入口、会话缓存、响应归一化、request id 和下游代理边界。
- Auth + Gateway + Redis 是否能支持登录、当前用户读取、登出和认证上下文注入。
- 现有测试是否覆盖了文档声明的关键风险，哪些系统级 smoke 仍需补齐。

## 2. 依据文档和实现来源

| 类型 | 文件 | 本轮使用方式 |
| --- | --- | --- |
| 需求 / 架构 | `docs/architecture/service-boundaries.md` | 定义 Gateway/Auth 职责边界、禁止 Gateway 承担业务持久化或模型/对象存储细节。 |
| 需求 / 技术基线 | `docs/architecture/technology-decisions.md` | 确认 Go 1.25、PostgreSQL、Redis、opaque token、argon2id、Go testing 等基线。 |
| 测试策略 | `docs/testing/strategy.md` | 明确当前缺少后端跨服务 E2E smoke，并将 Auth -> Gateway -> Domain 作为目标。 |
| Auth 预期 | `docs/services/auth/README.md` | 定义用户、会话、token hash、Gateway Redis 协作和内部 API 行为。 |
| Auth 实现说明 | `docs/services/auth/docs/implementation.md` | 当前状态为 implemented；缺口包括 Gateway/Auth/Redis E2E smoke 和管理员初始化说明。 |
| Gateway 预期 | `docs/services/gateway/README.md` | 定义统一入口、会话缓存、公开 API、下游上下文注入和错误 envelope。 |
| Gateway 实现说明 | `docs/services/gateway/docs/implementation.md` | 当前状态为 partial；缺真实 Redis/downstream 集成验证。 |
| Gateway 契约 | `docs/services/gateway/api/public.openapi.yaml`、`docs/services/gateway/docs/active-api-owner-map.md` | 97 个 active operations、4 个 Auth 公开操作和 owner proxy 路由的机器/人工核对来源。 |
| 本地联调 | `deploy/docker-compose.yml`、`deploy/.env.example`、`deploy/seeds/*.sql` | 作为真实 PostgreSQL/Redis/Auth/Gateway/system smoke 的候选工作区。 |
| 实现代码 | `services/auth/**`、`services/gateway/**` | 核对 handler、proxy、session cache、readyz、auth client、service-token、测试覆盖。 |

## 3. 测试分类设计

| 分类 | 目的 | 代表测试 |
| --- | --- | --- |
| 文档 / 契约一致性 | 确认 active API、owner、REST 命名、响应 envelope 和 service boundary 未漂移。 | Gateway active API verifier、route matrix tests、OpenAPI/owner map 对齐。 |
| Auth 服务单元 / 包测试 | 验证用户、会话、密码、token hash、service-token、repository mapping 和错误归一化。 | `cd services/auth && go test ./...`。 |
| Gateway 服务单元 / 包测试 | 验证 auth proxy、Redis session cache 行为、request id、下游 header 注入、admin 鉴权、proxy 错误脱敏、binary/SSE proxy 和中间件。 | `cd services/gateway && go test ./...`。 |
| 构建测试 | 确认服务入口能独立 build，符合独立 Go module 约束。 | `go build ./cmd/server`。 |
| 配置 / Compose 测试 | 确认本地测试工作区能解析，服务依赖、端口和环境变量没有明显配置错误。 | `docker compose -f deploy/docker-compose.yml --env-file deploy/.env.example config --quiet`。 |
| 迁移 / 持久化测试 | 验证 Auth PostgreSQL schema 和 seed 可从空库 apply。 | Compose migration 或 goose apply。 |
| Auth + Gateway + Redis smoke | 验证真实登录、Gateway session cache、`/users/me`、登出和 session 失效。 | 通过 `deploy/docker-compose.yml` 或等价本地进程执行 HTTP smoke。 |
| Gateway 下游代理 smoke | 验证登录后的用户上下文能服务其他模块，不局限于 Auth/Gateway 内部。 | 访问 Knowledge/Document/QA 任一 authenticated active route，检查 status/envelope/header 语义。 |
| 安全负向测试 | 验证未认证、错误 token、错误 service-token、下游错误不会泄露 token、DB URL、object key 或 provider 原始错误。 | 单元测试 + smoke 中的 401/403/502 样例。 |

## 4. 计划用例矩阵

| ID | 分类 | 用例 | 预期 | 结果 |
| --- | --- | --- | --- | --- |
| AG-001 | Auth 单元 | Auth 全包测试 | 通过；覆盖密码、session、repository/http/config 等现有用例。 | pass |
| AG-002 | Auth 构建 | Auth server build | `go build ./cmd/server` 成功。 | pass |
| AG-003 | Gateway 单元 | Gateway 全包测试 | 通过；覆盖 route matrix、auth proxy、proxy headers、中间件等现有用例。 | pass |
| AG-004 | Gateway 构建 | Gateway server build | `go build ./cmd/server` 成功。 | pass |
| AG-005 | Gateway 契约 | active API verifier | 97 个 active operations 与 OpenAPI/owner 规则一致。 | pass |
| AG-006 | Compose 配置 | deploy compose config | 基础 profile 配置可解析。 | pass |
| AG-007 | Auth 迁移 | Auth migration apply / Compose migration | 空库迁移和 seed 可应用，或记录环境阻塞。 | pass |
| AG-008 | 登录 smoke | `POST /api/v1/sessions` | seeded admin 或新建用户可通过 Gateway 创建 session，响应只返回 opaque token，不暴露 Redis key/hash。 | pass |
| AG-009 | 当前用户 smoke | `GET /api/v1/users/me` | 携带 token 可返回用户摘要，roles/permissions 与 Auth 权威一致。 | pass |
| AG-010 | 下游代理 smoke | 认证访问一个 owner route | Gateway 注入用户上下文并返回 owner service 的稳定 envelope / 业务状态。 | pass with fake owner |
| AG-011 | 登出 smoke | `DELETE /api/v1/sessions/current` 后再访问 `/users/me` | 登出返回 204，旧 token 后续返回 401。 | pass |
| AG-012 | 负向 smoke | 无 token 或错误 token 访问受保护路由 | 返回统一 `401 unauthorized` error envelope。 | pass |
| AG-013 | 收尾检查 | `git diff --check` | 无 whitespace error。 | pass |

## 5. 命令执行记录

> 本节随测试执行实时更新。

| 时间 | ID | 命令 | 结果 | 证据 / 备注 |
| --- | --- | --- | --- | --- |
| 2026-07-01 02:04 CST | AG-001 | `cd services/auth && go test ./...` | pass | `cmd/server` no tests；`internal/config`、`internal/http`、`internal/repository`、`internal/service` 全部 ok。 |
| 2026-07-01 02:04 CST | AG-002 | `cd services/auth && go build ./cmd/server` | pass | 无输出；生成的本地二进制已清理。 |
| 2026-07-01 02:04 CST | AG-003 | `cd services/gateway && go test ./...` | pass | `internal/http` 0.539s；`internal/config`、`middleware`、`authclient` ok。 |
| 2026-07-01 02:04 CST | AG-004 | `cd services/gateway && go build ./cmd/server` | pass | 无输出；生成的本地二进制已清理。 |
| 2026-07-01 02:04 CST | AG-005 | `python3 -m unittest scripts.tests.test_verify_gateway_active_api` | pass | 8 tests ran in 0.024s, OK。 |
| 2026-07-01 02:04 CST | AG-005 | `python3 scripts/verify_gateway_active_api.py` | pass | `Gateway active API contract verification passed.` |
| 2026-07-01 02:04 CST | AG-006 | `docker compose -f deploy/docker-compose.yml --env-file deploy/.env.example config --quiet` | pass | 无输出，基础 profile 配置可解析。 |
| 2026-07-01 02:04 CST | AG-006 | `docker compose -f deploy/docker-compose.yml --env-file deploy/.env.example --profile ai config --quiet` | pass | 无输出，AI profile 配置可解析。 |
| 2026-07-01 02:05 CST | AG-006 | `docker compose ... --profile ai up -d --build` | blocked | 两次失败于 Docker Hub metadata/token 获取：`dial tcp 104.23.124.189:443: i/o timeout` / EOF。已改用本机源码 + Docker 基础设施 smoke。 |
| 2026-07-01 02:09 CST | AG-007 | `docker compose ... up -d --no-build postgres redis` | pass | `postgres:16-alpine`、`redis:7-alpine` 启动并 healthy；init SQL 创建 `auth_system` 等数据库。 |
| 2026-07-01 02:09 CST | AG-007 | `docker compose ... up --no-build migrate-auth` | pass | `0001_create_auth_core_tables.sql`、`0002_seed_auth_roles_permissions.sql` OK；goose migrated to version 2。 |
| 2026-07-01 02:10 CST | AG-007 | `AUTH_* go run ./cmd/server` + `curl /readyz` | pass | Auth 本机当前源码 `:18001` ready，PostgreSQL dependency ready。 |
| 2026-07-01 02:10 CST | AG-006 | `GATEWAY_* go run ./cmd/server` + `curl /healthz` / `/readyz` / metrics | pass | Gateway 本机当前源码 `:18080` health/ready 200；metrics port `:19091` 暴露 `gateway_http_requests_total`。 |
| 2026-07-01 02:11 CST | AG-012 | `GET /api/v1/users/me` without/with invalid bearer token | pass | 无 token 返回 `401 unauthorized` / `authentication required`；错误 token 返回 `401 unauthorized` / `invalid authentication`；requestId 保留。 |
| 2026-07-01 02:12 CST | AG-008/009/011 | Gateway create user -> `/users/me` -> logout -> `/users/me` | pass | 创建用户返回 standard role 和 `document:read,knowledge:read,qa:use,report:read`；token length 50；Redis key 为 `gateway:session:hmac-sha256:v1:*`；logout 204；旧 token 后续 401。 |
| 2026-07-01 02:12 CST | AG-008/012 | Gateway login + bad password | pass | `POST /api/v1/sessions` 成功返回 session；错误密码返回统一 `401 unauthorized`，未区分账号/密码。 |
| 2026-07-01 02:12 CST | AG-010 | Authenticated `GET /api/v1/knowledge-bases` while Knowledge offline | pass | 认证通过后返回稳定 `502 dependency_error`，message 为 `knowledge service is unavailable`，未泄露下游内部细节。 |
| 2026-07-01 02:13 CST | AG-010 | Fake Knowledge header capture via Gateway proxy | pass | Gateway 转发到 `/internal/v1/knowledge-bases`；捕获 `X-Caller-Service: gateway`、配置的 `X-Service-Token`、真实 `X-User-Id`、`standard` role 和权限；前端伪造 `X-User-*` 未透传。 |
| 2026-07-01 02:14 CST | AG-008 | Redis session value/TTL check | pass | 精确匹配测试 username；key prefix 为 `gateway:session:hmac-sha256:v1:`；TTL 86400；value 含 `accessTokenHash`，不含 `accessToken` 字段。 |
| 2026-07-01 02:17 CST | AG-010 | `docker compose ... up -d --no-build file parser knowledge` | blocked | 缺少 `software-teamwork-local-parser:latest`；重新构建 parser 又依赖 Docker Hub `python:3.12-slim` metadata，受前述网络超时阻塞。真实 Knowledge owner-service smoke 未完成。 |
| 2026-07-01 02:20 CST | AG-013 | `git diff --check` | pass | 无输出。 |
| 2026-07-01 02:20 CST | AG-013 | `rg -n '[ \t]+$' docs/tests/0701/auth-gateway-test-report.md .trellis/tasks/07-01-auth-gateway-test-audit` | pass | 无匹配；只检查本轮触碰文件，未改动并行存在的 `file-module-test-report.md`。 |

## 6. 当前结果摘要

结论：Auth 与 Gateway 的当前源码通过服务级测试、构建、Gateway active API 契约校验、Compose 配置校验，并通过了本机当前源码 + Docker PostgreSQL/Redis 的 Auth/Gateway/Redis smoke。核心登录、创建用户、当前用户读取、登出、Redis session cache、负向认证和 Gateway 下游 header 注入均符合预期。

仍未完成的范围：完整 `deploy/docker-compose.yml --profile ai up -d --build` 因 Docker Hub metadata/token 请求超时失败，无法在本轮用“重新构建全部服务镜像”的方式证明完整系统栈。真实 Knowledge owner-service smoke 也因本机缺少 cached parser image 且无法重新构建 parser 被阻塞。已改用本机当前源码运行 Auth/Gateway，并用 Docker 只承载 PostgreSQL、Redis 和 Auth migration；Gateway 下游代理能力通过 fake Knowledge header capture 证明。

## 7. 初步风险与观察

- Auth 与 Gateway 的实现文档都把 Gateway/Auth/Redis 真实端到端 smoke 标为缺口；本轮已补一条本机当前源码 smoke 证据。
- Gateway `authenticateRequest` 命中 Redis 后仍会通过 Auth client 刷新 session/user 权威信息；smoke 已启动真实 Auth 服务验证该路径。
- Gateway readyz 实现会检查 Redis、Auth ready 和 owner base URL 是否已配置，但不会主动探测 Knowledge/QA/Document/AI Gateway 是否在线。本轮在 owner 服务未启动时 `/readyz` 仍返回 200；这符合当前代码，但与“真实依赖 ready/smoke 未验证”的文档风险相吻合，后续如要让 readyz 表示完整系统可用，需要单独决策。
- `deploy/.env.example` 的 demo admin seed 未在本轮验证；本轮改用 Gateway `POST /api/v1/users` 创建唯一测试用户，避免依赖 seed 密码正确性。管理员初始化公开 smoke 仍是 Auth 文档里的待补项。
- 完整 Compose build 两次失败在 Docker Hub metadata/token 获取阶段，错误为 `dial tcp 104.23.124.189:443: i/o timeout` / EOF，不是项目测试或代码失败。
- 真实 Knowledge 链路的 no-build smoke 失败于缺失 `software-teamwork-local-parser:latest`；这说明后续要把跨服务 smoke 稳定化，不能依赖开发机刚好存在 cached image。

## 8. 后续可追加测试想法

- 把 Auth/Gateway/Redis smoke 固化成脚本或 Go integration test，并在 Docker 可用时纳入 CI optional job。
- 增加 Redis 中 session key TTL 和 value 脱敏校验，确保不存在原始 access token。
- 增加 Gateway 对 Auth authority refresh 的集成测试：禁用用户、撤销 session、角色权限变更后旧缓存如何失效。
- 为 owner proxy 增加真实下游 header 捕获 smoke，证明 `X-User-Id`、`X-User-Roles`、`X-User-Permissions`、`X-Service-Token` 不可由前端伪造。
- 增加 OpenAPI schema 级别响应字段校验，而不只校验 method/path/owner/operationId。
