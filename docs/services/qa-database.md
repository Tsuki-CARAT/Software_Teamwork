# QA 模块数据库说明

本文档说明 `qa` 模块当前设计的 PostgreSQL 数据模型，用于配合 [`QA 服务接口文档`](qa.md) 理解会话、消息、流式回答、引用、配置和检索测试等数据如何落库。

来源设计位于：

```text
D:\ACADEMIC\By_Course\3.大三下学期\软件项目综合实践\0628\qa-system-design
```

该目录当前包含本地 PostgreSQL Docker Compose、初始化 SQL、开发种子数据和 ER 图。本文档只把该设计纳入项目文档索引，不移动或修改外部设计文件。

## 部署概览

本地数据库使用 PostgreSQL 16：

| 项 | 值 |
| --- | --- |
| Docker service | `postgres` |
| Container | `qa-system-postgres` |
| Image | `postgres:16-alpine` |
| Database | `qa_system` |
| User | `qa_app` |
| Password | `qa_app_dev` |
| Port | `5432` |
| Time zone | `Asia/Shanghai` |
| Volume | `qa_pg_data` |

首次启动时，PostgreSQL 官方镜像会按文件名顺序执行 `init/` 下的 SQL：

| 文件 | 作用 |
| --- | --- |
| `01_extensions.sql` | 启用 `pgcrypto`，提供 `gen_random_uuid()`；设置时区。 |
| `02_schema.sql` | 创建全部业务表、约束和索引。 |
| `03_seed_dev.sql` | 写入本地开发用的默认 LLM 配置和 QA 配置。 |

启动命令：

```bash
docker compose up -d
```

连接命令：

```bash
docker exec -it qa-system-postgres psql -U qa_app -d qa_system
```

验证表结构：

```sql
SELECT table_name
FROM information_schema.tables
WHERE table_schema = 'public'
ORDER BY table_name;
```

## 设计原则

- 主键统一使用 `UUID`，默认值由 `gen_random_uuid()` 生成。
- 时间字段使用 `TIMESTAMPTZ`。
- 半结构化扩展字段使用 `JSONB`。
- 外部系统标识使用 `external_*` 字段，例如 `external_user_id`、`external_kb_id`、`external_doc_id` 和 `external_chunk_id`。
- 当前 schema 只做逻辑外键关联，不创建物理 FK 约束，便于早期多服务并行开发和跨服务 ID 对接。
- QA 数据库只保存 QA 自己拥有的业务状态和必要快照，不复制用户、知识库、文档原文件等主数据。

## 表分组

| 分组 | 表 | 说明 |
| --- | --- | --- |
| 运行配置与管理 | `qa_config_versions` | QA 检索参数配置版本。 |
| 运行配置与管理 | `qa_config_knowledge_bases` | 某个 QA 配置版本绑定的默认知识库快照。 |
| 运行配置与管理 | `llm_config_versions` | AI Gateway profile、模型、超时和生成参数版本。 |
| 运行配置与管理 | `admin_audit_logs` | 管理员配置变更审计日志。 |
| 对话与流式问答 | `conversations` | QA 会话。 |
| 对话与流式问答 | `messages` | 会话内消息。 |
| 对话与流式问答 | `response_runs` | 一次助手回答生成运行。 |
| 对话与流式问答 | `message_content_blocks` | 消息内容块，支持流式写入和多块内容。 |
| 对话与流式问答 | `response_process_steps` | 可向用户展示的处理步骤。 |
| 对话与流式问答 | `response_stream_events` | SSE 事件短期存储。 |
| 对话与流式问答 | `citations` | 回答引用快照。 |
| 检索体验测试 | `retrieval_test_runs` | 管理员检索测试运行。 |
| 检索体验测试 | `retrieval_test_results` | 检索测试结果快照。 |

## 核心关系

```text
conversations 1 ── N messages
conversations 1 ── N response_runs
messages      1 ── N message_content_blocks
messages      1 ── N citations
response_runs 1 ── N response_process_steps
response_runs 1 ── N response_stream_events
qa_config_versions  1 ── N qa_config_knowledge_bases
qa_config_versions  1 ── N response_runs
llm_config_versions 1 ── N response_runs
qa_config_versions  1 ── N retrieval_test_runs
retrieval_test_runs 1 ── N retrieval_test_results
```

外部边界：

| 字段 | 外部 owner | 说明 |
| --- | --- | --- |
| `external_user_id` | `auth` | 已认证用户 ID，由 gateway 注入 `X-User-Id` 后传给 QA。 |
| `external_kb_id` | `knowledge` | 知识库 ID，QA 只保存引用或配置快照。 |
| `external_doc_id` | `knowledge` / `file` | 文档 ID，原文件内容仍由 file-owned API 提供。 |
| `external_chunk_id` | `knowledge` | 文档切片 ID，向量索引和 chunk 主数据归 `knowledge`。 |

