# AI Gateway 默认 Profile Seed 与 Token Hash 运行手册

本文档说明如何在本地或 CI 环境中初始化 AI Gateway 的服务访问凭证和三类默认 Model Profile（chat、embedding、rerank），以满足 QA、Knowledge、Document 等下游服务的联调需求。

---

## 1. 生成服务令牌哈希（X-Service-Token）

AI Gateway 使用 SHA-256 哈希验证入站请求的 `X-Service-Token` 头，**不存储明文令牌**。

### 1.1 生成令牌

选择一个随机令牌字符串（生产环境请用密码管理器生成，≥32 位随机字符）：

```bash
# 示例：本地 / CI 使用固定测试令牌（仅非生产环境）
SERVICE_TOKEN="local-dev-service-token-$(openssl rand -hex 16)"
echo "token: $SERVICE_TOKEN"
```

### 1.2 计算哈希并写入配置

```bash
TOKEN_HASH="sha256:$(echo -n "$SERVICE_TOKEN" | sha256sum | awk '{print $1}')"
echo "config value: $TOKEN_HASH"
```

将 `$TOKEN_HASH` 写入 AI Gateway 的环境变量 `AI_GATEWAY_SERVICE_TOKEN_HASHES`（或对应的 `.env` / Kubernetes Secret）。支持逗号分隔多个哈希，用于轮换令牌。

> **注意**：`sha256sum` 的 `-n` 参数确保不附带换行符；如果使用 `openssl`：
> ```bash
> TOKEN_HASH="sha256:$(echo -n "$SERVICE_TOKEN" | openssl dgst -sha256 | awk '{print $2}')"
> ```

---

## 2. 创建默认 Model Profile

所有请求均需携带：

```
X-Service-Token: <明文令牌>
X-Caller-Service: gateway          # 管理接口使用 gateway
Content-Type: application/json
```

将 `AI_GATEWAY_BASE_URL` 设置为 AI Gateway 服务地址。默认监听端口为 `8086`（由 `AI_GATEWAY_HTTP_ADDR` 控制），本地示例：

```bash
AI_GATEWAY_BASE_URL="http://localhost:8086"
```

### 2.1 默认 Chat Profile

```bash
curl -s -X POST "$AI_GATEWAY_BASE_URL/internal/v1/model-profiles" \
  -H "X-Service-Token: $SERVICE_TOKEN" \
  -H "X-Caller-Service: gateway" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "default-chat",
    "purpose": "chat",
    "provider": "siliconflow",
    "baseUrl": "https://api.siliconflow.cn/v1",
    "model": "Qwen/Qwen2.5-72B-Instruct",
    "apiKey": "'"$SILICONFLOW_API_KEY"'",
    "enabled": true,
    "isDefault": true,
    "supportsStreaming": true,
    "timeoutMs": 60000
  }' | jq .
```

### 2.2 默认 Embedding Profile

```bash
curl -s -X POST "$AI_GATEWAY_BASE_URL/internal/v1/model-profiles" \
  -H "X-Service-Token: $SERVICE_TOKEN" \
  -H "X-Caller-Service: gateway" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "default-embedding",
    "purpose": "embedding",
    "provider": "siliconflow",
    "baseUrl": "https://api.siliconflow.cn/v1",
    "model": "BAAI/bge-m3",
    "apiKey": "'"$SILICONFLOW_API_KEY"'",
    "enabled": true,
    "isDefault": true,
    "dimensions": 1024,
    "timeoutMs": 30000
  }' | jq .
```

### 2.3 默认 Rerank Profile

```bash
curl -s -X POST "$AI_GATEWAY_BASE_URL/internal/v1/model-profiles" \
  -H "X-Service-Token: $SERVICE_TOKEN" \
  -H "X-Caller-Service: gateway" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "default-rerank",
    "purpose": "rerank",
    "provider": "siliconflow",
    "baseUrl": "https://api.siliconflow.cn/v1",
    "model": "BAAI/bge-reranker-v2-m3",
    "apiKey": "'"$SILICONFLOW_API_KEY"'",
    "enabled": true,
    "isDefault": true,
    "topN": 5,
    "timeoutMs": 30000
  }' | jq .
```

