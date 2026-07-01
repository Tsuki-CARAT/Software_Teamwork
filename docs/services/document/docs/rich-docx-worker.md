# Document 富 DOCX Worker 工具链基线

本文记录 Document 服务富 DOCX 生成 worker 工具链的选型决策、版本固定、调用边界、`SimpleDOCXGenerator` 职责分工和本地 smoke 验证步骤。

本文是 C-011（#307）的主要交付物。技术基线权威来源见 [`technology-decisions.md`](../../architecture/technology-decisions.md)；服务实现状态见 [`implementation.md`](implementation.md)。

## 1. 工具链选型决策

### 1.1 选型结论

| 候选工具 | 决策 | 理由 |
| --- | --- | --- |
| **Pandoc 3.10** | **选定 — 富 DOCX 主工具链** | 纯 CLI、无 GUI 依赖、进程外调用简单；官方 Docker 镜像维护积极；Markdown → DOCX 转换覆盖所有目标格式（标题、段落、表格、列表）；镜像体积适中（约 100 MB）。 |
| LibreOffice headless | **暂不引入，保留后续候选** | 镜像体积大（500 MB+），本地构建慢；Pandoc 已满足首期富 DOCX 需求；引入时必须固定镜像 tag + digest，不能依赖运行环境自带 `soffice`。 |

### 1.2 背景

当前 `SimpleDOCXGenerator`（`services/document/internal/service/docx_generator.go`）使用 Go 标准库 `archive/zip` + 手写 OOXML 实现基础 DOCX 导出，已覆盖当前生产路径。富 DOCX 阶段需要支持更完整的 Markdown 语法、页眉页脚、自动目录和样式自定义；Pandoc 能在进程外以 DOCX reference 模板驱动这些能力，同时保持 Document worker 无额外 Go 依赖。

### 1.3 决策边界

- C-011 固定工具链选型和调用边界文档。
- Dockerfile 接入、运行时调用和 smoke 联调是后续任务。
- C-011 不改动任何 Go 代码，不更新 Dockerfile，不替换当前生产路径。

## 2. 版本固定

