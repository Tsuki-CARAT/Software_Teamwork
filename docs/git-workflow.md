# GitHub CLI 工作流

本文档给出本项目推荐的 `gh` CLI 操作流程。所有日常修改默认都必须通过个人
fork 的独立分支向主仓库 `develop` 发起 Pull Request。

以下示例中：

- `Sakayori-Iroha-168/Software_Teamwork` 是主仓库。
- `YOUR_NAME/Software_Teamwork` 是你的个人 fork。
- `TARGET_BRANCH` 默认设为 `develop`。

## 1. 登录 GitHub CLI

```bash
gh auth login
gh auth status
```

## 2. Fork 主仓库

```bash
gh repo fork Sakayori-Iroha-168/Software_Teamwork --remote --clone=false
```

如果已经 fork 过，可以跳过这一步。

## 3. 配置 Remote

确认 remote：

```bash
git remote -v
```

推荐配置：

```bash
git remote set-url origin git@github.com:YOUR_NAME/Software_Teamwork.git
git remote add upstream git@github.com:Sakayori-Iroha-168/Software_Teamwork.git
```

如果 `upstream` 已存在：

```bash
git remote set-url upstream git@github.com:Sakayori-Iroha-168/Software_Teamwork.git
```

最终应满足：

```text
origin    -> 你的个人 fork
upstream  -> 主仓库
```

## 4. 从最新目标分支创建分支

```bash
TARGET_BRANCH=develop  # 特殊任务才改为 keep-shitting
git fetch upstream
git switch -c L1nggTeam/feat/login-page "upstream/$TARGET_BRANCH"
```

不要从 `main`、本地旧分支或主仓库临时分支创建开发分支。日常开发使用
`upstream/develop`；只有任务明确要求时才使用 `upstream/keep-shitting`。

## 5. 提交修改

```bash
git status
git add .
git commit -m "feat(frontend): add login page"
```

Commit message 必须遵循 [Conventional Commits](../.trellis/spec/guides/commit-convention.md)。
AI agent 在执行 `git commit` 前还必须阅读
[.agents/git-commit-checklist.md](../.agents/git-commit-checklist.md)。

## 6. 推送到个人 fork

```bash
git push -u origin L1nggTeam/feat/login-page
```

## 7. 创建 PR 到主仓库目标分支

```bash
TARGET_BRANCH=develop  # 特殊任务才改为 keep-shitting
gh pr create \
  --repo Sakayori-Iroha-168/Software_Teamwork \
  --base "$TARGET_BRANCH" \
  --head YOUR_NAME:L1nggTeam/feat/login-page \
  --title "feat(frontend): add login page" \
  --body-file .github/pull_request_template.md
```

注意：

- `--base` 默认必须是 `develop`；只有任务明确要求时才使用 `keep-shitting`。
- `--base` 不能是 `main`。
- `--head` 必须是 `YOUR_NAME:<branch>`，也就是个人 fork 中的分支。
- 可选使用 `gh pr edit <PR_NUMBER> --add-label <LABEL>` 添加小组 label。

## 8. PR 前同步最新目标分支

如果主仓库目标分支更新了，需要 rebase：

```bash
git fetch upstream
git rebase "upstream/$TARGET_BRANCH"
git push --force-with-lease
```

禁止使用普通 `--force`。只使用 `--force-with-lease`。

## 9. 查看 PR 状态

```bash
gh pr status
gh pr checks <PR_NUMBER> --repo Sakayori-Iroha-168/Software_Teamwork
gh pr view --web
```

## 10. 常见错误

### PR 目标分支选成 main

关闭该 PR，重新向允许的目标分支发起 PR：

```bash
gh pr close <PR_NUMBER> --repo Sakayori-Iroha-168/Software_Teamwork
gh pr create --repo Sakayori-Iroha-168/Software_Teamwork --base "$TARGET_BRANCH"
```

### 添加小组 label

```bash
gh pr edit <PR_NUMBER> \
  --repo Sakayori-Iroha-168/Software_Teamwork \
  --add-label L1nggTeam
```

### 分支落后目标分支

```bash
git fetch upstream
git rebase "upstream/$TARGET_BRANCH"
git push --force-with-lease
```

### Commit message 不规范

修改最近一次 commit：

```bash
git commit --amend -m "fix(backend): handle empty user response"
git push --force-with-lease
```

修改多个 commit：

```bash
git fetch upstream
git rebase -i upstream/develop
git push --force-with-lease
```
