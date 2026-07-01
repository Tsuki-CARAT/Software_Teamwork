---
name: Task Issue
about: Create a tracked task for issue-driven development
title: '[A/B/C/F/S-001] 中文任务标题'
labels: ''
assignees: ''
---

## 认领规则

- 本任务为自领任务，当前不预分配 Assignee。
- 只允许 1 名主责人完成；认领前请在本 issue 评论 `认领：@你的 GitHub 用户名`，自动化会在校验通过后把评论者设为 Assignee。
- 可以请其他成员 review 或协助排障，但主责人只能有 1 个；如需转让，请在 issue 评论中交接清楚。
- 一切冲突以 `docs/` 为准；如果代码或旧本地草稿与 `docs/` 冲突，按 `docs/` 修改代码或同步公开文档。

## 任务信息

- 编号：`A/B/C/F/S-001`
- 状态：`Draft / Ready / In Progress / Blocked / Review / Done`
- 主责小组：`L1nggTeam / JerryTeam / PrimeTeam / Frontend / Special`
- View：`Platform / QA / Report / Frontend / Special`
- 优先级：`P0 / P1 / P2`
- 批次：`Batch 0 / Batch 1 / Batch 2 / Batch 3 / Batch 4`
- 模块：`gateway / auth / file / knowledge / qa / document / frontend / ai-gateway / parser / openapi / deploy / ci`
- 预期工时：`待估 / 0.5h / 1d`
- 实际工时：`未填写 / 0.5h / 1d`
- Risk：`Normal / Needs Decision / Blocked`
- 依赖任务：无 / #118 #125
- 阻塞任务：无 / #126 #127
- 并行任务：无 / #128
- 依赖原因：写清楚依赖的接口、schema、数据结构、环境变量、服务能力或验收条件。
- 建议分支：`group/type/short-title`
- GitHub Project：`Software Teamwork`
- Project sync：`pending / synced / blocked`

## 权威依据

- `docs/...`
- `docs/services/...`
- GitHub issue 或 PR 链接

## 任务范围

- ...
- ...
- ...

## 交付物

- ...
- ...
- ...

## 验收标准

- [ ] ...
- [ ] ...
- [ ] ...

## 边界与不做内容

- ...

## PR 要求

- PR 目标分支必须是主仓库 `develop`。
- Commit message 使用 Conventional Commits。
- PR 描述列出完成范围、验证命令、未完成风险和关联 issue。
