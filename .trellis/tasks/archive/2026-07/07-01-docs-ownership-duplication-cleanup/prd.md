# 清理 docs 文档定位重复

## Goal

清理 `docs/` 中已经审计出的文档定位重复和内容重复风险，让读者能明确区分稳定契约、服务级设计草案、业务语义说明、实现状态和前端接入建议，减少同一 API 清单或 schema 在多处并行维护导致的漂移。

## What I Already Know

- 当前稳定前端公开契约以 `docs/services/gateway/api/public.openapi.yaml` 为准。
- `docs/services/<service>/api/public.openapi.yaml` 的定位在总文档中被描述为 Gateway 暴露子契约，但 Knowledge/QA/Document 实际保留了服务级 public draft / design context，其中部分 path 还未进入 Gateway active paths。
- Gateway active path 明细同时出现在 Gateway README、active-api-owner-map、frontend-backend-contract 和多个服务 README。
- QA README 维护了较完整的 endpoint 详情、请求/响应示例和 SSE 事件协议，容易被当作第二份公开 API 契约。
- Document 前端 API 设计文档重新定义了 envelope、TS 类型和 API 函数签名，但前端基线要求从 Gateway OpenAPI 生成类型。

## Requirements

- 明确文档层级：
  - Gateway public OpenAPI 是稳定 browser-facing / frontend-facing 契约源。
  - 服务级 `api/public.openapi.yaml` 是服务 owner 维护的 public/Gateway-facing 设计或子契约文件；如果包含未进入 Gateway active paths 的内容，必须明确标记为 candidate/draft，不得被描述为前端稳定契约。
  - Markdown 只解释业务语义、边界、工作流或接入建议，不替代 OpenAPI schema。
- 压缩重复的 active API 明细：
  - Gateway README 保留 active owner map 链接和路径分组说明，不维护逐项明细表。
  - 架构前后端契约保留通用规则和高层资源入口，不复制完整 active path 清单。
  - 服务 README 保留 owner 资源范围和业务语义，避免和 OpenAPI/owner map 并行维护完整 endpoint 清单。
- 降级 QA README 的完整 endpoint 契约：
  - 保留 Agent Host、边界、数据来源、SSE 安全语义和 OpenAPI 维护规则。
  - 移除或压缩逐 endpoint 请求/响应示例，改为指向 Gateway OpenAPI、服务级 public draft 和数据模型文档。
- 清理 Document 前端 API 设计：
  - 保留页面到 Gateway operation/resource 的映射和 typed fetch 使用建议。
  - 删除或降级手写 envelope、TS 类型和函数签名，避免替代 generated OpenAPI 类型。
- 不改变实际 API 行为、OpenAPI schema 内容或服务代码。

## Acceptance Criteria

- [x] `docs/README.md` 和 `docs/architecture/technology-decisions.md` 对服务级 `public.openapi.yaml` 的说明与 Knowledge/QA/Document 现状一致。
- [x] Gateway active API 明细只有 Gateway OpenAPI 和 owner map 承担权威清单职责；其他 Markdown 不再复制完整 active path 表。
- [x] QA README 不再维护完整 endpoint 请求/响应示例作为第二份契约。
- [x] Document frontend API design 不再维护手写 envelope、TS 类型和 API 函数签名作为实现依据。
- [x] 相关文档中的相对链接仍然有效，Markdown/YAML 基本格式检查通过。

## Out of Scope

- 不重命名 `public.openapi.yaml` / `internal.openapi.yaml`。
- 不删除 Knowledge/QA/Document 服务级 public draft 中的 candidate paths。
- 不修改服务代码、前端代码或生成的 API 类型。
- 不重新设计 Gateway OpenAPI schema。

## Technical Notes

- 重点文件：
  - `docs/README.md`
  - `docs/architecture/technology-decisions.md`
  - `docs/services/gateway/README.md`
  - `docs/architecture/frontend-backend-contract.md`
  - `docs/services/knowledge/README.md`
  - `docs/services/qa/README.md`
  - `docs/services/document/README.md`
  - `docs/services/document/docs/frontend-api-design.md`
  - `docs/services/document/docs/requirements.md`
- 验证命令：
  - `git diff --check`
  - Markdown link/path spot checks with `rg`
  - YAML parse of `docs/services/*/api/*.openapi.yaml` if OpenAPI YAML is touched