## 运行配置与管理

### qa_config_versions

保存 QA 检索配置版本。配置采用版本化设计，避免历史回答无法追溯当时使用的 Top K、阈值和重排序策略。

关键字段：

| 字段 | 说明 |
| --- | --- |
| `id` | 配置版本 ID。 |
| `version_no` | 递增版本号，唯一。 |
| `top_k` | 向量检索返回数量，默认 `5`。 |
| `similarity_threshold` | 相似度阈值，默认 `0.7000`。 |
| `use_rerank` | 是否启用重排序。 |
| `rerank_threshold` | 重排序分数阈值。 |
| `rerank_top_n` | 重排序后保留数量。 |
| `is_active` | 是否当前生效。 |
| `created_by_user_id` | 创建配置的外部用户 ID。 |

约束：

- `version_no` 唯一。
- 当前 schema 没有限制只能有一个 `is_active = true`，实现层需要保证激活配置时先停用旧版本。

### qa_config_knowledge_bases

保存某个 QA 配置版本绑定的知识库快照。

关键字段：

| 字段 | 说明 |
| --- | --- |
| `config_id` | QA 配置版本 ID。 |
| `external_kb_id` | 外部知识库 ID。 |
| `kb_type` | 知识库类型，例如术语库或技术监督库。 |
| `display_name_snapshot` | 创建配置时的知识库名称快照。 |
| `sort_order` | 展示或检索排序。 |

主键为 `(config_id, external_kb_id)`。

### llm_config_versions

保存 LLM 配置版本。Provider 密钥由 AI Gateway 管理；QA 只保存 profile 引用、模型名、超时和生成参数。

关键字段：

| 字段 | 说明 |
| --- | --- |
| `provider` | 首期固定为 `ai-gateway`，用于兼容旧配置视图。 |
| `profile_id` | AI Gateway 模型 profile ID。 |
| `api_url` | 废弃字段；首期不写入 provider 地址，模型入口由 AI Gateway profile 决定。 |
| `model_name` | 模型名称。 |
| `api_key_secret_ref` | 废弃字段；provider 密钥不归 QA 保存。 |
| `api_key_last4` | 废弃字段；provider 密钥不归 QA 展示。 |
| `timeout_seconds` | 请求超时时间，默认 `60`。 |
| `temperature` | 生成温度，默认 `0.70`。 |
| `max_tokens` | 最大输出 token，默认 `4096`。 |
| `is_active` | 是否当前生效。 |

### admin_audit_logs

保存管理员配置变更审计日志。

关键字段：

| 字段 | 说明 |
| --- | --- |
| `external_user_id` | 操作人用户 ID。 |
| `action` | 操作类型。 |
| `target_type` | 操作对象类型，例如 `qa_config` 或 `llm_config`。 |
| `target_id` | 操作对象 ID。 |
| `before_data` / `after_data` | 变更前后快照。 |
| `request_id` | gateway 请求追踪 ID。 |
| `ip_address` | 来源 IP。 |

## 对话与流式问答

### conversations

保存用户 QA 会话。

关键字段：

| 字段 | 说明 |
| --- | --- |
| `external_user_id` | 会话归属用户。 |
| `title` | 会话标题。 |
| `status` | `active` 或 `archived`。 |
| `deleted_at` | 软删除时间。 |

约束：

- `status IN ('active', 'archived')`。

接口映射：

- `POST /api/v1/qa-sessions`
- `GET /api/v1/qa-sessions`
- `GET /api/v1/qa-sessions/{sessionId}`
- `PATCH /api/v1/qa-sessions/{sessionId}`
- `DELETE /api/v1/qa-sessions/{sessionId}`

### messages

保存会话内消息的元数据。消息正文不直接放在该表，而是放在 `message_content_blocks`。

关键字段：

| 字段 | 说明 |
| --- | --- |
| `conversation_id` | 所属会话。 |
| `role` | `user`、`assistant` 或 `system`。 |
| `sequence_no` | 会话内顺序号。 |
| `status` | `streaming`、`completed`、`stopped` 或 `failed`。 |
| `model_name` | 生成助手消息所使用的模型。 |
| `error_code` / `error_message` | 消息失败原因。 |
| `completed_at` | 消息完成时间。 |

约束：

- `role IN ('user', 'assistant', 'system')`。
- `status IN ('streaming', 'completed', 'stopped', 'failed')`。
- `(conversation_id, sequence_no)` 唯一。

### response_runs

保存一次助手回答生成运行。一次用户消息通常对应一次 `response_run`，其中记录意图、路由、配置版本、token 使用量、延迟和结束状态。

关键字段：