---

## 3. 验证 Readiness

```bash
curl -s "$AI_GATEWAY_BASE_URL/readyz" | jq .
```

期望响应（所有三类 profile 就绪）：

```json
{
  "data": {
    "status": "ok",
    "checks": [
      { "name": "config_store", "status": "ok" },
      { "name": "chat_profile", "status": "ok" },
      { "name": "embedding_profile", "status": "ok" },
      { "name": "rerank_profile", "status": "ok" }
    ]
  },
  "requestId": "..."
}
```

如果任一 profile 缺失，`status` 为 `"degraded"`，对应的 `check.status` 为 `"missing"`。

---

## 4. PostgreSQL DB smoke（显式启用）

普通 `go test ./...` 不依赖本地 PostgreSQL。需要验证 migration、profile CRUD、credential encryption / rotation、invocation 记录和 usage aggregate 时，提供独立测试库连接串：

```bash
cd services/ai-gateway
AI_GATEWAY_TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/ai_gateway_test?sslmode=disable" \
go test ./internal/repository -run '^TestPostgresRepositoryDBSmoke$' -count=1 -v
```

该测试会：

- 创建随机 schema，并只在该 schema 内应用 `services/ai-gateway/migrations/*.sql` 的 `-- +goose Up` 段。
- 通过真实 `PostgresRepository` 创建、更新、删除 model profile。
- 验证 provider credential 以 AES-GCM 密文保存，响应和断言不会打印明文 API key。
- 更新 `apiKey` 以验证旧 credential 变为 `rotated`、新 credential 为 `active`。
- 写入一条 provider invocation 和一条 invocation attempt，并验证 `model_usage_aggregates` 按 caller/profile/operation 聚合。
- 测试结束后删除随机 schema。

如果本地只需要验证 migration apply，可使用项目固定 goose 版本：

```bash
cd services/ai-gateway
go run github.com/pressly/goose/v3/cmd/goose@v3.27.1 -dir migrations postgres "$AI_GATEWAY_DATABASE_URL" up
```

不要把生产或共享环境连接串用于 DB smoke，除非该连接串指向可清理的临时数据库。

---

## 5. Credential rotation

AI Gateway 不提供单独的 `/rotate` action path。轮换 provider credential 时，对目标 profile 执行 `PATCH /internal/v1/model-profiles/{profileId}` 并写入新的 `apiKey`：

```bash
curl -s -X PATCH "$AI_GATEWAY_BASE_URL/internal/v1/model-profiles/$PROFILE_ID" \
  -H "X-Service-Token: $SERVICE_TOKEN" \
  -H "X-Caller-Service: gateway" \
  -H "X-Request-Id: rotate-$PROFILE_ID-$(date +%s)" \
  -H "Content-Type: application/json" \
  -d '{
    "apiKey": "'"$NEW_PROVIDER_API_KEY"'"
  }' | jq .
```

期望响应只包含 `apiKeyConfigured: true` 和新的 profile 元数据，不返回明文 key。安全验证只能查询状态和脱敏字段，例如：

```sql
SELECT profile_id, status, key_last4, encryption_key_version, created_at, rotated_at
FROM provider_credentials
WHERE profile_id = 'mp_chat_default'
ORDER BY created_at DESC;
```

不要查询或输出 `ciphertext`、`nonce`、provider key 明文、service token 明文、数据库连接串或完整 provider 错误体。旧 active credential 应在轮换后变为 `rotated`，新 credential 应为 `active`。

---

## 6. 无真实密钥的 CI / 本地环境（fake provider）

CI 环境通常没有 SiliconFlow API Key。所有 provider smoke 测试均使用 `httptest.Server` 作为受控 fake provider，**无需真实密钥即可运行**：

```bash
cd services/ai-gateway
go test ./...
```