| 项目 | 固定值 | 说明 |
| --- | --- | --- |
| 镜像 tag | `pandoc/core:3.10` | Docker Hub 官方镜像，tag 已固定 |
| 镜像 digest | 落地 Dockerfile 时固定 `sha256:...` | 参考 [2.1 节](#21-digest-获取方式) |
| 配置变量 | `DOCUMENT_PANDOC_PATH`（默认 `pandoc`） | 已存在于 `services/document/internal/config/config.go`，当前未实际调用 |
| Compose 变量 | `DOCUMENT_PANDOC_PATH: pandoc` | 已存在于 `deploy/docker-compose.yml`，当前未挂载 CLI |

C-011 阶段仅固定镜像 tag；Dockerfile 接入时必须同步写入 digest，禁止以 `latest` 或无摘要形式部署到生产环境。

### 2.1 digest 获取方式

在网络可访问环境执行：

```bash
docker pull pandoc/core:3.10
docker inspect pandoc/core:3.10 --format '{{.RepoDigests}}'
# 输出示例：[pandoc/core@sha256:abcdef1234567890...]
```

中国大陆环境请参考 `deploy/.env.china.example` 中的 registry rewrite 策略，或配置 daemon mirror 后重新 pull。

## 3. Document Service ↔ 富 DOCX Worker 调用边界

### 3.1 输入

| 项目 | 规格 |
| --- | --- |
| 格式 | GFM Markdown（UTF-8） |
| 内容 | 章节标题（H1/H2 层级）+ 正文段落 + GFM 表格 + 无序/有序列表 |
| 来源 | 由 Document worker 从已保存 `ReportSection.content` 构造 |
| 传递方式 | 写入临时文件（`os.MkdirTemp` 创建隔离目录，权限 0700）→ 将路径传给 Pandoc subprocess |

### 3.2 输出

| 项目 | 规格 |
| --- | --- |
| 格式 | Office Open XML DOCX（`.docx`） |
| 接收 | 从输出临时文件读取 bytes |
| 后续 | 上传 File Service → 更新 `ReportFile` 元数据 → 删除临时文件 |

### 3.3 错误处理

| 错误场景 | 处理方式 |
| --- | --- |
| subprocess 非零退出码 | 返回 `CodeDependency` 错误；日志只记录 `reportID`、`jobID`；不记录章节正文内容 |
| 输出文件为空或小于 1 KB | 返回 `CodeDependency` 错误，标注"空 DOCX 输出" |
| context 超时 | 返回 `CodeDependency` timeout 错误；强制终止 subprocess |
| Pandoc binary 不可用（PATH 查找失败） | 回退到 `SimpleDOCXGenerator`；写 `warn` 日志（含 `reportID`，不含内容）；见 [4.1 节](#41-fallback-策略) |

错误响应和日志不得包含：章节正文内容、prompt、`file_ref`、object key、API key、provider 原始错误或完整临时文件路径。

### 3.4 超时约束

| 参数 | 推荐值 | 说明 |
| --- | --- | --- |
| 单文档 Pandoc 调用超时 | 30 秒 | 通过 `context.WithTimeout` 传入，不可省略 |
| 超时后 | 强制终止 subprocess | 使用 context cancel 触发 `cmd.Process.Kill()` |

### 3.5 临时文件管理

```
临时目录：os.MkdirTemp（权限 0700），不直接写入系统 /tmp 根目录
输入文件（.md）：defer os.Remove，无论成功/失败均清理
输出文件（.docx）：defer os.Remove，bytes 读取完毕后清理
清理失败：写 warn 日志（不含文件内容），不影响主流程返回值
```

### 3.6 敏感数据

| 数据类型 | 约束 |
| --- | --- |
| 章节正文内容 | 不得写入日志，不得写入 HTTP 错误响应 |
| 临时文件路径 | 不得出现在 HTTP 响应或错误 `message` 中 |
| `reportID`、`jobID` | 可写入日志（低基数字段） |
| 输出文件大小 | 可写入日志 |

## 4. SimpleDOCXGenerator vs 富 DOCX Worker 职责边界

| 维度 | SimpleDOCXGenerator（当前生产） | 富 DOCX Worker — Pandoc 3.10（待接入） |
| --- | --- | --- |
| 实现位置 | `services/document/internal/service/docx_generator.go` | Pandoc CLI（`pandoc/core:3.10` 镜像） |
| 运行时依赖 | Zero — Go 标准库 `archive/zip` | Pandoc CLI 或 Docker 镜像 |
| 生产状态 | **当前生产路径** | 工具链已固定（C-011），Dockerfile 接入是后续任务 |
| DOCX 能力 | 基础标题/段落/表格，纯文本扁平化 | 完整 GFM Markdown、reference.docx 样式、页眉页脚、自动目录 |
| fallback 角色 | Pandoc 不可用或超时时的 fallback | 首选富 DOCX 路径 |

### 4.1 Fallback 策略

以下策略在 Dockerfile 接入时实现，C-011 仅记录约束。

```
触发条件（任一）：
  1. exec.LookPath(pandocPath) 返回错误（binary 不可用）
  2. Pandoc subprocess context 超时
  3. Pandoc 输出文件为空（< 1 KB）

fallback 行为：
  - 使用 SimpleDOCXGenerator 生成基础 DOCX
  - 写 warn 级别日志（含 reportID、jobID；不含正文内容）
  - 不允许静默回退 — 必须写日志
  - ReportFile 元数据可标注 generatorHint: "simple" 用于排查
```

## 5. 本地 Smoke 验证

以下命令验证 `pandoc/core:3.10` 在本地可生成合法 DOCX，无需启动其他服务。

### 5.1 验证 Pandoc 版本

```bash
docker pull pandoc/core:3.10
docker run --rm pandoc/core:3.10 pandoc --version
# 期望首行：pandoc 3.10
```

### 5.2 准备测试 Markdown

将以下内容保存为 `smoke-test.md`：

```markdown
# 第一章 检查概况

## 1.1 概述

本次迎峰度夏检查覆盖全厂主要运行设备，检查周期为 2026 年 6 月。

## 1.2 检查结论

整体运行状态良好，发现问题 3 项。

- 主变温度偏高，建议增加冷却频次
- 循环水泵振动超标，建议更换轴承
- DCS 控制参数漂移，建议校准

## 1.3 数据汇总

| 设备名称 | 当前状态 | 处置建议 |
|----------|----------|----------|
| 主变压器 | 温度偏高 | 增加冷却 |
| 循环水泵 | 振动超标 | 更换轴承 |
| DCS 系统 | 参数漂移 | 校准参数 |
```

### 5.3 生成 DOCX

```bash
# Linux / macOS
docker run --rm \
  -v "$(pwd):/data" \
  pandoc/core:3.10 \
  -f gfm -t docx \
  /data/smoke-test.md -o /data/smoke-test.docx

# Windows PowerShell
docker run --rm -v "${PWD}:/data" pandoc/core:3.10 `
  -f gfm -t docx /data/smoke-test.md -o /data/smoke-test.docx
```

### 5.4 验证输出

```bash
# Linux / macOS
file smoke-test.docx                                          # 期望：Microsoft Word 2007+
unzip -l smoke-test.docx | grep "word/document.xml"          # 期望：列表中包含该文件

# Windows PowerShell
(Get-Item smoke-test.docx).Length                            # 期望：> 4096 bytes
```

最终验证：用 Word 或 LibreOffice Writer 打开 `smoke-test.docx`，确认 H1/H2 标题层级、正文段落、无序列表和表格均正确渲染。

## 6. 后续接入约束备忘

C-011 不执行以下操作；后续 Dockerfile 接入任务时必须完成：

| 约束 | 说明 |
| --- | --- |
| Dockerfile 写入镜像 + digest | `pandoc/core:3.10@sha256:...`；禁止以 `latest` 或无摘要部署 |
| `go.mod` 无需变更 | 富 DOCX 通过 subprocess 调用，不引入 Go binding |
| 环境变量已预留 | `DOCUMENT_PANDOC_PATH` 已在 `config.go` 和 `docker-compose.yml` 存在，接入时无需新增 |
| 接入后同步本文 | 补充实际 digest 值、接入 PR 链接 |
| 接入后同步 technology-decisions.md | 写入 digest 并更新部署说明 |
