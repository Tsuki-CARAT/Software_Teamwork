# 同步待确认决策到需求与接口文档

## Goal

把 GitHub Discussion #48 中已经明确的产品与接口决策同步到现有需求分析、待确认决策清单、三份 API 契约和 OpenAPI 草稿中，减少“待确认”表述，让后续实现能直接按定稿边界推进。

## What I already know

- Discussion #48 明确了主要决策：
  - D1 认证方式：Bearer Token/JWT。
  - D2 API 前缀：`/api/v1/...`。
  - D3 异步任务机制：倾向 Redis 队列 + PostgreSQL 持久化状态。
  - D4 会话历史：服务端 PostgreSQL 为权威数据源，前端只缓存 `sessionId` 等恢复信息。
  - D5 报告支撑材料：独立资源，复用 `file` service；`file` service 是后端与 MinIO/文件能力的中间件。
  - D6 OCR/解析：首期用外部服务跑通 pipeline，后续持续迭代。
  - D7 模型供应商：抽象为 AI gateway，服务保证 OpenAI-compatible API，各组先单供应商跑通，通过 config 修改 `baseURL` 和 `apiKey`。
  - D8 数据分析：统计指标本期做；智能问答里的 Excel/表格数据分析意图本期不做，后续由负责同学调研后与组长定设计。
  - D9 数据权限：角色级 RBAC 即可。
  - D10 审计日志：当前暂时忽略，后续可能单独服务，先保证调试可用。
- 用户追加要求：其余未定细节由我先给出首期默认决策，不再保留为实现阻塞项。
- 本轮新增默认决策：
  - OCR/解析首期使用外部 HTTP 解析服务，通过 `parser.baseUrl`、`apiKey`、`timeoutSeconds`、`maxConcurrency` 配置。
  - Redis 只作为队列和短期协调；PostgreSQL job 为权威状态，自动重试最多 3 次，保留最近 10 次尝试摘要。
  - Qdrant 使用版本化 collection；embedding 维度或模型族变化时创建新 collection 并后台重建。
  - MinIO bucket 首期拆为 `source-files`、`templates`、`generated-reports` 三类。
  - 管理员和超级管理员可按角色级 RBAC 查看、软删除全站会话和报告。
  - 报告大纲首期以数据库模板结构配置为权威；`mode=ai` 保留但返回不支持。
  - DOCX 导出首期必须有默认 `styleProfile`，编号只验收 `global`。
- 当前已有文档：
  - `docs/需求分析/整体需求分析.md`
  - `docs/需求分析/待确认决策清单.md`
  - `docs/接口契约/知识管理-api契约.md`
  - `docs/接口契约/智能问答-api契约.md`
  - `docs/接口契约/报告生成-api契约.md`
  - `docs/接口契约/openapi/*.yaml`
- OpenAPI YAML 已经使用 `/api/v1` server 和 JWT bearer security scheme，但描述字段和 Markdown 契约需要同步收敛。

## Requirements

- 更新 `待确认决策清单.md`，把 Discussion #48 已确认项改为“已确认/暂缓/后续调研”，并保留对应来源链接。
- 更新 `待确认决策清单.md`，把其余未定项改为首期默认决策，并说明后续只做能力增强。
- 更新 `整体需求分析.md` 中角色权限、基础设施、核心对象、流程、优先级、待确认问题等仍然含旧决策的段落。
- 更新三份 API 契约：
  - 认证统一为 Bearer Token/JWT。
  - API 前缀明确为 `/api/v1`，不再写成待确认。
  - 异步任务描述改为 Redis 队列 + PostgreSQL 持久化状态。
  - 模型调用改为通过 AI gateway / OpenAI-compatible API。
  - 数据权限改为角色级 RBAC。
  - 审计日志改为暂缓，不作为首期强制要求。
  - 报告支撑材料归属改为独立资源，复用 `file` service，必要时复用 `knowledge` 检索。
- 同步 OpenAPI YAML 中认证、模型配置、异步任务和权限相关描述，保持和 Markdown 契约一致。
- 同步 OpenAPI YAML 中 job 重试摘要、parser 配置、报告导出配置和不支持能力的描述。
- 不实现后端、前端、Swagger UI 运行时代码。

## Acceptance Criteria

- [ ] 文档不再把 D1、D2、D4、D5、D6、D7、D9、D10 写成未决项。
- [ ] D3 明确首期采用 Redis 队列 + PostgreSQL 持久化状态，且业务真相仍在 PostgreSQL。
- [ ] D8 明确统计指标本期做，智能问答“数据分析意图”本期不做。
- [ ] 三份 API 契约认证均为 Bearer Token/JWT，SSE 与普通 JSON 接口使用同一套 Bearer 鉴权。
- [ ] 三份 API 契约和 OpenAPI YAML 的模型调用描述统一指向 AI gateway / OpenAI-compatible API。
- [ ] 其余未定细节均有首期默认决策，不再出现“由实现任务决定/待确认”的阻塞表述。
- [ ] OpenAPI YAML 覆盖 parser 配置、job 重试摘要、QA `unsupported_intent`、报告 `unsupported_mode` 和导出默认配置说明。
- [ ] Markdown 代码块、相对链接、OpenAPI YAML parse 和 OpenAPI validator 检查通过。

## Out of Scope

- 不创建真实 `ai-gateway` 服务。
- 不改后端实现、数据库 migration 或前端页面。
- 不新增 Swagger UI 运行时。
- 不提交或归档，除非用户后续要求。

## Technical Notes

- Discussion: `https://github.com/Sakayori-Iroha-168/Software_Teamwork/discussions/48`
- Relevant comments:
  - D1/D2/D4/D8/D10 QA 对齐：`#discussioncomment-17459396`
  - D7 AI gateway：`#discussioncomment-17459473`
  - D6 OCR 分期：`#discussioncomment-17459492`
  - D5 支撑材料和 file service：`#discussioncomment-17459535`
  - D9 RBAC：`#discussioncomment-17459574`