| 字段 | 说明 |
| --- | --- |
| `conversation_id` | 所属会话。 |
| `user_message_id` | 触发本次回答的用户消息。 |
| `assistant_message_id` | 本次运行生成的助手消息。 |
| `qa_config_version_id` | 本次运行使用的 QA 配置版本。 |
| `llm_config_version_id` | 本次运行使用的 LLM 配置版本。 |
| `request_id` | gateway 请求追踪 ID。 |
| `intent_type` | 识别出的意图，例如 `knowledge_qa`。 |
| `route` | 路由分支，例如 RAG 或普通对话。 |
| `confidence` | 意图置信度。 |
| `status` | `running`、`completed`、`stopped` 或 `failed`。 |
| `stop_reason` | 停止原因。 |
| `retry_count` | 重试次数。 |
| `prompt_tokens` / `completion_tokens` / `reasoning_tokens` | token 使用量。 |
| `latency_ms` | 总耗时。 |

### message_content_blocks

保存消息内容块，支持流式写入、未来多模态或工具内容扩展。

关键字段：

| 字段 | 说明 |
| --- | --- |
| `message_id` | 所属消息。 |
| `block_order` | 消息内内容块顺序。 |
| `block_type` | 内容块类型，例如 `text`、`displayable_reasoning`。 |
| `content` | 文本内容。 |
| `status` | `streaming`、`completed` 或 `stopped`。 |
| `provider_block_id` | 上游模型供应商内容块 ID。 |
| `provider_metadata` | 供应商扩展元数据。 |

约束：

- `(message_id, block_order)` 唯一。
- 只能保存可展示内容，不应保存私有思维链、完整 prompt 或敏感工具参数。

### response_process_steps

保存可向前端展示的处理步骤，例如意图识别、检索、生成和校验。

关键字段：

| 字段 | 说明 |
| --- | --- |
| `response_run_id` | 所属回答运行。 |
| `step_order` | 步骤顺序。 |
| `step_type` | 步骤类型。 |
| `label` | 前端展示标签。 |
| `detail` | 可展示详情。 |
| `status` | 当前步骤状态。 |

约束：

- `(response_run_id, step_order)` 唯一。

### response_stream_events

短期保存 SSE 事件，用于断线恢复、调试或回放。该表不是长期消息内容主表，最终内容仍应写入 `messages` 和 `message_content_blocks`。

关键字段：

| 字段 | 说明 |
| --- | --- |
| `id` | 自增事件 ID。 |
| `response_run_id` | 所属回答运行。 |
| `event_seq` | 本次回答运行内的事件序号。 |
| `event_type` | `intent`、`step`、`token`、`citation`、`done` 或 `error`。 |
| `payload` | SSE payload。 |
| `expires_at` | 事件过期时间。 |

约束：

- `(response_run_id, event_seq)` 唯一。
- `event_type` 当前只允许 `intent`、`step`、`token`、`citation`、`done`、`error`。
- `heartbeat` 属于传输层保活事件，可不持久化。

### citations

保存回答引用快照。QA 保存的是回答生成时引用到的知识片段快照，知识库、文档和 chunk 主数据仍由 `knowledge` 或 `file` 服务拥有。

关键字段：

| 字段 | 说明 |
| --- | --- |
| `message_id` | 引用所属助手消息。 |
| `citation_no` | 回答中的引用序号。 |
| `char_start` / `char_end` | 引用在回答文本中的字符范围。 |
| `external_kb_id` | 外部知识库 ID。 |
| `external_doc_id` | 外部文档 ID。 |
| `external_chunk_id` | 外部 chunk ID。 |
| `doc_name` | 文档名称快照。 |
| `quote_text` | 引用片段文本。 |
| `context` | 引用上下文。 |
| `page_number` | 页码。 |
| `score` | 相关性分数。 |
| `metadata` | 其他引用元数据。 |

约束：

- `(message_id, citation_no)` 唯一。

## 检索体验测试

### retrieval_test_runs

保存管理员发起的一次检索体验测试运行。正式知识检索能力仍由 `knowledge` 服务拥有；QA 只保存测试运行和结果快照。

关键字段：

| 字段 | 说明 |
| --- | --- |
| `qa_config_version_id` | 使用的 QA 配置版本。 |
| `external_user_id` | 发起测试的用户 ID。 |
| `query` | 测试查询文本。 |
| `overrides` | 临时覆盖参数。 |
| `status` | `running`、`completed` 或 `failed`。 |
| `result_count` | 命中数量。 |
| `latency_ms` | 耗时。 |
| `error_message` | 失败原因。 |

### retrieval_test_results

保存一次检索测试的结果快照。

关键字段：