如果只需要验证 provider adapter 回归样本，使用 [Provider Adapter 文档](provider-adapters.md#回归样本入口) 中维护的窄化测试命令。

### 6.1 受控 fake provider seed profile

HTTP smoke 测试在内存 repository 中注册以下 profile，不需要外部数据库或真实密钥：

| 用途 | provider | baseUrl | model | 关键默认值 |
| --- | --- | --- | --- | --- |
| chat | `openai_compatible` | `<httptest>/v1` | `provider-model` | `supportsStreaming=true` |
| embedding | `siliconflow` | `<httptest>/v1` | `BAAI/bge-m3` | `dimensions=1024` |
| rerank | `siliconflow` | `<httptest>/v1` | `BAAI/bge-reranker-v2-m3` | `topN=3` |

### 6.2 受控 fake provider 预期响应形态

Chat 成功样本返回 OpenAI-compatible chat completion：

```json
{
  "id": "chatcmpl_test",
  "object": "chat.completion",
  "model": "provider-model",
  "choices": [
    {
      "index": 0,
      "message": { "role": "assistant", "content": "ok" },
      "finish_reason": "stop"
    }
  ],
  "usage": { "prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2 }
}
```

Embedding 成功样本返回 OpenAI-compatible list；AI Gateway 校验 `data[]` 数量、`index` 顺序、`object=embedding` 和 embedding payload：

```json
{
  "object": "list",
  "data": [
    { "object": "embedding", "index": 0, "embedding": [0.11, 0.12] },
    { "object": "embedding", "index": 1, "embedding": [0.21, 0.22] }
  ],
  "model": "BAAI/bge-m3",
  "usage": { "prompt_tokens": 7, "total_tokens": 7 }
}
```

Rerank 成功样本覆盖 provider `results[]` / `relevance_score` 形态；AI Gateway 会归一化为 `data[]`、补齐输入文档 ID、映射 `meta.tokens` 到 usage，并按 `top_n` 截断：

```json
{
  "results": [
    { "index": 1, "relevance_score": 0.91 },
    { "index": 0, "relevance_score": 0.42 }
  ],
  "model": "BAAI/bge-reranker-v2-m3",
  "meta": { "tokens": { "input_tokens": 9, "output_tokens": 1 } }
}
```

### 6.3 真实 provider smoke（显式启用）

真实 provider smoke 默认跳过，只有显式设置开关和凭证时才会外联。设置 `AI_GATEWAY_REAL_CHAT_MODEL` 时会同时执行非流式 chat 和 streaming chat；streaming 子用例发送 `stream=true` 和 `Accept: text/event-stream`，并验证 SSE `data:` 与 `[DONE]`：

```bash
cd services/ai-gateway
AI_GATEWAY_REAL_PROVIDER_SMOKE=1 \
AI_GATEWAY_REAL_PROVIDER_BASE_URL="https://api.siliconflow.cn/v1" \
AI_GATEWAY_REAL_PROVIDER_API_KEY="$SILICONFLOW_API_KEY" \
AI_GATEWAY_REAL_CHAT_MODEL="Qwen/Qwen2.5-72B-Instruct" \
AI_GATEWAY_REAL_EMBEDDING_MODEL="BAAI/bge-m3" \
AI_GATEWAY_REAL_EMBEDDING_DIMENSIONS="1024" \
AI_GATEWAY_REAL_RERANK_MODEL="BAAI/bge-reranker-v2-m3" \
go test ./internal/http -run TestRealProviderSmoke_ExplicitEnvOnly -count=1 -v
```

只需要跑单项时，可以只设置对应的 `AI_GATEWAY_REAL_*_MODEL`。如果没有设置任何 operation model，测试会失败并提示至少设置 chat、embedding 或 rerank 的一个模型变量。

---

## 7. 安全边界

| 禁止行为 | 原因 |
|----------|------|
| 将 `apiKey` 写入日志或响应 | API Key 仅加密存储，永远不出现在 API 响应中 |
| 在 `defaultParameters` 中包含 `api_key`、`token`、`secret` 等敏感键 | 服务层会拒绝含敏感键的 JSON 对象 |
| 将 `baseUrl` 设置为包含凭证或敏感 query 参数的 URL | 服务层校验会拒绝此类 URL |
| 将 provider 原始错误体转发给调用方 | 所有 provider 错误均经过归一化，不透传原始响应 |

---

## 8. 常见失败与诊断

| 场景 | 表现 | 排查方向 |
| --- | --- | --- |
| 服务令牌缺失或错误 | HTTP 管理接口返回 `unauthorized`；模型调用接口返回 OpenAI-style `authentication_error` / `unauthorized`。 | 检查 `X-Service-Token` 明文是否和 `AI_GATEWAY_SERVICE_TOKEN_HASHES` 的 SHA-256 hash 匹配。 |
| `X-Caller-Service` 缺失或不允许 | 返回 `unauthorized` 或 `permission_error` / `forbidden`。 | 使用 `gateway` 创建 profile；模型调用使用 `qa`、`knowledge` 或 `document` 等允许值。 |
| 默认 profile 缺失 | 模型调用返回 `not_found` / `default model profile not found`；`/readyz` 中对应 check 为 `missing`。 | 按第 2 节创建默认 profile，或在请求中传入正确 `profile_id`。 |
| credential 未配置或被禁用 | 返回 `dependency_error` / `model profile credential is not configured`。 | 重新写入 profile `apiKey`，确认 credential 状态为 active。 |
| provider 认证失败 | fake/真实 provider smoke 返回 `authentication_error` 或 `dependency_error`，不会透传 provider 原始 body。 | 检查 provider API key、baseUrl 和账号权限；测试响应中不应出现 API key 或 provider 原始错误。 |
| provider 响应格式不匹配 | 返回 `dependency_error` / `provider returned an invalid response`，调用摘要 status 为 failed。 | Chat 需 `object=chat.completion` 且 `choices` 非空；embedding 需 `object=list`、数量和 index 与输入一致；rerank 需合法 `data[]` 或 `results[]`、score 和 index。 |
| 真实 provider smoke 被跳过 | `go test -v` 显示 skip real provider smoke。 | 设置 `AI_GATEWAY_REAL_PROVIDER_SMOKE=1`、base URL、API key 和至少一个 operation model 环境变量。 |
| DB smoke 被跳过 | `go test -v` 显示 skip PostgreSQL DB smoke。 | 设置 `AI_GATEWAY_TEST_DATABASE_URL`，并确认该用户可创建和删除临时 schema。 |
| Credential rotation 后调用失败 | 模型调用返回 `dependency_error` 或 credential 未配置。 | 确认 PATCH 成功，旧 credential 已 `rotated`，新 credential 为 `active`；不要从响应或日志查找明文 key。 |
| 跨服务调用被拒绝 | 模型调用接口返回 OpenAI-style `authentication_error` 或 `permission_error`。 | 检查 `X-Service-Token` 是否匹配 hash，`X-Caller-Service` 是否为 `qa`、`knowledge` 或 `document`，以及请求是否携带可追踪 `X-Request-Id`。 |

---

## 9. 下游服务接入示例

QA / Knowledge / Document 服务接入 AI Gateway 时，在请求头中携带：

```
X-Service-Token: <服务令牌明文>
X-Caller-Service: qa            # 或 knowledge / document
X-Request-Id: <上游请求 ID>     # 可选，用于全链路追踪
```

所有示例中的 `model` 必须与选中的 profile `model` 完全一致。指定 `profile_id` 时，AI Gateway 会使用该 profile；不指定时使用对应 purpose 的默认 enabled profile。响应、日志和测试输出不得包含 provider API key、service token、完整 prompt、文档全文、object key、embedding payload 或 provider 原始错误体。

### 9.1 QA chat

QA 使用 AI Gateway chat completion。错误会归一为 OpenAI-style error，QA 再映射为自己的 dependency/validation 语义：

```bash
curl -s -X POST "$AI_GATEWAY_BASE_URL/internal/v1/chat/completions" \
  -H "X-Service-Token: $SERVICE_TOKEN" \
  -H "X-Caller-Service: qa" \
  -H "X-Request-Id: req-qa-smoke-001" \
  -H "Content-Type: application/json" \
  -d '{
    "profile_id": "mp_chat_default",
    "model": "Qwen/Qwen2.5-72B-Instruct",
    "messages": [
      { "role": "user", "content": "用一句话回答：AI Gateway 是否可用？" }
    ],
    "metadata": { "workflow": "qa-smoke" }
  }' | jq .
```

### 9.2 QA streaming chat

```bash
curl -N -s -X POST "$AI_GATEWAY_BASE_URL/internal/v1/chat/completions" \
  -H "X-Service-Token: $SERVICE_TOKEN" \
  -H "X-Caller-Service: qa" \
  -H "X-Request-Id: req-qa-stream-smoke-001" \
  -H "Accept: text/event-stream" \
  -H "Content-Type: application/json" \
  -d '{
    "profile_id": "mp_chat_default",
    "model": "Qwen/Qwen2.5-72B-Instruct",
    "stream": true,
    "messages": [
      { "role": "user", "content": "Return the single word ok." }
    ]
  }'
```

### 9.3 Knowledge embedding

```bash
curl -s -X POST "$AI_GATEWAY_BASE_URL/internal/v1/embeddings" \
  -H "X-Service-Token: $SERVICE_TOKEN" \
  -H "X-Caller-Service: knowledge" \
  -H "X-Request-Id: req-knowledge-embedding-smoke-001" \
  -H "Content-Type: application/json" \
  -d '{
    "profile_id": "mp_embedding_default",
    "model": "BAAI/bge-m3",
    "input": ["短片段 A", "短片段 B"],
    "encoding_format": "float"
  }' | jq '.object, .model, (.data | length), .usage'
```

Knowledge 只把 embedding 结果写入自己的向量存储；AI Gateway 只记录低敏调用摘要，例如 caller、profile、operation、input count、dimension 和 usage。

### 9.4 Knowledge rerank

```bash
curl -s -X POST "$AI_GATEWAY_BASE_URL/internal/v1/rerankings" \
  -H "X-Service-Token: $SERVICE_TOKEN" \
  -H "X-Caller-Service: knowledge" \
  -H "X-Request-Id: req-knowledge-rerank-smoke-001" \
  -H "Content-Type: application/json" \
  -d '{
    "profile_id": "mp_rerank_default",
    "model": "BAAI/bge-reranker-v2-m3",
    "query": "保护定值如何检查？",
    "documents": [
      { "id": "chunk-1", "text": "继电保护定值应核对版本、审批记录和现场执行一致性。" },
      { "id": "chunk-2", "text": "食堂菜单和库房盘点记录。" }
    ],
    "top_n": 1,
    "metadata": { "knowledgeBaseId": "kb-smoke" }
  }' | jq .
```

AI Gateway 响应只返回 `document_id`、`index` 和 `score`，不返回文档正文；引用、权限和 hydrate 仍由 Knowledge/QA 负责。

### 9.5 Document chat

Document 使用 chat profile 生成大纲或章节内容，但报告 job、章节状态、文件和操作日志仍归 Document 拥有：

```bash
curl -s -X POST "$AI_GATEWAY_BASE_URL/internal/v1/chat/completions" \
  -H "X-Service-Token: $SERVICE_TOKEN" \
  -H "X-Caller-Service: document" \
  -H "X-Request-Id: req-document-chat-smoke-001" \
  -H "Content-Type: application/json" \
  -d '{
    "profile_id": "mp_chat_default",
    "model": "Qwen/Qwen2.5-72B-Instruct",
    "messages": [
      { "role": "user", "content": "为迎峰度夏检查报告生成一个三项大纲。" }
    ],
    "metadata": { "workflow": "document-outline-smoke" }
  }' | jq .
```

### 9.6 Document profile validation

Document 管理端校验 report settings 里的 profile 时，只读取 AI Gateway profile 元数据，不接触 provider key：

```bash
curl -s "$AI_GATEWAY_BASE_URL/internal/v1/model-profiles/mp_chat_default" \
  -H "X-Service-Token: $SERVICE_TOKEN" \
  -H "X-Caller-Service: document" \
  -H "X-Request-Id: req-document-profile-smoke-001" | jq .
```

验证点：

- `data.purpose` 必须为 `chat`。
- `data.enabled` 必须为 `true`。
- `data.apiKeyConfigured` 只能表示配置状态，不返回明文 key。
- 缺失或 disabled profile 应由 Document 映射为设置校验失败，不应继续创建报告生成任务。