| 字段 | 说明 |
| --- | --- |
| `test_run_id` | 所属检索测试运行。 |
| `rank_no` | 排名。 |
| `external_kb_id` | 外部知识库 ID。 |
| `external_doc_id` | 外部文档 ID。 |
| `external_chunk_id` | 外部 chunk ID。 |
| `doc_name` | 文档名称快照。 |
| `text_snapshot` | 命中文本快照。 |
| `vector_score` | 向量相似度分数。 |
| `rerank_score` | 重排序分数。 |
| `metadata` | 页码、chunk index 等扩展信息。 |

约束：

- `(test_run_id, rank_no)` 唯一。

## 主要写入流程

### 创建会话

```text
gateway validates session -> injects X-User-Id
qa inserts conversations(external_user_id, title, status='active')
qa returns QASession
```

### 创建消息并流式回答

```text
qa inserts user message
qa inserts assistant message(status='streaming')
qa inserts response_runs(status='running')
qa writes response_process_steps and response_stream_events while streaming
qa appends text into message_content_blocks
qa writes citations when retrieval references are confirmed
qa marks response_runs and assistant message completed / failed / stopped
```

### 创建配置版本

```text
qa inserts qa_config_versions or llm_config_versions
qa writes admin_audit_logs with before_data and after_data
service layer ensures only one active version
future response_runs reference the selected version
```

### 检索体验测试

```text
qa inserts retrieval_test_runs(status='running')
qa calls knowledge retrieval using current config plus overrides
qa inserts retrieval_test_results snapshots
qa updates retrieval_test_runs(status, result_count, latency_ms)
```

## 索引

当前 schema 针对高频过滤和查询字段创建索引：

| 索引 | 用途 |
| --- | --- |
| `idx_conversations_external_user_id` | 按用户查询会话列表。 |
| `idx_conversations_created_at` | 按创建时间倒序查询会话。 |
| `idx_messages_conversation_id` | 查询会话内消息。 |
| `idx_messages_created_at` | 按创建时间查询消息。 |
| `idx_response_runs_conversation_id` | 查询会话下回答运行。 |
| `idx_response_runs_user_message_id` | 通过用户消息定位回答运行。 |
| `idx_response_runs_started_at` | 统计和排序回答运行。 |
| `idx_response_runs_request_id` | 通过 request id 排查问题。 |
| `idx_message_content_blocks_message_id` | 查询消息内容块。 |
| `idx_response_process_steps_run_id` | 查询回答处理步骤。 |
| `idx_response_stream_events_run_id` | 查询回答流式事件。 |
| `idx_response_stream_events_expires_at` | 清理过期事件。 |
| `idx_citations_message_id` | 查询消息引用。 |
| `idx_retrieval_test_runs_created_at` | 查询检索测试历史。 |
| `idx_retrieval_test_results_run_id` | 查询检索测试结果。 |
| `idx_admin_audit_logs_created_at` | 查询审计日志。 |
| `idx_admin_audit_logs_external_user_id` | 按用户查询审计日志。 |

## 开发种子数据

`03_seed_dev.sql` 写入两条本地开发数据：

| 表 | 数据 |
| --- | --- |
| `llm_config_versions` | `version_no = 1`，`provider = ai-gateway`，`profile_id = mp_chat_default`，`model_name = gpt-4o-mini`，`is_active = true`。 |
| `qa_config_versions` | `version_no = 1`，`top_k = 5`，`similarity_threshold = 0.7000`，`use_rerank = false`，`is_active = true`。 |

这些数据仅用于本地开发，不应作为生产配置。

## 迁移规则

PostgreSQL Docker 初始化脚本只会在数据目录为空时执行一次。已经被同学本地 volume 执行过的 `init/*.sql` 不应直接改写。

后续 schema 变更建议：

```text
migrations/20250628_add_xxx.sql
```

执行方式示例：

```bash
docker exec -i qa-system-postgres psql -U qa_app -d qa_system < migrations/20250628_add_xxx.sql
```

如果只是本地重建测试库，可以删除 volume 后重新初始化：

```bash
docker compose down -v
docker compose up -d
```

## 与接口文档的关系

- `conversations`、`messages`、`response_runs`、`message_content_blocks`、`response_process_steps`、`response_stream_events` 和 `citations` 支撑 [`QA 服务接口文档`](qa.md) 中的会话、消息、SSE 和引用接口。
- `qa_config_versions`、`qa_config_knowledge_bases` 和 `llm_config_versions` 支撑 QA 配置与 LLM 配置接口。
- `retrieval_test_runs` 和 `retrieval_test_results` 支撑管理员检索体验测试接口。
- `admin_audit_logs` 支撑配置变更审计。
- 当前 QA 接口仍是服务文档草案，尚未升级为 `docs/api/gateway.openapi.yaml` 的稳定公开契约。
